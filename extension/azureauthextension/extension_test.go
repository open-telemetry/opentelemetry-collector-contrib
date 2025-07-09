// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureauthextension

import (
	"context"
	"net/http"
	"net/url"
	"testing"

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
	m.On("GetToken").Return(azcore.AccessToken{Token: "test"}, nil)
	auth := authenticator{
		credential: &m,
	}
	_, err := auth.GetToken(context.Background(), policy.TokenRequestOptions{})
	require.NoError(t, err)
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
		headers     map[string][]string
		expectedErr string
	}{
		"missing_authorization_header": {
			headers: map[string][]string{
				"Host": {"Host"},
			},
			expectedErr: `missing "Authorization" header`,
		},
		"missing_host_header": {
			headers: map[string][]string{
				"Authorization": {"Bearer token"},
			},
			expectedErr: `missing "Host" header`,
		},
		"invalid_authorization_format": {
			headers: map[string][]string{
				"Authorization": {"Invalid authorization format"},
				"Host":          {"Host"},
			},
			expectedErr: "authorization header does not follow expected format",
		},
		"invalid_schema": {
			headers: map[string][]string{
				"Authorization": {"Invalid schema"},
				"Host":          {"Host"},
			},
			expectedErr: `expected "Bearer" as schema, got "Invalid"`,
		},
		"invalid_token": {
			headers: map[string][]string{
				"Authorization": {"Bearer invalid"},
				"Host":          {"Host"},
			},
			expectedErr: `unauthorized: invalid token`,
		},
		"valid_authenticate": {
			headers: map[string][]string{
				"Authorization": {"Bearer test"},
				"Host":          {"Host"},
			},
		},
	}

	m := mockTokenCredential{}
	m.On("GetToken").Return(azcore.AccessToken{Token: "test"}, nil)
	auth := authenticator{
		credential: &m,
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := auth.Authenticate(context.Background(), test.headers)
			if test.expectedErr != "" {
				require.ErrorContains(t, err, test.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
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
	m.On("GetToken").Return(azcore.AccessToken{Token: "test"}, nil)
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

func (m *mockTokenCredential) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	args := m.Called()
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
