// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xcompression

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configcompression"
)

func BenchmarkDecompressReaderPoolDefaultCompressionLevel(b *testing.B) {
	payload := bytes.Repeat([]byte("The quick brown fox jumps over the lazy dog. "), 1024)
	benchmarks := []configcompression.Type{
		"none",
		configcompression.TypeGzip,
		configcompression.TypeZlib,
		configcompression.TypeDeflate,
		configcompression.TypeLz4,
		configcompression.TypeSnappy,
		configcompression.TypeZstd,
	}

	writerFactory := NewWriterFactory()
	readerFactory := NewReaderFactory()
	params := configcompression.CompressionParams{Level: configcompression.DefaultCompressionLevel}
	for _, typ := range benchmarks {
		b.Run(string(typ), func(b *testing.B) {
			writer, err := writerFactory.NewWriter(typ, params)
			require.NoError(b, err)

			var compressed bytes.Buffer
			written, err := writer.CompressBytes(&compressed, payload)
			require.NoError(b, err)
			require.Equal(b, int64(len(payload)), written)

			b.Run("cached-reader", func(b *testing.B) {
				reader, err := readerFactory.NewReader(typ, params)
				require.NoError(b, err)

				b.SetBytes(int64(len(payload)))
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					n, err := reader.DecompressBytes(io.Discard, compressed.Bytes())
					if err != nil || n != int64(len(payload)) {
						b.Fatal("Must not error and decompress all bytes")
					}
				}
			})

			b.Run("new-pool-per-op", func(b *testing.B) {
				b.SetBytes(int64(len(payload)))
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					reader, err := readerFactory.NewReader(typ, params)
					if err != nil {
						b.Fatal(err)
					}
					n, err := reader.DecompressBytes(io.Discard, compressed.Bytes())
					if err != nil || n != int64(len(payload)) {
						b.Fatal("Must not error and decompress all bytes")
					}
				}
			})
		})
	}
}
