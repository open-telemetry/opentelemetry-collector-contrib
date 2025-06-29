// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oidcauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oidcauthextension"

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
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
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

var (
	_ extension.Extension  = (*oidcExtension)(nil)
	_ extensionauth.Server = (*oidcExtension)(nil)
)

type ProviderContainer struct {
	providerCfg ProviderCfg
	provider    *oidc.Provider
	verifier    *oidc.IDTokenVerifier
	client      *http.Client
	transport   *http.Transport
}

type oidcExtension struct {
	cfg *Config

	providerContainers map[string]*ProviderContainer
	logger             *zap.Logger
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

func newExtension(cfg *Config, logger *zap.Logger) extension.Extension {
	if cfg.Attribute == "" {
		cfg.Attribute = defaultAttribute
	}

	return &oidcExtension{
		cfg:                cfg,
		logger:             logger,
		providerContainers: make(map[string]*ProviderContainer),
	}
}

func (e *oidcExtension) Start(ctx context.Context, _ component.Host) error {
	var errs error
	for _, providerCfg := range e.cfg.getProviderConfigs() {
		errs = multierr.Append(errs, e.processProviderConfig(ctx, providerCfg))
	}
	if errs != nil {
		return fmt.Errorf("failed to get configuration from the auth server: %w", errs)
	}

	return nil
}

func (e *oidcExtension) Shutdown(context.Context) error {
	for _, p := range e.providerContainers {
		if p.client != nil {
			p.client.CloseIdleConnections()
		}
		if p.transport != nil {
			p.transport.CloseIdleConnections()
		}
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
	unsafeIssuer, err := getUnsafeIssuer(raw)
	if err != nil {
		return ctx, fmt.Errorf("failed to parse the token: %w", err)
	}
	providerContainer, ok := e.providerContainers[unsafeIssuer]
	if !ok {
		return ctx, fmt.Errorf("no OIDC provider configured for the issuer %q", unsafeIssuer)
	}

	idToken, err := providerContainer.verifier.Verify(ctx, raw)
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

	subject, err := getSubjectFromClaims(claims, providerContainer.providerCfg.UsernameClaim, idToken.Subject)
	if err != nil {
		return ctx, fmt.Errorf("failed to get subject from claims in the token: %w", err)
	}
	membership, err := getGroupsFromClaims(claims, providerContainer.providerCfg.GroupsClaim)
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

func (e *oidcExtension) processProviderConfig(ctx context.Context, p ProviderCfg) error {
	providerContainer := ProviderContainer{
		providerCfg: p,
	}

	providerContainer.transport = &http.Transport{
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

	cert, err := getIssuerCACertFromPath(p.IssuerCAPath)
	if err != nil {
		providerContainer.transport.CloseIdleConnections()
		return err // the errors from this path have enough context already
	}

	if cert != nil {
		providerContainer.transport.TLSClientConfig = &tls.Config{
			RootCAs: x509.NewCertPool(),
		}
		providerContainer.transport.TLSClientConfig.RootCAs.AddCert(cert)
	}

	providerContainer.client = &http.Client{
		Timeout:   5 * time.Second,
		Transport: providerContainer.transport,
	}
	oidcContext := oidc.ClientContext(ctx, providerContainer.client)
	providerContainer.provider, err = oidc.NewProvider(oidcContext, p.IssuerURL)
	if err != nil {
		providerContainer.transport.CloseIdleConnections()
		providerContainer.client.CloseIdleConnections()
		return fmt.Errorf("failed to create OIDC provider for %q: %w", p.IssuerURL, err)
	}
	providerContainer.verifier = providerContainer.provider.Verifier(&oidc.Config{
		ClientID:          p.Audience,
		SkipClientIDCheck: p.IgnoreAudience,
	})

	e.providerContainers[p.IssuerURL] = &providerContainer

	return nil
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

type idToken struct {
	Issuer string `json:"iss"`
}

// Get the issuer from the raw ID token.
// This function is unsafe because it does not verify the token's signature.
// It should only be used to determine which verifier to use for the token.
func getUnsafeIssuer(rawIDToken string) (string, error) {
	// TODO: it would be nice if we didn't have to parse the JWT here and then again in the verifier...
	payload, err := parseJWT(rawIDToken)
	if err != nil {
		return "", fmt.Errorf("oidc: malformed jwt: %w", err)
	}
	var token idToken
	if err := json.Unmarshal(payload, &token); err != nil {
		return "", fmt.Errorf("oidc: failed to unmarshal claims: %w", err)
	}
	return token.Issuer, nil
}

// https://github.com/coreos/go-oidc/blob/a7c457eacb849c163a496b29274242474a8f44ab/oidc/verify.go#L148
func parseJWT(p string) ([]byte, error) {
	parts := strings.Split(p, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("oidc: malformed jwt, expected 3 parts got %d", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("oidc: malformed jwt payload: %w", err)
	}
	return payload, nil
}
