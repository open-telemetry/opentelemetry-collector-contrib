// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windows

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/unicode"
)

func TestBufferReadBytes(t *testing.T) {
	buffer := NewBuffer()
	utf8 := []byte("test")
	utf16, _ := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder().Bytes(utf8)
	copy(buffer.buffer, utf16)
	offset := uint32(len(utf16))
	bytes, err := buffer.ReadBytes(offset)
	require.NoError(t, err)
	require.Equal(t, utf8, bytes)
}

func TestBufferReadWideBytes(t *testing.T) {
	buffer := NewBuffer()
	utf8 := []byte("test")
	utf16, _ := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder().Bytes(utf8)
	copy(buffer.buffer, utf16)
	offset := uint32(len(utf16) / 2)
	bytes, err := buffer.ReadWideChars(offset)
	require.NoError(t, err)
	require.Equal(t, utf8, bytes)
}

func TestBufferReadString(t *testing.T) {
	buffer := NewBuffer()
	utf8 := []byte("test")
	utf16, _ := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder().Bytes(utf8)
	copy(buffer.buffer, utf16)
	offset := uint32(len(utf16))
	result, err := buffer.ReadString(offset)
	require.NoError(t, err)
	require.Equal(t, "test", result)
}

func TestBufferUpdateSize(t *testing.T) {
	buffer := NewBuffer()
	buffer.UpdateSizeBytes(1)
	require.Equal(t, 1, len(buffer.buffer))
}

func TestBufferUpdateSizeWide(t *testing.T) {
	buffer := NewBuffer()
	buffer.UpdateSizeWide(1)
	require.Equal(t, 2, len(buffer.buffer))
}

func TestBufferSize(t *testing.T) {
	buffer := NewBuffer()
	require.Equal(t, uint32(defaultBufferSize), buffer.SizeBytes())
}

func TestBufferSizeWide(t *testing.T) {
	buffer := NewBuffer()
	require.Equal(t, uint32(defaultBufferSize/2), buffer.SizeWide())
}

func TestBufferFirstByte(t *testing.T) {
	buffer := NewBuffer()
	buffer.buffer[0] = '1'
	require.Equal(t, &buffer.buffer[0], buffer.FirstByte())
}
