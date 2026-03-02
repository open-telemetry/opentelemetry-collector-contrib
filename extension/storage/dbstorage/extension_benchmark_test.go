// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dbstorage

import (
	"context"
	"crypto/rand"
	"fmt"
	"path/filepath"
	"strconv"
	"testing"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/xextension/storage"
)

const testBenchRecordSize = 10

type testBatchBenchmark struct {
	name        string
	backend     string
	singleBatch bool
	records     int
}

func BenchmarkBatchGet(b *testing.B) {
	backends := getBatchBenchExtensions(b, "Get")
	benchmarks := generateBatchBenchmarks(backends)

	for _, bench := range benchmarks {
		b.Run(fmt.Sprintf("backend=%s/op=%s/rows=%d", bench.backend, bench.name, bench.records), func(b *testing.B) {
			client, err := backends[bench.backend].GetClient(b.Context(), component.KindExporter, newTestEntity(bench.name), fmt.Sprintf("%d", bench.records))
			if err != nil {
				b.Fatal(err)
			}
			defer client.Close(b.Context())

			// Populate records and create set of Get Operations
			ops := getBatchBenchmarkOps(storage.Get, bench.records, bench.singleBatch)

			// Reset benchmark timer
			b.ResetTimer()

			// Run Benchmark
			for b.Loop() {
				if err := client.Batch(b.Context(), ops...); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkBatchSet(b *testing.B) {
	backends := getBatchBenchExtensions(b, "Set")
	benchmarks := generateBatchBenchmarks(backends)

	for _, bench := range benchmarks {
		b.Run(fmt.Sprintf("backend=%s/op=%s/rows=%d", bench.backend, bench.name, bench.records), func(b *testing.B) {
			client, err := backends[bench.backend].GetClient(b.Context(), component.KindExporter, newTestEntity(bench.name), fmt.Sprintf("%d", bench.records))
			if err != nil {
				b.Fatal(err)
			}
			defer client.Close(b.Context())

			// Populate records and create set of Get Operations
			ops := getBatchBenchmarkOps(storage.Get, bench.records, bench.singleBatch)

			// Reset benchmark timer
			b.ResetTimer()

			// Run Benchmark
			for b.Loop() {
				if err := client.Batch(b.Context(), ops...); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkBatchDelete(b *testing.B) {
	backends := getBatchBenchExtensions(b, "Delete")
	benchmarks := generateBatchBenchmarks(backends)

	for _, bench := range benchmarks {
		b.Run(fmt.Sprintf("backend=%s/op=%s/rows=%d", bench.backend, bench.name, bench.records), func(b *testing.B) {
			client, err := backends[bench.backend].GetClient(b.Context(), component.KindExporter, newTestEntity(bench.name), fmt.Sprintf("%d", bench.records))
			if err != nil {
				b.Fatal(err)
			}
			defer client.Close(b.Context())

			// Populate records and create set of Get Operations
			ops := getBatchBenchmarkOps(storage.Get, bench.records, bench.singleBatch)

			// Reset benchmark timer
			b.ResetTimer()

			// Run Benchmark
			for b.Loop() {
				if err := client.Batch(b.Context(), ops...); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func getBatchBenchmarkOps(op storage.OpType, opsCount int, isSingleBatch bool) []*storage.Operation {
	// Create set of operations to run in Benchmark
	ops := make([]*storage.Operation, 0, opsCount)
	// if it's multi-query bench - prepend single different operation
	if !isSingleBatch {
		opType := storage.Delete
		if op == opType {
			opType = storage.Get
		}
		ops = append(ops, &storage.Operation{
			Type: opType,
			Key:  "-1",
		})
	}
	for r := range opsCount {
		var value []byte
		key := strconv.Itoa(r)
		if op == storage.Set {
			value = randBytes(testBenchRecordSize)
		}

		ops = append(ops, &storage.Operation{
			Type:  op,
			Key:   key,
			Value: value,
		})
	}

	return ops
}

func generateBatchBenchmarks(backends map[string]storage.Extension) []testBatchBenchmark {
	benchmarks := []testBatchBenchmark{}

	for d := range backends {
		for _, n := range []int{2, 10, 100, 200, 500, 1000} {
			for _, bt := range []string{"MultiQuery", "SingleQuery"} {
				benchmarks = append(benchmarks,
					testBatchBenchmark{
						name:        bt,
						backend:     d,
						singleBatch: bt == "SingleQuery",
						records:     n,
					},
				)
			}
		}
	}

	return benchmarks
}

func getBatchBenchExtensions(b *testing.B, batchType string) map[string]storage.Extension {
	backends := map[string]storage.Extension{}

	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, fmt.Sprintf("benchBatch%s.sqlite", batchType))
	seSQLite, err := newSqliteTestExtension(dbPath)
	if err != nil {
		b.Fatal(err)
	}
	backends[driverSQLite] = seSQLite

	sePostgreSQL, ctr, err := newPostgresTestExtension()
	b.Cleanup(func() {
		if ctrErr := ctr.Terminate(context.Background()); ctrErr != nil { //nolint:usetesting
			b.Fatal(ctrErr)
		}
	})
	if err != nil {
		b.Fatal(err)
	}
	backends[driverPostgreSQL] = sePostgreSQL

	for _, se := range backends {
		err = se.Start(b.Context(), componenttest.NewNopHost())
		if err != nil {
			b.Fatal(err)
		}
		b.Cleanup(func() {
			err = se.Shutdown(context.Background()) //nolint:usetesting
			if err != nil {
				b.Fatal(err)
			}
		})
	}

	return backends
}

func randBytes(n int) []byte {
	const alphanum = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

	bytes := make([]byte, n)
	//nolint:errcheck
	rand.Read(bytes)
	for i, b := range bytes {
		bytes[i] = alphanum[b%byte(len(alphanum))]
	}

	return bytes
}
