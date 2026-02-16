// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sattributesprocessor

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// BenchmarkProcessTraces benchmarks the trace processing with k8s attributes enrichment
func BenchmarkProcessTraces(b *testing.B) {
	cfg := createDefaultConfig().(*Config)
	cfg.Extract.Metadata = []string{
		"k8s.pod.name",
		"k8s.pod.uid",
		"k8s.deployment.name",
		"k8s.namespace.name",
		"k8s.node.name",
	}

	p, err := newTracesProcessor(
		cfg,
		consumertest.NewNop(),
	)
	require.NoError(b, err)

	require.NoError(b, p.Start(b.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(b, p.Shutdown(b.Context()))
	}()

	ctx := client.NewContext(b.Context(), client.Info{
		Addr: &net.IPAddr{
			IP: net.IPv4(1, 1, 1, 1),
		},
	})

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-service")
	spans := rs.ScopeSpans().AppendEmpty().Spans()
	span := spans.AppendEmpty()
	span.SetName("test-span")
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		err := p.ConsumeTraces(ctx, traces)
		require.NoError(b, err)
	}
}

// BenchmarkProcessMetrics benchmarks the metrics processing with k8s attributes enrichment
func BenchmarkProcessMetrics(b *testing.B) {
	cfg := createDefaultConfig().(*Config)
	cfg.Extract.Metadata = []string{
		"k8s.pod.name",
		"k8s.pod.uid",
		"k8s.deployment.name",
		"k8s.namespace.name",
		"k8s.node.name",
	}

	p, err := newMetricsProcessor(
		cfg,
		consumertest.NewNop(),
	)
	require.NoError(b, err)

	require.NoError(b, p.Start(b.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(b, p.Shutdown(b.Context()))
	}()

	ctx := client.NewContext(b.Context(), client.Info{
		Addr: &net.IPAddr{
			IP: net.IPv4(1, 1, 1, 1),
		},
	})

	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "test-service")
	metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetName("test-metric")
	metric.SetEmptyGauge()
	dp := metric.Gauge().DataPoints().AppendEmpty()
	dp.SetIntValue(42)
	dp.SetTimestamp(pcommon.Timestamp(1))

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		err := p.ConsumeMetrics(ctx, metrics)
		require.NoError(b, err)
	}
}

// BenchmarkProcessLogs benchmarks the log processing with k8s attributes enrichment
func BenchmarkProcessLogs(b *testing.B) {
	cfg := createDefaultConfig().(*Config)
	cfg.Extract.Metadata = []string{
		"k8s.pod.name",
		"k8s.pod.uid",
		"k8s.deployment.name",
		"k8s.namespace.name",
		"k8s.node.name",
	}

	p, err := newLogsProcessor(
		cfg,
		consumertest.NewNop(),
	)
	require.NoError(b, err)

	require.NoError(b, p.Start(b.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(b, p.Shutdown(b.Context()))
	}()

	ctx := client.NewContext(b.Context(), client.Info{
		Addr: &net.IPAddr{
			IP: net.IPv4(1, 1, 1, 1),
		},
	})

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "test-service")
	logRecord := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	logRecord.Body().SetStr("test log message")
	logRecord.SetTimestamp(pcommon.Timestamp(1))

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		err := p.ConsumeLogs(ctx, logs)
		require.NoError(b, err)
	}
}

// BenchmarkProcessProfiles benchmarks the profile processing with k8s attributes enrichment
func BenchmarkProcessProfiles(b *testing.B) {
	cfg := createDefaultConfig().(*Config)
	cfg.Extract.Metadata = []string{
		"k8s.pod.name",
		"k8s.pod.uid",
		"k8s.deployment.name",
		"k8s.namespace.name",
		"k8s.node.name",
	}

	p, err := newProfilesProcessor(
		cfg,
		consumertest.NewNop(),
	)
	require.NoError(b, err)

	require.NoError(b, p.Start(b.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(b, p.Shutdown(b.Context()))
	}()

	ctx := client.NewContext(b.Context(), client.Info{
		Addr: &net.IPAddr{
			IP: net.IPv4(1, 1, 1, 1),
		},
	})

	profiles := pprofile.NewProfiles()
	rp := profiles.ResourceProfiles().AppendEmpty()
	rp.Resource().Attributes().PutStr("service.name", "test-service")
	profile := rp.ScopeProfiles().AppendEmpty().Profiles().AppendEmpty()
	profile.SetProfileID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		err := p.ConsumeProfiles(ctx, profiles)
		require.NoError(b, err)
	}
}

// BenchmarkProcessTracesMultipleResources benchmarks trace processing with multiple resource spans
func BenchmarkProcessTracesMultipleResources(b *testing.B) {
	cfg := createDefaultConfig().(*Config)
	cfg.Extract.Metadata = []string{
		"k8s.pod.name",
		"k8s.pod.uid",
		"k8s.deployment.name",
		"k8s.namespace.name",
	}

	p, err := newTracesProcessor(
		cfg,
		consumertest.NewNop(),
	)
	require.NoError(b, err)

	require.NoError(b, p.Start(b.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(b, p.Shutdown(b.Context()))
	}()

	ctx := client.NewContext(b.Context(), client.Info{
		Addr: &net.IPAddr{
			IP: net.IPv4(1, 1, 1, 1),
		},
	})

	traces := ptrace.NewTraces()
	for i := range 10 {
		rs := traces.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("service.name", "test-service")
		spans := rs.ScopeSpans().AppendEmpty().Spans()
		span := spans.AppendEmpty()
		span.SetName("test-span")
		span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, byte(i)})
		span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, byte(i)})
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		err := p.ConsumeTraces(ctx, traces)
		require.NoError(b, err)
	}
}
