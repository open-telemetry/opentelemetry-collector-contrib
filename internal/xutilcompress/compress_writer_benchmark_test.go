// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xutilcompress

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configcompression"
)

func BenchmarkCompressWriterPoolDefaultCompressionLevel(b *testing.B) {
	payload := bytes.Repeat([]byte("The quick brown fox jumps over the lazy dog. "), 1024)
	benchmarks := []struct {
		name   configcompression.Type
		params configcompression.CompressionParams
	}{
		{name: "none"},
		{name: configcompression.TypeGzip},
		{name: configcompression.TypeZlib},
		{name: configcompression.TypeDeflate},
		{name: configcompression.TypeLz4},
		{name: configcompression.TypeSnappy},
		{name: configcompression.TypeSnappyFramed},
		{name: configcompression.TypeZstd},
	}

	factory := NewWriterFactory()
	for _, benchmark := range benchmarks {
		b.Run(string(benchmark.name), func(b *testing.B) {
			writer, err := factory.Get(benchmark.name, configcompression.CompressionParams{Level: configcompression.DefaultCompressionLevel})
			require.NoError(b, err)

			b.SetBytes(int64(len(payload)))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				n, err := writer.CompressBytes(io.Discard, payload)
				if err != nil || n != int64(len(payload)) {
					b.Fatal("Must not error and read all bytes")
				}
			}
		})
	}
}
