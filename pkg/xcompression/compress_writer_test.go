// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xcompression

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"io"
	"testing"

	"github.com/klauspost/compress/snappy"
	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configcompression"
)

func TestNewCompressorWriterPools(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		constructor WriterConstructionFunc
		params      configcompression.CompressionParams
		errVal      string
	}{
		{
			name:        "gzip default level",
			constructor: newGZIPCompressWriterPool,
			params:      configcompression.CompressionParams{Level: configcompression.DefaultCompressionLevel},
		},
		{
			name:        "gzip invalid level",
			constructor: newGZIPCompressWriterPool,
			params:      configcompression.CompressionParams{Level: 99},
			errVal:      "gzip: invalid compression level: 99",
		},
		{
			name:        "zlib default level",
			constructor: newZLibCompressWriterPool,
			params:      configcompression.CompressionParams{Level: configcompression.DefaultCompressionLevel},
		},
		{
			name:        "zlib invalid level",
			constructor: newZLibCompressWriterPool,
			params:      configcompression.CompressionParams{Level: 99},
			errVal:      "zlib: invalid compression level: 99",
		},
		{
			name:        "flate default level",
			constructor: newFlateCompressWriterPool,
			params:      configcompression.CompressionParams{Level: configcompression.DefaultCompressionLevel},
		},
		{
			name:        "flate invalid level",
			constructor: newFlateCompressWriterPool,
			params:      configcompression.CompressionParams{Level: 99},
			errVal:      "flate: invalid compression level 99: want value in range [-2, 9]",
		},
		{
			name:        "lz4",
			constructor: newLZ4CompressWriterPool,
		},
		{
			name:        "zstd",
			constructor: newZSTDCompressWriterPool,
			params:      configcompression.CompressionParams{Level: 1},
		},
		{
			name:        "snappy",
			constructor: newSnappyCompressWriterPool,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cw, err := tc.constructor(tc.params)
			if tc.errVal != "" {
				assert.EqualError(t, err, tc.errVal, "Must match the expected value")
				assert.Nil(t, cw, "Must have a nil compress writer")
			} else {
				assert.NoError(t, err, "Must not return an error")
				assert.NotNil(t, cw, "Must have a valid compress writer")
			}
		})
	}
}

func TestCompressWriter(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		constructor WriterConstructionFunc
		params      configcompression.CompressionParams
		reader      func(r io.Reader) (io.Reader, error)
	}{
		{
			name: "none",
			constructor: func(configcompression.CompressionParams) (Writer, error) {
				return Nop{}, nil
			},
			reader: func(r io.Reader) (io.Reader, error) { return r, nil },
		},
		{
			name:        "gzip",
			constructor: newGZIPCompressWriterPool,
			params:      configcompression.CompressionParams{Level: configcompression.DefaultCompressionLevel},
			reader: func(r io.Reader) (io.Reader, error) {
				return gzip.NewReader(r)
			},
		},
		{
			name:        "zlib",
			constructor: newZLibCompressWriterPool,
			params:      configcompression.CompressionParams{Level: configcompression.DefaultCompressionLevel},
			reader: func(r io.Reader) (io.Reader, error) {
				return zlib.NewReader(r)
			},
		},
		{
			name:        "deflate",
			constructor: newFlateCompressWriterPool,
			params:      configcompression.CompressionParams{Level: configcompression.DefaultCompressionLevel},
			reader: func(r io.Reader) (io.Reader, error) {
				return flate.NewReader(r), nil
			},
		},
		{
			name:        "lz4",
			constructor: newLZ4CompressWriterPool,
			params:      configcompression.CompressionParams{Level: 0},
			reader: func(r io.Reader) (io.Reader, error) {
				return lz4.NewReader(r), nil
			},
		},
		{
			name:        "zstd",
			constructor: newZSTDCompressWriterPool,
			params:      configcompression.CompressionParams{Level: 1},
			reader: func(r io.Reader) (io.Reader, error) {
				return zstd.NewReader(r)
			},
		},
		{
			name:        "snappy",
			constructor: newSnappyCompressWriterPool,
			params:      configcompression.CompressionParams{Level: 0},
			reader: func(r io.Reader) (io.Reader, error) {
				return snappy.NewReader(r), nil
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cw, err := tc.constructor(tc.params)
			require.NoError(t, err, "Must not error when creating compress writer")

			expect := []byte("The quick brown fox jumped over the rabbit proof fence!")
			compressed := bytes.NewBuffer(nil)

			n, err := cw.CompressBytes(compressed, expect)
			require.NoError(t, err, "Must not error compressing raw bytes")
			assert.Equal(t, int64(len(expect)), n, "Must have read all the bytes provided")

			r, err := tc.reader(compressed)
			require.NoError(t, err, "Must create a reader for compressed bytes")
			actual := bytes.NewBuffer(nil)

			_, err = io.Copy(actual, r)
			require.NoError(t, err, "Must not error when reading compressed bytes")

			assert.Equal(
				t,
				expect,
				actual.Bytes(),
				"Must produce the same value as the original",
			)
		})
	}
}

func TestWriteLeaser(t *testing.T) {
	t.Parallel()

	f := NewWriterFactory()

	for _, tc := range []struct {
		name   string
		typ    configcompression.Type
		errVal string
	}{
		{
			name: "default",
			typ:  "",
		},
		{
			name: "none",
			typ:  "none",
		},
		{
			name: "gzip",
			typ:  configcompression.TypeGzip,
		},
		{
			name: "zlib",
			typ:  configcompression.TypeZlib,
		},
		{
			name: "deflate",
			typ:  configcompression.TypeDeflate,
		},
		{
			name: "lz4",
			typ:  configcompression.TypeLz4,
		},
		{
			name: "snappy",
			typ:  configcompression.TypeSnappy,
		},
		{
			name: "snappy-framed",
			typ:  configcompression.TypeSnappyFramed,
		},
		{
			name: "zstd",
			typ:  configcompression.TypeZstd,
		},
		{
			name:   "unsupported",
			typ:    "unsupported",
			errVal: `no configured compression writer for "unsupported"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rl, err := f.NewWriteLeaser(tc.typ, configcompression.CompressionParams{
				Level: configcompression.DefaultCompressionLevel,
			})
			if tc.errVal != "" {
				assert.EqualError(t, err, tc.errVal, "Must match the expected error value")
				return
			}
			err = rl.LeaseWriter(func(r ResettableWriter) error {
				require.NotNil(t, r, "Must have a valid value provided")
				return r.Close()
			})
			assert.NoError(t, err, "Must not return an error when checking lease is valid")
		})
	}
}
