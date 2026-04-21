// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureauthextension

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewAzureAuthenticator(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	ext, err := newAzureAuthenticator(cfg, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, ext)
}

func TestGetToken(t *testing.T) {
	m := mockTokenCredential{}
	m.On("GetToken", mock.Anything, mock.Anything).Return(azcore.AccessToken{Token: "test"}, nil)
	auth := authenticator{
		credential: &m,
	}
	_, err := auth.GetToken(t.Context(), policy.TokenRequestOptions{})
	require.NoError(t, err)
}

func TestToken(t *testing.T) {
	m := mockTokenCredential{}
	m.On("GetToken", mock.Anything, mock.Anything).Return(
		azcore.AccessToken{Token: "test", ExpiresOn: time.Now().Add(time.Hour)}, nil,
	)

	auth := authenticator{
		credential: &m,
	}

	token, err := auth.Token(t.Context())
	require.NoError(t, err)
	require.Equal(t, "test", token.AccessToken)
	m.AssertExpectations(t)
}

func TestGetHeaderValue(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		headers     map[string][]string
		header      string
		value       string
		expectedErr string
	}{
		"missing_header": {
			headers:     map[string][]string{},
			header:      "missing",
			expectedErr: `missing "missing" header`,
		},
		"empty_header": {
			headers: map[string][]string{
				"empty": {},
			},
			header:      "empty",
			expectedErr: `empty "empty" header`,
		},
		"valid_values": {
			headers: map[string][]string{
				"test": {"test"},
			},
			header: "test",
			value:  "test",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			v, err := getHeaderValue(test.header, test.headers)
			if test.expectedErr != "" {
				require.ErrorContains(t, err, test.expectedErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, v, test.value)
			}
		})
	}
}

func TestAuthenticate(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		headers      map[string][]string
		withVerifier bool
		verifierErr  error
		expectedErr  string
	}{
		"missing_authorization_header": {
			headers: map[string][]string{
				"Host": {"Host"},
			},
			expectedErr: `missing "Authorization" header`,
		},
		"invalid_authorization_format": {
			headers: map[string][]string{
				"Authorization": {"Invalid authorization format"},
			},
			expectedErr: "authorization header does not follow expected format",
		},
		"invalid_schema": {
			headers: map[string][]string{
				"Authorization": {"Invalid schema"},
			},
			expectedErr: `expected "Bearer" as schema, got "Invalid"`,
		},
		"missing_server_auth_config": {
			headers: map[string][]string{
				"Authorization": {"Bearer token"},
			},
			expectedErr: errServerAuthConfigRequired.Error(),
		},
		"invalid_token": {
			headers: map[string][]string{
				"Authorization": {"Bearer invalid"},
			},
			withVerifier: true,
			verifierErr:  errors.New("invalid token"),
			expectedErr:  "unauthorized: invalid token",
		},
		"valid_authenticate": {
			headers: map[string][]string{
				"Authorization": {"Bearer test"},
			},
			withVerifier: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			auth := authenticator{}
			if test.withVerifier {
				verifier := mockTokenVerifier{}
				verifier.On("Verify", mock.Anything, mock.Anything).Return(test.verifierErr)
				auth.verifier = &verifier
			}

			_, err := auth.Authenticate(t.Context(), test.headers)
			if test.expectedErr != "" {
				require.ErrorContains(t, err, test.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAuthenticateRejectsReplayedBearerToken(t *testing.T) {
	t.Parallel()

	verifier := mockTokenVerifier{}
	verifier.On("Verify", mock.Anything, "token-for-vault").Return(errors.New("audience mismatch"))

	auth := authenticator{
		verifier: &verifier,
	}

	_, err := auth.Authenticate(t.Context(), map[string][]string{
		"Authorization": {"Bearer token-for-vault"},
	})
	require.ErrorContains(t, err, "unauthorized: invalid token")
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		request     http.Request
		expectedErr string
	}{
		"empty_host_nil_url": {
			request: http.Request{
				Header: make(http.Header),
			},
			expectedErr: "unexpected nil request URL",
		},
		"empty_host_empty_url_host": {
			request: http.Request{
				Header: make(http.Header),
				URL:    &url.URL{},
			},
			expectedErr: "unexpected empty Host in request URL",
		},
		"valid_authorize_1": {
			request: http.Request{
				Header: make(http.Header),
				URL: &url.URL{
					Host: "test",
				},
			},
		},
		"valid_authorize_2": {
			request: http.Request{
				Header: make(http.Header),
				Host:   "override",
				URL: &url.URL{
					Host: "test",
				},
			},
		},
	}

	m := mockTokenCredential{}
	m.On("GetToken", mock.Anything, mock.Anything).Return(azcore.AccessToken{Token: "test"}, nil)
	auth := authenticator{
		credential: &m,
	}
	base := &mockRoundTripper{}
	base.On("RoundTrip", mock.Anything).Return(&http.Response{StatusCode: http.StatusOK}, nil)
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			r, err := auth.RoundTripper(base)
			require.NoError(t, err)

			_, err = r.RoundTrip(&test.request)
			if test.expectedErr != "" {
				require.ErrorContains(t, err, test.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

type mockTokenCredential struct {
	mock.Mock
}

var _ azcore.TokenCredential = (*mockTokenCredential)(nil)

func (m *mockTokenCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	args := m.Called(ctx, opts)
	return args.Get(0).(azcore.AccessToken), args.Error(1)
}

type mockRoundTripper struct {
	mock.Mock
}

var _ http.RoundTripper = (*mockRoundTripper)(nil)

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	return args.Get(0).(*http.Response), args.Error(1)
}

type mockTokenVerifier struct {
	mock.Mock
}

func (m *mockTokenVerifier) Verify(ctx context.Context, rawToken string) error {
	args := m.Called(ctx, rawToken)
	return args.Error(0)
}
