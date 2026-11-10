// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension"

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/multierr"
	"golang.org/x/oauth2"
)

const (
	clientAssertionTypeJWTBearer = "urn:ietf:params:oauth:client-assertion-type:jwt-bearer" //nolint:gosec // false positive, this is the assertion type name

	// clientAssertionLifetime is how long a signed client assertion is valid.
	// A new assertion is signed for every token request, so it only needs
	// to outlive a single round trip to the token endpoint.
	clientAssertionLifetime = 5 * time.Minute
)

// privateKeyJWTConfig fetches tokens with the client_credentials grant,
// authenticating the client at the token endpoint with a signed JWT
// ("private_key_jwt", RFC 7523 section 2.2 and OpenID Connect Core section 9).
type privateKeyJWTConfig struct {
	signer         *jwtSigner
	clientID       string
	clientIDFile   string
	tokenURL       string
	audience       string
	scopes         []string
	endpointParams url.Values
}

func newPrivateKeyJWTConfig(cfg *Config) (*privateKeyJWTConfig, error) {
	signer, err := newJWTSigner(cfg)
	if err != nil {
		return nil, err
	}
	return &privateKeyJWTConfig{
		signer:         signer,
		clientID:       cfg.ClientID,
		clientIDFile:   cfg.ClientIDFile,
		tokenURL:       cfg.TokenURL,
		audience:       cfg.Audience,
		scopes:         cfg.Scopes,
		endpointParams: cfg.EndpointParams,
	}, nil
}

func (c *privateKeyJWTConfig) TokenSource(ctx context.Context) oauth2.TokenSource {
	return privateKeyJWTSource{ctx: ctx, conf: c}
}

func (c *privateKeyJWTConfig) TokenEndpoint() string {
	return c.tokenURL
}

var _ oauth2.TokenSource = (*privateKeyJWTSource)(nil)

// privateKeyJWTSource signs a fresh client assertion for every token request.
type privateKeyJWTSource struct {
	ctx  context.Context
	conf *privateKeyJWTConfig
}

func (s privateKeyJWTSource) Token() (*oauth2.Token, error) {
	clientID, err := getActualValue(s.conf.clientID, s.conf.clientIDFile)
	if err != nil {
		return nil, multierr.Combine(errNoClientIDProvided, err)
	}

	audience := s.conf.tokenURL
	if s.conf.audience != "" {
		audience = s.conf.audience
	}

	now := time.Now()
	// RFC 7523 section 3: iss and sub must both be the client_id.
	assertion, err := s.conf.signer.sign(jwt.MapClaims{
		"iss": clientID,
		"sub": clientID,
		"aud": audience,
		"jti": uuid.New(),
		"iat": jwt.NewNumericDate(now),
		"exp": jwt.NewNumericDate(now.Add(clientAssertionLifetime)),
	})
	if err != nil {
		return nil, err
	}

	v := url.Values{}
	v.Set("grant_type", grantTypeClientCredentials)
	v.Set("client_id", clientID)
	v.Set("client_assertion_type", clientAssertionTypeJWTBearer)
	v.Set("client_assertion", assertion)
	if len(s.conf.scopes) > 0 {
		v.Set("scope", strings.Join(s.conf.scopes, " "))
	}

	return postTokenRequest(s.ctx, s.conf.tokenURL, v, s.conf.endpointParams)
}
