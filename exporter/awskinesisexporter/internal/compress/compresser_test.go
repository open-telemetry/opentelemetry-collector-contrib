// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package compress_test

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/compress"
)

func GzipDecompress(data []byte) ([]byte, error) {
	buf := bytes.NewBuffer(data)

	zr, err := gzip.NewReader(buf)
	if err != nil {
		return nil, err
	}

	out := bytes.Buffer{}
	if _, err = io.Copy(&out, zr); err != nil {
		zr.Close()
		return nil, err
	}
	zr.Close()
	return out.Bytes(), nil
}

func NoopDecompress(data []byte) ([]byte, error) {
	return data, nil
}

func ZlibDecompress(data []byte) ([]byte, error) {
	buf := bytes.NewBuffer(data)

	zr, err := zlib.NewReader(buf)
	if err != nil {
		return nil, err
	}

	out := bytes.Buffer{}
	if _, err = io.Copy(&out, zr); err != nil {
		zr.Close()
		return nil, err
	}
	zr.Close()
	return out.Bytes(), nil
}

func FlateDecompress(data []byte) ([]byte, error) {
	var err error
	buf := bytes.NewBuffer(data)
	zr := flate.NewReader(buf)
	out := bytes.Buffer{}
	if _, err = io.Copy(&out, zr); err != nil {
		zr.Close()
		return nil, err
	}
	zr.Close()
	return out.Bytes(), nil
}

func TestCompressorFormats(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		format     string
		decompress func(data []byte) ([]byte, error)
	}{
		{format: "none", decompress: NoopDecompress},
		{format: "noop", decompress: NoopDecompress},
		{format: "gzip", decompress: GzipDecompress},
		{format: "zlib", decompress: ZlibDecompress},
		{format: "flate", decompress: FlateDecompress},
	}

	const data = "You know nothing Jon Snow"
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("format_%s", tc.format), func(t *testing.T) {
			c, err := compress.NewCompressor(tc.format)
			require.NoError(t, err, "Must have a valid compression format")
			require.NotNil(t, c, "Must have a valid compressor")

			out, err := c.Do([]byte(data))
			assert.NoError(t, err, "Must not error when processing data")
			assert.NotNil(t, out, "Must have a valid record")
			outDecompress, err := tc.decompress(out)
			assert.NoError(t, err, "Decompression must have no errors")
			assert.Equal(t, []byte(data), outDecompress, "Data input should be the same after compression and decompression")

		})
	}
	_, err := compress.NewCompressor("invalid-format")
	assert.Error(t, err, "Must error when an invalid compression format is given")
}

func BenchmarkNoopCompressor_1000Bytes(b *testing.B) {
	benchmarkCompressor(b, "none", 1000)
}

func BenchmarkNoopCompressor_1Mb(b *testing.B) {
	benchmarkCompressor(b, "noop", 131072)
}

func BenchmarkZlibCompressor_1000Bytes(b *testing.B) {
	benchmarkCompressor(b, "zlib", 1000)
}

func BenchmarkZlibCompressor_1Mb(b *testing.B) {
	benchmarkCompressor(b, "zlib", 131072)
}

func BenchmarkFlateCompressor_1000Bytes(b *testing.B) {
	benchmarkCompressor(b, "flate", 1000)
}

func BenchmarkFlateCompressor_1Mb(b *testing.B) {
	benchmarkCompressor(b, "flate", 131072)
}

func BenchmarkGzipCompressor_1000Bytes(b *testing.B) {
	benchmarkCompressor(b, "gzip", 1000)
}

func BenchmarkGzipCompressor_1Mb(b *testing.B) {
	benchmarkCompressor(b, "gzip", 131072)
}

func benchmarkCompressor(b *testing.B, format string, len int) {
	b.Helper()

	source := rand.NewSource(time.Now().UnixMilli())
	genRand := rand.New(source)

	compressor, err := compress.NewCompressor(format)
	require.NoError(b, err, "Must not error when given a valid format")
	require.NotNil(b, compressor, "Must have a valid compressor")

	data := make([]byte, len)
	for i := 0; i < len; i++ {
		data[i] = byte(genRand.Int31())
	}
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		out, err := compressor.Do(data)
		assert.NoError(b, err, "Must not error when processing data")
		assert.NotNil(b, out, "Must have a valid byte array after")
	}
}
