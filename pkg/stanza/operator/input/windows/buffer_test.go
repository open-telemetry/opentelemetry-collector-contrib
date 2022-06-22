// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build windows
// +build windows

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
	for i, b := range utf16 {
		buffer.buffer[i] = b
	}
	offset := uint32(len(utf16))
	bytes, err := buffer.ReadBytes(offset)
	require.NoError(t, err)
	require.Equal(t, utf8, bytes)
}

func TestBufferReadWideBytes(t *testing.T) {
	buffer := NewBuffer()
	utf8 := []byte("test")
	utf16, _ := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder().Bytes(utf8)
	for i, b := range utf16 {
		buffer.buffer[i] = b
	}
	offset := uint32(len(utf16) / 2)
	bytes, err := buffer.ReadWideChars(offset)
	require.NoError(t, err)
	require.Equal(t, utf8, bytes)
}

func TestBufferReadString(t *testing.T) {
	buffer := NewBuffer()
	utf8 := []byte("test")
	utf16, _ := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder().Bytes(utf8)
	for i, b := range utf16 {
		buffer.buffer[i] = b
	}
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
