// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package marshaler

import (
	"fmt"
	"testing"

	"go.opentelemetry.io/collector/pdata/plog"
)

// Data sizes to benchmark
var benchmarkSizes = []int{1_000, 5_000, 10_000, 50_000, 100_000, 500_000, 1_000_000}

// generateLogs creates plog.Logs with the specified number of log records
func generateLogs(count int) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "benchmark-service")
	rl.Resource().Attributes().PutStr("host.name", "benchmark-host")

	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("benchmark-scope")
	sl.Scope().SetVersion("1.0.0")

	for i := 0; i < count; i++ {
		lr := sl.LogRecords().AppendEmpty()
		lr.SetTimestamp(1234567890000000000)
		lr.SetObservedTimestamp(1234567890000000000)
		lr.SetSeverityNumber(plog.SeverityNumberInfo)
		lr.SetSeverityText("INFO")
		lr.Body().SetStr(fmt.Sprintf("Log message %d with some additional content to make it more realistic", i))
		lr.Attributes().PutStr("key1", "value1")
		lr.Attributes().PutStr("key2", "value2")
		lr.Attributes().PutInt("counter", int64(i))
		lr.Attributes().PutBool("enabled", true)
		lr.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
		lr.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	}
	return logs
}

// ============================================================================
// Individual Logs Marshaler Benchmarks
// ============================================================================

func BenchmarkPdataLogsMarshaler_Proto(b *testing.B) {
	marshaler := NewPdataLogsMarshaler(&plog.ProtoMarshaler{})
	for _, size := range benchmarkSizes {
		logs := generateLogs(size)
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := marshaler.MarshalLogs(logs)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkPdataLogsMarshaler_JSON(b *testing.B) {
	marshaler := NewPdataLogsMarshaler(&plog.JSONMarshaler{})
	for _, size := range benchmarkSizes {
		logs := generateLogs(size)
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := marshaler.MarshalLogs(logs)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkRawLogsMarshaler(b *testing.B) {
	marshaler := RawLogsMarshaler{}
	for _, size := range benchmarkSizes {
		logs := generateLogs(size)
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := marshaler.MarshalLogs(logs)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkOpenSearchLogsMarshaler(b *testing.B) {
	marshaler := &OpenSearchLogsMarshaler{}
	for _, size := range benchmarkSizes {
		logs := generateLogs(size)
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := marshaler.MarshalLogs(logs)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkOpenSearchLogsMarshaler_UnixTimestamps(b *testing.B) {
	marshaler := &OpenSearchLogsMarshaler{unixTimestamps: true}
	for _, size := range benchmarkSizes {
		logs := generateLogs(size)
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := marshaler.MarshalLogs(logs)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// ============================================================================
// Combined Comparison Benchmark
// ============================================================================

// BenchmarkAllLogsMarshalers runs all logs marshalers for easy comparison
func BenchmarkAllLogsMarshalers(b *testing.B) {
	marshalers := map[string]LogsMarshaler{
		"PdataProto":       NewPdataLogsMarshaler(&plog.ProtoMarshaler{}),
		"PdataJSON":        NewPdataLogsMarshaler(&plog.JSONMarshaler{}),
		"Raw":              RawLogsMarshaler{},
		"OpenSearch":       &OpenSearchLogsMarshaler{},
		"OpenSearchUnixTs": &OpenSearchLogsMarshaler{unixTimestamps: true},
	}

	for name, marshaler := range marshalers {
		for _, size := range benchmarkSizes {
			logs := generateLogs(size)
			b.Run(fmt.Sprintf("marshaler=%s/size=%d", name, size), func(b *testing.B) {
				b.ResetTimer()
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					_, err := marshaler.MarshalLogs(logs)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

// ============================================================================
// Batch Handling Benchmarks for OpenSearchLogsMarshaler
// ============================================================================

// batchConfig defines the structure of a batch for benchmarking
type batchConfig struct {
	name               string
	numResources       int
	numScopesPerRes    int
	numRecordsPerScope int
}

// generateBatchLogs creates plog.Logs with the specified batch configuration
func generateBatchLogs(cfg batchConfig) plog.Logs {
	logs := plog.NewLogs()

	for r := 0; r < cfg.numResources; r++ {
		rl := logs.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", fmt.Sprintf("service-%d", r))
		rl.Resource().Attributes().PutStr("host.name", fmt.Sprintf("host-%d", r))
		rl.Resource().Attributes().PutStr("deployment.environment", "production")
		rl.Resource().Attributes().PutInt("resource.index", int64(r))

		for s := 0; s < cfg.numScopesPerRes; s++ {
			sl := rl.ScopeLogs().AppendEmpty()
			sl.Scope().SetName(fmt.Sprintf("scope-%d-%d", r, s))
			sl.Scope().SetVersion("1.0.0")
			sl.Scope().Attributes().PutStr("library.language", "go")
			sl.SetSchemaUrl("https://opentelemetry.io/schemas/1.21.0")

			for rec := 0; rec < cfg.numRecordsPerScope; rec++ {
				lr := sl.LogRecords().AppendEmpty()
				lr.SetTimestamp(1234567890000000000)
				lr.SetObservedTimestamp(1234567890000000000)
				lr.SetSeverityNumber(plog.SeverityNumberInfo)
				lr.SetSeverityText("INFO")
				lr.Body().SetStr(fmt.Sprintf("Log message from resource %d, scope %d, record %d", r, s, rec))
				lr.Attributes().PutStr("key1", "value1")
				lr.Attributes().PutStr("key2", "value2")
				lr.Attributes().PutInt("resource.index", int64(r))
				lr.Attributes().PutInt("scope.index", int64(s))
				lr.Attributes().PutInt("record.index", int64(rec))
				lr.Attributes().PutBool("enabled", true)

				// Add trace context to some records (every 3rd record)
				if rec%3 == 0 {
					lr.SetTraceID([16]byte{byte(r), byte(s), byte(rec), 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
					lr.SetSpanID([8]byte{byte(r), byte(s), byte(rec), 4, 5, 6, 7, 8})
				}
			}
		}
	}
	return logs
}

// BenchmarkOpenSearchLogsMarshalerBatchConfigurations benchmarks different batch structures
func BenchmarkOpenSearchLogsMarshalerBatchConfigurations(b *testing.B) {
	marshaler := &OpenSearchLogsMarshaler{}

	// Different batch configurations to test various scenarios
	configs := []batchConfig{
		// 500k total logs - various configurations
		{"10res_10scopes_5000rec", 10, 10, 5000}, // 10 * 10 * 5000 = 500,000
		{"50res_10scopes_1000rec", 50, 10, 1000}, // 50 * 10 * 1000 = 500,000
		{"100res_50scopes_100rec", 100, 50, 100}, // 100 * 50 * 100 = 500,000
		{"1res_1scope_500000rec", 1, 1, 500000},  // 1 * 1 * 500000 = 500,000

		// 1M total logs - various configurations
		{"10res_10scopes_10000rec", 10, 10, 10000}, // 10 * 10 * 10000 = 1,000,000
		{"100res_10scopes_1000rec", 100, 10, 1000}, // 100 * 10 * 1000 = 1,000,000
		{"100res_100scopes_100rec", 100, 100, 100}, // 100 * 100 * 100 = 1,000,000
		{"1res_1scope_1000000rec", 1, 1, 1000000},  // 1 * 1 * 1000000 = 1,000,000
	}

	for _, cfg := range configs {
		logs := generateBatchLogs(cfg)
		totalRecords := cfg.numResources * cfg.numScopesPerRes * cfg.numRecordsPerScope

		b.Run(fmt.Sprintf("%s_total=%d", cfg.name, totalRecords), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				messages, err := marshaler.MarshalLogs(logs)
				if err != nil {
					b.Fatal(err)
				}
				// Verify correct number of messages
				if len(messages) != totalRecords {
					b.Fatalf("expected %d messages, got %d", totalRecords, len(messages))
				}
			}
		})
	}
}

// BenchmarkOpenSearchLogsMarshalerBatchScaling benchmarks how the marshaler scales
// with increasing batch sizes while keeping the structure constant
func BenchmarkOpenSearchLogsMarshalerBatchScaling(b *testing.B) {
	marshaler := &OpenSearchLogsMarshaler{}

	// Scale by increasing records per scope (10 resources x 10 scopes x N records)
	// This reaches 500k at 5000 records and 1M at 10000 records
	b.Run("ScaleByRecords", func(b *testing.B) {
		recordCounts := []int{100, 1000, 5000, 10000} // 10k, 100k, 500k, 1M total
		for _, count := range recordCounts {
			cfg := batchConfig{
				name:               fmt.Sprintf("records_%d", count),
				numResources:       10,
				numScopesPerRes:    10,
				numRecordsPerScope: count,
			}
			logs := generateBatchLogs(cfg)
			totalRecords := cfg.numResources * cfg.numScopesPerRes * cfg.numRecordsPerScope

			b.Run(fmt.Sprintf("records=%d_total=%d", count, totalRecords), func(b *testing.B) {
				b.ResetTimer()
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					_, err := marshaler.MarshalLogs(logs)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	})

	// Scale by increasing resources (N resources x 10 scopes x 1000 records)
	// This reaches 500k at 50 resources and 1M at 100 resources
	b.Run("ScaleByResources", func(b *testing.B) {
		resourceCounts := []int{1, 10, 50, 100} // 10k, 100k, 500k, 1M total
		for _, count := range resourceCounts {
			cfg := batchConfig{
				name:               fmt.Sprintf("resources_%d", count),
				numResources:       count,
				numScopesPerRes:    10,
				numRecordsPerScope: 1000,
			}
			logs := generateBatchLogs(cfg)
			totalRecords := cfg.numResources * cfg.numScopesPerRes * cfg.numRecordsPerScope

			b.Run(fmt.Sprintf("resources=%d_total=%d", count, totalRecords), func(b *testing.B) {
				b.ResetTimer()
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					_, err := marshaler.MarshalLogs(logs)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	})

	// Scale by increasing scopes (10 resources x N scopes x 1000 records)
	// This reaches 500k at 50 scopes and 1M at 100 scopes
	b.Run("ScaleByScopes", func(b *testing.B) {
		scopeCounts := []int{1, 10, 50, 100} // 10k, 100k, 500k, 1M total
		for _, count := range scopeCounts {
			cfg := batchConfig{
				name:               fmt.Sprintf("scopes_%d", count),
				numResources:       10,
				numScopesPerRes:    count,
				numRecordsPerScope: 1000,
			}
			logs := generateBatchLogs(cfg)
			totalRecords := cfg.numResources * cfg.numScopesPerRes * cfg.numRecordsPerScope

			b.Run(fmt.Sprintf("scopes=%d_total=%d", count, totalRecords), func(b *testing.B) {
				b.ResetTimer()
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					_, err := marshaler.MarshalLogs(logs)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	})
}

// BenchmarkOpenSearchLogsMarshalerBatchVsSingle compares processing many small batches
// versus fewer large batches with the same total number of records
func BenchmarkOpenSearchLogsMarshalerBatchVsSingle(b *testing.B) {
	marshaler := &OpenSearchLogsMarshaler{}
	totalRecords := 10000

	// Single large batch
	b.Run("SingleBatch", func(b *testing.B) {
		logs := generateBatchLogs(batchConfig{
			numResources:       1,
			numScopesPerRes:    1,
			numRecordsPerScope: totalRecords,
		})
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := marshaler.MarshalLogs(logs)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	// Many small batches (simulated as separate calls)
	b.Run("ManySmallBatches_100x100", func(b *testing.B) {
		batches := make([]plog.Logs, 100)
		for i := 0; i < 100; i++ {
			batches[i] = generateBatchLogs(batchConfig{
				numResources:       1,
				numScopesPerRes:    1,
				numRecordsPerScope: 100,
			})
		}
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			for _, batch := range batches {
				_, err := marshaler.MarshalLogs(batch)
				if err != nil {
					b.Fatal(err)
				}
			}
		}
	})

	// Medium batches
	b.Run("MediumBatches_10x1000", func(b *testing.B) {
		batches := make([]plog.Logs, 10)
		for i := 0; i < 10; i++ {
			batches[i] = generateBatchLogs(batchConfig{
				numResources:       1,
				numScopesPerRes:    1,
				numRecordsPerScope: 1000,
			})
		}
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			for _, batch := range batches {
				_, err := marshaler.MarshalLogs(batch)
				if err != nil {
					b.Fatal(err)
				}
			}
		}
	})
}

// BenchmarkOpenSearchLogsMarshaler_TimestampFormats compares ISO8601 vs Unix timestamps
// across different batch sizes
func BenchmarkOpenSearchLogsMarshaler_TimestampFormats(b *testing.B) {
	batchSizes := []int{100, 1000, 10000}

	for _, size := range batchSizes {
		logs := generateBatchLogs(batchConfig{
			numResources:       5,
			numScopesPerRes:    4,
			numRecordsPerScope: size / 20, // Distribute across structure
		})

		b.Run(fmt.Sprintf("ISO8601_size=%d", size), func(b *testing.B) {
			marshaler := &OpenSearchLogsMarshaler{unixTimestamps: false}
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := marshaler.MarshalLogs(logs)
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(fmt.Sprintf("UnixMilli_size=%d", size), func(b *testing.B) {
			marshaler := &OpenSearchLogsMarshaler{unixTimestamps: true}
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := marshaler.MarshalLogs(logs)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
