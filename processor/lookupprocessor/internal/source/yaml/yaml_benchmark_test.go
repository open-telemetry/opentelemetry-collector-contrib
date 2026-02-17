// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package yaml

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"
)

func BenchmarkYAMLSourceLookup(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("entries_%d", size), func(b *testing.B) {
			source := createBenchmarkSource(b, size)
			keys := generateKeys(size, 100)

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				for _, key := range keys {
					_, _, _ = source.Lookup(b.Context(), key)
				}
			}
		})
	}
}

func BenchmarkYAMLSourceLookupParallel(b *testing.B) {
	source := createBenchmarkSource(b, 1000)
	keys := generateKeys(1000, 100)

	b.ReportAllocs()
	b.ResetTimer()
	b.SetParallelism(4)
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_, _, _ = source.Lookup(b.Context(), keys[i%len(keys)])
			i++
		}
	})
}

func createBenchmarkSource(b *testing.B, numEntries int) lookupsource.Source {
	b.Helper()

	tmpDir := b.TempDir()
	yamlPath := filepath.Join(tmpDir, "mappings.yaml")

	var content strings.Builder
	for i := range numEntries {
		fmt.Fprintf(&content, "key%d: value%d\n", i, i)
	}
	err := os.WriteFile(yamlPath, []byte(content.String()), 0o600)
	require.NoError(b, err)

	factory := NewFactory()
	cfg := &Config{Path: yamlPath}

	source, err := factory.CreateSource(b.Context(), lookupsource.CreateSettings{}, cfg)
	require.NoError(b, err)

	err = source.Start(b.Context(), componenttest.NewNopHost())
	require.NoError(b, err)

	b.Cleanup(func() {
		_ = source.Shutdown(b.Context())
	})

	return source
}

func generateKeys(mapSize, numKeys int) []string {
	keys := make([]string, numKeys)
	for i := range numKeys {
		keys[i] = fmt.Sprintf("key%d", i%mapSize)
	}
	return keys
}
