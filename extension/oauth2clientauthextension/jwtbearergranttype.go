// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
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
	var sig *jwt.SigningMethodRSA
	switch cfg.SignatureAlgorithm {
	case jwt.SigningMethodRS256.Name:
		sig = jwt.SigningMethodRS256
	case jwt.SigningMethodRS384.Name:
		sig = jwt.SigningMethodRS384
	case jwt.SigningMethodRS512.Name:
		sig = jwt.SigningMethodRS512
	case "":
		sig = jwt.SigningMethodRS256
	default:
		return nil, errInvalidSignatureAlg
	}

	clientID, err := getActualValue(cfg.ClientID, cfg.ClientIDFile)
	if err != nil {
		return nil, multierr.Combine(errNoClientIDProvided, err)
	}

	clientCertificate, err := getActualValue(string(cfg.ClientCertificateKey), cfg.ClientCertificateKeyFile)
	if err != nil {
		return nil, multierr.Combine(errNoClientCertificateProvided, err)
	}

	iss := cfg.Iss
	if iss == "" {
		iss = clientID
	}

	return &jwtGrantTypeConfig{
		PrivateKey:       []byte(clientCertificate),
		PrivateKeyID:     cfg.ClientCertificateKeyID,
		Scopes:           cfg.Scopes,
		TokenURL:         cfg.TokenURL,
		SigningAlgorithm: sig,
		Iss:              iss,
		Subject:          cfg.ClientID,
		Audience:         cfg.Audience,
		PrivateClaims:    cfg.Claims,
		EndpointParams:   cfg.EndpointParams,
	}, nil
}

// Config is the configuration for using JWT to fetch tokens,
// commonly known as "two-legged OAuth 2.0".
type jwtGrantTypeConfig struct {
	// Iss is the OAuth client identifier used when communicating with
	// the configured OAuth provider.
	Iss string

	// PrivateKey contains the contents of an RSA private key or the
	// contents of a PEM file that contains a private key. The provided
	// private key is used to sign JWT payloads.
	// PEM containers with a passphrase are not supported.
	// Use the following command to convert a PKCS 12 file into a PEM.
	//
	//    $ openssl pkcs12 -in key.p12 -out key.pem -nodes
	//
	PrivateKey []byte

	// SigningAlgorithm is the RSA algorithm used to sign JWT payloads
	SigningAlgorithm *jwt.SigningMethodRSA

	// PrivateKeyID contains an optional hint indicating which key is being
	// used.
	PrivateKeyID string

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

	// PrivateClaims optionally specifies custom private claims in the JWT.
	// See http://tools.ietf.org/html/draft-jones-json-web-token-10#section-4.3
	PrivateClaims map[string]any
}

// TokenSource returns a JWT TokenSource using the configuration
// in c and the HTTP client from the provided context.
func (c *jwtGrantTypeConfig) TokenSource(ctx context.Context) oauth2.TokenSource {
	return oauth2.ReuseTokenSource(nil, jwtSource{ctx, c})
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
	pk, err := jwt.ParseRSAPrivateKeyFromPEM(js.conf.PrivateKey)
	if err != nil {
		return nil, err
	}
	hc := oauth2.NewClient(js.ctx, nil)
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

	maps.Copy(claims, js.conf.PrivateClaims)

	assertion := jwt.NewWithClaims(js.conf.SigningAlgorithm, claims)
	if js.conf.PrivateKeyID != "" {
		assertion.Header["kid"] = js.conf.PrivateKeyID
	}
	payload, err := assertion.SignedString(pk)
	if err != nil {
		return nil, err
	}
	v := url.Values{}
	v.Set("grant_type", grantTypeJWTBearer)
	v.Set("assertion", payload)
	if scopes != "" {
		v.Set("scope", scopes)
	}

	for k, p := range js.conf.EndpointParams {
		// Allow grant_type to be overridden to allow interoperability with
		// non-compliant implementations.
		if _, ok := v[k]; ok && k != "grant_type" {
			return nil, fmt.Errorf("oauth2: cannot overwrite parameter %q", k)
		}
		v[k] = p
	}

	resp, err := hc.PostForm(js.conf.TokenURL, v)
	if err != nil {
		return nil, fmt.Errorf("oauth2: cannot fetch token: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("oauth2: cannot fetch token: %w", err)
	}
	if c := resp.StatusCode; c < 200 || c > 299 {
		return nil, &oauth2.RetrieveError{
			Response: resp,
			Body:     body,
		}
	}
	// tokenRes is the JSON response body.
	var tokenRes struct {
		oauth2.Token
	}
	if err := json.Unmarshal(body, &tokenRes); err != nil {
		return nil, fmt.Errorf("oauth2: cannot fetch token: %w", err)
	}
	token := &oauth2.Token{
		AccessToken: tokenRes.AccessToken,
		TokenType:   tokenRes.TokenType,
	}
	if secs := tokenRes.ExpiresIn; secs > 0 {
		token.Expiry = time.Now().Add(time.Duration(secs) * time.Second)
	}
	return token, nil
}
