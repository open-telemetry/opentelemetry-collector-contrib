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
			converter := newTracesRequestConverter(exp)
			pusher := newTracesRequestPusher(exp)
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
