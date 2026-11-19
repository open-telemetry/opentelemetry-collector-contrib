// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xcompression

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/configcompression"
)

func TestWriterFactoryNewWriter(t *testing.T) {
	t.Parallel()

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
			name: "lz4",
			typ:  configcompression.TypeLz4,
		},
		{
			name:   "unsupported",
			typ:    configcompression.Type("unsupported"),
			errVal: `no configured compression writer for "unsupported"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := NewWriterFactory()

			if w, err := f.NewWriter(tc.typ, configcompression.CompressionParams{Level: configcompression.DefaultCompressionLevel}); tc.errVal == "" {
				assert.NoError(t, err, "Must not report an error")
				assert.NotNil(t, w, "Must have a valid writer")
			} else {
				assert.EqualError(t, err, tc.errVal, "Must match the expected error message")
				assert.Nil(t, w, "Must have a nil writer")
			}
		})
	}
}

func TestWriterFactoryNewWriteLeaser(t *testing.T) {
	t.Parallel()

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
			name: "lz4",
			typ:  configcompression.TypeLz4,
		},
		{
			name:   "unsupported",
			typ:    configcompression.Type("unsupported"),
			errVal: `no configured compression writer for "unsupported"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := NewWriterFactory()

			if w, err := f.NewWriteLeaser(tc.typ, configcompression.CompressionParams{Level: configcompression.DefaultCompressionLevel}); tc.errVal == "" {
				assert.NoError(t, err, "Must not report an error")
				assert.NotNil(t, w, "Must have a valid write leaser")
			} else {
				assert.EqualError(t, err, tc.errVal, "Must match the expected error message")
				assert.Nil(t, w, "Must have a nil write leaser")
			}
		})
	}
}
