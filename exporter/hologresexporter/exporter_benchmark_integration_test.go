// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package hologresexporter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.uber.org/zap"
)

// BenchmarkIntegration_TracesPush benchmarks real Hologres trace writes.
func BenchmarkIntegration_TracesPush(b *testing.B) {
	sizes := []int{10, 100, 1000}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("spans_%d", size), func(b *testing.B) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			prefix := fmt.Sprintf("bench_traces_%d_%d", size, time.Now().UnixNano())
			cfg := &Config{
				DSN:             testDSN,
				TracesTableName: prefix,
				CreateSchema:    true,
				TTL:             24 * time.Hour,
			}

			exp := newTracesExporter(zap.NewNop(), cfg)
			if err := exp.start(ctx, nil); err != nil {
				b.Fatalf("failed to start traces exporter: %v", err)
			}
			pool := exp.db.(*hologresPool).pool
			defer func() {
				dropTablesQuiet(ctx, pool, cfg.TracesTableName)
				_ = exp.shutdown(ctx)
			}()

			td := generateTraces(size)

			b.ReportAllocs()
			b.SetBytes(int64(size * 512)) // approximate bytes per span
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := exp.pushTraceData(ctx, td); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()

			elapsed := b.Elapsed()
			b.ReportMetric(float64(size), "items/op")
			if elapsed > 0 {
				b.ReportMetric(float64(int64(size)*int64(b.N))/elapsed.Seconds(), "items/s")
			}
		})
	}
}

// BenchmarkIntegration_LogsPush benchmarks real Hologres log writes.
func BenchmarkIntegration_LogsPush(b *testing.B) {
	sizes := []int{10, 100, 1000}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("logs_%d", size), func(b *testing.B) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			prefix := fmt.Sprintf("bench_logs_%d_%d", size, time.Now().UnixNano())
			cfg := &Config{
				DSN:           testDSN,
				LogsTableName: prefix,
				CreateSchema:  true,
				TTL:           24 * time.Hour,
			}

			exp := newLogsExporter(zap.NewNop(), cfg)
			if err := exp.start(ctx, nil); err != nil {
				b.Fatalf("failed to start logs exporter: %v", err)
			}
			pool := exp.db.(*hologresPool).pool
			defer func() {
				dropTablesQuiet(ctx, pool, cfg.LogsTableName)
				_ = exp.shutdown(ctx)
			}()

			ld := generateLogs(size)

			b.ReportAllocs()
			b.SetBytes(int64(size * 256)) // approximate bytes per log
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := exp.pushLogData(ctx, ld); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()

			elapsed := b.Elapsed()
			b.ReportMetric(float64(size), "items/op")
			if elapsed > 0 {
				b.ReportMetric(float64(int64(size)*int64(b.N))/elapsed.Seconds(), "items/s")
			}
		})
	}
}

// BenchmarkIntegration_MetricsPush benchmarks real Hologres metric writes.
func BenchmarkIntegration_MetricsPush(b *testing.B) {
	sizes := []int{10, 100, 1000}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("datapoints_%d", size), func(b *testing.B) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			prefix := fmt.Sprintf("bench_metrics_%d_%d", size, time.Now().UnixNano())
			cfg := &Config{
				DSN:              testDSN,
				MetricsTableName: prefix,
				CreateSchema:     true,
				TTL:              24 * time.Hour,
			}

			exp := newMetricsExporter(zap.NewNop(), cfg)
			if err := exp.start(ctx, nil); err != nil {
				b.Fatalf("failed to start metrics exporter: %v", err)
			}
			pool := exp.db.(*hologresPool).pool
			defer func() {
				dropTablesQuiet(ctx, pool,
					cfg.MetricsTableName+"_gauge",
					cfg.MetricsTableName+"_sum",
					cfg.MetricsTableName+"_histogram",
					cfg.MetricsTableName+"_summary",
					cfg.MetricsTableName+"_exp_histogram",
				)
				_ = exp.shutdown(ctx)
			}()

			md := generateMetrics(size)

			b.ReportAllocs()
			b.SetBytes(int64(size * 200)) // approximate bytes per data point
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := exp.pushMetricData(ctx, md); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()

			elapsed := b.Elapsed()
			b.ReportMetric(float64(size), "items/op")
			if elapsed > 0 {
				b.ReportMetric(float64(int64(size)*int64(b.N))/elapsed.Seconds(), "items/s")
			}
		})
	}
}

// dropTablesQuiet and testDSN are defined in exporter_integration_test.go.
// generateTraces, generateLogs, generateMetrics are defined in exporter_benchmark_test.go.
