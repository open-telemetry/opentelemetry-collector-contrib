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
	grantTypeJWTBearer = "urn:ietf:params:oauth:grant-type:jwt-bearer" //nolint:gosec // false positive, this is the grant-type name
)

func newJwtGrantTypeConfig(cfg *Config) (*jwtGrantTypeConfig, error) {
	signer, err := newJWTSigner(cfg)
	if err != nil {
		return nil, err
	}

	clientID, err := getActualValue(cfg.ClientID, cfg.ClientIDFile)
	if err != nil {
		return nil, multierr.Combine(errNoClientIDProvided, err)
	}

	iss := cfg.Iss
	if iss == "" {
		iss = clientID
	}

	return &jwtGrantTypeConfig{
		signer:         signer,
		Scopes:         cfg.Scopes,
		TokenURL:       cfg.TokenURL,
		Iss:            iss,
		Subject:        cfg.ClientID,
		Audience:       cfg.Audience,
		EndpointParams: cfg.EndpointParams,
	}, nil
}

// jwtGrantTypeConfig is the configuration for using the JWT bearer
// authorization grant (RFC 7523 section 2.1) to fetch tokens,
// commonly known as "two-legged OAuth 2.0".
type jwtGrantTypeConfig struct {
	// signer signs the JWT assertion.
	signer *jwtSigner

	// Iss is the OAuth client identifier used when communicating with
	// the configured OAuth provider.
	Iss string

	// Subject is the optional user to impersonate.
	Subject string

	// Scopes optionally specifies a list of requested permission scopes.
	Scopes []string

	// TokenURL is the endpoint required to complete the 2-legged JWT flow.
	TokenURL string

	// EndpointParams specifies additional parameters for requests to the token endpoint.
	EndpointParams url.Values

	// Expires optionally specifies how long the token is valid for.
	Expires time.Duration

	// Audience optionally specifies the intended audience of the
	// request.  If empty, the value of TokenURL is used as the
	// intended audience.
	Audience string
}

// TokenSource returns a JWT TokenSource using the configuration
// in c and the HTTP client from the provided context.
func (c *jwtGrantTypeConfig) TokenSource(ctx context.Context) oauth2.TokenSource {
	return jwtSource{ctx, c}
}

func (c *jwtGrantTypeConfig) TokenEndpoint() string {
	return c.TokenURL
}

// jwtSource implements TokenSource
var _ oauth2.TokenSource = (*jwtSource)(nil)

// jwtSource is a source that always does a signed JWT request for a token.
// It should typically be wrapped with a reuseTokenSource.
type jwtSource struct {
	ctx  context.Context
	conf *jwtGrantTypeConfig
}

func (js jwtSource) Token() (*oauth2.Token, error) {
	audience := js.conf.TokenURL
	if aud := js.conf.Audience; aud != "" {
		audience = aud
	}
	expiration := time.Now().Add(30 * time.Minute)
	if t := js.conf.Expires; t > 0 {
		expiration = time.Now().Add(t)
	}
	scopes := strings.Join(js.conf.Scopes, " ")

	claims := jwt.MapClaims{
		"iss": js.conf.Iss,
		"sub": js.conf.Subject,
		"jti": uuid.New(),
		"aud": audience,
		"iat": jwt.NewNumericDate(time.Now()),
		"exp": jwt.NewNumericDate(expiration),
	}

	if scopes != "" {
		claims["scope"] = scopes
	}

	payload, err := js.conf.signer.sign(claims)
	if err != nil {
		return nil, err
	}
	v := url.Values{}
	v.Set("grant_type", grantTypeJWTBearer)
	v.Set("assertion", payload)
	if scopes != "" {
		v.Set("scope", scopes)
	}

	return postTokenRequest(js.ctx, js.conf.TokenURL, v, js.conf.EndpointParams)
}
