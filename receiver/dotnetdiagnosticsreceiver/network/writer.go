// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package network

import (
	"encoding/binary"
	"io"
	"unicode/utf16"
)

var ByteOrder = binary.LittleEndian

// ByteBuffer is an interface extracted from bytes.Buffer so that it can be
// swapped for testing.
type ByteBuffer interface {
	io.Writer
	io.ByteWriter
	Bytes() []byte
	Len() int
	Reset()
}

func WriteUTF16String(buf ByteBuffer, s string) error {
	encoded := utf16.Encode([]rune(s))
	err := binary.Write(buf, ByteOrder, int32(len(encoded)+1))
	if err != nil {
		return err
	}

	err = binary.Write(buf, ByteOrder, encoded)
	if err != nil {
		return err
	}

	_, err = buf.Write([]byte{0, 0})
	return err
}
