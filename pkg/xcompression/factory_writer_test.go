// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xcompression

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestWriterFactoryUnsetLevel(t *testing.T) {
	t.Parallel()

	payload := bytes.NewBuffer(nil)
	for i := range 2000 {
		fmt.Fprintf(payload, "request %d served by host-%d in %dms\n", i, i%7, i%250)
	}

	compress := func(t *testing.T, typ configcompression.Type, level configcompression.Level) []byte {
		w, err := NewWriterFactory().NewWriter(typ, configcompression.CompressionParams{Level: level})
		require.NoError(t, err, "Must not error when creating the writer")
		out := bytes.NewBuffer(nil)
		_, err = w.CompressBytes(out, payload.Bytes())
		require.NoError(t, err, "Must not error when compressing")
		return out.Bytes()
	}

	for _, typ := range []configcompression.Type{
		configcompression.TypeGzip,
		configcompression.TypeZlib,
		configcompression.TypeDeflate,
	} {
		t.Run(string(typ), func(t *testing.T) {
			t.Parallel()

			unset := compress(t, typ, 0)
			assert.Equal(t, compress(t, typ, configcompression.DefaultCompressionLevel), unset, "Must match the output of the default level")
			assert.NotEqual(t, compress(t, typ, 1), unset, "Must not match the output of another level")
			assert.Less(t, len(unset), payload.Len(), "Must compress the payload")
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
