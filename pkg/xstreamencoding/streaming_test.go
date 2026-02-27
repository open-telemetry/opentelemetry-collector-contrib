// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xstreamencoding // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xstreamencoding"

import (
	"bufio"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

func TestStreamScannerHelper_constructor(t *testing.T) {
	t.Run("IO reader get converted to bufio.Reader", func(t *testing.T) {
		reader := strings.NewReader("test")

		helper, err := NewScannerHelper(reader)
		require.NoError(t, err)

		assert.IsType(t, &bufio.Reader{}, helper.bufReader)
	})

	t.Run("Bufio.Reader remains unchanged", func(t *testing.T) {
		reader := strings.NewReader("test")
		bufReader := bufio.NewReader(reader)

		helper, err := NewScannerHelper(bufReader)
		require.NoError(t, err)

		assert.Equal(t, bufReader, helper.bufReader)
	})

	t.Run("Offset is checked and error wil be returned if incorrect", func(t *testing.T) {
		reader := strings.NewReader("test")
		bufReader := bufio.NewReader(reader)

		_, err := NewScannerHelper(bufReader, encoding.WithOffset(10))
		require.ErrorContains(t, err, "failed to discard offset 10")
	})
}

func TestStreamScannerHelper_ScanString(t *testing.T) {
	input := "line1\nline2\nline3\n"
	helper, err := NewScannerHelper(strings.NewReader(input))
	require.NoError(t, err)

	line, flush, err := helper.ScanString()
	require.NoError(t, err)
	assert.Equal(t, "line1", line)
	assert.False(t, flush)
	require.Equal(t, int64(6), helper.Offset())

	line, flush, err = helper.ScanString()
	require.NoError(t, err)
	assert.Equal(t, "line2", line)
	assert.False(t, flush)
	require.Equal(t, int64(12), helper.Offset())

	line, flush, err = helper.ScanString()
	require.NoError(t, err)
	assert.Equal(t, "line3", line)
	assert.False(t, flush)
	require.Equal(t, int64(18), helper.Offset())

	_, flush, err = helper.ScanString()
	assert.ErrorIs(t, err, io.EOF)
	assert.True(t, flush)
}

func TestStreamScannerHelper_ScanBytes(t *testing.T) {
	input := "line1\nline2\nline3\n"
	helper, err := NewScannerHelper(strings.NewReader(input))
	require.NoError(t, err)

	bytes, flush, err := helper.ScanBytes()
	require.NoError(t, err)
	assert.Equal(t, []byte("line1"), bytes)
	assert.False(t, flush)
	require.Equal(t, int64(6), helper.Offset())

	bytes, _, err = helper.ScanBytes()
	require.NoError(t, err)
	assert.Equal(t, []byte("line2"), bytes)
	assert.False(t, flush)
	require.Equal(t, int64(12), helper.Offset())

	bytes, _, err = helper.ScanBytes()
	require.NoError(t, err)
	assert.Equal(t, []byte("line3"), bytes)
	assert.False(t, flush)
	require.Equal(t, int64(18), helper.Offset())

	_, flush, err = helper.ScanString()
	assert.ErrorIs(t, err, io.EOF)
	assert.True(t, flush)
}

func TestStreamScannerHelper_InitialOffset(t *testing.T) {
	input := "line1\nline2\nline3\n"

	// Skip "line1\n" by setting offset to 6
	helper, err := NewScannerHelper(strings.NewReader(input), encoding.WithOffset(6))
	require.NoError(t, err)

	require.Equal(t, int64(6), helper.Offset())

	line, flush, err := helper.ScanString()
	require.NoError(t, err)
	assert.Equal(t, "line2", line)
	assert.False(t, flush)
	require.Equal(t, int64(12), helper.Offset())

	line, flush, err = helper.ScanString()
	require.NoError(t, err)
	assert.Equal(t, "line3", line)
	assert.False(t, flush)
	require.Equal(t, int64(18), helper.Offset())

	_, flush, err = helper.ScanString()
	assert.ErrorIs(t, err, io.EOF)
	assert.True(t, flush)
}

func TestStreamBatchHelper_ShouldFlush(t *testing.T) {
	helper := NewBatchHelper(encoding.WithFlushBytes(5), encoding.WithFlushItems(5))

	assert.False(t, helper.ShouldFlush())

	helper.IncrementBytes(5)
	assert.True(t, helper.ShouldFlush())

	helper.Reset()
	assert.False(t, helper.ShouldFlush())

	helper.IncrementItems(5)
	assert.True(t, helper.ShouldFlush())
}
