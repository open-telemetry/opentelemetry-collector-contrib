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

func BenchmarkCompressWriterPoolDefaultCompressionLevel(b *testing.B) {
	payload := bytes.Repeat([]byte("The quick brown fox jumps over the lazy dog. "), 1024)
	benchmarks := []configcompression.Type{
		"none",
		configcompression.TypeGzip,
		configcompression.TypeZlib,
		configcompression.TypeDeflate,
		configcompression.TypeLz4,
		configcompression.TypeSnappy,
		configcompression.TypeSnappyFramed,
		configcompression.TypeZstd,
	}

	factory := NewWriterFactory()
	params := configcompression.CompressionParams{Level: configcompression.DefaultCompressionLevel}
	for _, typ := range benchmarks {
		b.Run(string(typ), func(b *testing.B) {
			b.Run("cached-writer", func(b *testing.B) {
				writer, err := factory.NewWriter(typ, params)
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

			b.Run("new-pool-per-op", func(b *testing.B) {
				b.SetBytes(int64(len(payload)))
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					writer, err := factory.NewWriter(typ, params)
					if err != nil {
						b.Fatal(err)
					}
					n, err := writer.CompressBytes(io.Discard, payload)
					if err != nil || n != int64(len(payload)) {
						b.Fatal("Must not error and read all bytes")
					}
				}
			})
		})
	}
}
