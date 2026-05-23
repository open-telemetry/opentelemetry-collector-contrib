// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupprocessor

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/internal/metadata"
)

// BenchmarkProcessorLookup benchmarks the core processor overhead.
// Uses noop source to isolate processor logic from source implementation.
func BenchmarkProcessorLookup(b *testing.B) {
	benchmarks := []struct {
		name          string
		numLogRecords int
		numLookups    int
	}{
		{"1_log_1_lookup", 1, 1},
		{"10_logs_1_lookup", 10, 1},
		{"100_logs_1_lookup", 100, 1},
		{"100_logs_3_lookups", 100, 3},
		{"1000_logs_1_lookup", 1000, 1},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			runLookupBenchmark(b, bm.numLogRecords, bm.numLookups)
		})
	}
}

func runLookupBenchmark(b *testing.B, numLogRecords, numLookups int) {
	lookups := make([]LookupConfig, numLookups)
	for i := range numLookups {
		lookups[i] = LookupConfig{
			Key: fmt.Sprintf(`log.attributes["lookup.key.%d"]`, i),
			Attributes: []AttributeMapping{
				{
					Destination: fmt.Sprintf("result.%d", i),
					Default:     "default-value", // noop returns not found, so default is applied
				},
			},
		}
	}

	factory := NewFactory()
	cfg := &Config{
		Source:  SourceConfig{Type: "noop"},
		Lookups: lookups,
	}

	p, err := factory.CreateLogs(b.Context(), processortest.NewNopSettings(metadata.Type), cfg, dropLogsSink{})
	require.NoError(b, err)

	require.NoError(b, p.Start(b.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(b, p.Shutdown(b.Context()))
	}()

	logs := generateLogs(numLogRecords, numLookups)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		clone := plog.NewLogs()
		logs.CopyTo(clone)
		err := p.ConsumeLogs(b.Context(), clone)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkProcessorThroughput measures parallel throughput.
func BenchmarkProcessorThroughput(b *testing.B) {
	factory := NewFactory()
	cfg := &Config{
		Source: SourceConfig{Type: "noop"},
		Lookups: []LookupConfig{
			{
				Key: `log.attributes["lookup.key.0"]`,
				Attributes: []AttributeMapping{
					{
						Destination: "user.name",
						Default:     "Unknown",
					},
				},
			},
		},
	}

	p, err := factory.CreateLogs(b.Context(), processortest.NewNopSettings(metadata.Type), cfg, dropLogsSink{})
	require.NoError(b, err)

	require.NoError(b, p.Start(b.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(b, p.Shutdown(b.Context()))
	}()

	logs := generateLogs(100, 1)

	b.ReportAllocs()
	b.ResetTimer()
	b.SetParallelism(4)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			clone := plog.NewLogs()
			logs.CopyTo(clone)
			err := p.ConsumeLogs(b.Context(), clone)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkAnyToString benchmarks the value conversion function.
func BenchmarkAnyToString(b *testing.B) {
	b.Run("string", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_ = anyToString("test-value")
		}
	})

	b.Run("int64", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_ = anyToString(int64(12345))
		}
	})

	b.Run("float64", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_ = anyToString(123.456)
		}
	})

	b.Run("bool", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_ = anyToString(true)
		}
	})
}

func generateLogs(numLogRecords, numAttributes int) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "benchmark-service")

	sl := rl.ScopeLogs().AppendEmpty()
	for i := range numLogRecords {
		lr := sl.LogRecords().AppendEmpty()
		lr.Body().SetStr(fmt.Sprintf("Log message %d", i))
		for j := range numAttributes {
			lr.Attributes().PutStr(fmt.Sprintf("lookup.key.%d", j), fmt.Sprintf("value-%d-%d", i, j))
		}
	}
	return logs
}

type dropLogsSink struct{}

func (dropLogsSink) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (dropLogsSink) ConsumeLogs(context.Context, plog.Logs) error {
	return nil
}
