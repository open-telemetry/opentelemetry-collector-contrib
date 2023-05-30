// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package f5cloudexporter

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
)

type ErrorRoundTripper struct{}

func (ert *ErrorRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("error")
}

func TestF5CloudAuthRoundTripper_RoundTrip(t *testing.T) {
	validTokenSource := createMockTokenSource()
	source := "tests"

	defaultRoundTripper := (http.RoundTripper)(http.DefaultTransport.(*http.Transport).Clone())
	errorRoundTripper := &ErrorRoundTripper{}

	tests := []struct {
		name        string
		rt          http.RoundTripper
		token       oauth2.TokenSource
		shouldError bool
	}{
		{
			name:        "Test valid token source",
			rt:          defaultRoundTripper,
			token:       validTokenSource,
			shouldError: false,
		},
		{
			name:        "Test invalid token source",
			rt:          defaultRoundTripper,
			token:       &InvalidTokenSource{},
			shouldError: true,
		},
		{
			name:        "Test error in next round tripper",
			rt:          errorRoundTripper,
			token:       validTokenSource,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "Bearer test_access_token", r.Header.Get("Authorization"))
				assert.Equal(t, "tests", r.Header.Get(sourceHeader))
			}))
			defer server.Close()
			rt, err := newF5CloudAuthRoundTripper(tt.token, source, tt.rt)
			assert.NoError(t, err)
			req, err := http.NewRequest("POST", server.URL, strings.NewReader(""))
			assert.NoError(t, err)
			res, err := rt.RoundTrip(req)
			if tt.shouldError {
				assert.Nil(t, res)
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, res.StatusCode, 200)
		})
	}
}

func TestCreateF5CloudAuthRoundTripperWithToken(t *testing.T) {
	defaultRoundTripper := (http.RoundTripper)(http.DefaultTransport.(*http.Transport).Clone())

	token := createMockTokenSource()
	source := "test"

	tests := []struct {
		name        string
		token       oauth2.TokenSource
		source      string
		rt          http.RoundTripper
		shouldError bool
	}{
		{
			name:        "success_case",
			token:       token,
			source:      source,
			rt:          defaultRoundTripper,
			shouldError: false,
		},
		{
			name:        "no_token_provided_error",
			token:       nil,
			source:      source,
			rt:          defaultRoundTripper,
			shouldError: true,
		},
		{
			name:        "no_source_provided_error",
			token:       token,
			source:      "",
			rt:          defaultRoundTripper,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newF5CloudAuthRoundTripper(tt.token, tt.source, tt.rt)
			if tt.shouldError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func createMockTokenSource() oauth2.TokenSource {
	tkn := &oauth2.Token{
		AccessToken:  "test_access_token",
		TokenType:    "",
		RefreshToken: "",
		Expiry:       time.Time{},
	}

	return oauth2.StaticTokenSource(tkn)
}

type InvalidTokenSource struct{}

func (ts *InvalidTokenSource) Token() (*oauth2.Token, error) {
	return nil, fmt.Errorf("bad TokenSource for testing")
}
