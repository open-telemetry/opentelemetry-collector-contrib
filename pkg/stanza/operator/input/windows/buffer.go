// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windows // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"

import (
	"bytes"
	"fmt"

	"golang.org/x/text/encoding/unicode"
)

// defaultBufferSize is the default size of the buffer.
const defaultBufferSize = 16384

// bytesPerWChar is the number bytes in a Windows wide character.
const bytesPerWChar = 2

// Buffer is a buffer of utf-16 bytes.
type Buffer struct {
	buffer []byte
}

// ReadBytes will read UTF-8 bytes from the buffer, where offset is the number of bytes to be read
func (b *Buffer) ReadBytes(offset uint32) ([]byte, error) {
	utf16 := b.buffer[:offset]
	utf8, err := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder().Bytes(utf16)
	if err != nil {
		return nil, fmt.Errorf("failed to convert buffer contents to utf8: %w", err)
	}

	return bytes.Trim(utf8, "\u0000"), nil
}

// ReadWideChars will read UTF-8 bytes from the buffer, where offset is the number of wchars to read
func (b *Buffer) ReadWideChars(offset uint32) ([]byte, error) {
	return b.ReadBytes(offset * bytesPerWChar)
}

// ReadString will read a UTF-8 string from the buffer.
func (b *Buffer) ReadString(offset uint32) (string, error) {
	bytes, err := b.ReadBytes(offset)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// UpdateSizeBytes will update the size of the buffer to fit size bytes.
func (b *Buffer) UpdateSizeBytes(size uint32) {
	b.buffer = make([]byte, size)
}

// UpdateSizeWide will update the size of the buffer to fit size wchars.
func (b *Buffer) UpdateSizeWide(size uint32) {
	b.buffer = make([]byte, bytesPerWChar*size)
}

// SizeBytes will return the size of the buffer as number of bytes.
func (b *Buffer) SizeBytes() uint32 {
	return uint32(len(b.buffer))
}

// SizeWide returns the size of the buffer as number of wchars
func (b *Buffer) SizeWide() uint32 {
	return uint32(len(b.buffer) / bytesPerWChar)
}

// FirstByte will return a pointer to the first byte.
func (b *Buffer) FirstByte() *byte {
	return &b.buffer[0]
}

// NewBuffer creates a new buffer with the default buffer size
func NewBuffer() Buffer {
	return Buffer{
		buffer: make([]byte, defaultBufferSize),
	}
}
