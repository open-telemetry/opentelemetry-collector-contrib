// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"maps"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
)

type (
	grpcTraces struct {
		ptraceotlp.UnimplementedGRPCServer
		n atomic.Int32
	}
	grpcMetrics struct {
		pmetricotlp.UnimplementedGRPCServer
		n atomic.Int32
	}
	grpcLogs struct {
		plogotlp.UnimplementedGRPCServer
		n atomic.Int32
	}
)

func (b *grpcTraces) Export(_ context.Context, r ptraceotlp.ExportRequest) (ptraceotlp.ExportResponse, error) {
	b.n.Add(int32(r.Traces().SpanCount()))
	return ptraceotlp.NewExportResponse(), nil
}

func (b *grpcMetrics) Export(_ context.Context, r pmetricotlp.ExportRequest) (pmetricotlp.ExportResponse, error) {
	b.n.Add(int32(r.Metrics().MetricCount()))
	return pmetricotlp.NewExportResponse(), nil
}

func (b *grpcLogs) Export(_ context.Context, r plogotlp.ExportRequest) (plogotlp.ExportResponse, error) {
	b.n.Add(int32(r.Logs().LogRecordCount()))
	return plogotlp.NewExportResponse(), nil
}

// TestOTLPGRPCUnchanged checks that a configuration without protocol::otlp_http still sends
// traces, metrics and logs over OTLP/gRPC to the resolved backend.
func TestOTLPGRPCUnchanged(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	traces, metrics, logs := &grpcTraces{}, &grpcMetrics{}, &grpcLogs{}
	srv := grpc.NewServer()
	ptraceotlp.RegisterGRPCServer(srv, traces)
	pmetricotlp.RegisterGRPCServer(srv, metrics)
	plogotlp.RegisterGRPCServer(srv, logs)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	cfg := createDefaultConfig().(*Config)
	cfg.QueueSettings = configoptional.None[exporterhelper.QueueBatchConfig]()
	cfg.Protocol.OTLP.ClientConfig.TLS.Insecure = true
	cfg.Protocol.OTLP.QueueConfig = configoptional.None[exporterhelper.QueueBatchConfig]()
	cfg.Protocol.OTLP.RetryConfig.Enabled = false
	cfg.Resolver.Static = configoptional.Some(StaticResolver{Hostnames: []string{lis.Addr().String()}})
	require.False(t, cfg.Protocol.OTLPHTTP.HasValue())

	ctx := t.Context()
	set := exportertest.NewNopSettings(metadata.Type)
	host := componenttest.NewNopHost()
	shutdownCtx := context.WithoutCancel(ctx)

	te, err := createTracesExporter(ctx, set, cfg)
	require.NoError(t, err)
	require.NoError(t, te.Start(ctx, host))
	t.Cleanup(func() { require.NoError(t, te.Shutdown(shutdownCtx)) })
	me, err := createMetricsExporter(ctx, set, cfg)
	require.NoError(t, err)
	require.NoError(t, me.Start(ctx, host))
	t.Cleanup(func() { require.NoError(t, me.Shutdown(shutdownCtx)) })
	le, err := createLogsExporter(ctx, set, cfg)
	require.NoError(t, err)
	require.NoError(t, le.Start(ctx, host))
	t.Cleanup(func() { require.NoError(t, le.Shutdown(shutdownCtx)) })

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "svc")
	rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName("span")
	require.NoError(t, te.ConsumeTraces(ctx, td))

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "svc")
	rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("metric")
	require.NoError(t, me.ConsumeMetrics(ctx, md))

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "svc")
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log")
	require.NoError(t, le.ConsumeLogs(ctx, ld))

	assert.Eventually(t, func() bool {
		return traces.n.Load() == 1 && metrics.n.Load() == 1 && logs.n.Load() == 1
	}, 5*time.Second, 10*time.Millisecond)
}

func TestNewBackendDefaultsToGRPC(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	set := exportertest.NewNopSettings(metadata.Type)

	f, backendCfg, backendSet := newBackend(cfg, set, "10.0.0.1:4317")

	assert.Equal(t, otlpexporter.NewFactory().Type(), f.Type())
	want := buildExporterConfig(cfg, "10.0.0.1:4317")
	assert.Equal(t, &want, backendCfg)
	assert.Equal(t, "10.0.0.1:4317", backendCfg.(*otlpexporter.Config).ClientConfig.Endpoint)
	assert.Equal(t, otlpexporter.NewFactory().Type(), backendSet.ID.Type())
}

// TestRetryFlagsFollowUserConfig covers partially set retry blocks: keys the user did not write keep
// the defaults of the sub-exporter, and enabled: false is honored.
func TestRetryFlagsFollowUserConfig(t *testing.T) {
	for _, proto := range []string{"otlp", "otlp_http"} {
		endpoint := map[string]any{"endpoint": "placeholder:4317"}
		if proto == "otlp_http" {
			endpoint = map[string]any{"endpoint": "http://placeholder"}
		}
		withRetry := func(retry map[string]any) map[string]any {
			c := map[string]any{"retry_on_failure": retry}
			maps.Copy(c, endpoint)
			return map[string]any{
				"protocol": map[string]any{proto: c},
				"resolver": map[string]any{"static": map[string]any{"hostnames": []any{"a"}}},
			}
		}
		retryOf := func(cfg *Config) (bool, time.Duration) {
			if proto == "otlp" {
				return cfg.Protocol.OTLP.RetryConfig.Enabled, cfg.Protocol.OTLP.RetryConfig.MaxElapsedTime
			}
			h := cfg.Protocol.OTLPHTTP.Get()
			return h.RetryConfig.Enabled, h.RetryConfig.MaxElapsedTime
		}

		t.Run(proto+" max_elapsed_time only", func(t *testing.T) {
			cfg, err := unmarshalConfig(t, withRetry(map[string]any{"max_elapsed_time": "10m"}))
			require.NoError(t, err)
			enabled, elapsed := retryOf(cfg)
			assert.True(t, enabled, "retry stays enabled when only max_elapsed_time is set")
			assert.Equal(t, 10*time.Minute, elapsed)
		})
		t.Run(proto+" disabled", func(t *testing.T) {
			cfg, err := unmarshalConfig(t, withRetry(map[string]any{"enabled": false}))
			require.NoError(t, err)
			enabled, _ := retryOf(cfg)
			assert.False(t, enabled)
		})
	}
}
