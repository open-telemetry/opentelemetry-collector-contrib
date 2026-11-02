// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/testdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadata"
)

// BenchmarkTracesExporterRequestPath measures the gate-ON path
// (convert pdata to records, then push). It is the counterpart to
// BenchmarkTracesExporter (gate-OFF) and uses the same kfake-backed
// setup so the two can be compared with benchstat.
func BenchmarkTracesExporterRequestPath(b *testing.B) {
	const topic = "otlp_spans"
	set := exportertest.NewNopSettings(metadata.Type)
	for _, size := range batchSizes {
		b.Run(fmt.Sprintf("batch_%d", size), func(b *testing.B) {
			cfg := createDefaultConfig().(*Config)
			exp := newTracesExporter(*cfg, set)
			configureExporterBench(b, exp, *cfg, topic)
			converter := newRequestConverter(exp)
			pusher := newRequestPusher(exp)
			data := testdata.GenerateTraces(size)
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					req, err := converter(b.Context(), data)
					require.NoError(b, err)
					require.NoError(b, pusher(b.Context(), req))
				}
			})
		})
	}
}

// BenchmarkLogsExporterRequestPath is the logs counterpart to
// BenchmarkTracesExporterRequestPath.
func BenchmarkLogsExporterRequestPath(b *testing.B) {
	const topic = "otlp_logs"
	set := exportertest.NewNopSettings(metadata.Type)
	for _, size := range batchSizes {
		b.Run(fmt.Sprintf("batch_%d", size), func(b *testing.B) {
			cfg := createDefaultConfig().(*Config)
			exp := newLogsExporter(*cfg, set)
			configureExporterBench(b, exp, *cfg, topic)
			converter := newRequestConverter(exp)
			pusher := newRequestPusher(exp)
			data := testdata.GenerateLogs(size)
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					req, err := converter(b.Context(), data)
					require.NoError(b, err)
					require.NoError(b, pusher(b.Context(), req))
				}
			})
		})
	}
}

// BenchmarkMetricsExporterRequestPath is the metrics counterpart to
// BenchmarkTracesExporterRequestPath.
func BenchmarkMetricsExporterRequestPath(b *testing.B) {
	const topic = "otlp_metrics"
	set := exportertest.NewNopSettings(metadata.Type)
	for _, size := range batchSizes {
		b.Run(fmt.Sprintf("batch_%d", size), func(b *testing.B) {
			cfg := createDefaultConfig().(*Config)
			exp := newMetricsExporter(*cfg, set)
			configureExporterBench(b, exp, *cfg, topic)
			converter := newRequestConverter(exp)
			pusher := newRequestPusher(exp)
			data := testdata.GenerateMetrics(size)
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					req, err := converter(b.Context(), data)
					require.NoError(b, err)
					require.NoError(b, pusher(b.Context(), req))
				}
			})
		})
	}
}

// BenchmarkProfilesExporterRequestPath is the profiles counterpart to
// BenchmarkTracesExporterRequestPath.
func BenchmarkProfilesExporterRequestPath(b *testing.B) {
	const topic = "otlp_profiles"
	set := exportertest.NewNopSettings(metadata.Type)
	for _, size := range batchSizes {
		b.Run(fmt.Sprintf("batch_%d", size), func(b *testing.B) {
			cfg := createDefaultConfig().(*Config)
			exp := newProfilesExporter(*cfg, set)
			configureExporterBench(b, exp, *cfg, topic)
			converter := newRequestConverter(exp)
			pusher := newRequestPusher(exp)
			data := testdata.GenerateProfiles(size)
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					req, err := converter(b.Context(), data)
					require.NoError(b, err)
					require.NoError(b, pusher(b.Context(), req))
				}
			})
		})
	}
}
