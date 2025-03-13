// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package basicauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension"

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/tg123/go-htpasswd"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
	creds "google.golang.org/grpc/credentials"
)

var (
	errNoAuth              = errors.New("no basic auth provided")
	errInvalidCredentials  = errors.New("invalid credentials")
	errInvalidSchemePrefix = errors.New("invalid authorization scheme prefix")
	errInvalidFormat       = errors.New("invalid authorization format")
)

func newClientAuthExtension(cfg *Config) *basicAuthClient {
	return &basicAuthClient{clientAuth: cfg.ClientAuth}
}

func newServerAuthExtension(cfg *Config) (extensionauth.Server, error) {
	if cfg.Htpasswd == nil || (cfg.Htpasswd.File == "" && cfg.Htpasswd.Inline == "") {
		return nil, errNoCredentialSource
	}

	return &basicAuthServer{
		htpasswd: cfg.Htpasswd,
	}, nil
}

var _ extensionauth.Server = (*basicAuthServer)(nil)

type basicAuthServer struct {
	htpasswd  *HtpasswdSettings
	matchFunc func(username, password string) bool
	component.ShutdownFunc
}

func (ba *basicAuthServer) Start(_ context.Context, _ component.Host) error {
	var rs []io.Reader

	if ba.htpasswd.File != "" {
		f, err := os.Open(ba.htpasswd.File)
		if err != nil {
			return fmt.Errorf("open htpasswd file: %w", err)
		}
		defer f.Close()

		rs = append(rs, f)
		rs = append(rs, strings.NewReader("\n"))
	}

	// Ensure that the inline content is read the last.
	// This way the inline content will override the content from file.
	rs = append(rs, strings.NewReader(ba.htpasswd.Inline))
	mr := io.MultiReader(rs...)

	htp, err := htpasswd.NewFromReader(mr, htpasswd.DefaultSystems, nil)
	if err != nil {
		return fmt.Errorf("read htpasswd content: %w", err)
	}

	ba.matchFunc = htp.Match

	return nil
}

func (ba *basicAuthServer) Authenticate(ctx context.Context, headers map[string][]string) (context.Context, error) {
	auth := getAuthHeader(headers)
	if auth == "" {
		return ctx, errNoAuth
	}

	authData, err := parseBasicAuth(auth)
	if err != nil {
		return ctx, err
	}

	if !ba.matchFunc(authData.username, authData.password) {
		return ctx, errInvalidCredentials
	}

	cl := client.FromContext(ctx)
	cl.Auth = authData
	return client.NewContext(ctx, cl), nil
}

func getAuthHeader(h map[string][]string) string {
	const (
		canonicalHeaderKey = "Authorization"
		metadataKey        = "authorization"
	)

	authHeaders, ok := h[canonicalHeaderKey]

	if !ok {
		authHeaders, ok = h[metadataKey]
	}

	if !ok {
		for k, v := range h {
			if strings.EqualFold(k, metadataKey) {
				authHeaders = v
				break
			}
		}
	}

	if len(authHeaders) == 0 {
		return ""
	}

	return authHeaders[0]
}

// See: https://github.com/golang/go/blob/1a8b4e05b1ff7a52c6d40fad73bcad612168d094/src/net/http/request.go#L950
func parseBasicAuth(auth string) (*authData, error) {
	const prefix = "Basic "
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return nil, errInvalidSchemePrefix
	}

	encoded := auth[len(prefix):]
	decodedBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errInvalidFormat
	}
	decoded := string(decodedBytes)

	si := strings.IndexByte(decoded, ':')
	if si < 0 {
		return nil, errInvalidFormat
	}

	return &authData{
		username: decoded[:si],
		password: decoded[si+1:],
		raw:      encoded,
	}, nil
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

// perRPCAuth is a gRPC credentials.PerRPCCredentials implementation that returns an 'authorization' header.
type perRPCAuth struct {
	metadata map[string]string
}

// GetRequestMetadata returns the request metadata to be used with the RPC.
func (p *perRPCAuth) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return p.metadata, nil
}

// RequireTransportSecurity always returns true for this implementation.
func (p *perRPCAuth) RequireTransportSecurity() bool {
	return true
}

type basicAuthRoundTripper struct {
	base     http.RoundTripper
	authData *ClientAuthSettings
}

func (b *basicAuthRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	newRequest := request.Clone(request.Context())
	newRequest.SetBasicAuth(b.authData.Username, string(b.authData.Password))
	return b.base.RoundTrip(newRequest)
}

var (
	_ extension.Extension      = (*basicAuthClient)(nil)
	_ extensionauth.HTTPClient = (*basicAuthClient)(nil)
	_ extensionauth.GRPCClient = (*basicAuthClient)(nil)
)

type basicAuthClient struct {
	component.StartFunc
	component.ShutdownFunc

	clientAuth *ClientAuthSettings
}

func (ba *basicAuthClient) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	if strings.Contains(ba.clientAuth.Username, ":") {
		return nil, errInvalidFormat
	}
	return &basicAuthRoundTripper{
		base:     base,
		authData: ba.clientAuth,
	}, nil
}

func (ba *basicAuthClient) PerRPCCredentials() (creds.PerRPCCredentials, error) {
	if strings.Contains(ba.clientAuth.Username, ":") {
		return nil, errInvalidFormat
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(ba.clientAuth.Username + ":" + string(ba.clientAuth.Password)))
	return &perRPCAuth{
		metadata: map[string]string{
			"authorization": fmt.Sprintf("Basic %s", encoded),
		},
	}, nil
}
