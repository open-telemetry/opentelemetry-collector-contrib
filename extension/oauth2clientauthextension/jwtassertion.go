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
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/multierr"
	"golang.org/x/oauth2"
)

// parseSigningMethod returns the RSA signing method for the configured
// algorithm name, defaulting to RS256.
func parseSigningMethod(alg string) (*jwt.SigningMethodRSA, error) {
	switch alg {
	case jwt.SigningMethodRS256.Name, "":
		return jwt.SigningMethodRS256, nil
	case jwt.SigningMethodRS384.Name:
		return jwt.SigningMethodRS384, nil
	case jwt.SigningMethodRS512.Name:
		return jwt.SigningMethodRS512, nil
	default:
		return nil, errInvalidSignatureAlg
	}
}

// jwtSigner signs JWTs with an RSA private key that is either configured
// inline or read from a file. When a file is configured, it is re-read on
// every signature so that rotated keys are picked up.
type jwtSigner struct {
	// key contains the contents of a PEM encoded RSA private key (PKCS#1 or PKCS#8).
	// PEM containers with a passphrase are not supported.
	key     []byte
	keyFile string
	keyID   string
	method  *jwt.SigningMethodRSA
	// privateClaims are merged into the claims of every signed JWT, overriding
	// any claims with the same name.
	privateClaims map[string]any
}

func newJWTSigner(cfg *Config) (*jwtSigner, error) {
	method, err := parseSigningMethod(cfg.SignatureAlgorithm)
	if err != nil {
		return nil, err
	}
	s := &jwtSigner{
		key:           []byte(cfg.ClientCertificateKey),
		keyFile:       cfg.ClientCertificateKeyFile,
		keyID:         cfg.ClientCertificateKeyID,
		method:        method,
		privateClaims: cfg.Claims,
	}
	// Load the key once up front so that misconfiguration is reported early.
	if _, err := s.loadKey(); err != nil {
		return nil, multierr.Combine(errNoClientCertificateProvided, err)
	}
	return s, nil
}

func (s *jwtSigner) loadKey() ([]byte, error) {
	if s.keyFile == "" {
		return s.key, nil
	}
	key, err := readCredentialsFile(s.keyFile)
	if err != nil {
		return nil, err
	}
	return []byte(key), nil
}

// sign merges the private claims into claims and returns the signed JWT.
func (s *jwtSigner) sign(claims jwt.MapClaims) (string, error) {
	keyPEM, err := s.loadKey()
	if err != nil {
		return "", err
	}
	pk, err := jwt.ParseRSAPrivateKeyFromPEM(keyPEM)
	if err != nil {
		return "", err
	}

	maps.Copy(claims, s.privateClaims)

	token := jwt.NewWithClaims(s.method, claims)
	if s.keyID != "" {
		token.Header["kid"] = s.keyID
	}
	return token.SignedString(pk)
}

// postTokenRequest sends a form-encoded token request to tokenURL using the
// HTTP client from ctx. endpointParams are merged into v; they may override
// grant_type but no other parameter already set in v.
func postTokenRequest(ctx context.Context, tokenURL string, v, endpointParams url.Values) (*oauth2.Token, error) {
	for k, p := range endpointParams {
		// Allow grant_type to be overridden to allow interoperability with
		// non-compliant implementations.
		if _, ok := v[k]; ok && k != "grant_type" {
			return nil, fmt.Errorf("oauth2: cannot overwrite parameter %q", k)
		}
		v[k] = p
	}

	hc := oauth2.NewClient(ctx, nil)
	resp, err := hc.PostForm(tokenURL, v)
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
