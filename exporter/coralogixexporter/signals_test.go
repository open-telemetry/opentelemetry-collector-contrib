// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
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
	exp.EnableRateLimit(dropReasonAuth, "Unauthenticated")
	exp.EnableRateLimit(dropReasonAuth, "Unauthenticated")

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

	exp.EnableRateLimit(dropReasonAuth, "Unauthenticated")
	exp.EnableRateLimit(dropReasonAuth, "Unauthenticated")

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
	exp.EnableRateLimit(dropReasonAuth, "Unauthenticated")
	exp.EnableRateLimit(dropReasonAuth, "Unauthenticated")

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
			err: func() error {
				st := status.New(codes.ResourceExhausted, "resource exhausted")
				st, _ = st.WithDetails(&errdetails.RetryInfo{
					RetryDelay: durationpb.New(time.Second),
				})
				return st.Err()
			}(),
			expected: exporterhelper.NewThrottleRetry(status.Error(codes.ResourceExhausted, "resource exhausted"), time.Second),
		},
		{
			name:     "unauthenticated wraps as authentication failed",
			err:      status.Error(codes.Unauthenticated, "bad key"),
			expected: consumererror.NewPermanent(errors.New("authentication failed (private key rotated or invalid)")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp := &signalExporter{
				config: &Config{
					Protocol: grpcProtocol,
					RateLimiter: RateLimiterConfig{
						Enabled:   true,
						Threshold: 10,
						Duration:  time.Minute,
					},
				},
				settings: componenttest.NewNopTelemetrySettings(),
				rateError: rateError{
					enabled:   true,
					threshold: 10,
					duration:  time.Minute,
				},
			}
			err := exp.processError(tt.err)
			if tt.expected == nil {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				if consumererror.IsPermanent(err) {
					assert.True(t, consumererror.IsPermanent(err))
					if tt.name == "unauthenticated wraps as authentication failed" {
						assert.Contains(t, err.Error(), "authentication failed (private key rotated or invalid)")
						assert.NotContains(t, err.Error(), "rate limit exceeded")
					}
				} else {
					assert.Contains(t, err.Error(), tt.expected.Error())
				}
			}
		})
	}
}

func TestFakeBadSituations_LogMessages(t *testing.T) {
	cases := []struct {
		name            string
		protocol        string
		threshold       int
		inject          func(*signalExporter)
		wantOpenMsg     string
		wantStatus      string
		wantReason      string
		wantHint        bool
		wantErrSubstr   string
		forbidErrSubstr string
	}{
		{
			name:      "grpc_unauthenticated",
			protocol:  grpcProtocol,
			threshold: 2,
			inject: func(e *signalExporter) {
				err := status.Error(codes.Unauthenticated, "bad key")
				for range e.rateError.threshold {
					_ = e.processError(err)
				}
			},
			wantOpenMsg:     "Authentication with Coralogix failed; dropping data until the key is fixed",
			wantStatus:      "Unauthenticated",
			wantReason:      "authentication",
			wantHint:        true,
			wantErrSubstr:   "authentication failed (private key rotated or invalid)",
			forbidErrSubstr: "rate limit exceeded",
		},
		{
			name:      "grpc_permission_denied",
			protocol:  grpcProtocol,
			threshold: 2,
			inject: func(e *signalExporter) {
				err := status.Error(codes.PermissionDenied, "no access")
				for range e.rateError.threshold {
					_ = e.processError(err)
				}
			},
			wantOpenMsg:     "Authentication with Coralogix failed; dropping data until the key is fixed",
			wantStatus:      "PermissionDenied",
			wantReason:      "authentication",
			wantHint:        true,
			wantErrSubstr:   "authentication failed (private key rotated or invalid)",
			forbidErrSubstr: "rate limit exceeded",
		},
		{
			name:      "http_401",
			protocol:  httpProtocol,
			threshold: 2,
			inject: func(e *signalExporter) {
				err := &httpError{StatusCode: http.StatusUnauthorized, Message: "unauthorized"}
				for range e.rateError.threshold {
					_ = e.processError(err)
				}
			},
			wantOpenMsg:     "Authentication with Coralogix failed; dropping data until the key is fixed",
			wantStatus:      "401 Unauthorized",
			wantReason:      "authentication",
			wantHint:        true,
			wantErrSubstr:   "authentication failed (private key rotated or invalid)",
			forbidErrSubstr: "rate limit exceeded",
		},
		{
			name:      "http_403",
			protocol:  httpProtocol,
			threshold: 2,
			inject: func(e *signalExporter) {
				err := &httpError{StatusCode: http.StatusForbidden, Message: "forbidden"}
				for range e.rateError.threshold {
					_ = e.processError(err)
				}
			},
			wantOpenMsg:     "Authentication with Coralogix failed; dropping data until the key is fixed",
			wantStatus:      "403 Forbidden",
			wantReason:      "authentication",
			wantHint:        true,
			wantErrSubstr:   "authentication failed (private key rotated or invalid)",
			forbidErrSubstr: "rate limit exceeded",
		},
		{
			name:      "grpc_quota_resource_exhausted",
			protocol:  grpcProtocol,
			threshold: 2,
			inject: func(e *signalExporter) {
				err := status.Error(codes.ResourceExhausted, "quota")
				for range e.rateError.threshold {
					_ = e.processError(err)
				}
			},
			wantOpenMsg:     "Coralogix quota exceeded; dropping data",
			wantStatus:      "ResourceExhausted",
			wantReason:      "quota",
			wantHint:        false,
			wantErrSubstr:   "quota exceeded",
			forbidErrSubstr: "rate limit exceeded",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			core, logs := observer.New(zapcore.InfoLevel)
			settings := componenttest.NewNopTelemetrySettings()
			settings.Logger = zap.New(core)

			exp := &signalExporter{
				config: &Config{
					Protocol: tc.protocol,
					RateLimiter: RateLimiterConfig{
						Enabled:   true,
						Threshold: tc.threshold,
						Duration:  time.Minute,
					},
				},
				settings: settings,
				rateError: rateError{
					enabled:   true,
					threshold: tc.threshold,
					duration:  time.Minute,
				},
			}

			tc.inject(exp)
			require.False(t, exp.canSend())

			windowErr := exp.rateError.GetError()
			require.Error(t, windowErr)
			assert.Contains(t, windowErr.Error(), tc.wantErrSubstr)
			assert.NotContains(t, windowErr.Error(), tc.forbidErrSubstr)

			errorLogs := logs.FilterLevelExact(zapcore.ErrorLevel).All()
			require.Len(t, errorLogs, 1)
			assert.Equal(t, tc.wantOpenMsg, errorLogs[0].Message)
			fields := errorLogs[0].ContextMap()
			assert.Equal(t, tc.wantReason, fields["reason"])
			assert.Equal(t, tc.wantStatus, fields["status"])
			if tc.wantHint {
				assert.Equal(t, "check private_key and restart the collector after a key rotation", fields["hint"])
			} else {
				_, hasHint := fields["hint"]
				assert.False(t, hasHint)
			}

			t.Logf("open_log msg=%q fields=%v", errorLogs[0].Message, fields)
			t.Logf("window_err=%q", windowErr.Error())

			past := time.Now().Add(-2 * time.Minute)
			exp.rateError.timestamp.Store(&past)
			require.True(t, exp.canSend())
			closeLogs := logs.FilterMessage("Drop window closed; resuming sends to Coralogix").All()
			require.Len(t, closeLogs, 1)
			assert.Equal(t, tc.wantReason, closeLogs[0].ContextMap()["reason"])
			t.Logf("close_log msg=%q fields=%v", closeLogs[0].Message, closeLogs[0].ContextMap())
		})
	}
}

func TestProcessError_DefaultProtocolIsGRPC(t *testing.T) {
	exp := &signalExporter{
		config: &Config{
			Protocol: "", // collector default when protocol is omitted
			RateLimiter: RateLimiterConfig{
				Enabled:   true,
				Threshold: 1,
				Duration:  time.Minute,
			},
		},
		settings: componenttest.NewNopTelemetrySettings(),
		rateError: rateError{
			enabled:   true,
			threshold: 1,
			duration:  time.Minute,
		},
	}

	err := exp.processError(status.Error(codes.Unauthenticated, "bad key"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication failed (private key rotated or invalid)")
	assert.True(t, exp.rateError.isRateLimited(), "empty protocol must still open the rate limiter window")
}

func TestProcessError_AuthOpensDropWindowAndLogsOnce(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	settings := componenttest.NewNopTelemetrySettings()
	settings.Logger = zap.New(core)

	exp := &signalExporter{
		config: &Config{
			Protocol: grpcProtocol,
			RateLimiter: RateLimiterConfig{
				Enabled:   true,
				Threshold: 3,
				Duration:  time.Minute,
			},
		},
		settings: settings,
		rateError: rateError{
			enabled:   true,
			threshold: 3,
			duration:  time.Minute,
		},
	}

	authErr := status.Error(codes.Unauthenticated, "bad key")
	for range exp.rateError.threshold {
		err := exp.processError(authErr)
		require.Error(t, err)
		assert.True(t, consumererror.IsPermanent(err))
		assert.Contains(t, err.Error(), "authentication failed (private key rotated or invalid)")
		assert.NotContains(t, err.Error(), "rate limit exceeded")
	}

	assert.False(t, exp.canSend())
	windowErr := exp.rateError.GetError()
	require.Error(t, windowErr)
	assert.Contains(t, windowErr.Error(), "authentication failed")
	assert.NotContains(t, windowErr.Error(), "rate limit")

	errorLogs := logs.FilterLevelExact(zapcore.ErrorLevel).All()
	require.Len(t, errorLogs, 1)
	assert.Equal(t, "Authentication with Coralogix failed; dropping data until the key is fixed", errorLogs[0].Message)
	fields := errorLogs[0].ContextMap()
	assert.Equal(t, "authentication", fields["reason"])
	assert.Equal(t, "Unauthenticated", fields["status"])
	assert.Equal(t, "check private_key and restart the collector after a key rotation", fields["hint"])
}

func TestProcessError_HTTPAuthOpensDropWindowAndLogsOnce(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	settings := componenttest.NewNopTelemetrySettings()
	settings.Logger = zap.New(core)

	exp := &signalExporter{
		config: &Config{
			Protocol: httpProtocol,
			RateLimiter: RateLimiterConfig{
				Enabled:   true,
				Threshold: 2,
				Duration:  time.Minute,
			},
		},
		settings: settings,
		rateError: rateError{
			enabled:   true,
			threshold: 2,
			duration:  time.Minute,
		},
	}

	authErr := &httpError{
		StatusCode: http.StatusUnauthorized,
		Message:    "unauthorized",
	}
	for range exp.rateError.threshold {
		err := exp.processError(authErr)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "authentication failed (private key rotated or invalid)")
		assert.NotContains(t, err.Error(), "rate limit exceeded")
	}

	assert.False(t, exp.canSend())
	errorLogs := logs.FilterLevelExact(zapcore.ErrorLevel).All()
	require.Len(t, errorLogs, 1)
	assert.Equal(t, "Authentication with Coralogix failed; dropping data until the key is fixed", errorLogs[0].Message)
	fields := errorLogs[0].ContextMap()
	assert.Equal(t, "authentication", fields["reason"])
	assert.Equal(t, "401 Unauthorized", fields["status"])
}

func TestSignalExporter_CanSend_LogsClose(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	settings := component.TelemetrySettings{Logger: zap.New(core)}

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
	exp.settings = settings

	exp.EnableRateLimit(dropReasonAuth, "Unauthenticated")
	assert.False(t, exp.canSend())

	past := time.Now().Add(-2 * time.Minute)
	exp.rateError.timestamp.Store(&past)

	assert.True(t, exp.canSend())
	infoLogs := logs.FilterLevelExact(zapcore.InfoLevel).All()
	require.Len(t, infoLogs, 1)
	assert.Equal(t, "Drop window closed; resuming sends to Coralogix", infoLogs[0].Message)
	assert.Equal(t, "authentication", infoLogs[0].ContextMap()["reason"])
}

func TestSignalExporter_AuthorizationHeader(t *testing.T) {
	privateKey := "test-private-key"
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: configopaque.String(privateKey),
	}

	exp, err := newSignalExporter(cfg, exportertest.NewNopSettings(exportertest.NopType), "", nil)
	require.NoError(t, err)

	wrapper := &signalConfigWrapper{config: &cfg.Logs}
	err = exp.startSignalExporter(t.Context(), componenttest.NewNopHost(), wrapper)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, exp.shutdown(t.Context()))
	}()

	authHeader, ok := wrapper.config.ClientConfig.Headers.Get("Authorization")
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
				Headers: configopaque.MapList{
					{Name: "Custom-Header", Value: "custom-value"},
					{Name: "X-Test", Value: "test-value"},
				},
			},
		},
		{
			name: "traces",
			config: configgrpc.ClientConfig{
				Headers: configopaque.MapList{
					{Name: "Custom-Header", Value: "custom-value"},
					{Name: "X-Test", Value: "test-value"},
				},
			},
		},
		{
			name: "metrics",
			config: configgrpc.ClientConfig{
				Headers: configopaque.MapList{
					{Name: "Custom-Header", Value: "custom-value"},
					{Name: "X-Test", Value: "test-value"},
				},
			},
		},
		{
			name: "profiles",
			config: configgrpc.ClientConfig{
				Headers: configopaque.MapList{
					{Name: "Custom-Header", Value: "custom-value"},
					{Name: "X-Test", Value: "test-value"},
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

			headers := wrapper.config.ClientConfig.Headers
			require.Len(t, headers, 3)

			authHeader, ok := headers.Get("Authorization")
			require.True(t, ok)
			assert.Equal(t, configopaque.String("Bearer "+privateKey), authHeader)

			customHeader, ok := headers.Get("Custom-Header")
			require.True(t, ok)
			assert.Equal(t, configopaque.String("custom-value"), customHeader)

			testHeader, ok := headers.Get("X-Test")
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
				cfg.Logs.ClientConfig.Endpoint = tt.signalEndpoint
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
				cfg.Logs.ClientConfig.Endpoint = tt.signalEndpoint
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
