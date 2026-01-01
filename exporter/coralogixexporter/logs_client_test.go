// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestNewLogsExporter(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		shouldError bool
	}{
		{
			name: "Valid domain config",
			cfg: &Config{
				Domain:     "test.domain.com",
				PrivateKey: "test-key",
			},
			shouldError: false,
		},
		{
			name: "Valid logs endpoint config",
			cfg: &Config{
				Logs: TransportConfig{
					ClientConfig: configgrpc.ClientConfig{
						Endpoint: "localhost:4317",
					},
				},
				PrivateKey: "test-key",
			},
			shouldError: false,
		},
		{
			name: "Missing both domain and endpoint",
			cfg: &Config{
				PrivateKey: "test-key",
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp, err := newLogsExporter(tt.cfg, exportertest.NewNopSettings(exportertest.NopType))
			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, exp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, exp)
			}
		})
	}
}

func TestLogsExporter_Start(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
	}

	exp, err := newLogsExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	err = exp.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	assert.NotNil(t, exp.clientConn)
	assert.NotNil(t, exp.grpcLogsExporter)
	_, ok := exp.config.Logs.Headers.Get("Authorization")
	assert.True(t, ok)

	// Test shutdown
	err = exp.shutdown(t.Context())
	require.NoError(t, err)
}

func TestLogsExporter_EnhanceContext(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
		Logs: TransportConfig{
			ClientConfig: configgrpc.ClientConfig{
				Headers: configopaque.MapList{
					{Name: "test-header", Value: "test-value"},
				},
			},
		},
	}

	exp, err := newLogsExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	ctx := t.Context()
	enhancedCtx := exp.enhanceContext(ctx)
	assert.NotEqual(t, ctx, enhancedCtx)
}

func TestLogsExporter_PushLogs(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
	}

	exp, err := newLogsExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	err = exp.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = exp.shutdown(t.Context())
		require.NoError(t, err)
	}()

	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs()
	rl := resourceLogs.AppendEmpty()

	resource := rl.Resource()
	resource.Attributes().PutStr("service.name", "test-service")

	err = exp.pushLogs(t.Context(), logs)
	assert.Error(t, err)
}

func TestLogsExporter_PushLogs_WhenCannotSend(t *testing.T) {
	tests := []struct {
		description string
		enabled     bool
	}{
		{
			description: "Rate limit exceeded config enabled",
			enabled:     true,
		},
		{
			description: "Rate limit exceeded config disabled",
			enabled:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			cfg := &Config{
				Domain:     "test.domain.com",
				PrivateKey: "test-key",
				RateLimiter: RateLimiterConfig{
					Enabled:   tt.enabled,
					Threshold: 1,
					Duration:  time.Minute,
				},
			}

			exp, err := newLogsExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
			require.NoError(t, err)

			err = exp.start(t.Context(), componenttest.NewNopHost())
			require.NoError(t, err)
			defer func() {
				err = exp.shutdown(t.Context())
				require.NoError(t, err)
			}()

			exp.EnableRateLimit()
			exp.EnableRateLimit()

			logs := plog.NewLogs()
			resourceLogs := logs.ResourceLogs()
			rl := resourceLogs.AppendEmpty()
			resource := rl.Resource()
			resource.Attributes().PutStr("service.name", "test-service")

			err = exp.pushLogs(t.Context(), logs)
			assert.Error(t, err)
			if tt.enabled {
				assert.Contains(t, err.Error(), "rate limit exceeded")
			} else {
				assert.Contains(t, err.Error(), "no such host")
			}
		})
	}
}

type mockLogsServer struct {
	plogotlp.UnimplementedGRPCServer
	recvCount      int
	partialSuccess *plogotlp.ExportPartialSuccess
	t              testing.TB
}

func (m *mockLogsServer) Export(ctx context.Context, req plogotlp.ExportRequest) (plogotlp.ExportResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		m.t.Error("No metadata found in context")
		return plogotlp.NewExportResponse(), errors.New("no metadata found")
	}

	authHeader := md.Get("authorization")
	if len(authHeader) == 0 {
		m.t.Error("No Authorization header found")
		return plogotlp.NewExportResponse(), errors.New("no authorization header")
	}

	if authHeader[0] != "Bearer test-key" {
		m.t.Errorf("Expected Authorization header 'Bearer test-key', got %s", authHeader[0])
		return plogotlp.NewExportResponse(), errors.New("invalid authorization header")
	}

	assertAcceptEncodingGzip(m.t, md)

	m.recvCount += req.Logs().LogRecordCount()
	resp := plogotlp.NewExportResponse()
	if m.partialSuccess != nil {
		resp.PartialSuccess().SetErrorMessage(m.partialSuccess.ErrorMessage())
		resp.PartialSuccess().SetRejectedLogRecords(m.partialSuccess.RejectedLogRecords())
	}
	return resp, nil
}

func startMockOtlpLogsServer(tb testing.TB) (endpoint string, stopFn func(), srv *mockLogsServer) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	srv = &mockLogsServer{t: tb}
	plogotlp.RegisterGRPCServer(grpcServer, srv)
	go func() {
		_ = grpcServer.Serve(ln)
	}()
	return ln.Addr().String(), func() {
		grpcServer.Stop()
		ln.Close()
	}, srv
}

func BenchmarkLogsExporter_PushLogs(b *testing.B) {
	endpoint, stopFn, mockSrv := startMockOtlpLogsServer(b)
	defer stopFn()

	cfg := &Config{
		Logs: TransportConfig{
			ClientConfig: configgrpc.ClientConfig{
				Endpoint: endpoint,
				TLS: configtls.ClientConfig{
					Insecure: true,
				},
			},
		},
		PrivateKey: "test-key",
	}

	exp, err := newLogsExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	if err != nil {
		b.Fatalf("failed to create logs exporter: %v", err)
	}
	if err := exp.start(b.Context(), componenttest.NewNopHost()); err != nil {
		b.Fatalf("failed to start logs exporter: %v", err)
	}
	defer func() {
		_ = exp.shutdown(b.Context())
	}()

	testCases := []int{
		100000,
		500000,
		1000000,
		5000000,
		10000000,
		50000000,
	}
	for _, numLogs := range testCases {
		b.Run("numLogs="+fmt.Sprint(numLogs), func(b *testing.B) {
			for b.Loop() {
				logs := plog.NewLogs()
				rl := logs.ResourceLogs().AppendEmpty()
				rl.Resource().Attributes().PutStr("service.name", "benchmark-service")
				sl := rl.ScopeLogs().AppendEmpty()
				for range numLogs {
					logRecord := sl.LogRecords().AppendEmpty()
					logRecord.Body().SetStr("benchmark log message")
					logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
				}
				_ = exp.pushLogs(b.Context(), logs)
			}
		})
	}
	b.Logf("Total logs received by mock server: %d", mockSrv.recvCount)
}

func TestLogsExporter_PushLogs_PartialSuccess(t *testing.T) {
	endpoint, stopFn, mockSrv := startMockOtlpLogsServer(t)
	defer stopFn()

	cfg := &Config{
		Logs: TransportConfig{
			ClientConfig: configgrpc.ClientConfig{
				Endpoint: endpoint,
				TLS: configtls.ClientConfig{
					Insecure: true,
				},
			},
		},
		PrivateKey: "test-key",
	}

	exp, err := newLogsExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	err = exp.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = exp.shutdown(t.Context())
		require.NoError(t, err)
	}()

	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs()
	rl := resourceLogs.AppendEmpty()
	resource := rl.Resource()
	resource.Attributes().PutStr("service.name", "test-service")

	scopeLogs := rl.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName("test-scope")

	log1 := scopeLogs.LogRecords().AppendEmpty()
	log1.SetSeverityText("INFO")
	log2 := scopeLogs.LogRecords().AppendEmpty()
	log2.SetSeverityText("ERROR")

	partialSuccess := plogotlp.NewExportPartialSuccess()
	partialSuccess.SetErrorMessage("some logs were rejected")
	partialSuccess.SetRejectedLogRecords(1)
	mockSrv.partialSuccess = &partialSuccess

	core, observed := observer.New(zapcore.ErrorLevel)
	logger := zap.New(core)
	exp.settings.Logger = logger

	err = exp.pushLogs(t.Context(), logs)
	require.NoError(t, err)

	entries := observed.All()
	found := false
	for _, entry := range entries {
		if entry.Message == "Partial success response from Coralogix" &&
			entry.Level == zapcore.ErrorLevel &&
			entry.ContextMap()["message"] == "some logs were rejected" &&
			entry.ContextMap()["rejected_log_records"] == int64(1) {
			found = true
		}
	}
	assert.True(t, found, "Expected partial success log with correct fields")
}

func TestLogsExporter_PushLogs_Performance(t *testing.T) {
	isIntegrationTest := os.Getenv("INTEGRATION_TEST")
	if isIntegrationTest != "true" {
		t.Skip("Skipping E2E test: INTEGRATION_TEST not set")
	}

	endpoint, stopFn, mockSrv := startMockOtlpLogsServer(t)
	defer stopFn()

	cfg := &Config{
		Logs: TransportConfig{
			ClientConfig: configgrpc.ClientConfig{
				Endpoint: endpoint,
				TLS: configtls.ClientConfig{
					Insecure: true,
				},
			},
		},
		PrivateKey: "test-key",
		RateLimiter: RateLimiterConfig{
			Enabled:   true,
			Threshold: 3,
			Duration:  time.Second,
		},
	}

	exp, err := newLogsExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	err = exp.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = exp.shutdown(t.Context())
		require.NoError(t, err)
	}()

	t.Run("Under rate limit", func(t *testing.T) {
		mockSrv.recvCount = 0
		logs := plog.NewLogs()
		rl := logs.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", "test-service")
		sl := rl.ScopeLogs().AppendEmpty()

		logCount := 3000
		for i := range logCount {
			logRecord := sl.LogRecords().AppendEmpty()
			logRecord.Body().SetStr(fmt.Sprintf("test log message %d", i))
			logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			logRecord.SetSeverityText("INFO")
		}

		start := time.Now()
		err = exp.pushLogs(t.Context(), logs)
		duration := time.Since(start)

		require.NoError(t, err)
		assert.Equal(t, logCount, mockSrv.recvCount, "Expected to receive exactly %d logs", logCount)
		assert.Less(t, duration, time.Millisecond*100, "Operation took longer than 100 milliseconds")
	})

	t.Run("Over rate limit", func(t *testing.T) {
		mockSrv.recvCount = 0

		for range 5 {
			exp.EnableRateLimit()
		}

		logs := plog.NewLogs()
		rl := logs.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", "test-service")
		sl := rl.ScopeLogs().AppendEmpty()

		logCount := 7000
		for i := range logCount {
			logRecord := sl.LogRecords().AppendEmpty()
			logRecord.Body().SetStr(fmt.Sprintf("test log message %d", i))
			logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			logRecord.SetSeverityText("INFO")
		}

		start := time.Now()
		err = exp.pushLogs(t.Context(), logs)
		duration := time.Since(start)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rate limit exceeded")
		assert.Less(t, duration, time.Millisecond*100, "Operation took longer than 100 milliseconds")
		assert.Zero(t, mockSrv.recvCount, "Expected no logs to be received due to rate limiting")
	})

	t.Run("Rate limit reset", func(t *testing.T) {
		mockSrv.recvCount = 0

		require.Eventually(t, func() bool {
			testLogs := plog.NewLogs()
			testRl := testLogs.ResourceLogs().AppendEmpty()
			testRl.Resource().Attributes().PutStr("service.name", "test-service")
			testSl := testRl.ScopeLogs().AppendEmpty()
			testLogRecord := testSl.LogRecords().AppendEmpty()
			testLogRecord.Body().SetStr("test log message")
			testLogRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			testLogRecord.SetSeverityText("INFO")

			errPush := exp.pushLogs(t.Context(), testLogs)
			return errPush == nil
		}, 3*time.Second, 100*time.Millisecond, "Rate limit should reset within 3 seconds")

		// It's 1 because the last push is successful
		require.Equal(t, 1, mockSrv.recvCount)

		mockSrv.recvCount = 0

		logs := plog.NewLogs()
		rl := logs.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", "test-service")
		sl := rl.ScopeLogs().AppendEmpty()

		logCount := 3000
		for i := range logCount {
			logRecord := sl.LogRecords().AppendEmpty()
			logRecord.Body().SetStr(fmt.Sprintf("test log message %d", i))
			logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			logRecord.SetSeverityText("INFO")
		}

		start := time.Now()
		err = exp.pushLogs(t.Context(), logs)
		duration := time.Since(start)

		require.NoError(t, err)
		assert.Equal(t, logCount, mockSrv.recvCount, "Expected to receive exactly %d logs after rate limit reset", logCount)
		assert.Less(t, duration, time.Millisecond*100, "Operation took longer than 100 milliseconds")
	})
}

func TestLogsExporter_RateLimitErrorCountReset(t *testing.T) {
	endpoint, stopFn, srv := startMockOtlpLogsServer(t)
	defer stopFn()

	cfg := &Config{
		Logs: TransportConfig{
			ClientConfig: configgrpc.ClientConfig{
				Endpoint: endpoint,
				TLS: configtls.ClientConfig{
					Insecure: true,
				},
			},
		},
		PrivateKey: "test-key",
		RateLimiter: RateLimiterConfig{
			Enabled:   true,
			Threshold: 5,
			Duration:  time.Second,
		},
	}

	exp, err := newLogsExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	err = exp.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = exp.shutdown(t.Context())
		require.NoError(t, err)
	}()

	for range 5 {
		exp.EnableRateLimit()
	}
	assert.Equal(t, int32(5), exp.rateError.errorCount.Load())

	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resource := resourceLogs.Resource()
	resource.Attributes().PutStr("service.name", "test-service")
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	logRecord := scopeLogs.LogRecords().AppendEmpty()
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	logRecord.Body().SetStr("test log message")

	err = exp.pushLogs(t.Context(), logs)
	assert.Error(t, err)
	assert.Equal(t, int32(5), exp.rateError.errorCount.Load())
	assert.Equal(t, 0, srv.recvCount)

	require.Eventually(t, func() bool {
		err = exp.pushLogs(t.Context(), logs)
		return err == nil &&
			exp.rateError.errorCount.Load() == 0 &&
			srv.recvCount > 0
	}, 3*time.Second, 100*time.Millisecond)
}

func TestLogsExporter_RateLimitCounterResetOnSuccess(t *testing.T) {
	endpoint, stopFn, srv := startMockOtlpLogsServer(t)
	defer stopFn()

	cfg := &Config{
		Logs: TransportConfig{
			ClientConfig: configgrpc.ClientConfig{
				Endpoint: endpoint,
				TLS: configtls.ClientConfig{
					Insecure: true,
				},
			},
		},
		PrivateKey: "test-key",
		RateLimiter: RateLimiterConfig{
			Enabled:   true,
			Threshold: 5,
			Duration:  time.Second,
		},
	}

	exp, err := newLogsExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	err = exp.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = exp.shutdown(t.Context())
		require.NoError(t, err)
	}()

	createTestLogs := func() plog.Logs {
		logs := plog.NewLogs()
		resourceLogs := logs.ResourceLogs().AppendEmpty()
		resource := resourceLogs.Resource()
		resource.Attributes().PutStr("service.name", "test-service")
		scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
		logRecord := scopeLogs.LogRecords().AppendEmpty()
		logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		logRecord.Body().SetStr("test log message")
		return logs
	}

	t.Run("Initial successful push", func(t *testing.T) {
		logs := createTestLogs()
		err = exp.pushLogs(t.Context(), logs)
		require.NoError(t, err)
		assert.Equal(t, int32(0), exp.rateError.errorCount.Load())
		assert.Equal(t, 1, srv.recvCount)
	})

	t.Run("Trigger errors below threshold", func(t *testing.T) {
		for range 4 {
			exp.EnableRateLimit()
		}
		assert.Equal(t, int32(4), exp.rateError.errorCount.Load())
		assert.False(t, exp.rateError.isRateLimited(), "Should not be rate limited yet")
	})

	t.Run("Successful push after errors", func(t *testing.T) {
		logs := createTestLogs()
		err = exp.pushLogs(t.Context(), logs)
		require.NoError(t, err)
		assert.Equal(t, int32(0), exp.rateError.errorCount.Load())
		assert.Equal(t, 2, srv.recvCount)
	})

	t.Run("Verify error count stays at 0", func(t *testing.T) {
		logs := createTestLogs()
		err = exp.pushLogs(t.Context(), logs)
		require.NoError(t, err)
		assert.Equal(t, int32(0), exp.rateError.errorCount.Load())
		assert.Equal(t, 3, srv.recvCount)
	})
}
