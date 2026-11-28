// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xcompression

import (
	"bytes"
	"testing"

	"github.com/klauspost/compress/snappy"
	"github.com/stretchr/testify/require"
)

func TestDecodeSnappyBlock(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte("a"), 101)
	compressed := snappy.Encode(nil, payload)

	t.Run("at limit", func(t *testing.T) {
		decoded, err := DecodeSnappyBlock(compressed, len(payload))
		require.NoError(t, err)
		require.Equal(t, payload, decoded)
	})

	t.Run("decoded size exceeds limit", func(t *testing.T) {
		_, err := DecodeSnappyBlock(compressed, 100)
		require.ErrorIs(t, err, ErrDecodedSizeExceeded)
		require.ErrorContains(t, err, "(101 vs 100)")
	})

	t.Run("truncated block", func(t *testing.T) {
		_, err := DecodeSnappyBlock(compressed[:len(compressed)-1], len(payload))
		require.ErrorIs(t, err, snappy.ErrCorrupt)
	})
}
