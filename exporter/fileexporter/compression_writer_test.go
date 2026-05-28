// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter

import (
	"bytes"
	"io"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
)

func TestCompressingWriter_Zstd(t *testing.T) {
	var buf bytes.Buffer
	base := &nopWriteCloser{&buf}

	cw, err := newCompressingWriter(base, compressionZSTD, 3, nil)
	require.NoError(t, err)

	testData := []byte("hello world from zstd compression")
	n, err := cw.Write(testData)
	require.NoError(t, err)
	require.Equal(t, len(testData), n)

	err = cw.Close()
	require.NoError(t, err)

	require.Positive(t, buf.Len())

	// Decompress with standard zstd decoder
	decoder, err := zstd.NewReader(&buf)
	require.NoError(t, err)
	defer decoder.Close()

	decompressed, err := io.ReadAll(decoder)
	require.NoError(t, err)
	require.Equal(t, testData, decompressed)
}

func TestCompressingWriter_MultipleWrites_Zstd(t *testing.T) {
	var buf bytes.Buffer
	base := &nopWriteCloser{&buf}

	cw, err := newCompressingWriter(base, compressionZSTD, 0, nil)
	require.NoError(t, err)

	messages := []string{
		"first message\n",
		"second message\n",
		"third message\n",
	}
	for _, msg := range messages {
		_, writeErr := cw.Write([]byte(msg))
		require.NoError(t, writeErr)
	}

	err = cw.Close()
	require.NoError(t, err)

	// Decompress and verify all messages are present
	decoder, err := zstd.NewReader(&buf)
	require.NoError(t, err)
	defer decoder.Close()

	decompressed, err := io.ReadAll(decoder)
	require.NoError(t, err)

	expected := "first message\nsecond message\nthird message\n"
	require.Equal(t, expected, string(decompressed))
}

func TestCompressingWriter_UnsupportedCompression(t *testing.T) {
	var buf bytes.Buffer
	base := &nopWriteCloser{&buf}

	_, err := newCompressingWriter(base, "snappy", 0, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported compression")
}

func TestCompressingWriter_Flush(t *testing.T) {
	var buf bytes.Buffer
	base := &nopWriteCloser{&buf}

	cw, err := newCompressingWriter(base, compressionZSTD, 0, nil)
	require.NoError(t, err)

	testData := []byte("data to flush")
	_, err = cw.Write(testData)
	require.NoError(t, err)

	// flush() should not error
	err = cw.flush()
	require.NoError(t, err)

	err = cw.Close()
	require.NoError(t, err)
}

func TestZstdEncoderLevelFromZstd(t *testing.T) {
	tests := []struct {
		level    int
		expected zstd.EncoderLevel
	}{
		{1, zstd.SpeedFastest},
		{3, zstd.SpeedDefault},
		{6, zstd.SpeedBetterCompression},
		{11, zstd.SpeedBestCompression},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, zstd.EncoderLevelFromZstd(tt.level), "level %d", tt.level)
	}
}
