// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter

import (
	"context"
	"errors"
	"fmt"
	"net"
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
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestNewMetricsExporter(t *testing.T) {
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
			name: "Valid metrics endpoint config",
			cfg: &Config{
				Metrics: configgrpc.ClientConfig{
					Endpoint: "localhost:4317",
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
			exp, err := newMetricsExporter(tt.cfg, exportertest.NewNopSettings(exportertest.NopType))
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

func TestMetricsExporter_Start(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
		Metrics: configgrpc.ClientConfig{
			Headers: map[string]configopaque.String{},
		},
	}

	exp, err := newMetricsExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	err = exp.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	assert.NotNil(t, exp.clientConn)
	assert.NotNil(t, exp.metricExporter)
	assert.Contains(t, exp.config.Metrics.Headers, "Authorization")

	// Test shutdown
	err = exp.shutdown(context.Background())
	require.NoError(t, err)
}

func TestMetricsExporter_EnhanceContext(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
		Metrics: configgrpc.ClientConfig{
			Headers: map[string]configopaque.String{
				"test-header": "test-value",
			},
		},
	}

	exp, err := newMetricsExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	ctx := context.Background()
	enhancedCtx := exp.enhanceContext(ctx)
	assert.NotEqual(t, ctx, enhancedCtx)
}

func TestMetricsExporter_PushMetrics(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
		Metrics: configgrpc.ClientConfig{
			Headers: map[string]configopaque.String{},
		},
	}

	exp, err := newMetricsExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	// Initialize the exporter by calling start
	err = exp.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = exp.shutdown(context.Background())
		require.NoError(t, err)
	}()

	// Create test metrics
	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()
	rm := resourceMetrics.AppendEmpty()

	resource := rm.Resource()
	resource.Attributes().PutStr("service.name", "test-service")

	err = exp.pushMetrics(context.Background(), metrics)
	assert.Error(t, err)
}

func TestMetricsExporter_PushMetrics_WhenCannotSend(t *testing.T) {
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
				Metrics: configgrpc.ClientConfig{
					Headers: map[string]configopaque.String{},
				},
				RateLimiter: RateLimiterConfig{
					Enabled:   tt.enabled,
					Threshold: 1,
					Duration:  time.Minute,
				},
			}

			exp, err := newMetricsExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
			require.NoError(t, err)

			err = exp.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err)
			defer func() {
				err = exp.shutdown(context.Background())
				require.NoError(t, err)
			}()

			rateLimitErr := errors.New("rate limit exceeded")
			exp.EnableRateLimit(rateLimitErr)
			exp.EnableRateLimit(rateLimitErr)

			metrics := pmetric.NewMetrics()
			resourceMetrics := metrics.ResourceMetrics()
			rm := resourceMetrics.AppendEmpty()
			resource := rm.Resource()
			resource.Attributes().PutStr("service.name", "test-service")

			err = exp.pushMetrics(context.Background(), metrics)
			assert.Error(t, err)
			if tt.enabled {
				assert.Contains(t, err.Error(), "rate limit exceeded")
			} else {
				assert.Contains(t, err.Error(), "no such host")
			}
		})
	}
}

type mockMetricsServer struct {
	pmetricotlp.UnimplementedGRPCServer
	recvCount      int
	partialSuccess *pmetricotlp.ExportPartialSuccess
	t              testing.TB
}

func (m *mockMetricsServer) Export(ctx context.Context, req pmetricotlp.ExportRequest) (pmetricotlp.ExportResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		m.t.Error("No metadata found in context")
		return pmetricotlp.NewExportResponse(), errors.New("no metadata found")
	}

	authHeader := md.Get("authorization")
	if len(authHeader) == 0 {
		m.t.Error("No Authorization header found")
		return pmetricotlp.NewExportResponse(), errors.New("no authorization header")
	}

	if authHeader[0] != "Bearer test-key" {
		m.t.Errorf("Expected Authorization header 'Bearer test-key', got %s", authHeader[0])
		return pmetricotlp.NewExportResponse(), errors.New("invalid authorization header")
	}

	m.recvCount += req.Metrics().DataPointCount()
	resp := pmetricotlp.NewExportResponse()
	if m.partialSuccess != nil {
		resp.PartialSuccess().SetErrorMessage(m.partialSuccess.ErrorMessage())
		resp.PartialSuccess().SetRejectedDataPoints(m.partialSuccess.RejectedDataPoints())
	}
	return resp, nil
}

func startMockOtlpMetricsServer(tb testing.TB) (endpoint string, stopFn func(), srv *mockMetricsServer) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	srv = &mockMetricsServer{t: tb}
	pmetricotlp.RegisterGRPCServer(grpcServer, srv)
	go func() {
		_ = grpcServer.Serve(ln)
	}()
	return ln.Addr().String(), func() {
		grpcServer.Stop()
		ln.Close()
	}, srv
}

func BenchmarkMetricsExporter_PushMetrics(b *testing.B) {
	endpoint, stopFn, mockSrv := startMockOtlpMetricsServer(b)
	defer stopFn()

	cfg := &Config{
		Metrics: configgrpc.ClientConfig{
			Endpoint: endpoint,
			TLS: configtls.ClientConfig{
				Insecure: true,
			},
			Headers: map[string]configopaque.String{},
		},
		PrivateKey: "test-key",
	}

	exp, err := newMetricsExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	if err != nil {
		b.Fatalf("failed to create metrics exporter: %v", err)
	}
	if err := exp.start(context.Background(), componenttest.NewNopHost()); err != nil {
		b.Fatalf("failed to start metrics exporter: %v", err)
	}
	defer func() {
		_ = exp.shutdown(context.Background())
	}()

	testCases := []int{
		100000,
		500000,
		1000000,
		5000000,
		10000000,
		50000000,
	}
	for _, numMetrics := range testCases {
		b.Run("numMetrics="+fmt.Sprint(numMetrics), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				metrics := pmetric.NewMetrics()
				rm := metrics.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("service.name", "benchmark-service")
				sm := rm.ScopeMetrics().AppendEmpty()
				for j := 0; j < numMetrics; j++ {
					metric := sm.Metrics().AppendEmpty()
					metric.SetName("benchmark_metric")
					metric.SetUnit("1")
					metric.SetEmptyGauge()
					dp := metric.Gauge().DataPoints().AppendEmpty()
					dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
					dp.SetDoubleValue(float64(j))
				}
				_ = exp.pushMetrics(context.Background(), metrics)
			}
		})
	}
	b.Logf("Total metrics received by mock server: %d", mockSrv.recvCount)
}

func TestMetricsExporter_PushMetrics_PartialSuccess(t *testing.T) {
	endpoint, stopFn, mockSrv := startMockOtlpMetricsServer(t)
	defer stopFn()

	cfg := &Config{
		Metrics: configgrpc.ClientConfig{
			Endpoint: endpoint,
			TLS: configtls.ClientConfig{
				Insecure: true,
			},
			Headers: map[string]configopaque.String{},
		},
		PrivateKey: "test-key",
	}

	exp, err := newMetricsExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	err = exp.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = exp.shutdown(context.Background())
		require.NoError(t, err)
	}()

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()
	rm := resourceMetrics.AppendEmpty()
	resource := rm.Resource()
	resource.Attributes().PutStr("service.name", "test-service")

	scopeMetrics := rm.ScopeMetrics().AppendEmpty()
	scopeMetrics.Scope().SetName("test-scope")

	metric1 := scopeMetrics.Metrics().AppendEmpty()
	metric1.SetName("metric1")
	metric2 := scopeMetrics.Metrics().AppendEmpty()
	metric2.SetName("metric2")

	partialSuccess := pmetricotlp.NewExportPartialSuccess()
	partialSuccess.SetErrorMessage("some metrics were rejected")
	partialSuccess.SetRejectedDataPoints(1)
	mockSrv.partialSuccess = &partialSuccess

	core, observed := observer.New(zapcore.ErrorLevel)
	logger := zap.New(core)
	exp.settings.Logger = logger

	err = exp.pushMetrics(context.Background(), metrics)
	require.NoError(t, err)

	entries := observed.All()
	found := false
	for _, entry := range entries {
		if entry.Message == "Partial success response from Coralogix" &&
			entry.Level == zapcore.ErrorLevel &&
			entry.ContextMap()["message"] == "some metrics were rejected" &&
			entry.ContextMap()["rejected_data_points"] == int64(1) {
			fields := entry.ContextMap()
			var names []string
			if arr, ok := fields["metric_names"].([]string); ok {
				names = arr
			} else if arr, ok := fields["metric_names"].([]any); ok {
				for _, v := range arr {
					if s, ok := v.(string); ok {
						names = append(names, s)
					}
				}
			}
			assert.Contains(t, names, "metric1")
			assert.Contains(t, names, "metric2")
			found = true
		}
	}
	assert.True(t, found, "Expected partial success log with correct fields and metric names")
}
