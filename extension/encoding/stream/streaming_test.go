// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stream // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"

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

		helper := NewScannerHelper(reader)

		assert.IsType(t, &bufio.Reader{}, helper.bufReader)
	})

	t.Run("Bufio.Reader remains unchanged", func(t *testing.T) {
		reader := strings.NewReader("test")
		bufReader := bufio.NewReader(reader)

		helper := NewScannerHelper(bufReader)

		assert.Equal(t, bufReader, helper.bufReader)
	})
}

func TestStreamScannerHelper_ScanString(t *testing.T) {
	input := "line1\nline2\nline3\n"
	helper := NewScannerHelper(strings.NewReader(input))

	line, flush, err := helper.ScanString()
	require.NoError(t, err)
	assert.Equal(t, "line1", line)
	assert.False(t, flush)

	line, flush, err = helper.ScanString()
	require.NoError(t, err)
	assert.Equal(t, "line2", line)
	assert.False(t, flush)

	line, flush, err = helper.ScanString()
	require.NoError(t, err)
	assert.Equal(t, "line3", line)
	assert.False(t, flush)

	_, flush, err = helper.ScanString()
	assert.ErrorIs(t, err, io.EOF)
	assert.True(t, flush)
}

func TestStreamScannerHelper_ScanBytes(t *testing.T) {
	input := "line1\nline2\nline3\n"
	helper := NewScannerHelper(strings.NewReader(input))

	bytes, flush, err := helper.ScanBytes()
	require.NoError(t, err)
	assert.Equal(t, []byte("line1"), bytes)
	assert.False(t, flush)

	bytes, _, err = helper.ScanBytes()
	require.NoError(t, err)
	assert.Equal(t, []byte("line2"), bytes)
	assert.False(t, flush)

	bytes, _, err = helper.ScanBytes()
	require.NoError(t, err)
	assert.Equal(t, []byte("line3"), bytes)
	assert.False(t, flush)

	_, flush, err = helper.ScanString()
	assert.ErrorIs(t, err, io.EOF)
	assert.True(t, flush)
}

func TestStreamScannerHelper_FlushByItems(t *testing.T) {
	input := "a\nb\nc\n"
	helper := NewScannerHelper(strings.NewReader(input), encoding.WithFlushItems(2))

	_, flush, err := helper.ScanString()
	require.NoError(t, err)
	assert.False(t, flush)

	_, flush, err = helper.ScanString()
	require.NoError(t, err)
	assert.True(t, flush)

	_, flush, err = helper.ScanString()
	require.NoError(t, err)
	assert.False(t, flush)
}

func TestStreamScannerHelper_FlushByBytes(t *testing.T) {
	input := "aaa\nbbb\n"

	// Each line is 3 bytes + 1 newline = 4 bytes
	helper := NewScannerHelper(strings.NewReader(input), encoding.WithFlushBytes(4))

	_, flush, err := helper.ScanString()
	require.NoError(t, err)
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
