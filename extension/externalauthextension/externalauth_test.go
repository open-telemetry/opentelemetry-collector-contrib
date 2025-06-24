package externalauthextension

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	auth "go.opentelemetry.io/collector/extension/extensionauth"
	"go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/zap"
)

func TestNewExternalAuth(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *Config
		expectErr bool
	}{
		{
			name: "Valid config",
			cfg: &Config{
				Endpoint:        "http://example.com",
				RefreshInterval: "1h",
				Header:          DefaultAuthorizationHeader,
				ExpectedCodes:   []int{200, 201},
				Scheme:          "Bearer",
				Method:          "GET",
			},
			expectErr: false,
		},
		{
			name: "Invalid config",
			cfg: &Config{
				Endpoint:        "",
				RefreshInterval: "1h",
				Header:          DefaultAuthorizationHeader,
				ExpectedCodes:   []int{200},
				Scheme:          "Bearer",
				Method:          "GET",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			telemetry := component.TelemetrySettings{
				Logger:        zap.NewNop(),
				MeterProvider: noop.NewMeterProvider(),
			}
			server, err := newExternalAuth(tt.cfg, telemetry)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, server)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, server)
				assert.Implements(t, (*auth.Server)(nil), server)
			}
		})
	}
}
func TestServerAuthenticate(t *testing.T) {
	tests := []struct {
		name      string
		headers   map[string][]string
		setup     func(*externalauth)
		server    *httptest.Server
		expectErr error
	}{
		{
			name: "Empty authorization key",
			headers: map[string][]string{
				DefaultAuthorizationHeader: {"Bearer "},
			},
			expectErr: errInvalidAuthHeader,
		},
		{
			name: "Wrong authorization scheme",
			headers: map[string][]string{
				DefaultAuthorizationHeader: {"Wrong token1"},
			},
			expectErr: errInvalidAuthHeader,
		},
		{
			name: "Multiple authorization headers",
			headers: map[string][]string{
				DefaultAuthorizationHeader: {"Bearer token1", "Bearer token2"},
			},
			expectErr: errInvalidAuthHeader,
		},
		{
			name: "Missing authorization headers",
			headers: map[string][]string{
				DefaultAuthorizationHeader: {""},
			},
			expectErr: errInvalidAuthHeader,
		},
		{
			name: "HTTP request creation fails",
			headers: map[string][]string{
				DefaultAuthorizationHeader: {"Bearer token"},
			},
			setup: func(e *externalauth) {
				e.endpoint = "http://[::1]:NamedPort" // Invalid URL to force http.NewRequest to fail
			},
			expectErr: errSendRequest,
		},
		{
			name:      "No authorization header",
			headers:   map[string][]string{},
			expectErr: errNoAuthHeader,
		},
		{
			name: "Invalid authorization header",
			headers: map[string][]string{
				DefaultAuthorizationHeader: {"Wrong"},
			},
			expectErr: errInvalidAuthHeader,
		},
		{
			name: "Valid headers",
			headers: map[string][]string{
				DefaultAuthorizationHeader: {"Bearer token"},
			},
			setup: func(e *externalauth) {
				e.tokenCache.addToken("default_token", true)
			},
			expectErr: nil,
		},
		{
			name: "Valid headers, token exists and is cached as invalid",
			headers: map[string][]string{
				DefaultAuthorizationHeader: {"Bearer token"},
			},
			setup: func(e *externalauth) {
				e.tokenCache.addToken("token", false)
			},
			expectErr: errUnauthorized,
		},
		{
			name: "Valid headers, token exists and is valid",
			headers: map[string][]string{
				DefaultAuthorizationHeader: {"Bearer token"},
			},
			setup: func(e *externalauth) {
				e.tokenCache.addToken("token", true)
			},
			expectErr: nil,
		},
		{
			name: "Valid headers, token exists and is cached as invalid",
			headers: map[string][]string{
				DefaultAuthorizationHeader: {"Bearer token"},
			},
			setup: func(e *externalauth) {
				e.tokenCache.addToken("token", false)
			},
			expectErr: errUnauthorized,
		},
		{
			name: "Valid headers, token does not exist in cache",
			headers: map[string][]string{
				DefaultAuthorizationHeader: {"Bearer token"},
			},
			expectErr: nil,
		},
		{
			name: "Wait: Valid headers, token does not exist in cache",
			headers: map[string][]string{
				DefaultAuthorizationHeader: {"Bearer token"},
			},
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(2 * time.Second)
			})),
			expectErr: errSendRequest,
		},
		{
			name: "Wait: Valid headers, token exists but is expired",
			headers: map[string][]string{
				DefaultAuthorizationHeader: {"Bearer token"},
			},
			setup: func(e *externalauth) {
				e.tokenCache.addToken("token", true)
				e.tokenCache.setTokenExpiry("token", time.Now().Add(-2*time.Hour))
			},
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(2 * time.Second)
			})),
			expectErr: errSendRequest,
		},
		{
			name: "Valid headers, token exists but is expired and valid",
			headers: map[string][]string{
				DefaultAuthorizationHeader: {"Bearer token"},
			},
			setup: func(e *externalauth) {
				e.tokenCache.addToken("token", true)
				e.tokenCache.setTokenExpiry("oken", time.Now().Add(-2*time.Hour))
			},
			expectErr: nil,
		},
		{
			name: "Valid headers, token exists but is expired and invalid",
			headers: map[string][]string{
				DefaultAuthorizationHeader: {"Bearer token"},
			},
			setup: func(e *externalauth) {
				e.tokenCache.addToken("token", false)
				e.tokenCache.setTokenExpiry("token", time.Now().Add(-2*time.Hour))
			},
			expectErr: nil,
		},
		{
			name: "Invalid headers, token does not exist in cache",
			headers: map[string][]string{
				DefaultAuthorizationHeader: {"Bearer wrongtoken"},
			},
			expectErr: errUnauthorized,
		},
		{
			name: "Invalid headers, token is cached as invalid",
			headers: map[string][]string{
				DefaultAuthorizationHeader: {"Bearer wrongtoken"},
			},
			setup: func(e *externalauth) {
				e.tokenCache.addToken("wrongtoken", false)
			},
			expectErr: errUnauthorized,
		},
		{
			name: "Invalid headers, token is cached as invalid, and expired",
			headers: map[string][]string{
				DefaultAuthorizationHeader: {"Bearer wrongtoken"},
			},
			setup: func(e *externalauth) {
				e.tokenCache.addToken("wrongtoken", false)
				e.tokenCache.setTokenExpiry("wrongtoken", time.Now().Add(-2*time.Hour))
			},
			expectErr: errUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.server == nil {
				tt.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != "/test" {
						w.WriteHeader(http.StatusUnauthorized)
					} else if r.Header.Get(DefaultAuthorizationHeader) != "Bearer token" {
						w.WriteHeader(http.StatusUnauthorized)
					} else {
						w.WriteHeader(http.StatusOK)
					}
				}))
			}
			defer tt.server.Close()

			telemetry := component.TelemetrySettings{
				Logger:        zap.NewNop(),
				MeterProvider: noop.NewMeterProvider(),
			}

			metrics, err := newAuthMetrics(telemetry.MeterProvider.Meter("externalauth"))
			assert.NoError(t, err)

			e := &externalauth{
				tokenCache:      newTokenCache(),
				header:          DefaultAuthorizationHeader,
				scheme:          "Bearer",
				endpoint:        tt.server.URL + "/test",
				expectedCodes:   []int{200},
				method:          "GET",
				refreshInterval: "1h",
				client: &http.Client{
					Timeout: 1 * time.Second,
				},
				telemetry: telemetry,
				metrics:   metrics,
			}
			if tt.setup != nil {
				tt.setup(e)
			}
			_, err = e.Authenticate(context.Background(), tt.headers)
			if tt.expectErr != nil {
				assert.Equal(t, tt.expectErr, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
func TestStart(t *testing.T) {
	tests := []struct {
		name          string
		httpTimeout   time.Duration
		expectedError error
	}{
		{
			name:          "Valid start",
			httpTimeout:   2 * time.Second,
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &externalauth{
				telemetry:         componenttest.NewNopTelemetrySettings(),
				httpClientTimeout: tt.httpTimeout,
				endpoint:          "http://example.com",
			}
			err := e.Start(context.Background(), nil)
			assert.Equal(t, tt.expectedError, err)
			assert.NotNil(t, e.client)
			assert.Equal(t, tt.httpTimeout, e.client.Timeout)
		})
	}
}
func TestShutdown(t *testing.T) {
	tests := []struct {
		name          string
		expectedError error
	}{
		{
			name:          "Valid shutdown",
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &externalauth{
				telemetry: componenttest.NewNopTelemetrySettings(),
			}
			err := e.Shutdown(context.Background())
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestHeaderBasedEndpointMapping(t *testing.T) {
	tests := []struct {
		name           string
		headers        map[string][]string
		expectedURL    string
		serverResponse int
		expectErr      error
	}{
		{
			name: "stage destination",
			headers: map[string][]string{
				"Authorization": {"Bearer token"},
				"Destination":   {"stage"},
			},
			expectedURL:    "/stage-auth",
			serverResponse: http.StatusOK,
			expectErr:      nil,
		},
		{
			name: "prod destination",
			headers: map[string][]string{
				"Authorization": {"Bearer token"},
				"Destination":   {"prod"},
			},
			expectedURL:    "/prod-auth",
			serverResponse: http.StatusOK,
			expectErr:      nil,
		},
		{
			name: "dev destination",
			headers: map[string][]string{
				"Authorization": {"Bearer token"},
				"Destination":   {"dev"},
			},
			expectedURL:    "/dev-auth",
			serverResponse: http.StatusOK,
			expectErr:      nil,
		},
		{
			name: "case insensitive destination header",
			headers: map[string][]string{
				"Authorization": {"Bearer token"},
				"destination":   {"stage"},
			},
			expectedURL:    "/stage-auth",
			serverResponse: http.StatusOK,
			expectErr:      nil,
		},
		{
			name: "unknown destination falls back to default",
			headers: map[string][]string{
				"Authorization": {"Bearer token"},
				"Destination":   {"unknown"},
			},
			expectedURL:    "/default-auth",
			serverResponse: http.StatusOK,
			expectErr:      nil,
		},
		{
			name: "no destination header falls back to default",
			headers: map[string][]string{
				"Authorization": {"Bearer token"},
			},
			expectedURL:    "/default-auth",
			serverResponse: http.StatusOK,
			expectErr:      nil,
		},
		{
			name: "stage destination with auth failure",
			headers: map[string][]string{
				"Authorization": {"Bearer invalid"},
				"Destination":   {"stage"},
			},
			expectedURL:    "/stage-auth",
			serverResponse: http.StatusUnauthorized,
			expectErr:      errUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calledURL string

			// Create a single test server that tracks which URL was called
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calledURL = r.URL.Path

				if r.Header.Get("Authorization") != "Bearer token" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				w.WriteHeader(tt.serverResponse)
			}))
			defer server.Close()

			telemetry := component.TelemetrySettings{
				Logger:        zap.NewNop(),
				MeterProvider: noop.NewMeterProvider(),
			}

			metrics, err := newAuthMetrics(telemetry.MeterProvider.Meter("externalauth"))
			assert.NoError(t, err)

			e := &externalauth{
				tokenCache: newTokenCache(),
				header:     "Authorization",
				scheme:     "Bearer",
				endpoint:   server.URL + "/default-auth",
				headerEndpointMapping: map[string]map[string]string{
					"Destination": {
						"stage": server.URL + "/stage-auth",
						"prod":  server.URL + "/prod-auth",
						"dev":   server.URL + "/dev-auth",
					},
				},
				expectedCodes:   []int{200},
				method:          "GET",
				refreshInterval: "1h",
				client: &http.Client{
					Timeout: 1 * time.Second,
				},
				telemetry: telemetry,
				metrics:   metrics,
			}

			_, err = e.Authenticate(context.Background(), tt.headers)

			// Verify the correct endpoint was called
			assert.Equal(t, tt.expectedURL, calledURL)

			if tt.expectErr != nil {
				assert.Equal(t, tt.expectErr, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
