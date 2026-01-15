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
		numAttributes int
	}{
		{"1_log_1_attr", 1, 1},
		{"10_logs_1_attr", 10, 1},
		{"100_logs_1_attr", 100, 1},
		{"100_logs_3_attrs", 100, 3},
		{"1000_logs_1_attr", 1000, 1},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			runLookupBenchmark(b, bm.numLogRecords, bm.numAttributes)
		})
	}
}

func runLookupBenchmark(b *testing.B, numLogRecords, numAttributes int) {
	attrs := make([]AttributeConfig, numAttributes)
	for i := range numAttributes {
		attrs[i] = AttributeConfig{
			Key:           fmt.Sprintf("result.%d", i),
			FromAttribute: fmt.Sprintf("lookup.key.%d", i),
			Default:       "default-value", // noop returns not found, so use default
			Action:        ActionUpsert,
			Context:       ContextRecord,
		}
	}

	factory := NewFactory()
	cfg := &Config{
		Source:     SourceConfig{Type: "noop"},
		Attributes: attrs,
	}

	p, err := factory.CreateLogs(b.Context(), processortest.NewNopSettings(metadata.Type), cfg, dropLogsSink{})
	require.NoError(b, err)

	require.NoError(b, p.Start(b.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(b, p.Shutdown(b.Context()))
	}()

	logs := generateLogs(numLogRecords, numAttributes)

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
		Attributes: []AttributeConfig{
			{
				Key:           "user.name",
				FromAttribute: "user.id",
				Default:       "Unknown",
				Action:        ActionUpsert,
				Context:       ContextRecord,
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

// BenchmarkValueToString benchmarks the value conversion function.
func BenchmarkValueToString(b *testing.B) {
	logs := plog.NewLogs()
	lr := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	b.Run("string", func(b *testing.B) {
		lr.Attributes().PutStr("key", "test-value")
		val, _ := lr.Attributes().Get("key")
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_ = valueToString(val)
		}
	})

	b.Run("int", func(b *testing.B) {
		lr.Attributes().PutInt("key", 12345)
		val, _ := lr.Attributes().Get("key")
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_ = valueToString(val)
		}
	})

	b.Run("double", func(b *testing.B) {
		lr.Attributes().PutDouble("key", 123.456)
		val, _ := lr.Attributes().Get("key")
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_ = valueToString(val)
		}
	})

	b.Run("bool", func(b *testing.B) {
		lr.Attributes().PutBool("key", true)
		val, _ := lr.Attributes().Get("key")
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_ = valueToString(val)
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
