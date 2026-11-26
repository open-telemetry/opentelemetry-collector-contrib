// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xcompression

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/configcompression"
)

func TestReaderFactoryNewReader(t *testing.T) {
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

			f := NewReaderFactory()

			if r, err := f.NewReader(tc.typ, configcompression.CompressionParams{Level: configcompression.DefaultCompressionLevel}); tc.errVal == "" {
				assert.NoError(t, err, "Must not report an error")
				assert.NotNil(t, r, "Must have a valid reader")
			} else {
				assert.EqualError(t, err, tc.errVal, "Must match the expected error message")
				assert.Nil(t, r, "Must have a nil reader")
			}
		})
	}
}

func TestReaderFactoryNewReadLeaser(t *testing.T) {
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

			f := NewReaderFactory()

			if r, err := f.NewReadLeaser(tc.typ, configcompression.CompressionParams{Level: configcompression.DefaultCompressionLevel}); tc.errVal == "" {
				assert.NoError(t, err, "Must not report an error")
				assert.NotNil(t, r, "Must have a valid reader")
			} else {
				assert.EqualError(t, err, tc.errVal, "Must match the expected error message")
				assert.Nil(t, r, "Must have a nil reader")
			}
		})
	}
}
