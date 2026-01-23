// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package encoding // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamUnmarshalOptions(t *testing.T) {
	opts := StreamUnmarshalOptions{}
	WithFlushBytes(100)(&opts)
	WithFlushItems(50)(&opts)
	WithStreamReaderBuffer(124)(&opts)

	assert.Equal(t, int64(100), opts.FlushBytes)
	assert.Equal(t, int64(50), opts.FlushItems)
	assert.Equal(t, 124, opts.StreamReaderBuffer)
}

func TestStreamScannerHelper_ScanString(t *testing.T) {
	input := "line1\nline2\nline3\n"
	helper := NewStreamScannerHelper(strings.NewReader(input))

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
	helper := NewStreamScannerHelper(strings.NewReader(input))

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
	helper := NewStreamScannerHelper(strings.NewReader(input), WithFlushItems(2))

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
	helper := NewStreamScannerHelper(strings.NewReader(input), WithFlushBytes(4))

	_, flush, err := helper.ScanString()
	require.NoError(t, err)
	assert.True(t, flush)
}

func TestStreamBatchHelper_ShouldFlush(t *testing.T) {
	helper := NewStreamBatchHelper(WithFlushBytes(5), WithFlushItems(5))

	assert.False(t, helper.ShouldFlush())

	helper.IncrementBytes(5)
	assert.True(t, helper.ShouldFlush())

	helper.Reset()
	assert.False(t, helper.ShouldFlush())

	helper.IncrementItems(5)
	assert.True(t, helper.ShouldFlush())
}
