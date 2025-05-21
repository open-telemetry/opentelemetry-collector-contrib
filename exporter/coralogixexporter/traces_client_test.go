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
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
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
				Traces: configgrpc.ClientConfig{
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
		Traces: configgrpc.ClientConfig{
			Headers: map[string]configopaque.String{},
		},
	}

	exp, err := newTracesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	err = exp.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	assert.NotNil(t, exp.clientConn)
	assert.NotNil(t, exp.traceExporter)
	assert.Contains(t, exp.config.Traces.Headers, "Authorization")

	// Test shutdown
	err = exp.shutdown(context.Background())
	require.NoError(t, err)
}

func TestTracesExporter_EnhanceContext(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
		Traces: configgrpc.ClientConfig{
			Headers: map[string]configopaque.String{
				"test-header": "test-value",
			},
		},
	}

	exp, err := newTracesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	ctx := context.Background()
	enhancedCtx := exp.enhanceContext(ctx)
	assert.NotEqual(t, ctx, enhancedCtx)
}

func TestTracesExporter_PushTraces(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
		Traces: configgrpc.ClientConfig{
			Headers: map[string]configopaque.String{},
		},
	}

	exp, err := newTracesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	// Initialize the exporter by calling start
	err = exp.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = exp.shutdown(context.Background())
		require.NoError(t, err)
	}()

	// Create test traces
	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans()
	rs := resourceSpans.AppendEmpty()

	resource := rs.Resource()
	resource.Attributes().PutStr("service.name", "test-service")

	err = exp.pushTraces(context.Background(), traces)
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
				Traces: configgrpc.ClientConfig{
					Headers: map[string]configopaque.String{},
				},
				RateLimiter: RateLimiterConfig{
					Enabled:   tt.configEnabled,
					Threshold: 1,
					Duration:  time.Minute,
				},
			}

			exp, err := newTracesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
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

			traces := ptrace.NewTraces()
			resourceSpans := traces.ResourceSpans()
			rs := resourceSpans.AppendEmpty()
			resource := rs.Resource()
			resource.Attributes().PutStr("service.name", "test-service")

			err = exp.pushTraces(context.Background(), traces)
			assert.Error(t, err)
			if tt.configEnabled {
				assert.Contains(t, err.Error(), rateLimitErr.Error())
			} else {
				assert.Contains(t, err.Error(), "no such host")
			}
		})
	}
}

type mockTracesServer struct {
	ptraceotlp.UnimplementedGRPCServer
	recvCount int
}

func (m *mockTracesServer) Export(_ context.Context, req ptraceotlp.ExportRequest) (ptraceotlp.ExportResponse, error) {
	m.recvCount += req.Traces().ResourceSpans().Len()
	return ptraceotlp.NewExportResponse(), nil
}

func startMockOtlpTracesServer(tb testing.TB) (endpoint string, stopFn func(), srv *mockTracesServer) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	srv = &mockTracesServer{}
	ptraceotlp.RegisterGRPCServer(grpcServer, srv)
	go func() {
		_ = grpcServer.Serve(ln)
	}()
	return ln.Addr().String(), func() {
		grpcServer.Stop()
		ln.Close()
	}, srv
}

func BenchmarkTracesExporter_PushTraces(b *testing.B) {
	endpoint, stopFn, mockSrv := startMockOtlpTracesServer(b)
	defer stopFn()

	cfg := &Config{
		Traces: configgrpc.ClientConfig{
			Endpoint: endpoint,
			TLSSetting: configtls.ClientConfig{
				Insecure: true,
			},
			Headers: map[string]configopaque.String{},
		},
		PrivateKey: "test-key",
	}

	exp, err := newTracesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	if err != nil {
		b.Fatalf("failed to create traces exporter: %v", err)
	}
	if err := exp.start(context.Background(), componenttest.NewNopHost()); err != nil {
		b.Fatalf("failed to start traces exporter: %v", err)
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
	for _, numTraces := range testCases {
		b.Run("numTraces="+fmt.Sprint(numTraces), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				traces := ptrace.NewTraces()
				rs := traces.ResourceSpans().AppendEmpty()
				rs.Resource().Attributes().PutStr("service.name", "benchmark-service")
				ss := rs.ScopeSpans().AppendEmpty()
				for j := 0; j < numTraces; j++ {
					span := ss.Spans().AppendEmpty()
					span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
					span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
					span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
					span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(time.Second)))
				}
				_ = exp.pushTraces(context.Background(), traces)
			}
		})
	}
	b.Logf("Total traces received by mock server: %d", mockSrv.recvCount)
}
