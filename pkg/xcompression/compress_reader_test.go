// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xcompression

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configcompression"
)

func TestDecompressReader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name configcompression.Type
	}{
		{name: "none"},
		{name: configcompression.TypeGzip},
		{name: configcompression.TypeZlib},
		{name: configcompression.TypeDeflate},
		{name: configcompression.TypeLz4},
		{name: configcompression.TypeSnappy},
		{name: configcompression.TypeZstd},
	}

	writerFactory := NewWriterFactory()
	readerFactory := NewReaderFactory()

	expect := []byte("The quick brown fox jumped over the rabbit proof fence!")
	for _, test := range tests {
		t.Run(string(test.name), func(t *testing.T) {
			t.Parallel()
			params := configcompression.CompressionParams{}

			writer, err := writerFactory.Get(test.name, params)
			require.NoError(t, err)
			reader, err := readerFactory.Get(test.name, params)
			require.NoError(t, err)

			var compressed bytes.Buffer
			written, err := writer.CompressBytes(&compressed, expect)
			require.NoError(t, err)
			assert.Equal(t, int64(len(expect)), written)

			var fromBytes bytes.Buffer
			read, err := reader.DecompressBytes(&fromBytes, compressed.Bytes())
			require.NoError(t, err)
			assert.Equal(t, int64(len(expect)), read)
			assert.Equal(t, expect, fromBytes.Bytes())

			var fromReader bytes.Buffer
			read, err = reader.Decompress(&fromReader, bytes.NewReader(compressed.Bytes()))
			require.NoError(t, err)
			assert.Equal(t, int64(len(expect)), read)
			assert.Equal(t, expect, fromReader.Bytes())
		})
	}
}
