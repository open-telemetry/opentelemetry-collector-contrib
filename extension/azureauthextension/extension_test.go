// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureauthextension

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.uber.org/zap"
)

func TestNewAzureAuthenticator(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		cfg            *Config
		expectedServer configServer
		expectedScopes []string
		expectedErr    string
	}{
		"use_default": {
			cfg: &Config{UseDefault: true},
		},
		"managed_identity_system": {
			cfg: &Config{Managed: configoptional.Some(ManagedIdentity{})},
		},
		"managed_identity_user": {
			cfg: &Config{Managed: configoptional.Some(ManagedIdentity{ClientID: "client"})},
		},
		"workload_identity": {
			cfg: &Config{Workload: configoptional.Some(WorkloadIdentity{
				TenantID:           "tenant",
				ClientID:           "client",
				FederatedTokenFile: "/tmp/token",
			})},
		},
		"service_principal_secret": {
			cfg: &Config{ServicePrincipal: configoptional.Some(ServicePrincipal{
				TenantID:     "tenant",
				ClientID:     "client",
				ClientSecret: "secret",
			})},
		},
		"service_principal_missing_cert_file": {
			cfg: &Config{ServicePrincipal: configoptional.Some(ServicePrincipal{
				TenantID:              "tenant",
				ClientID:              "client",
				ClientCertificatePath: "/nonexistent/cert.pem",
			})},
			expectedErr: "could not read the certificate file",
		},
		"server_config_propagated": {
			cfg: &Config{
				UseDefault: true,
				Server: configoptional.Some(Server{
					IssuerURL: "https://issuer.example",
					Audience:  "api://aud",
				}),
			},
			expectedServer: configServer{
				issuerURL: "https://issuer.example",
				audience:  "api://aud",
			},
		},
		"scopes_propagated": {
			cfg: &Config{
				UseDefault: true,
				Scopes:     []string{"https://example.com/.default"},
			},
			expectedScopes: []string{"https://example.com/.default"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a, err := newAzureAuthenticator(tc.cfg, zap.NewNop())
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, a)
			require.NotNil(t, a.credential)
			require.Equal(t, tc.expectedServer, a.server)
			require.Equal(t, tc.expectedScopes, a.scopes)
		})
	}
}

func TestStart(t *testing.T) {
	t.Parallel()

	var issuerURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"issuer":%q,"jwks_uri":%q,"id_token_signing_alg_values_supported":["RS256"]}`, issuerURL, issuerURL+"/jwks")
		case "/jwks":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"keys":[]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	issuerURL = srv.URL

	tests := map[string]struct {
		server         configServer
		expectVerifier bool
		expectedErr    string
	}{
		"no_server_config": {},
		"only_issuer_set": {
			server: configServer{issuerURL: issuerURL},
		},
		"only_audience_set": {
			server: configServer{audience: "aud"},
		},
		"valid_discovery": {
			server:         configServer{issuerURL: issuerURL, audience: "aud"},
			expectVerifier: true,
		},
		"unreachable_issuer": {
			server:      configServer{issuerURL: "http://127.0.0.1:1", audience: "aud"},
			expectedErr: "failed to initialize OIDC provider",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a := &authenticator{server: tc.server}
			err := a.Start(t.Context(), nil)
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
			if tc.expectVerifier {
				require.NotNil(t, a.verifier)
			} else {
				require.Nil(t, a.verifier)
			}
		})
	}
}

func TestGetToken(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		withCredential bool
		expectedErr    string
	}{
		"nil_credential": {
			expectedErr: "credentials were not initialized",
		},
		"valid": {
			withCredential: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a := authenticator{}
			if tc.withCredential {
				m := &mockTokenCredential{}
				m.On("GetToken", mock.Anything, mock.Anything).Return(azcore.AccessToken{Token: "test"}, nil)
				a.credential = m
			}
			_, err := a.GetToken(t.Context(), policy.TokenRequestOptions{})
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestGetTokenForHost(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		scopes         []string
		host           string
		credentialErr  error
		expectedScopes []string
		expectedErr    string
	}{
		"host_derived_scope": {
			host:           "management.azure.com",
			expectedScopes: []string{"https://management.azure.com/.default"},
		},
		"configured_scopes_override_host": {
			scopes:         []string{"https://custom.example/.default"},
			host:           "management.azure.com",
			expectedScopes: []string{"https://custom.example/.default"},
		},
		"credential_error": {
			host:          "management.azure.com",
			credentialErr: errors.New("token fetch failed"),
			expectedErr:   "token fetch failed",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := &mockTokenCredential{}
			m.On("GetToken", mock.Anything, mock.Anything).Return(azcore.AccessToken{Token: "token"}, tc.credentialErr)

			a := authenticator{credential: m, scopes: tc.scopes}
			tok, err := a.getTokenForHost(t.Context(), tc.host)
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "token", tok)

			m.AssertCalled(t, "GetToken", mock.Anything, policy.TokenRequestOptions{Scopes: tc.expectedScopes})
		})
	}
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
		request       http.Request
		credentialErr error
		expectedErr   string
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
		"credential_error": {
			request: http.Request{
				Header: make(http.Header),
				URL:    &url.URL{Host: "test"},
			},
			credentialErr: errors.New("token fetch failed"),
			expectedErr:   "azure_auth: failed to get token",
		},
		"valid_authorize_from_url": {
			request: http.Request{
				Header: make(http.Header),
				URL:    &url.URL{Host: "test"},
			},
		},
		"valid_authorize_host_overrides_url": {
			request: http.Request{
				Header: make(http.Header),
				Host:   "override",
				URL:    &url.URL{Host: "test"},
			},
		},
	}

	base := &mockRoundTripper{}
	base.On("RoundTrip", mock.Anything).Return(&http.Response{StatusCode: http.StatusOK}, nil)
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			m := &mockTokenCredential{}
			m.On("GetToken", mock.Anything, mock.Anything).Return(azcore.AccessToken{Token: "test"}, test.credentialErr)
			auth := authenticator{credential: m}

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
