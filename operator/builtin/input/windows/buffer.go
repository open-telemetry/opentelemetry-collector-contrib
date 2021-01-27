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

// +build windows

package windows

import (
	"bytes"
	"fmt"

	"golang.org/x/text/encoding/unicode"
)

// defaultBufferSize is the default size of the buffer.
const defaultBufferSize = 16384

// Buffer is a buffer of utf-16 bytes.
type Buffer struct {
	buffer []byte
}

// ReadBytes will read UTF-8 bytes from the buffer.
func (b *Buffer) ReadBytes(offset uint32) ([]byte, error) {
	utf16 := b.buffer[:offset]
	utf8, err := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder().Bytes(utf16)
	if err != nil {
		return nil, fmt.Errorf("failed to convert buffer contents to utf8: %s", err)
	}

	return bytes.Trim(utf8, "\u0000"), nil
}

// ReadString will read a UTF-8 string from the buffer.
func (b *Buffer) ReadString(offset uint32) (string, error) {
	bytes, err := b.ReadBytes(offset)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// UpdateSize will update the size of the buffer.
func (b *Buffer) UpdateSize(size uint32) {
	b.buffer = make([]byte, size)
}

// Size will return the size of the buffer.
func (b *Buffer) Size() uint32 {
	return uint32(len(b.buffer))
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
