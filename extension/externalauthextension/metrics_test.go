package externalauthextension

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.uber.org/zap"
)

func TestMetrics(t *testing.T) {
	// Create a manual reader for testing
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("externalauth")

	// Create metrics
	metrics, err := newAuthMetrics(meter)
	require.NoError(t, err)
	require.NotNil(t, metrics)

	// Test context and attributes
	ctx := context.Background()
	user := "testuser"
	code := 200
	telemetryType := "metrics"

	// Test auth attempts with user (basic_auth format)
	metrics.authAttempts.Add(ctx, 1, metric.WithAttributes(
		attribute.String("user", user),
		attribute.String("telemetry_type", telemetryType),
	))

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(ctx, &rm))
	require.Len(t, rm.ScopeMetrics, 1)
	require.Greater(t, len(rm.ScopeMetrics[0].Metrics), 0)

	// Find and verify auth_attempts_total metric
	found := false
	for _, m := range rm.ScopeMetrics[0].Metrics {
		if m.Name == "externalauth_attempts_total" {
			found = true
			require.Equal(t, "Total number of authentication attempts", m.Description)
			sum, ok := m.Data.(metricdata.Sum[int64])
			require.True(t, ok)
			require.Equal(t, 1, len(sum.DataPoints))
			require.Equal(t, int64(1), sum.DataPoints[0].Value)
			metricdatatest.AssertHasAttributes(t, m,
				attribute.String("user", user),
				attribute.String("telemetry_type", telemetryType))
			break
		}
	}
	require.True(t, found, "externalauth_attempts_total metric not found")

	// Test auth successes
	metrics.authSuccesses.Add(ctx, 1, metric.WithAttributes(
		attribute.String("user", user),
		attribute.Int("code", code),
		attribute.String("telemetry_type", telemetryType),
	))
	require.NoError(t, reader.Collect(ctx, &rm))

	// Find and verify auth_successes_total metric
	found = false
	for _, m := range rm.ScopeMetrics[0].Metrics {
		if m.Name == "externalauth_successes_total" {
			found = true
			require.Equal(t, "Total number of successful authentications", m.Description)
			sum, ok := m.Data.(metricdata.Sum[int64])
			require.True(t, ok)
			require.Equal(t, 1, len(sum.DataPoints))
			require.Equal(t, int64(1), sum.DataPoints[0].Value)
			metricdatatest.AssertHasAttributes(t, m,
				attribute.String("user", user),
				attribute.Int("code", code),
				attribute.String("telemetry_type", telemetryType))
			break
		}
	}
	require.True(t, found, "externalauth_successes_total metric not found")

	// Test auth failures
	metrics.authFailures.Add(ctx, 1, metric.WithAttributes(
		attribute.String("user", user),
		attribute.Int("code", 401),
		attribute.String("telemetry_type", telemetryType),
	))
	require.NoError(t, reader.Collect(ctx, &rm))

	// Find and verify auth_failures_total metric
	found = false
	for _, m := range rm.ScopeMetrics[0].Metrics {
		if m.Name == "externalauth_failures_total" {
			found = true
			require.Equal(t, "Total number of failed authentications", m.Description)
			sum, ok := m.Data.(metricdata.Sum[int64])
			require.True(t, ok)
			require.Equal(t, 1, len(sum.DataPoints))
			require.Equal(t, int64(1), sum.DataPoints[0].Value)
			metricdatatest.AssertHasAttributes(t, m,
				attribute.String("user", user),
				attribute.Int("code", 401),
				attribute.String("telemetry_type", telemetryType))
			break
		}
	}
	require.True(t, found, "externalauth_failures_total metric not found")

	// Test auth latency
	metrics.authLatency.Record(ctx, 0.1, metric.WithAttributes(
		attribute.String("user", user),
		attribute.String("telemetry_type", telemetryType),
	))
	require.NoError(t, reader.Collect(ctx, &rm))

	// Find and verify auth_latency_seconds metric
	found = false
	for _, m := range rm.ScopeMetrics[0].Metrics {
		if m.Name == "externalauth_latency_seconds" {
			found = true
			require.Equal(t, "Authentication latency in seconds", m.Description)
			hist, ok := m.Data.(metricdata.Histogram[float64])
			require.True(t, ok)
			require.Equal(t, 1, len(hist.DataPoints))
			require.Equal(t, 0.1, hist.DataPoints[0].Sum)
			metricdatatest.AssertHasAttributes(t, m,
				attribute.String("user", user),
				attribute.String("telemetry_type", telemetryType))
			break
		}
	}
	require.True(t, found, "externalauth_latency_seconds metric not found")

	// Test cache hits
	metrics.cacheHits.Add(ctx, 1, metric.WithAttributes(
		attribute.String("user", user),
		attribute.String("telemetry_type", telemetryType),
	))
	require.NoError(t, reader.Collect(ctx, &rm))

	// Find and verify auth_cache_hits_total metric
	found = false
	for _, m := range rm.ScopeMetrics[0].Metrics {
		if m.Name == "externalauth_cache_hits_total" {
			found = true
			require.Equal(t, "Total number of cache hits", m.Description)
			sum, ok := m.Data.(metricdata.Sum[int64])
			require.True(t, ok)
			require.Equal(t, 1, len(sum.DataPoints))
			require.Equal(t, int64(1), sum.DataPoints[0].Value)
			metricdatatest.AssertHasAttributes(t, m,
				attribute.String("user", user),
				attribute.String("telemetry_type", telemetryType))
			break
		}
	}
	require.True(t, found, "externalauth_cache_hits_total metric not found")

	// Test cache misses
	metrics.cacheMisses.Add(ctx, 1, metric.WithAttributes(
		attribute.String("user", user),
		attribute.String("telemetry_type", telemetryType),
	))
	require.NoError(t, reader.Collect(ctx, &rm))

	// Find and verify auth_cache_misses_total metric
	found = false
	for _, m := range rm.ScopeMetrics[0].Metrics {
		if m.Name == "externalauth_cache_misses_total" {
			found = true
			require.Equal(t, "Total number of cache misses", m.Description)
			sum, ok := m.Data.(metricdata.Sum[int64])
			require.True(t, ok)
			require.Equal(t, int64(1), sum.DataPoints[0].Value)
			metricdatatest.AssertHasAttributes(t, m,
				attribute.String("user", user),
				attribute.String("telemetry_type", telemetryType))
			break
		}
	}
	require.True(t, found, "externalauth_cache_misses_total metric not found")

	// Test remote auth calls
	metrics.remoteAuthCalls.Add(ctx, 1, metric.WithAttributes(
		attribute.String("user", user),
		attribute.String("telemetry_type", telemetryType),
	))
	require.NoError(t, reader.Collect(ctx, &rm))

	// Find and verify auth_remote_calls_total metric
	found = false
	for _, m := range rm.ScopeMetrics[0].Metrics {
		if m.Name == "externalauth_remote_calls_total" {
			found = true
			require.Equal(t, "Total number of remote authentication calls", m.Description)
			sum, ok := m.Data.(metricdata.Sum[int64])
			require.True(t, ok)
			require.Equal(t, 1, len(sum.DataPoints))
			require.Equal(t, int64(1), sum.DataPoints[0].Value)
			metricdatatest.AssertHasAttributes(t, m,
				attribute.String("user", user),
				attribute.String("telemetry_type", telemetryType))
			break
		}
	}
	require.True(t, found, "externalauth_remote_calls_total metric not found")

	// Verify all metrics were recorded
	assert.NoError(t, provider.Shutdown(ctx))
}

func TestMetricsWithServer(t *testing.T) {
	// Create a test server that simulates different authentication scenarios
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(DefaultAuthorizationHeader) == "Bearer validtoken:pass" {
			w.WriteHeader(http.StatusOK)
			time.Sleep(100 * time.Millisecond) // Add latency for metrics
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer server.Close()

	tests := []struct {
		name           string
		tokenFormat    string
		headers        map[string][]string
		expectedError  error
		expectedUser   string
		expectedCode   int
		expectCacheHit bool
		expectUserAttr bool
	}{
		{
			name:        "Successful authentication with basic_auth",
			tokenFormat: "basic_auth",
			headers: map[string][]string{
				DefaultAuthorizationHeader: {"Bearer validtoken:pass"},
			},
			expectedError:  nil,
			expectedUser:   "validtoken",
			expectedCode:   http.StatusOK,
			expectCacheHit: false,
			expectUserAttr: true,
		},
		{
			name:        "Failed authentication with basic_auth",
			tokenFormat: "basic_auth",
			headers: map[string][]string{
				DefaultAuthorizationHeader: {"Bearer invalidtoken:pass"},
			},
			expectedError:  errUnauthorized,
			expectedUser:   "invalidtoken",
			expectedCode:   http.StatusUnauthorized,
			expectCacheHit: false,
			expectUserAttr: true,
		},
		{
			name:        "Successful authentication with raw format",
			tokenFormat: "raw",
			headers: map[string][]string{
				DefaultAuthorizationHeader: {"Bearer validtoken:pass"},
			},
			expectedError:  nil,
			expectedUser:   "",
			expectedCode:   http.StatusOK,
			expectCacheHit: false,
			expectUserAttr: false, // No user attribute for raw format
		},
		{
			name:        "Cache hit with valid token (basic_auth)",
			tokenFormat: "basic_auth",
			headers: map[string][]string{
				DefaultAuthorizationHeader: {"Bearer validtoken:pass"},
			},
			expectedError:  nil,
			expectedUser:   "validtoken",
			expectedCode:   http.StatusOK,
			expectCacheHit: true,
			expectUserAttr: true,
		},
		{
			name:        "Cache hit with valid token (raw format)",
			tokenFormat: "raw",
			headers: map[string][]string{
				DefaultAuthorizationHeader: {"Bearer validtoken:pass"},
			},
			expectedError:  nil,
			expectedUser:   "",
			expectedCode:   http.StatusOK,
			expectCacheHit: false,
			expectUserAttr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new reader and provider for each test case
			reader := sdkmetric.NewManualReader()
			provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

			// Create external auth instance
			cfg := &Config{
				Endpoint:      server.URL,
				TelemetryType: "metrics",
				TokenFormat:   tt.tokenFormat,
			}

			telemetry := component.TelemetrySettings{
				Logger:        zap.NewNop(),
				MeterProvider: provider,
			}

			e, err := newExternalAuth(cfg, telemetry)
			require.NoError(t, err)
			require.NotNil(t, e)

			ctx := context.Background()
			_, err = e.Authenticate(ctx, tt.headers)
			if tt.expectedError != nil {
				assert.Equal(t, tt.expectedError, err)
			} else {
				assert.NoError(t, err)
			}

			// Collect and verify metrics
			var rm metricdata.ResourceMetrics
			require.NoError(t, reader.Collect(ctx, &rm))
			require.Len(t, rm.ScopeMetrics, 1)

			// Helper function to find metric by name
			findMetric := func(name string) *metricdata.Metrics {
				for _, m := range rm.ScopeMetrics[0].Metrics {
					if m.Name == name {
						return &m
					}
				}
				return nil
			}

			// Helper function to check if metric has user attribute
			hasUserAttribute := func(m metricdata.Metrics) bool {
				switch data := m.Data.(type) {
				case metricdata.Sum[int64]:
					for _, dp := range data.DataPoints {
						if dp.Attributes.HasValue(attribute.Key("user")) {
							return true
						}
					}
				case metricdata.Histogram[float64]:
					for _, dp := range data.DataPoints {
						if dp.Attributes.HasValue(attribute.Key("user")) {
							return true
						}
					}
				}
				return false
			}

			// Verify auth attempts
			if m := findMetric("externalauth_attempts_total"); m != nil {
				sum, ok := m.Data.(metricdata.Sum[int64])
				require.True(t, ok)
				require.Equal(t, int64(1), sum.DataPoints[0].Value)
				metricdatatest.AssertHasAttributes(t, *m,
					attribute.String("telemetry_type", "metrics"))

				// Check user attribute based on token format
				if tt.expectUserAttr {
					metricdatatest.AssertHasAttributes(t, *m, attribute.String("user", tt.expectedUser))
				} else {
					assert.False(t, hasUserAttribute(*m), "User attribute should not be present for raw format")
				}
			}

			// Verify auth successes/failures
			if tt.expectedError == nil {
				if m := findMetric("externalauth_successes_total"); m != nil {
					sum, ok := m.Data.(metricdata.Sum[int64])
					require.True(t, ok)
					require.Equal(t, int64(1), sum.DataPoints[0].Value)
					metricdatatest.AssertHasAttributes(t, *m,
						attribute.Int("code", tt.expectedCode),
						attribute.String("telemetry_type", "metrics"))

					// Check user attribute based on token format
					if tt.expectUserAttr {
						metricdatatest.AssertHasAttributes(t, *m, attribute.String("user", tt.expectedUser))
					} else {
						assert.False(t, hasUserAttribute(*m), "User attribute should not be present for raw format")
					}
				}
			} else {
				if m := findMetric("externalauth_failures_total"); m != nil {
					sum, ok := m.Data.(metricdata.Sum[int64])
					require.True(t, ok)
					require.Equal(t, int64(1), sum.DataPoints[0].Value)
					metricdatatest.AssertHasAttributes(t, *m,
						attribute.Int("code", tt.expectedCode),
						attribute.String("telemetry_type", "metrics"))

					// Check user attribute based on token format
					if tt.expectUserAttr {
						metricdatatest.AssertHasAttributes(t, *m, attribute.String("user", tt.expectedUser))
					} else {
						assert.False(t, hasUserAttribute(*m), "User attribute should not be present for raw format")
					}
				}
			}

			// Verify latency
			if m := findMetric("externalauth_latency_seconds"); m != nil {
				hist, ok := m.Data.(metricdata.Histogram[float64])
				require.True(t, ok)
				require.Equal(t, 1, len(hist.DataPoints))
				require.Greater(t, hist.DataPoints[0].Sum, float64(0))
				metricdatatest.AssertHasAttributes(t, *m,
					attribute.String("telemetry_type", "metrics"))

				// Check user attribute based on token format
				if tt.expectUserAttr {
					metricdatatest.AssertHasAttributes(t, *m, attribute.String("user", tt.expectedUser))
				} else {
					assert.False(t, hasUserAttribute(*m), "User attribute should not be present for raw format")
				}
			}

			// Verify cache hits/misses
			if tt.expectCacheHit {
				if m := findMetric("externalauth_cache_hits_total"); m != nil {
					sum, ok := m.Data.(metricdata.Sum[int64])
					require.True(t, ok)
					require.Equal(t, int64(1), sum.DataPoints[0].Value)
					metricdatatest.AssertHasAttributes(t, *m,
						attribute.String("telemetry_type", "metrics"))

					// Check user attribute based on token format
					if tt.expectUserAttr {
						metricdatatest.AssertHasAttributes(t, *m, attribute.String("user", tt.expectedUser))
					} else {
						assert.False(t, hasUserAttribute(*m), "User attribute should not be present for raw format")
					}
				}
			} else {
				if m := findMetric("externalauth_cache_misses_total"); m != nil {
					sum, ok := m.Data.(metricdata.Sum[int64])
					require.True(t, ok)
					require.Equal(t, int64(1), sum.DataPoints[0].Value)
					metricdatatest.AssertHasAttributes(t, *m,
						attribute.String("telemetry_type", "metrics"))

					// Check user attribute based on token format
					if tt.expectUserAttr {
						metricdatatest.AssertHasAttributes(t, *m, attribute.String("user", tt.expectedUser))
					} else {
						assert.False(t, hasUserAttribute(*m), "User attribute should not be present for raw format")
					}
				}
			}

			// Verify remote calls (should be 1 for non-cache hits)
			if !tt.expectCacheHit {
				if m := findMetric("externalauth_remote_calls_total"); m != nil {
					sum, ok := m.Data.(metricdata.Sum[int64])
					require.True(t, ok)
					require.Equal(t, int64(1), sum.DataPoints[0].Value)
					metricdatatest.AssertHasAttributes(t, *m,
						attribute.String("telemetry_type", "metrics"))

					// Check user attribute based on token format
					if tt.expectUserAttr {
						metricdatatest.AssertHasAttributes(t, *m, attribute.String("user", tt.expectedUser))
					} else {
						assert.False(t, hasUserAttribute(*m), "User attribute should not be present for raw format")
					}
				}
			}
		})
	}
}
