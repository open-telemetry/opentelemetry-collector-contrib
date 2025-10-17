// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestSignalExporter_CanSend_AfterRateLimitTimeout(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
		RateLimiter: RateLimiterConfig{
			Enabled:   true,
			Threshold: 1,
			Duration:  time.Minute,
		},
	}

	exp, err := newSignalExporter(cfg, exportertest.NewNopSettings(exportertest.NopType), "", nil)
	require.NoError(t, err)

	// Add two rate limit errors
	exp.EnableRateLimit()
	exp.EnableRateLimit()

	assert.False(t, exp.canSend())

	// Mock the time to be after the rate limit timeout (1 minute)
	now := time.Now().Add(-2 * time.Minute)
	exp.rateError.timestamp.Store(&now)

	assert.True(t, exp.canSend())
	assert.False(t, exp.rateError.isRateLimited())
}

func TestSignalExporter_CanSend_FeatureDisabled(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
		RateLimiter: RateLimiterConfig{
			Enabled:   false,
			Threshold: 1,
			Duration:  time.Minute,
		},
	}

	exp, err := newSignalExporter(cfg, exportertest.NewNopSettings(exportertest.NopType), "", nil)
	require.NoError(t, err)

	exp.EnableRateLimit()
	exp.EnableRateLimit()

	assert.True(t, exp.canSend())
}

func TestSignalExporter_CanSend_BeforeRateLimitTimeout(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
		RateLimiter: RateLimiterConfig{
			Enabled:   true,
			Threshold: 1,
			Duration:  time.Minute,
		},
	}

	exp, err := newSignalExporter(cfg, exportertest.NewNopSettings(exportertest.NopType), "", nil)
	require.NoError(t, err)

	// Add two rate limit errors
	exp.EnableRateLimit()
	exp.EnableRateLimit()

	// Mock the time to be before the rate limit timeout (30 seconds ago)
	now := time.Now().Add(-30 * time.Second)
	exp.rateError.timestamp.Store(&now)

	// Should not be able to send because we're still within the timeout period
	assert.False(t, exp.canSend())
	assert.True(t, exp.rateError.isRateLimited())
}

func TestProcessError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected error
	}{
		{
			name:     "nil error returns nil",
			err:      nil,
			expected: nil,
		},
		{
			name:     "non-gRPC error returns permanent error",
			err:      errors.New("some error"),
			expected: consumererror.NewPermanent(errors.New("some error")),
		},
		{
			name:     "OK status returns nil",
			err:      status.Error(codes.OK, "ok"),
			expected: nil,
		},
		{
			name:     "permanent error returns permanent error",
			err:      status.Error(codes.InvalidArgument, "invalid argument"),
			expected: consumererror.NewPermanent(status.Error(codes.InvalidArgument, "invalid argument")),
		},
		{
			name:     "retryable error returns original error",
			err:      status.Error(codes.Unavailable, "unavailable"),
			expected: status.Error(codes.Unavailable, "unavailable"),
		},
		{
			name: "throttled error returns throttle retry",
			err:  status.Error(codes.ResourceExhausted, "resource exhausted"),
			expected: func() error {
				st := status.New(codes.ResourceExhausted, "resource exhausted")
				st, _ = st.WithDetails(&errdetails.RetryInfo{
					RetryDelay: durationpb.New(time.Second),
				})
				return exporterhelper.NewThrottleRetry(st.Err(), time.Second)
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp := &signalExporter{
				config: &Config{
					Protocol: grpcProtocol,
				},
			}
			err := exp.processError(tt.err)
			if tt.expected == nil {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				if consumererror.IsPermanent(err) {
					assert.True(t, consumererror.IsPermanent(err))
				} else {
					assert.Contains(t, err.Error(), tt.expected.Error())
				}
			}
		})
	}
}

func TestSignalExporter_AuthorizationHeader(t *testing.T) {
	privateKey := "test-private-key"
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: configopaque.String(privateKey),
		Logs: TransportConfig{
			ClientConfig: configgrpc.ClientConfig{
				Headers: map[string]configopaque.String{},
			},
		},
	}

	exp, err := newSignalExporter(cfg, exportertest.NewNopSettings(exportertest.NopType), "", nil)
	require.NoError(t, err)

	wrapper := &signalConfigWrapper{config: &cfg.Logs}
	err = exp.startSignalExporter(t.Context(), componenttest.NewNopHost(), wrapper)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, exp.shutdown(t.Context()))
	}()

	authHeader, ok := wrapper.config.Headers["Authorization"]
	require.True(t, ok, "Authorization header should be present")
	assert.Equal(t, configopaque.String("Bearer "+privateKey), authHeader, "Authorization header should be in Bearer format")

	mdValue := exp.metadata.Get("Authorization")
	require.Len(t, mdValue, 1, "Authorization header should be present in metadata")
	assert.Equal(t, "Bearer "+privateKey, mdValue[0], "Authorization header in metadata should be in Bearer format")
}

func TestSignalExporter_CustomHeadersAndAuthorization(t *testing.T) {
	tests := []struct {
		name   string
		config configgrpc.ClientConfig
	}{
		{
			name: "logs",
			config: configgrpc.ClientConfig{
				Headers: map[string]configopaque.String{
					"Custom-Header": "custom-value",
					"X-Test":        "test-value",
				},
			},
		},
		{
			name: "traces",
			config: configgrpc.ClientConfig{
				Headers: map[string]configopaque.String{
					"Custom-Header": "custom-value",
					"X-Test":        "test-value",
				},
			},
		},
		{
			name: "metrics",
			config: configgrpc.ClientConfig{
				Headers: map[string]configopaque.String{
					"Custom-Header": "custom-value",
					"X-Test":        "test-value",
				},
			},
		},
		{
			name: "profiles",
			config: configgrpc.ClientConfig{
				Headers: map[string]configopaque.String{
					"Custom-Header": "custom-value",
					"X-Test":        "test-value",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			privateKey := "test-private-key"
			cfg := &Config{
				Domain:     "test.domain.com",
				PrivateKey: configopaque.String(privateKey),
			}

			switch tt.name {
			case "logs":
				cfg.Logs = TransportConfig{ClientConfig: tt.config}
			case "traces":
				cfg.Traces = TransportConfig{ClientConfig: tt.config}
			case "metrics":
				cfg.Metrics = TransportConfig{ClientConfig: tt.config}
			case "profiles":
				cfg.Profiles = tt.config
			}

			exp, err := newSignalExporter(cfg, exportertest.NewNopSettings(exportertest.NopType), "", nil)
			require.NoError(t, err)

			transportConfig := TransportConfig{ClientConfig: tt.config}
			wrapper := &signalConfigWrapper{config: &transportConfig}
			err = exp.startSignalExporter(t.Context(), componenttest.NewNopHost(), wrapper)
			require.NoError(t, err)
			defer func() {
				require.NoError(t, exp.shutdown(t.Context()))
			}()

			headers := wrapper.config.Headers
			require.Len(t, headers, 3)

			authHeader, ok := headers["Authorization"]
			require.True(t, ok)
			assert.Equal(t, configopaque.String("Bearer "+privateKey), authHeader)

			customHeader, ok := headers["Custom-Header"]
			require.True(t, ok)
			assert.Equal(t, configopaque.String("custom-value"), customHeader)

			testHeader, ok := headers["X-Test"]
			require.True(t, ok)
			assert.Equal(t, configopaque.String("test-value"), testHeader)

			mdAuth := exp.metadata.Get("Authorization")
			require.Len(t, mdAuth, 1)
			assert.Equal(t, "Bearer "+privateKey, mdAuth[0])

			mdCustom := exp.metadata.Get("Custom-Header")
			require.Len(t, mdCustom, 1)
			assert.Equal(t, "custom-value", mdCustom[0])

			mdTest := exp.metadata.Get("X-Test")
			require.Len(t, mdTest, 1)
			assert.Equal(t, "test-value", mdTest[0])
		})
	}
}

func TestSignalExporter_HTTPClientWithDomainAndSignalSettings(t *testing.T) {
	tests := []struct {
		name           string
		protocol       string
		domain         string
		domainProxy    string
		domainTimeout  time.Duration
		signalEndpoint string
		signalProxy    string
		signalTimeout  time.Duration
		expectError    bool
	}{
		{
			name:          "domain_settings_only",
			protocol:      "http",
			domain:        "coralogix.com",
			domainProxy:   "http://domain-proxy:8080",
			domainTimeout: 30 * time.Second,
		},
		{
			name:          "signal_settings_override_domain",
			protocol:      "http",
			domain:        "coralogix.com",
			domainProxy:   "http://domain-proxy:8080",
			domainTimeout: 30 * time.Second,
			signalProxy:   "http://signal-proxy:8080",
			signalTimeout: 60 * time.Second,
		},
		{
			name:           "signal_endpoint_uses_signal_settings",
			protocol:       "http",
			signalEndpoint: "ingress.coralogix.com:443",
			signalProxy:    "http://signal-proxy:8080",
			signalTimeout:  45 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Protocol:   tt.protocol,
				Domain:     tt.domain,
				PrivateKey: "test-key",
				AppName:    "test-app",
				DomainSettings: TransportConfig{
					ProxyURL: tt.domainProxy,
					Timeout:  tt.domainTimeout,
				},
				Logs: TransportConfig{
					ProxyURL: tt.signalProxy,
					Timeout:  tt.signalTimeout,
				},
			}

			if tt.signalEndpoint != "" {
				cfg.Logs.Endpoint = tt.signalEndpoint
			}

			exp, err := newLogsExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
			require.NoError(t, err)

			ctx := t.Context()
			host := componenttest.NewNopHost()

			signalCfg := &signalConfigWrapper{config: &cfg.Logs}
			err = exp.startSignalExporter(ctx, host, signalCfg)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, exp.clientHTTP)
				// Clean up HTTP client
				if exp.clientHTTP != nil {
					exp.clientHTTP.CloseIdleConnections()
				}
			}
		})
	}
}

func TestSignalExporter_GRPCClientWithDomainAndSignalSettings(t *testing.T) {
	tests := []struct {
		name              string
		domain            string
		domainCompression configcompression.Type
		signalEndpoint    string
		signalCompression configcompression.Type
		signalWriteBuffer int
		expectError       bool
	}{
		{
			name:              "domain_settings_only",
			domain:            "coralogix.com",
			domainCompression: configcompression.TypeGzip,
		},
		{
			name:              "signal_settings_override_domain",
			domain:            "coralogix.com",
			domainCompression: configcompression.TypeGzip,
			signalCompression: configcompression.TypeZstd,
			signalWriteBuffer: 1024 * 1024,
		},
		{
			name:              "signal_endpoint_uses_signal_settings",
			signalEndpoint:    "ingress.coralogix.com:443",
			signalCompression: configcompression.TypeSnappy,
			signalWriteBuffer: 512 * 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Protocol:   "grpc",
				Domain:     tt.domain,
				PrivateKey: "test-key",
				AppName:    "test-app",
				DomainSettings: TransportConfig{
					ClientConfig: configgrpc.ClientConfig{
						Compression: tt.domainCompression,
					},
				},
				Logs: TransportConfig{
					ClientConfig: configgrpc.ClientConfig{
						Compression:     tt.signalCompression,
						WriteBufferSize: tt.signalWriteBuffer,
					},
				},
			}

			if tt.signalEndpoint != "" {
				cfg.Logs.Endpoint = tt.signalEndpoint
			}

			exp, err := newLogsExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
			require.NoError(t, err)

			ctx := t.Context()
			host := componenttest.NewNopHost()

			signalCfg := &signalConfigWrapper{config: &cfg.Logs}
			err = exp.startSignalExporter(ctx, host, signalCfg)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, exp.clientConn)
				// Clean up gRPC connection
				if exp.clientConn != nil {
					_ = exp.clientConn.Close()
				}
			}
		})
	}
}
