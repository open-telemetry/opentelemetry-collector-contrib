// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awssecretsmanagerauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/awssecretsmanagerauthextension"

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/tg123/go-htpasswd"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.uber.org/zap"
	creds "google.golang.org/grpc/credentials"
)

var (
	errNoAuth              = errors.New("no basic auth provided")
	errInvalidCredentials  = errors.New("invalid credentials")
	errInvalidSchemePrefix = errors.New("invalid authorization scheme prefix")
	errInvalidFormat       = errors.New("invalid authorization format")
)

// ---- server auth ----

var (
	_ extension.Extension  = (*serverAuth)(nil)
	_ extensionauth.Server = (*serverAuth)(nil)
)

type serverAuth struct {
	cfg       *Config
	logger    *zap.Logger
	matchFunc atomic.Pointer[func(username, password string) bool]
	poller    *poller
}

func newServerAuth(cfg *Config, logger *zap.Logger) *serverAuth {
	return &serverAuth{cfg: cfg, logger: logger}
}

func (s *serverAuth) Start(ctx context.Context, _ component.Host) error {
	p, err := newPoller(s.cfg, s.logger)
	if err != nil {
		return fmt.Errorf("init AWS Secrets Manager poller: %w", err)
	}
	return s.startWithPoller(ctx, p)
}

func (s *serverAuth) startWithClient(ctx context.Context, c secretsManagerClient) error {
	return s.startWithPoller(ctx, &poller{
		client:          c,
		secretARN:       s.cfg.SecretARN,
		refreshInterval: s.cfg.RefreshInterval,
		logger:          s.logger,
	})
}

func (s *serverAuth) startWithPoller(ctx context.Context, p *poller) error {
	s.poller = p

	valueKey := s.cfg.Htpasswd.ValueKey
	return s.poller.Start(ctx, func(secretValue string) {
		content := secretValue
		if valueKey != "" {
			var m map[string]string
			if jsonErr := json.Unmarshal([]byte(secretValue), &m); jsonErr != nil {
				s.logger.Error("failed to parse AWS secret as JSON for htpasswd", zap.Error(jsonErr))
				return
			}
			v, ok := m[valueKey]
			if !ok {
				s.logger.Error("value_key not found in secret", zap.String("key", valueKey))
				return
			}
			content = v
		}
		if reloadErr := s.reload(content); reloadErr != nil {
			s.logger.Error("failed to reload htpasswd from AWS secret", zap.Error(reloadErr))
		}
	})
}

func (s *serverAuth) Shutdown(_ context.Context) error {
	if s.poller != nil {
		s.poller.Shutdown()
	}
	return nil
}

func (s *serverAuth) reload(content string) error {
	htp, err := htpasswd.NewFromReader(strings.NewReader(content), htpasswd.DefaultSystems, nil)
	if err != nil {
		return fmt.Errorf("parse htpasswd content: %w", err)
	}
	fn := htp.Match
	s.matchFunc.Store(&fn)
	return nil
}

func (s *serverAuth) Authenticate(ctx context.Context, headers map[string][]string) (context.Context, error) {
	auth := getAuthHeader(headers)
	if auth == "" {
		return ctx, errNoAuth
	}
	ad, err := parseBasicAuth(auth)
	if err != nil {
		return ctx, err
	}
	fn := s.matchFunc.Load()
	if fn == nil || !(*fn)(ad.username, ad.password) {
		return ctx, errInvalidCredentials
	}
	cl := client.FromContext(ctx)
	cl.Auth = ad
	return client.NewContext(ctx, cl), nil
}

// ---- client auth ----

var (
	_ extension.Extension      = (*clientAuth)(nil)
	_ extensionauth.HTTPClient = (*clientAuth)(nil)
	_ extensionauth.GRPCClient = (*clientAuth)(nil)
)

type clientAuth struct {
	cfg          *Config
	logger       *zap.Logger
	poller       *poller
	awsUsername  atomic.Pointer[string]
	awsPassword  atomic.Pointer[string]
	grpcMetadata atomic.Pointer[map[string]string]
}

func newClientAuth(cfg *Config, logger *zap.Logger) *clientAuth {
	return &clientAuth{cfg: cfg, logger: logger}
}

func (c *clientAuth) Start(ctx context.Context, _ component.Host) error {
	p, err := newPoller(c.cfg, c.logger)
	if err != nil {
		return fmt.Errorf("init AWS Secrets Manager poller: %w", err)
	}
	return c.startWithPoller(ctx, p)
}

func (c *clientAuth) startWithClient(ctx context.Context, cl secretsManagerClient) error {
	return c.startWithPoller(ctx, &poller{
		client:          cl,
		secretARN:       c.cfg.SecretARN,
		refreshInterval: c.cfg.RefreshInterval,
		logger:          c.logger,
	})
}

func (c *clientAuth) startWithPoller(ctx context.Context, p *poller) error {
	c.poller = p

	usernameKey := c.cfg.ClientAuth.UsernameKey
	if usernameKey == "" {
		usernameKey = "username"
	}
	passwordKey := c.cfg.ClientAuth.PasswordKey
	if passwordKey == "" {
		passwordKey = "password"
	}

	return c.poller.Start(ctx, func(secretValue string) {
		var m map[string]string
		if jsonErr := json.Unmarshal([]byte(secretValue), &m); jsonErr != nil {
			c.logger.Error("failed to parse AWS secret as JSON for client auth", zap.Error(jsonErr))
			return
		}
		u := m[usernameKey]
		p := m[passwordKey]
		c.awsUsername.Store(&u)
		c.awsPassword.Store(&p)
		c.updateGRPCMetadata()
	})
}

func (c *clientAuth) Shutdown(_ context.Context) error {
	if c.poller != nil {
		c.poller.Shutdown()
	}
	return nil
}

func (c *clientAuth) updateGRPCMetadata() {
	encoded := base64.StdEncoding.EncodeToString([]byte(c.username() + ":" + c.password()))
	m := map[string]string{"authorization": fmt.Sprintf("Basic %s", encoded)}
	c.grpcMetadata.Store(&m)
}

func (c *clientAuth) username() string {
	if u := c.awsUsername.Load(); u != nil {
		return *u
	}
	return ""
}

func (c *clientAuth) password() string {
	if p := c.awsPassword.Load(); p != nil {
		return *p
	}
	return ""
}

func (c *clientAuth) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	if strings.Contains(c.username(), ":") {
		return nil, errInvalidFormat
	}
	return &roundTripper{base: base, client: c}, nil
}

func (c *clientAuth) PerRPCCredentials() (creds.PerRPCCredentials, error) {
	if strings.Contains(c.username(), ":") {
		return nil, errInvalidFormat
	}
	return &perRPCAuth{client: c}, nil
}

type roundTripper struct {
	base   http.RoundTripper
	client *clientAuth
}

func (r *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	newReq := req.Clone(req.Context())
	if newReq.Header == nil {
		newReq.Header = make(http.Header)
	}
	newReq.SetBasicAuth(r.client.username(), r.client.password())
	return r.base.RoundTrip(newReq)
}

type perRPCAuth struct {
	client *clientAuth
}

func (p *perRPCAuth) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return *p.client.grpcMetadata.Load(), nil
}

func (*perRPCAuth) RequireTransportSecurity() bool { return true }

// ---- shared helpers ----

func getAuthHeader(h map[string][]string) string {
	const (
		canonicalKey = "Authorization"
		metadataKey  = "authorization"
	)
	if v, ok := h[canonicalKey]; ok {
		return v[0]
	}
	if v, ok := h[metadataKey]; ok {
		return v[0]
	}
	for k, v := range h {
		if strings.EqualFold(k, metadataKey) && len(v) > 0 {
			return v[0]
		}
	}
	return ""
}

func parseBasicAuth(auth string) (*authData, error) {
	const prefix = "Basic "
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return nil, errInvalidSchemePrefix
	}
	encoded := auth[len(prefix):]
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errInvalidFormat
	}
	before, after, ok := strings.Cut(string(decoded), ":")
	if !ok {
		return nil, errInvalidFormat
	}
	return &authData{username: before, password: after, raw: encoded}, nil
}

var _ client.AuthData = (*authData)(nil)

type authData struct {
	username string
	password string
	raw      string
}

func (a *authData) GetAttribute(name string) any {
	switch name {
	case "username":
		return a.username
	case "raw":
		return a.raw
	default:
		return nil
	}
}

func (*authData) GetAttributeNames() []string {
	return []string{"username", "raw"}
}
