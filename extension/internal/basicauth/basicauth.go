// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package basicauth // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/basicauth"

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"go.opentelemetry.io/collector/client"
	creds "google.golang.org/grpc/credentials"
)

var (
	ErrNoAuth              = errors.New("no basic auth provided")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrInvalidSchemePrefix = errors.New("invalid authorization scheme prefix")
	ErrInvalidFormat       = errors.New("invalid authorization format")
)

// CredentialProvider supplies username and password for client-side basic auth.
type CredentialProvider interface {
	Username() string
	Password() string
}

// AuthData implements client.AuthData for basic auth.
type AuthData struct {
	username string
	password string
	raw      string
}

var _ client.AuthData = (*AuthData)(nil)

func (a *AuthData) GetAttribute(name string) any {
	switch name {
	case "username":
		return a.username
	case "raw":
		return a.raw
	default:
		return nil
	}
}

func (*AuthData) GetAttributeNames() []string {
	return []string{"username", "raw"}
}

// GetAuthHeader extracts the Authorization header value from a header map,
// handling canonical, lowercase, and case-insensitive lookups.
func GetAuthHeader(h map[string][]string) string {
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

// ParseBasicAuth parses a "Basic <base64>" authorization header value.
func ParseBasicAuth(auth string) (*AuthData, error) {
	const prefix = "Basic "
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return nil, ErrInvalidSchemePrefix
	}

	encoded := auth[len(prefix):]
	decodedBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, ErrInvalidFormat
	}
	decoded := string(decodedBytes)

	before, after, ok := strings.Cut(decoded, ":")
	if !ok {
		return nil, ErrInvalidFormat
	}

	return &AuthData{
		username: before,
		password: after,
		raw:      encoded,
	}, nil
}

// Authenticate performs server-side basic auth validation against the given header map.
// It extracts the Authorization header, parses the credentials, and validates them
// using the provided match function. On success, it stores the auth data in the context.
func Authenticate(ctx context.Context, headers map[string][]string, matchFunc func(username, password string) bool) (context.Context, error) {
	auth := GetAuthHeader(headers)
	if auth == "" {
		return ctx, ErrNoAuth
	}

	authData, err := ParseBasicAuth(auth)
	if err != nil {
		return ctx, err
	}

	if !matchFunc(authData.username, authData.password) {
		return ctx, ErrInvalidCredentials
	}

	cl := client.FromContext(ctx)
	cl.Auth = authData
	return client.NewContext(ctx, cl), nil
}

// RoundTripper wraps a base http.RoundTripper to inject basic auth credentials.
type RoundTripper struct {
	Base     http.RoundTripper
	Provider CredentialProvider
}

func (rt *RoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	newRequest := request.Clone(request.Context())
	if newRequest.Header == nil {
		newRequest.Header = make(http.Header)
	}
	newRequest.SetBasicAuth(rt.Provider.Username(), rt.Provider.Password())
	return rt.Base.RoundTrip(newRequest)
}

// NewRoundTripper creates an http.RoundTripper that adds basic auth from the provider.
func NewRoundTripper(base http.RoundTripper, provider CredentialProvider) (http.RoundTripper, error) {
	if strings.Contains(provider.Username(), ":") {
		return nil, ErrInvalidFormat
	}
	return &RoundTripper{Base: base, Provider: provider}, nil
}

// PerRPCAuth implements grpc credentials.PerRPCCredentials using a CredentialProvider.
type PerRPCAuth struct {
	Provider CredentialProvider
}

func (p *PerRPCAuth) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	encoded := base64.StdEncoding.EncodeToString(
		[]byte(p.Provider.Username() + ":" + p.Provider.Password()),
	)
	return map[string]string{
		"authorization": fmt.Sprintf("Basic %s", encoded),
	}, nil
}

func (*PerRPCAuth) RequireTransportSecurity() bool { return true }

// NewPerRPCCredentials creates gRPC PerRPCCredentials that add basic auth from the provider.
func NewPerRPCCredentials(provider CredentialProvider) (creds.PerRPCCredentials, error) {
	if strings.Contains(provider.Username(), ":") {
		return nil, ErrInvalidFormat
	}
	return &PerRPCAuth{Provider: provider}, nil
}
