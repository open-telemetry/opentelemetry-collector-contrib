// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oidcauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oidcauthextension"

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.uber.org/zap"
)

var _ extensionauth.Server = (*oidcExtension)(nil)

type oidcExtension struct {
	cfg *Config

	provider  *oidc.Provider
	verifier  *oidc.IDTokenVerifier
	client    *http.Client
	logger    *zap.Logger
	transport *http.Transport
}

var (
	errNoAudienceProvided                = errors.New("no Audience provided for the OIDC configuration")
	errNoIssuerURL                       = errors.New("no IssuerURL provided for the OIDC configuration")
	errInvalidAuthenticationHeaderFormat = errors.New("invalid authorization header format")
	errFailedToObtainClaimsFromToken     = errors.New("failed to get the subject from the token issued by the OIDC provider")
	errClaimNotFound                     = errors.New("username claim from the OIDC configuration not found on the token returned by the OIDC provider")
	errUsernameNotString                 = errors.New("the username returned by the OIDC provider isn't a regular string")
	errGroupsClaimNotFound               = errors.New("groups claim from the OIDC configuration not found on the token returned by the OIDC provider")
	errNotAuthenticated                  = errors.New("authentication didn't succeed")
)

func newExtension(cfg *Config, logger *zap.Logger) extensionauth.Server {
	if cfg.Attribute == "" {
		cfg.Attribute = defaultAttribute
	}

	return &oidcExtension{
		cfg:    cfg,
		logger: logger,
	}
}

func (e *oidcExtension) Start(ctx context.Context, _ component.Host) error {
	err := e.setProviderConfig(ctx, e.cfg)
	if err != nil {
		return fmt.Errorf("failed to get configuration from the auth server: %w", err)
	}
	e.verifier = e.provider.Verifier(&oidc.Config{
		ClientID: e.cfg.Audience,
	})
	return nil
}

func (e *oidcExtension) Shutdown(context.Context) error {
	if e.client != nil {
		e.client.CloseIdleConnections()
	}
	if e.transport != nil {
		e.transport.CloseIdleConnections()
	}

	return nil
}

// authenticate checks whether the given context contains valid auth data. Successfully authenticated calls will always return a nil error and a context with the auth data.
func (e *oidcExtension) Authenticate(ctx context.Context, headers map[string][]string) (context.Context, error) {
	var authHeaders []string
	for k, v := range headers {
		if strings.EqualFold(k, e.cfg.Attribute) {
			authHeaders = v
			break
		}
	}
	if len(authHeaders) == 0 {
		return ctx, errNotAuthenticated
	}

	// we only use the first header, if multiple values exist
	parts := strings.Split(authHeaders[0], " ")
	if len(parts) != 2 {
		return ctx, errInvalidAuthenticationHeaderFormat
	}

	raw := parts[1]
	idToken, err := e.verifier.Verify(ctx, raw)
	if err != nil {
		return ctx, fmt.Errorf("failed to verify token: %w", err)
	}

	claims := map[string]any{}
	if err = idToken.Claims(&claims); err != nil {
		// currently, this isn't a valid condition, the Verify call a few lines above
		// will already attempt to parse the payload as a json and set it as the claims
		// for the token. As we are using a map to hold the claims, there's no way to fail
		// to read the claims. It could fail if we were using a custom struct. Instead of
		// swallowing the error, it's better to make this future-proof, in case the underlying
		// code changes
		return ctx, errFailedToObtainClaimsFromToken
	}

	subject, err := getSubjectFromClaims(claims, e.cfg.UsernameClaim, idToken.Subject)
	if err != nil {
		return ctx, fmt.Errorf("failed to get subject from claims in the token: %w", err)
	}
	membership, err := getGroupsFromClaims(claims, e.cfg.GroupsClaim)
	if err != nil {
		return ctx, fmt.Errorf("failed to get groups from claims in the token: %w", err)
	}

	cl := client.FromContext(ctx)
	cl.Auth = &authData{
		raw:        raw,
		subject:    subject,
		membership: membership,
	}
	return client.NewContext(ctx, cl), nil
}

func (e *oidcExtension) setProviderConfig(ctx context.Context, config *Config) error {
	e.transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 10 * time.Second,
			DualStack: true,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	cert, err := getIssuerCACertFromPath(config.IssuerCAPath)
	if err != nil {
		return err // the errors from this path have enough context already
	}

	if cert != nil {
		e.transport.TLSClientConfig = &tls.Config{
			RootCAs: x509.NewCertPool(),
		}
		e.transport.TLSClientConfig.RootCAs.AddCert(cert)
	}

	e.client = &http.Client{
		Timeout:   5 * time.Second,
		Transport: e.transport,
	}
	oidcContext := oidc.ClientContext(ctx, e.client)
	provider, err := oidc.NewProvider(oidcContext, config.IssuerURL)
	e.provider = provider

	return err
}

func getSubjectFromClaims(claims map[string]any, usernameClaim string, fallback string) (string, error) {
	if len(usernameClaim) > 0 {
		username, found := claims[usernameClaim]
		if !found {
			return "", errClaimNotFound
		}

		sUsername, ok := username.(string)
		if !ok {
			return "", errUsernameNotString
		}

		return sUsername, nil
	}

	return fallback, nil
}

func getGroupsFromClaims(claims map[string]any, groupsClaim string) ([]string, error) {
	if len(groupsClaim) > 0 {
		var groups []string
		rawGroup, ok := claims[groupsClaim]
		if !ok {
			return nil, errGroupsClaimNotFound
		}
		switch v := rawGroup.(type) {
		case string:
			groups = append(groups, v)
		case []string:
			groups = v
		case []any:
			groups = make([]string, 0, len(v))
			for i := range v {
				groups = append(groups, fmt.Sprintf("%v", v[i]))
			}
		}

		return groups, nil
	}

	return []string{}, nil
}

func getIssuerCACertFromPath(path string) (*x509.Certificate, error) {
	if path == "" {
		return nil, nil
	}

	rawCA, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("could not read the CA file %q: %w", path, err)
	}

	if len(rawCA) == 0 {
		return nil, fmt.Errorf("could not read the CA file %q: empty file", path)
	}

	block, _ := pem.Decode(rawCA)
	if block == nil {
		return nil, fmt.Errorf("cannot decode the contents of the CA file %q: %w", path, err)
	}

	return x509.ParseCertificate(block.Bytes)
}
