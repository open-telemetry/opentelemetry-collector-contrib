// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clientauth // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension/internal/clientauth"

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
	"google.golang.org/api/idtoken"
)

func init() {
	// Make sure metadata.OnGCE always returns true, since the result is
	// cached.
	_ = os.Setenv("GCE_METADATA_HOST", "127.0.0.1")
}

type fakeMetadataToken struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int64  `json:"expires_in"`
}

func TestRoundTripper(t *testing.T) {
	fakeToken := fakeMetadataToken{
		AccessToken:  "accessToken",
		TokenType:    "tokenType",
		RefreshToken: "refreshToken",
		ExpiresIn:    1,
	}
	b, err := json.Marshal(fakeToken)
	assert.NoError(t, err)
	tokenString := string(b)
	// Mimic metadata server, and return the fake access token.
	srvProvidingTokens := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, tokenString)
	}))
	defer srvProvidingTokens.Close()
	t.Setenv("GCE_METADATA_HOST", srvProvidingTokens.Listener.Addr().String())
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	t.Setenv("HOME", t.TempDir())

	ca := clientAuthenticator{
		config: &Config{
			Project:      "my-project",
			QuotaProject: "other-project",
			TokenType:    accessToken,
			TokenHeader:  authorizationHeader,
		},
	}

	err = ca.Start(t.Context(), nil)
	assert.NoError(t, err)

	rt, err := ca.RoundTripper(roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, "other-project", r.Header.Get("X-Goog-User-Project"))
		assert.Equal(t, "my-project", r.Header.Get("X-Goog-Project-ID"))
		assert.Equal(t, "bar", r.Header.Get("foo"))
		if r.Header.Get("Authorization") != "tokenType accessToken" {
			// Don't print this out in-case it is a real access token.
			t.Error("Authorization header was incorrect. FindDefaultCredentials may have found real credentials.")
		}
		return &http.Response{}, nil
	}))
	assert.NotNil(t, rt)
	assert.NoError(t, err)
	header := make(http.Header)
	header.Set("foo", "bar")
	_, err = rt.RoundTrip(&http.Request{Header: header})
	assert.NoError(t, err)
}

func TestRoundTripperWithIDToken(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/fake_isa_creds.json")
	ca := clientAuthenticator{
		config: &Config{
			Project:      "my-project",
			QuotaProject: "other-project",
			TokenType:    idToken,
			Audience:     "http://example.com",
			TokenHeader:  authorizationHeader,
		},
		newIDTokenSource: func(_ context.Context, _ string, _ ...idtoken.ClientOption) (oauth2.TokenSource, error) {
			return &mockIDTokenSource{token: "dummy token"}, nil
		},
	}
	err := ca.Start(t.Context(), nil)
	assert.NoError(t, err)

	rt, err := ca.RoundTripper(roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, "Bearer dummy token", r.Header.Get("Authorization"))
		assert.Equal(t, "other-project", r.Header.Get("X-Goog-User-Project"))
		assert.Equal(t, "my-project", r.Header.Get("X-Goog-Project-ID"))
		return &http.Response{}, nil
	}))
	assert.NotNil(t, rt)
	assert.NoError(t, err)

	_, err = rt.RoundTrip(&http.Request{Header: make(http.Header)})
	assert.NoError(t, err)
}

func TestRoundTripperNotStarted(t *testing.T) {
	ca := clientAuthenticator{config: &Config{
		Project:      "my-project",
		QuotaProject: "other-project",
		TokenType:    accessToken,
		TokenHeader:  authorizationHeader,
	}}

	rt, err := ca.RoundTripper(roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, nil
	}))
	assert.Nil(t, rt)
	assert.Error(t, err)
}

func TestRoundTrip(t *testing.T) {
	tr := parameterTransport{
		config: &Config{
			Project:      "my-project",
			QuotaProject: "other-project",
			TokenType:    accessToken,
			TokenHeader:  authorizationHeader,
		},
		base: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			assert.Equal(t, "other-project", r.Header.Get("X-Goog-User-Project"))
			assert.Equal(t, "my-project", r.Header.Get("X-Goog-Project-ID"))
			assert.Equal(t, "bar", r.Header.Get("foo"))
			return &http.Response{}, nil
		}),
	}
	header := make(http.Header)
	header.Set("foo", "bar")
	_, err := tr.RoundTrip(&http.Request{Header: header})
	assert.NoError(t, err)
}

func TestRoundTripWithIDToken(t *testing.T) {
	tr := parameterTransport{
		config: &Config{
			Project:     "my-project",
			TokenType:   idToken,
			Audience:    "http://example.com",
			TokenHeader: authorizationHeader,
		},
		base: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			assert.Equal(t, "my-project", r.Header.Get("X-Goog-Project-ID"))
			assert.Equal(t, "bar", r.Header.Get("foo"))
			return &http.Response{}, nil
		}),
	}
	header := make(http.Header)
	header.Set("foo", "bar")
	_, err := tr.RoundTrip(&http.Request{Header: header})
	assert.NoError(t, err)
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func TestRoundTripperWithProxyAuth(t *testing.T) {
	fakeToken := fakeMetadataToken{
		AccessToken:  "accessToken",
		TokenType:    "tokenType",
		RefreshToken: "refreshToken",
		ExpiresIn:    1,
	}
	b, err := json.Marshal(fakeToken)
	assert.NoError(t, err)
	tokenString := string(b)
	srvProvidingTokens := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, tokenString)
	}))
	defer srvProvidingTokens.Close()
	t.Setenv("GCE_METADATA_HOST", srvProvidingTokens.Listener.Addr().String())
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	t.Setenv("HOME", t.TempDir())

	ca := clientAuthenticator{
		config: &Config{
			Project:      "my-project",
			QuotaProject: "other-project",
			TokenType:    accessToken,
			TokenHeader:  proxyAuthorizationHeader,
		},
	}
	err = ca.Start(t.Context(), nil)
	assert.NoError(t, err)

	rt, err := ca.RoundTripper(roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, "other-project", r.Header.Get("X-Goog-User-Project"))
		assert.Equal(t, "my-project", r.Header.Get("X-Goog-Project-ID"))
		assert.Equal(t, "tokenType accessToken", r.Header.Get("Proxy-Authorization"))
		assert.Empty(t, r.Header.Get("Authorization"))
		assert.Equal(t, "bar", r.Header.Get("foo"))
		return &http.Response{}, nil
	}))
	assert.NoError(t, err)
	assert.IsType(t, &proxyAuthTransport{}, rt)

	header := make(http.Header)
	header.Set("foo", "bar")
	_, err = rt.RoundTrip(&http.Request{Header: header})
	assert.NoError(t, err)
}

func TestProxyAuthTransportRoundTrip(t *testing.T) {
	token := &oauth2.Token{
		AccessToken: "test-token",
		TokenType:   "Bearer",
	}
	tr := &proxyAuthTransport{
		source: oauth2.StaticTokenSource(token),
		base: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			assert.Equal(t, "Bearer test-token", r.Header.Get("Proxy-Authorization"))
			assert.Empty(t, r.Header.Get("Authorization"))
			assert.Equal(t, "bar", r.Header.Get("foo"))
			return &http.Response{}, nil
		}),
	}
	header := make(http.Header)
	header.Set("foo", "bar")
	_, err := tr.RoundTrip(&http.Request{Header: header})
	assert.NoError(t, err)
}

func TestNilBase(t *testing.T) {
	tr := parameterTransport{
		config: &Config{},
		base:   nil,
	}
	_, err := tr.RoundTrip(&http.Request{})
	assert.Error(t, err)
}
