// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package checkpoint

import (
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/flush"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/tokenlen"
)

// createBenchmarkMetadata creates realistic metadata for benchmarking
func createBenchmarkMetadata(count int) []*reader.Metadata {
	rmds := make([]*reader.Metadata, count)
	now := time.Now()

	for i := range count {
		// Create a realistic fingerprint (1KB)
		fpBytes := make([]byte, fingerprint.DefaultSize)
		for j := range fpBytes {
			fpBytes[j] = byte((i + j) % 256)
		}

		rmds[i] = &reader.Metadata{
			Fingerprint: fingerprint.New(fpBytes),
			Offset:      int64(i * 1024),
			RecordNum:   int64(i * 100),
			FileAttributes: map[string]any{
				"log.file.name": "app.log",
				"log.file.path": "/var/log/app/app.log",
				"host.name":     "server-001",
				"service.name":  "my-service",
				"environment":   "production",
			},
			HeaderFinalized: true,
			FlushState: flush.State{
				LastDataChange: now,
				LastDataLength: 4096,
			},
			TokenLenState: tokenlen.State{
				MinimumLength: 256,
			},
			FileType: ".log",
		}
	}

	return rmds
}

// BenchmarkCheckpointEncoding benchmarks checkpoint encoding
// Toggle protobuf encoding: false = JSON (default), true = Protobuf
func BenchmarkCheckpointEncoding(b *testing.B) {
	// Toggle protobuf encoding: false = JSON (default), true = Protobuf
	setProtobufEncoding(b, true)

	benchmarks := []struct {
		name  string
		count int
	}{
		{"1_file", 1},
		{"10_files", 10},
		{"100_files", 100},
		{"1000_files", 1000},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			rmds := createBenchmarkMetadata(bm.count)
			ctx := b.Context()
			persister := testutil.NewUnscopedMockPersister()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				if err := Save(ctx, persister, rmds); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkCheckpointDecoding benchmarks checkpoint decoding
// Toggle protobuf encoding: false = JSON (default), true = Protobuf
func BenchmarkCheckpointDecoding(b *testing.B) {
	// Toggle protobuf encoding: false = JSON (default), true = Protobuf
	setProtobufEncoding(b, true)

	benchmarks := []struct {
		name  string
		count int
	}{
		{"1_file", 1},
		{"10_files", 10},
		{"100_files", 100},
		{"1000_files", 1000},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			rmds := createBenchmarkMetadata(bm.count)
			ctx := b.Context()
			persister := testutil.NewUnscopedMockPersister()

			// Setup: save once
			if err := Save(ctx, persister, rmds); err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				if _, err := Load(ctx, persister); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkCheckpointRoundTrip benchmarks full encode/decode cycle
// Toggle protobuf encoding: false = JSON (default), true = Protobuf
func BenchmarkCheckpointRoundTrip(b *testing.B) {
	// Toggle protobuf encoding: false = JSON (default), true = Protobuf
	setProtobufEncoding(b, true)

	benchmarks := []struct {
		name  string
		count int
	}{
		{"1_file", 1},
		{"10_files", 10},
		{"100_files", 100},
		{"1000_files", 1000},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			rmds := createBenchmarkMetadata(bm.count)
			ctx := b.Context()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				persister := testutil.NewUnscopedMockPersister()
				if err := Save(ctx, persister, rmds); err != nil {
					b.Fatal(err)
				}
				if _, err := Load(ctx, persister); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
