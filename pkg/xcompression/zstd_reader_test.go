// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xcompression

import (
	"bytes"
	"io"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
)

func TestNewZstdReaderMemoryLimit(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte("zstd payload"), 1024)
	encoder, err := zstd.NewWriter(nil, zstd.WithSingleSegment(true))
	require.NoError(t, err)
	defer encoder.Close()
	compressed := encoder.EncodeAll(payload, nil)

	t.Run("zero limit", func(t *testing.T) {
		reader, err := NewZstdReader(bytes.NewReader(compressed), 0)
		require.Error(t, err)
		require.Nil(t, reader)
	})

	t.Run("within limit", func(t *testing.T) {
		reader, err := NewZstdReader(bytes.NewReader(compressed), uint64(len(payload)))
		require.NoError(t, err)
		defer func() { require.NoError(t, reader.Close()) }()

		decoded, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.Equal(t, payload, decoded)
	})

	t.Run("stream exceeds limit", func(t *testing.T) {
		reader, err := NewZstdReader(bytes.NewReader(compressed), uint64(len(payload)-1))
		require.NoError(t, err)
		defer func() { require.NoError(t, reader.Close()) }()

		_, err = io.ReadAll(reader)
		require.ErrorIs(t, err, zstd.ErrDecoderSizeExceeded)
	})
}
