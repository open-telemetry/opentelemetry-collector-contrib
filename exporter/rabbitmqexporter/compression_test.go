// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqexporter

import (
	"bytes"
	"compress/gzip"
	"io"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configcompression"
)

func TestCompressBody(t *testing.T) {
	body := bytes.Repeat([]byte("otel rabbitmq "), 100)

	tests := []struct {
		name            string
		compression     configcompression.Type
		contentEncoding string
		decompress      func([]byte) ([]byte, error)
	}{
		{
			name:       "unset",
			decompress: func(b []byte) ([]byte, error) { return b, nil },
		},
		{
			name:        "none",
			compression: configcompression.Type("none"),
			decompress:  func(b []byte) ([]byte, error) { return b, nil },
		},
		{
			name:            "gzip",
			compression:     configcompression.TypeGzip,
			contentEncoding: "gzip",
			decompress: func(b []byte) ([]byte, error) {
				r, err := gzip.NewReader(bytes.NewReader(b))
				if err != nil {
					return nil, err
				}
				return io.ReadAll(r)
			},
		},
		{
			name:            "zstd",
			compression:     configcompression.TypeZstd,
			contentEncoding: "zstd",
			decompress: func(b []byte) ([]byte, error) {
				d, err := zstd.NewReader(nil)
				if err != nil {
					return nil, err
				}
				defer d.Close()
				return d.DecodeAll(b, nil)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, contentEncoding, err := compressBody(tt.compression, body)
			require.NoError(t, err)
			assert.Equal(t, tt.contentEncoding, contentEncoding)
			decompressed, err := tt.decompress(got)
			require.NoError(t, err)
			assert.Equal(t, body, decompressed)
		})
	}
}

func TestCompressBodyUnsupported(t *testing.T) {
	_, _, err := compressBody(configcompression.TypeSnappy, []byte("x"))
	assert.ErrorContains(t, err, "unsupported compression")
}
