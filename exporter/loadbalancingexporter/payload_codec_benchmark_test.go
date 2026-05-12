// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"runtime"
	"runtime/debug"
	"testing"

	"go.opentelemetry.io/collector/pdata/plog"
)

var queuePayloadCodecBenchmarkSink []byte

const benchmarkBytesPerMiB = 1024 * 1024

func BenchmarkQueuePayloadCodecZstdOptions(b *testing.B) {
	payload, err := (&plog.ProtoMarshaler{}).MarshalLogs(compressibleLogs(16000, 1024))
	if err != nil {
		b.Fatal(err)
	}

	tests := []struct {
		name                string
		zstd                ZstdPayloadCodecConfig
		maxLiveHeapAllocMiB float64
	}{
		{name: "default"},
		{
			name: "concurrency_1_window_2MiB",
			zstd: ZstdPayloadCodecConfig{
				EncoderConcurrency: 1,
				WindowSize:         2 << 20,
			},
			maxLiveHeapAllocMiB: 12,
		},
		{
			name: "concurrency_2_window_2MiB",
			zstd: ZstdPayloadCodecConfig{
				EncoderConcurrency: 2,
				WindowSize:         2 << 20,
			},
			maxLiveHeapAllocMiB: 14,
		},
		{
			name: "concurrency_1_window_2MiB_lowmem",
			zstd: ZstdPayloadCodecConfig{
				EncoderConcurrency: 1,
				WindowSize:         2 << 20,
				LowerEncoderMem:    true,
			},
			maxLiveHeapAllocMiB: 8,
		},
		{
			name: "concurrency_2_window_2MiB_lowmem",
			zstd: ZstdPayloadCodecConfig{
				EncoderConcurrency: 2,
				WindowSize:         2 << 20,
				LowerEncoderMem:    true,
			},
			maxLiveHeapAllocMiB: 10,
		},
		{
			name: "concurrency_1_window_1MiB",
			zstd: ZstdPayloadCodecConfig{
				EncoderConcurrency: 1,
				WindowSize:         1 << 20,
			},
			maxLiveHeapAllocMiB: 8,
		},
		{
			name: "concurrency_2_window_1MiB",
			zstd: ZstdPayloadCodecConfig{
				EncoderConcurrency: 2,
				WindowSize:         1 << 20,
			},
			maxLiveHeapAllocMiB: 10,
		},
		{
			name: "concurrency_1_window_1MiB_lowmem",
			zstd: ZstdPayloadCodecConfig{
				EncoderConcurrency: 1,
				WindowSize:         1 << 20,
				LowerEncoderMem:    true,
			},
			maxLiveHeapAllocMiB: 6,
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			codec, stats := newInitializedBenchmarkQueuePayloadCodec(b, tt.zstd, payload)
			b.Cleanup(func() {
				if err := codec.Close(); err != nil {
					b.Fatal(err)
				}
			})
			b.ReportAllocs()
			b.SetBytes(int64(len(payload)))
			b.ResetTimer()

			for range b.N {
				encoded, err := codec.Encode(payload)
				if err != nil {
					b.Fatal(err)
				}
				queuePayloadCodecBenchmarkSink = encoded
			}
			b.StopTimer()
			liveHeapAllocMiB := float64(stats.liveHeapAllocBytes) / benchmarkBytesPerMiB
			if tt.maxLiveHeapAllocMiB > 0 && liveHeapAllocMiB > tt.maxLiveHeapAllocMiB {
				b.Fatalf(
					"bounded zstd codec retained heap = %.2f MiB, want <= %.2f MiB",
					liveHeapAllocMiB,
					tt.maxLiveHeapAllocMiB,
				)
			}
			b.ReportMetric(float64(stats.firstEncodeAllocBytes)/benchmarkBytesPerMiB, "init_alloc_MiB/op")
			b.ReportMetric(liveHeapAllocMiB, "live_heap_alloc_MiB")
			b.ReportMetric(float64(stats.liveHeapInuseBytes)/benchmarkBytesPerMiB, "live_heap_inuse_MiB")
			b.ReportMetric(float64(stats.encodedBytes)/1024, "encoded_KiB")
		})
	}
}

type benchmarkQueuePayloadCodecStats struct {
	firstEncodeAllocBytes uint64
	liveHeapAllocBytes    uint64
	liveHeapInuseBytes    uint64
	encodedBytes          int
}

func newInitializedBenchmarkQueuePayloadCodec(
	b *testing.B,
	zstd ZstdPayloadCodecConfig,
	payload []byte,
) (*queuePayloadCodec, benchmarkQueuePayloadCodecStats) {
	b.Helper()

	queuePayloadCodecBenchmarkSink = nil
	debug.FreeOSMemory()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd, zstd)
	encoded, err := codec.Encode(payload)
	if err != nil {
		b.Fatal(err)
	}
	encodedBytes := len(encoded)
	queuePayloadCodecBenchmarkSink = nil
	encoded = nil

	debug.FreeOSMemory()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(codec)

	stats := benchmarkQueuePayloadCodecStats{
		encodedBytes: encodedBytes,
	}
	if after.TotalAlloc > before.TotalAlloc {
		stats.firstEncodeAllocBytes = after.TotalAlloc - before.TotalAlloc
	}
	if after.HeapAlloc > before.HeapAlloc {
		stats.liveHeapAllocBytes = after.HeapAlloc - before.HeapAlloc
	}
	if after.HeapInuse > before.HeapInuse {
		stats.liveHeapInuseBytes = after.HeapInuse - before.HeapInuse
	}
	return codec, stats
}
