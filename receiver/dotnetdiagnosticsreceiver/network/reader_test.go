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
	"testing"
	"unicode/utf16"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMReader_Read(t *testing.T) {
	r := &FakeRW{
		ReadErrIdx: -1,
		Responses: map[int][]byte{
			0: {42},
		},
	}
	multi := NewMultiReader(r, &NopBlobWriter{})
	var b byte
	err := multi.Read(&b)
	require.NoError(t, err)
	assert.Equal(t, byte(42), b)
	multi.Flush()
}

func TestMReader_Align(t *testing.T) {
	r := &FakeRW{
		ReadErrIdx: -1,
		Responses: map[int][]byte{
			0: make([]byte, 4),
		},
	}
	multi := NewMultiReader(r, &NopBlobWriter{})
	var b byte
	err := multi.Read(&b)
	require.NoError(t, err)
	assert.Equal(t, 1, multi.Pos())
	err = multi.Align()
	require.NoError(t, err)
	assert.Equal(t, 4, multi.Pos())
	// should do nothing, already aligned
	err = multi.Align()
	require.NoError(t, err)
	assert.Equal(t, 4, multi.Pos())
}

func TestMReader_AlignErr(t *testing.T) {
	r := &FakeRW{
		ReadErrIdx: 1,
		Responses: map[int][]byte{
			0: make([]byte, 1),
		},
	}
	multi := NewMultiReader(r, &NopBlobWriter{})
	_, _ = multi.ReadByte()
	err := multi.Align()
	require.Error(t, err)
}

func TestMReader_UTF16(t *testing.T) {
	p := strToUTF16Bytes("x\000")
	r := &FakeRW{
		ReadErrIdx: -1,
		Responses: map[int][]byte{
			0: {p[0]},
			1: {p[1]},
			2: {p[2]},
			3: {p[3]},
		},
	}
	multi := NewMultiReader(r, &NopBlobWriter{})
	utfRead, err := multi.ReadUTF16()
	require.NoError(t, err)
	assert.Equal(t, "x", utfRead)
}

func strToUTF16Bytes(msg string) []byte {
	encoded := utf16.Encode([]rune(msg))
	buf := &bytes.Buffer{}
	for _, u := range encoded {
		_ = binary.Write(buf, ByteOrder, u)
	}
	return buf.Bytes()
}

func TestMReader_UTF16Err(t *testing.T) {
	multi := NewMultiReader(&FakeRW{}, &NopBlobWriter{})
	_, err := multi.ReadUTF16()
	require.Error(t, err)
}

func TestMReader_ReadByte(t *testing.T) {
	r := &FakeRW{
		ReadErrIdx: -1,
		Responses: map[int][]byte{
			0: {111},
		},
	}
	multi := NewMultiReader(r, &NopBlobWriter{})
	b, err := multi.ReadByte()
	require.NoError(t, err)
	assert.Equal(t, byte(111), b)
}

func TestMReader_ReadByteErr(t *testing.T) {
	multi := NewMultiReader(&FakeRW{}, &NopBlobWriter{})
	_, err := multi.ReadByte()
	require.Error(t, err)
}

func TestMReader_AssertNextByteEquals(t *testing.T) {
	r := &FakeRW{
		ReadErrIdx: -1,
		Responses: map[int][]byte{
			0: {111},
		},
	}
	multi := NewMultiReader(r, &NopBlobWriter{})
	err := multi.AssertNextByteEquals(111)
	require.NoError(t, err)
}

func TestMReader_AssertNextByteEquals_ReadErr(t *testing.T) {
	multi := NewMultiReader(&FakeRW{}, &NopBlobWriter{})
	err := multi.AssertNextByteEquals(111)
	require.Error(t, err)
}

func TestMReader_AssertNextByteEquals_MismatchErr(t *testing.T) {
	r := &FakeRW{
		ReadErrIdx: -1,
		Responses: map[int][]byte{
			0: {111},
		},
	}
	multi := NewMultiReader(r, &NopBlobWriter{})
	err := multi.AssertNextByteEquals(222)
	require.Error(t, err, "next byte assertion failed")
}

func TestMReader_CompressedInt32_Simple(t *testing.T) {
	r := &FakeRW{
		ReadErrIdx: -1,
		Responses: map[int][]byte{
			0: {42 | 0x80},
		},
	}
	multi := NewMultiReader(r, &NopBlobWriter{})
	i, err := multi.ReadCompressedInt32()
	require.NoError(t, err)
	require.Equal(t, int32(42), i)
}

func TestMReader_CompressedInt32_MultiByte(t *testing.T) {
	r := &FakeRW{
		ReadErrIdx: -1,
		Responses: map[int][]byte{
			0: {1 | 0x80},
			1: {1},
		},
	}
	multi := NewMultiReader(r, &NopBlobWriter{})
	i, err := multi.ReadCompressedInt32()
	require.NoError(t, err)
	require.Equal(t, int32(0x81), i)
}

func TestMReader_CompressedInt32_LengthErr(t *testing.T) {
	r := &FakeRW{
		ReadErrIdx: -1,
		Responses:  mkMap(6, 0x80),
	}
	multi := NewMultiReader(r, &NopBlobWriter{})
	_, err := multi.ReadCompressedInt32()
	require.Error(t, err, "CompressedInt32 is too long")
}

func TestMReader_CompressedInt32_ReadErr(t *testing.T) {
	r := &FakeRW{
		ReadErrIdx: 0,
	}
	multi := NewMultiReader(r, &NopBlobWriter{})
	_, err := multi.ReadCompressedInt32()
	require.Error(t, err)
}

func TestMReader_CompressedInt64_Simple(t *testing.T) {
	r := &FakeRW{
		ReadErrIdx: -1,
		Responses: map[int][]byte{
			0: {42 | 0x80},
		},
	}
	multi := NewMultiReader(r, &NopBlobWriter{})
	i, err := multi.ReadCompressedInt64()
	require.NoError(t, err)
	require.Equal(t, int64(42), i)
}

func TestMReader_CompressedInt64_MultiByte(t *testing.T) {
	r := &FakeRW{
		ReadErrIdx: -1,
		Responses: map[int][]byte{
			0: {1 | 0x80},
			1: {1},
		},
	}
	multi := NewMultiReader(r, &NopBlobWriter{})
	i, err := multi.ReadCompressedInt64()
	require.NoError(t, err)
	require.Equal(t, int64(0x81), i)
}

func TestMReader_CompressedInt64_LengthErr(t *testing.T) {
	r := &FakeRW{
		ReadErrIdx: -1,
		Responses:  mkMap(11, 0x80),
	}
	multi := NewMultiReader(r, &NopBlobWriter{})
	_, err := multi.ReadCompressedInt64()
	require.Error(t, err, "CompressedInt64 is too long")
}

func TestMReader_CompressedInt64_ReadErr(t *testing.T) {
	r := &FakeRW{
		ReadErrIdx: 0,
	}
	multi := NewMultiReader(r, &NopBlobWriter{})
	_, err := multi.ReadCompressedInt64()
	require.Error(t, err)
}

func TestMReader_SeekReset(t *testing.T) {
	r := &FakeRW{
		ReadErrIdx: -1,
		Responses:  mkMap(6, 0),
	}
	multi := NewMultiReader(r, &NopBlobWriter{})
	_, err := multi.ReadByte()
	require.NoError(t, err)
	multi.Reset()
	assert.Equal(t, 1, multi.Pos())
	err = multi.Seek(4)
	require.NoError(t, err)
	assert.Equal(t, 5, multi.Pos())
	multi.Reset()
	assert.Equal(t, 1, multi.Pos())
}

func TestMReader_Seek_ReadErr(t *testing.T) {
	r := &FakeRW{ReadErrIdx: 0}
	multi := NewMultiReader(r, &NopBlobWriter{})
	err := multi.Seek(1)
	require.Error(t, err)
}

func TestMReader_ReadASCII(t *testing.T) {
	r := &FakeRW{
		ReadErrIdx: -1,
		Responses: map[int][]byte{
			0: []byte("abc"),
		},
	}
	multi := NewMultiReader(r, &NopBlobWriter{})
	msg, err := multi.ReadASCII(3)
	require.NoError(t, err)
	assert.Equal(t, "abc", msg)
}

func TestMReader_ReadASCIIErr(t *testing.T) {
	r := &FakeRW{
		ReadErrIdx: 0,
		Responses: map[int][]byte{
			0: []byte("abc"),
		},
	}
	multi := NewMultiReader(r, &NopBlobWriter{})
	_, err := multi.ReadASCII(3)
	require.Error(t, err)
}

func mkMap(size int, v byte) map[int][]byte {
	responses := map[int][]byte{}
	for i := 0; i < size; i++ {
		responses[i] = []byte{v}
	}
	return responses
}
