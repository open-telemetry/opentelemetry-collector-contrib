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
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter/internal/validation"
)

func TestNewTracesExporter(t *testing.T) {
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
			name: "Valid traces endpoint config",
			cfg: &Config{
				Traces: TransportConfig{
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
			exp, err := newTracesExporter(tt.cfg, exportertest.NewNopSettings(exportertest.NopType))
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

func TestTracesExporter_Start(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
	}

	exp, err := newTracesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	err = exp.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	assert.NotNil(t, exp.clientConn)
	assert.NotNil(t, exp.grpcTracesExporter)
	_, ok := exp.config.Traces.Headers.Get("Authorization")
	assert.True(t, ok)

	err = exp.shutdown(t.Context())
	require.NoError(t, err)
}

func TestTracesExporter_EnhanceContext(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
		Traces: TransportConfig{
			ClientConfig: configgrpc.ClientConfig{
				Headers: configopaque.MapList{
					{Name: "test-header", Value: "test-value"},
				},
			},
		},
	}

	exp, err := newTracesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	ctx := t.Context()
	enhancedCtx := exp.enhanceContext(ctx)
	assert.NotEqual(t, ctx, enhancedCtx)
}

func TestTracesExporter_PushTraces(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
	}

	exp, err := newTracesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	err = exp.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = exp.shutdown(t.Context())
		require.NoError(t, err)
	}()

	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans()
	rs := resourceSpans.AppendEmpty()

	resource := rs.Resource()
	resource.Attributes().PutStr("service.name", "test-service")

	err = exp.pushTraces(t.Context(), traces)
	assert.Error(t, err)
}

func TestTracesExporter_PushTraces_WhenCannotSend(t *testing.T) {
	tests := []struct {
		description   string
		configEnabled bool
	}{
		{
			description:   "Rate limit exceeded config enabled",
			configEnabled: true,
		},
		{
			description:   "Rate limit exceeded config disabled",
			configEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			cfg := &Config{
				Domain:     "test.domain.com",
				PrivateKey: "test-key",
				RateLimiter: RateLimiterConfig{
					Enabled:   tt.configEnabled,
					Threshold: 1,
					Duration:  time.Minute,
				},
			}

			exp, err := newTracesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
			require.NoError(t, err)

			err = exp.start(t.Context(), componenttest.NewNopHost())
			require.NoError(t, err)
			defer func() {
				err = exp.shutdown(t.Context())
				require.NoError(t, err)
			}()

			// Add two rate limit errors
			exp.EnableRateLimit()
			exp.EnableRateLimit()

			traces := ptrace.NewTraces()
			resourceSpans := traces.ResourceSpans()
			rs := resourceSpans.AppendEmpty()
			resource := rs.Resource()
			resource.Attributes().PutStr("service.name", "test-service")

			err = exp.pushTraces(t.Context(), traces)
			assert.Error(t, err)
			if tt.configEnabled {
				assert.Contains(t, err.Error(), "rate limit exceeded")
			} else {
				assert.Contains(t, err.Error(), "no such host")
			}
		})
	}
}

type mockTracesServer struct {
	ptraceotlp.UnimplementedGRPCServer
	recvCount      int
	partialSuccess *ptraceotlp.ExportPartialSuccess
	t              testing.TB
}

func (m *mockTracesServer) Export(ctx context.Context, req ptraceotlp.ExportRequest) (ptraceotlp.ExportResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		m.t.Error("No metadata found in context")
		return ptraceotlp.NewExportResponse(), errors.New("no metadata found")
	}

	authHeader := md.Get("authorization")
	if len(authHeader) == 0 {
		m.t.Error("No Authorization header found")
		return ptraceotlp.NewExportResponse(), errors.New("no authorization header")
	}

	if authHeader[0] != "Bearer test-key" {
		m.t.Errorf("Expected Authorization header 'Bearer test-key', got %s", authHeader[0])
		return ptraceotlp.NewExportResponse(), errors.New("invalid authorization header")
	}

	assertAcceptEncodingGzip(m.t, md)

	m.recvCount += req.Traces().SpanCount()
	resp := ptraceotlp.NewExportResponse()
	if m.partialSuccess != nil {
		resp.PartialSuccess().SetErrorMessage(m.partialSuccess.ErrorMessage())
		resp.PartialSuccess().SetRejectedSpans(m.partialSuccess.RejectedSpans())
	}
	return resp, nil
}

func startMockOtlpTracesServer(tb testing.TB) (endpoint string, stopFn func(), srv *mockTracesServer) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	srv = &mockTracesServer{t: tb}
	ptraceotlp.RegisterGRPCServer(grpcServer, srv)
	go func() {
		_ = grpcServer.Serve(ln)
	}()
	return ln.Addr().String(), func() {
		grpcServer.Stop()
		ln.Close()
	}, srv
}

func getTraceID(s string) [16]byte {
	var id [16]byte
	copy(id[:], s)
	return id
}

func getLoggedInvalidSpanSamples(t *testing.T, entry observer.LoggedEntry) []validation.InvalidSpanDetail {
	t.Helper()
	raw, ok := entry.ContextMap()["samples"]
	require.True(t, ok, "samples field missing")

	if samples, isTyped := raw.([]validation.InvalidSpanDetail); isTyped {
		return samples
	}

	rawSlice, ok := raw.([]any)
	require.True(t, ok, "samples field type mismatch")

	result := make([]validation.InvalidSpanDetail, 0, len(rawSlice))
	for _, item := range rawSlice {
		detail, ok := item.(validation.InvalidSpanDetail)
		require.True(t, ok, "invalid span detail type mismatch")
		result = append(result, detail)
	}
	return result
}

func TestTracesExporter_PushTraces_PartialSuccess(t *testing.T) {
	endpoint, stopFn, mockSrv := startMockOtlpTracesServer(t)
	defer stopFn()

	cfg := &Config{
		Traces: TransportConfig{
			ClientConfig: configgrpc.ClientConfig{
				Endpoint: endpoint,
				TLS: configtls.ClientConfig{
					Insecure: true,
				},
			},
		},
		PrivateKey: "test-key",
	}

	exp, err := newTracesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	err = exp.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = exp.shutdown(t.Context())
		require.NoError(t, err)
	}()

	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans()
	rs := resourceSpans.AppendEmpty()
	resource := rs.Resource()
	resource.Attributes().PutStr("service.name", "test-service")

	scopeSpans := rs.ScopeSpans().AppendEmpty()
	scopeSpans.Scope().SetName("test-scope")

	span1 := scopeSpans.Spans().AppendEmpty()
	span1.SetName("span1")
	traceID1 := getTraceID("traceid1")
	span1.SetTraceID(traceID1)
	span2 := scopeSpans.Spans().AppendEmpty()
	span2.SetName("span2")
	traceID2 := getTraceID("traceid2")
	span2.SetTraceID(traceID2)
	// Add another span with duplicate trace ID
	span3 := scopeSpans.Spans().AppendEmpty()
	span3.SetName("span3")
	span3.SetTraceID(traceID1) // Duplicate trace ID

	partialSuccess := ptraceotlp.NewExportPartialSuccess()
	partialSuccess.SetErrorMessage("some spans were rejected")
	partialSuccess.SetRejectedSpans(1)
	mockSrv.partialSuccess = &partialSuccess

	core, observed := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	exp.settings.Logger = logger

	err = exp.pushTraces(t.Context(), traces)
	require.NoError(t, err)

	entries := observed.All()
	found := false
	for _, entry := range entries {
		if entry.Message != "Partial success response from Coralogix" ||
			entry.Level != zapcore.ErrorLevel ||
			entry.ContextMap()["message"] != "some spans were rejected" ||
			entry.ContextMap()["rejected_spans"] != int64(1) {
			continue
		}

		// For unknown errors, we just log basic fields without validation details
		// Trace IDs are now only in the samples, not as a separate field
		found = true
	}
	assert.True(t, found, "Expected partial success log with correct fields")
}

// Removed TestTracesExporter_PushTraces_LogsInvalidSpanDetails test
// We no longer do proactive validation scanning for performance reasons.
// Validation details are only collected when partial success errors occur.

func TestTracesExporter_PushTraces_PartialSuccess_InvalidDurationDetails(t *testing.T) {
	endpoint, stopFn, mockSrv := startMockOtlpTracesServer(t)
	defer stopFn()

	cfg := &Config{
		Traces: TransportConfig{
			ClientConfig: configgrpc.ClientConfig{
				Endpoint: endpoint,
				TLS: configtls.ClientConfig{
					Insecure: true,
				},
			},
		},
		PrivateKey: "test-key",
	}

	exp, err := newTracesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	err = exp.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = exp.shutdown(t.Context())
		require.NoError(t, err)
	}()

	core, observed := observer.New(zapcore.DebugLevel)
	exp.settings.Logger = zap.New(core)

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "broken-service")
	rs.Resource().Attributes().PutStr("k8s.pod.name", "pod-1")

	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("test-scope")

	// Add one invalid span
	invalidSpan := ss.Spans().AppendEmpty()
	invalidSpan.SetName("invalid-span")
	invalidSpan.SetTraceID(getTraceID("trace-invalid"))
	invalidSpan.SetSpanID([8]byte{8, 7, 6, 5, 4, 3, 2, 1})
	invalidSpan.SetStartTimestamp(200)
	invalidSpan.SetEndTimestamp(100)

	partialSuccess := ptraceotlp.NewExportPartialSuccess()
	partialSuccess.SetErrorMessage("Invalid span duration detected")
	partialSuccess.SetRejectedSpans(1)
	mockSrv.partialSuccess = &partialSuccess

	err = exp.pushTraces(t.Context(), traces)
	require.NoError(t, err)

	entries := observed.All()
	foundError := false
	for _, entry := range entries {
		if entry.Message != "Partial success response from Coralogix" || entry.Level != zapcore.ErrorLevel {
			continue
		}
		foundError = true
		assert.Equal(t, int64(1), entry.ContextMap()["rejected_spans"])
		assert.Equal(t, "Invalid span duration detected", entry.ContextMap()["message"])
		assert.Equal(t, string(validation.ErrorTypeInvalidDuration), entry.ContextMap()["partial_success_type"])
		samples := getLoggedInvalidSpanSamples(t, entry)
		require.Len(t, samples, 1)
		sample := samples[0]
		assert.Equal(t, "invalid-span", sample.SpanDetails.SpanName)
		assert.Equal(t, "test-scope", sample.SpanDetails.InstrumentationScopeName)
		assert.Equal(t, int64(-100), sample.DurationNano) // Duration is negative
		assert.Equal(t, "broken-service", sample.SpanDetails.ResourceAttributes["service.name"])
		assert.Equal(t, "pod-1", sample.SpanDetails.ResourceAttributes["k8s.pod.name"])
	}
	assert.True(t, foundError, "Expected partial success log containing invalid span details")
}

// Removed TestCollectInvalidDurationSpans test
// This function has been moved to the validation package
// See validation/traces_test.go for tests
func BenchmarkTracesExporter_PushTraces(b *testing.B) {
	endpoint, stopFn, mockSrv := startMockOtlpTracesServer(b)
	defer stopFn()

	cfg := &Config{
		Traces: TransportConfig{
			ClientConfig: configgrpc.ClientConfig{
				Endpoint: endpoint,
				TLS: configtls.ClientConfig{
					Insecure: true,
				},
			},
		},
		PrivateKey: "test-key",
	}

	exp, err := newTracesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	if err != nil {
		b.Fatalf("failed to create traces exporter: %v", err)
	}
	err = exp.start(b.Context(), componenttest.NewNopHost())
	if err != nil {
		b.Fatalf("failed to start traces exporter: %v", err)
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
	for _, numTraces := range testCases {
		b.Run("numTraces="+fmt.Sprint(numTraces), func(b *testing.B) {
			for b.Loop() {
				traces := ptrace.NewTraces()
				rs := traces.ResourceSpans().AppendEmpty()
				rs.Resource().Attributes().PutStr("service.name", "benchmark-service")
				ss := rs.ScopeSpans().AppendEmpty()
				for j := range numTraces {
					span := ss.Spans().AppendEmpty()
					span.SetTraceID(getTraceID(fmt.Sprintf("trace%d", j)))
					span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
					span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
					span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(time.Second)))
				}
				_ = exp.pushTraces(b.Context(), traces)
			}
		})
	}
	b.Logf("Total traces received by mock server: %d", mockSrv.recvCount)
}

func TestTracesExporter_PushTraces_Performance(t *testing.T) {
	isIntegrationTest := os.Getenv("INTEGRATION_TEST")
	if isIntegrationTest != "true" {
		t.Skip("Skipping E2E test: INTEGRATION_TEST not set")
	}

	endpoint, stopFn, mockSrv := startMockOtlpTracesServer(t)
	defer stopFn()

	cfg := &Config{
		Traces: TransportConfig{
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

	exp, err := newTracesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	err = exp.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = exp.shutdown(t.Context())
		require.NoError(t, err)
	}()

	t.Run("Under rate limit", func(t *testing.T) {
		mockSrv.recvCount = 0
		traces := ptrace.NewTraces()
		rs := traces.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("service.name", "test-service")
		ss := rs.ScopeSpans().AppendEmpty()

		spanCount := 3000
		for i := range spanCount {
			span := ss.Spans().AppendEmpty()
			span.SetName(fmt.Sprintf("test_span_%d", i))
			span.SetTraceID(getTraceID(fmt.Sprintf("trace%d", i)))
			span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, byte(i % 256)})
			span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(time.Second)))
		}

		start := time.Now()
		err = exp.pushTraces(t.Context(), traces)
		duration := time.Since(start)

		require.NoError(t, err)
		assert.Equal(t, spanCount, mockSrv.recvCount, "Expected to receive exactly %d spans", spanCount)
		assert.Less(t, duration, time.Millisecond*100, "Operation took longer than 100 milliseconds")
	})

	t.Run("Over rate limit", func(t *testing.T) {
		mockSrv.recvCount = 0

		for range 5 {
			exp.EnableRateLimit()
		}

		traces := ptrace.NewTraces()
		rs := traces.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("service.name", "test-service")
		ss := rs.ScopeSpans().AppendEmpty()

		spanCount := 7000
		for i := range spanCount {
			span := ss.Spans().AppendEmpty()
			span.SetName(fmt.Sprintf("test_span_%d", i))
			span.SetTraceID(getTraceID(fmt.Sprintf("trace%d", i)))
			span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, byte(i % 256)})
			span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(time.Second)))
		}

		start := time.Now()
		err = exp.pushTraces(t.Context(), traces)
		duration := time.Since(start)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rate limit exceeded")
		assert.Less(t, duration, time.Millisecond*100, "Operation took longer than 100 milliseconds")
		assert.Zero(t, mockSrv.recvCount, "Expected no spans to be received due to rate limiting")
	})

	t.Run("Rate limit reset", func(t *testing.T) {
		mockSrv.recvCount = 0

		require.Eventually(t, func() bool {
			testTraces := ptrace.NewTraces()
			testRs := testTraces.ResourceSpans().AppendEmpty()
			testRs.Resource().Attributes().PutStr("service.name", "test-service")
			testSs := testRs.ScopeSpans().AppendEmpty()
			testSpan := testSs.Spans().AppendEmpty()
			testSpan.SetName("test-span")
			testSpan.SetTraceID(getTraceID("test-trace"))

			errPush := exp.pushTraces(t.Context(), testTraces)
			return errPush == nil
		}, 3*time.Second, 100*time.Millisecond, "Rate limit should reset within 3 seconds")

		require.Equal(t, 1, mockSrv.recvCount)
		mockSrv.recvCount = 0

		traces := ptrace.NewTraces()
		rs := traces.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("service.name", "test-service")
		ss := rs.ScopeSpans().AppendEmpty()

		spanCount := 3000
		for i := range spanCount {
			span := ss.Spans().AppendEmpty()
			span.SetName(fmt.Sprintf("test_span_%d", i))
			span.SetTraceID(getTraceID(fmt.Sprintf("trace%d", i)))
			span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, byte(i % 256)})
			span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(time.Second)))
		}

		start := time.Now()
		err = exp.pushTraces(t.Context(), traces)
		duration := time.Since(start)

		require.NoError(t, err)
		assert.Equal(t, spanCount, mockSrv.recvCount, "Expected to receive exactly %d spans after rate limit reset", spanCount)
		assert.Less(t, duration, time.Millisecond*100, "Operation took longer than 100 milliseconds")
	})
}

func TestTracesExporter_RateLimitErrorCountReset(t *testing.T) {
	endpoint, stopFn, srv := startMockOtlpTracesServer(t)
	defer stopFn()

	cfg := &Config{
		Traces: TransportConfig{
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

	exp, err := newTracesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
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

	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans()
	rs := resourceSpans.AppendEmpty()
	resource := rs.Resource()
	resource.Attributes().PutStr("service.name", "test-service")

	scopeSpans := rs.ScopeSpans().AppendEmpty()
	span := scopeSpans.Spans().AppendEmpty()
	span.SetName("test-span")

	err = exp.pushTraces(t.Context(), traces)
	assert.Error(t, err)
	assert.Equal(t, int32(5), exp.rateError.errorCount.Load())
	assert.Equal(t, 0, srv.recvCount)

	require.Eventually(t, func() bool {
		err = exp.pushTraces(t.Context(), traces)
		return err == nil &&
			exp.rateError.errorCount.Load() == 0 &&
			srv.recvCount == 1
	}, 3*time.Second, 100*time.Millisecond)
}

func TestTracesExporter_RateLimitCounterResetOnSuccess(t *testing.T) {
	endpoint, stopFn, srv := startMockOtlpTracesServer(t)
	defer stopFn()

	cfg := &Config{
		Traces: TransportConfig{
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

	exp, err := newTracesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	err = exp.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = exp.shutdown(t.Context())
		require.NoError(t, err)
	}()

	createTestTraces := func() ptrace.Traces {
		traces := ptrace.NewTraces()
		resourceSpans := traces.ResourceSpans().AppendEmpty()
		resource := resourceSpans.Resource()
		resource.Attributes().PutStr("service.name", "test-service")
		scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
		span := scopeSpans.Spans().AppendEmpty()
		span.SetName("test-span")
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(time.Second)))
		return traces
	}

	t.Run("Initial successful push", func(t *testing.T) {
		traces := createTestTraces()
		err = exp.pushTraces(t.Context(), traces)
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
		traces := createTestTraces()
		err = exp.pushTraces(t.Context(), traces)
		require.NoError(t, err)
		assert.Equal(t, int32(0), exp.rateError.errorCount.Load())
		assert.Equal(t, 2, srv.recvCount)
	})

	t.Run("Verify error count stays at 0", func(t *testing.T) {
		traces := createTestTraces()
		err = exp.pushTraces(t.Context(), traces)
		require.NoError(t, err)
		assert.Equal(t, int32(0), exp.rateError.errorCount.Load())
		assert.Equal(t, 3, srv.recvCount)
	})
}
