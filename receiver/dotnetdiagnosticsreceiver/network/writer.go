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
	"bytes"
	"encoding/binary"
	"unicode/utf16"
)

var ByteOrder = binary.LittleEndian

// Writes a length-prefixed, zero-terminated utf16 string to the passed-in
// buffer
func WriteUTF16String(buf *bytes.Buffer, s string) {
	encoded := utf16.Encode([]rune(s))
	_ = binary.Write(buf, ByteOrder, int32(len(encoded)+1))
	_ = binary.Write(buf, ByteOrder, encoded)
	_, _ = buf.Write([]byte{0, 0})
}
