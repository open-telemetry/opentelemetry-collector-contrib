// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package basicauth

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
)

type staticProvider struct {
	username string
	password string
}

func (s *staticProvider) Username() string { return s.username }
func (s *staticProvider) Password() string { return s.password }

func TestGetAuthHeader(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string][]string
		want    string
	}{
		{
			name:    "canonical",
			headers: map[string][]string{"Authorization": {"Basic abc"}},
			want:    "Basic abc",
		},
		{
			name:    "lowercase",
			headers: map[string][]string{"authorization": {"Basic abc"}},
			want:    "Basic abc",
		},
		{
			name:    "mixed case",
			headers: map[string][]string{"aUtHoRiZaTiOn": {"Basic abc"}},
			want:    "Basic abc",
		},
		{
			name:    "missing",
			headers: map[string][]string{},
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, GetAuthHeader(tt.headers))
		})
	}
}

func TestParseBasicAuth(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		encoded := base64.StdEncoding.EncodeToString([]byte("user:pass"))
		ad, err := ParseBasicAuth("Basic " + encoded)
		require.NoError(t, err)
		assert.Equal(t, "user", ad.GetAttribute("username"))
		assert.Equal(t, encoded, ad.GetAttribute("raw"))
		assert.Nil(t, ad.GetAttribute("nonexistent"))
		assert.Equal(t, []string{"username", "raw"}, ad.GetAttributeNames())
	})

	t.Run("wrong scheme", func(t *testing.T) {
		_, err := ParseBasicAuth("Bearer token")
		assert.ErrorIs(t, err, ErrInvalidSchemePrefix)
	})

	t.Run("invalid base64", func(t *testing.T) {
		_, err := ParseBasicAuth("Basic not-valid-base64!")
		assert.ErrorIs(t, err, ErrInvalidFormat)
	})

	t.Run("missing colon", func(t *testing.T) {
		encoded := base64.StdEncoding.EncodeToString([]byte("nocolon"))
		_, err := ParseBasicAuth("Basic " + encoded)
		assert.ErrorIs(t, err, ErrInvalidFormat)
	})
}

func TestAuthenticate(t *testing.T) {
	matchFunc := func(username, password string) bool {
		return username == "alice" && password == "secret"
	}

	t.Run("valid credentials", func(t *testing.T) {
		encoded := base64.StdEncoding.EncodeToString([]byte("alice:secret"))
		headers := map[string][]string{"Authorization": {"Basic " + encoded}}
		ctx, err := Authenticate(t.Context(), headers, matchFunc)
		require.NoError(t, err)
		cl := client.FromContext(ctx)
		assert.Equal(t, "alice", cl.Auth.GetAttribute("username"))
	})

	t.Run("invalid credentials", func(t *testing.T) {
		encoded := base64.StdEncoding.EncodeToString([]byte("alice:wrong"))
		headers := map[string][]string{"Authorization": {"Basic " + encoded}}
		_, err := Authenticate(t.Context(), headers, matchFunc)
		assert.ErrorIs(t, err, ErrInvalidCredentials)
	})

	t.Run("no header", func(t *testing.T) {
		_, err := Authenticate(t.Context(), map[string][]string{}, matchFunc)
		assert.ErrorIs(t, err, ErrNoAuth)
	})
}

type captureTransport struct{ last *http.Request }

func (c *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.last = req
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func TestRoundTripper(t *testing.T) {
	provider := &staticProvider{username: "user", password: "pass"}
	capture := &captureTransport{}
	rt, err := NewRoundTripper(capture, provider)
	require.NoError(t, err)

	req, _ := http.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
	_, err = rt.RoundTrip(req)
	require.NoError(t, err)

	expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
	assert.Equal(t, expected, capture.last.Header.Get("Authorization"))
}

func TestRoundTripper_ColonInUsername(t *testing.T) {
	provider := &staticProvider{username: "user:name", password: "pass"}
	_, err := NewRoundTripper(&captureTransport{}, provider)
	assert.ErrorIs(t, err, ErrInvalidFormat)
}

func TestPerRPCCredentials(t *testing.T) {
	provider := &staticProvider{username: "user", password: "pass"}
	cred, err := NewPerRPCCredentials(provider)
	require.NoError(t, err)

	md, err := cred.GetRequestMetadata(t.Context())
	require.NoError(t, err)
	expected := fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte("user:pass")))
	assert.Equal(t, expected, md["authorization"])
	assert.True(t, cred.RequireTransportSecurity())
}

func TestPerRPCCredentials_ColonInUsername(t *testing.T) {
	provider := &staticProvider{username: "user:name", password: "pass"}
	_, err := NewPerRPCCredentials(provider)
	assert.ErrorIs(t, err, ErrInvalidFormat)
}
