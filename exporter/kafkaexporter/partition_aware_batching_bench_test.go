// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter

import (
	"fmt"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/component/componenttest"
)

func BenchmarkPartitionTracesByID(b *testing.B) {
	for _, traceCount := range []int{1_000, 10_000} {
		cfg := createDefaultConfig().(*Config)
		cfg.PartitionTracesByID = true
		marshaler, err := getTracesMarshaler(cfg.Traces.Encoding, componenttest.NewNopHost())
		if err != nil {
			b.Fatal(err)
		}
		messenger := &kafkaTracesMessenger{config: *cfg, marshaler: marshaler}
		input := tracesWithUniqueIDs(traceCount)

		b.Run(fmt.Sprintf("per_key/traces=%d", traceCount), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				for _, traces := range messenger.partitionData(input) {
					if err := messenger.marshalData(traces, func([]byte, []byte) {}); err != nil {
						b.Fatal(err)
					}
				}
			}
		})

		b.Run(fmt.Sprintf("partition_aware/traces=%d/partitions=8", traceCount), func(b *testing.B) {
			partitioner, err := buildPartitioner(cfg.RecordPartitioner, componenttest.NewNopHost())
			if err != nil {
				b.Fatal(err)
			}
			aware := newPartitionAwarePartitioner(partitioner, cfg.PartitionAwareBatching.TopicCacheSize)
			aware.partitionCounts.Add(cfg.Traces.Topic, 8)

			b.ReportAllocs()
			for b.Loop() {
				batched, err := messenger.partitionAwareRecords(
					b.Context(), input, aware, func(kgo.Record) {},
				)
				if err != nil {
					b.Fatal(err)
				}
				if !batched {
					b.Fatal("partition-aware batching unexpectedly fell back")
				}
			}
		})
	}
}
