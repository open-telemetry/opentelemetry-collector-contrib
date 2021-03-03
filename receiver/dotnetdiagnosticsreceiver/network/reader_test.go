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
	"errors"
	"testing"
	"unicode/utf16"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMReader_Read(t *testing.T) {
	fr := newFakeReader([]byte{42}, -1)
	multi := NewMultiReader(fr)
	var b byte
	err := multi.Read(&b)
	require.NoError(t, err)
	assert.Equal(t, byte(42), b)
}

func TestMReader_Align(t *testing.T) {
	fr := newFakeReader(make([]byte, 4), -1)
	multi := NewMultiReader(fr)
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
	fr := newFakeReader(make([]byte, 1), 1)
	multi := NewMultiReader(fr)
	_, _ = multi.ReadByte()
	err := multi.Align()
	require.Error(t, err)
}

func TestMReader_UTF16(t *testing.T) {
	fr := newFakeReader(strToUTF16Bytes("hello\000"), -1)
	multi := NewMultiReader(fr)
	utfRead, err := multi.ReadUTF16()
	require.NoError(t, err)
	assert.Equal(t, "hello", utfRead)
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
	fr := newFakeReader(nil, 0)
	multi := NewMultiReader(fr)
	_, err := multi.ReadUTF16()
	require.Error(t, err)
}

func TestMReader_ReadByte(t *testing.T) {
	fr := newFakeReader([]byte{111}, -1)
	multi := NewMultiReader(fr)
	b, err := multi.ReadByte()
	require.NoError(t, err)
	assert.Equal(t, byte(111), b)
}

func TestMReader_ReadByteErr(t *testing.T) {
	fr := newFakeReader(nil, 0)
	multi := NewMultiReader(fr)
	_, err := multi.ReadByte()
	require.Error(t, err)
}

func TestMReader_AssertNextByteEquals(t *testing.T) {
	fr := newFakeReader([]byte{111}, -1)
	multi := NewMultiReader(fr)
	err := multi.AssertNextByteEquals(111)
	require.NoError(t, err)
}

func TestMReader_AssertNextByteEquals_ReadErr(t *testing.T) {
	fr := newFakeReader(nil, 0)
	multi := NewMultiReader(fr)
	err := multi.AssertNextByteEquals(111)
	require.Error(t, err)
}

func TestMReader_AssertNextByteEquals_MismatchErr(t *testing.T) {
	fr := newFakeReader([]byte{111}, -1)
	multi := NewMultiReader(fr)
	err := multi.AssertNextByteEquals(222)
	require.Error(t, err, "next byte assertion failed")
}

func TestMReader_CompressedInt32_Simple(t *testing.T) {
	fr := newFakeReader([]byte{42 | 0x80}, -1)
	multi := NewMultiReader(fr)
	i, err := multi.ReadCompressedInt32()
	require.NoError(t, err)
	require.Equal(t, int32(42), i)
}

func TestMReader_CompressedInt32_MultiByte(t *testing.T) {
	fr := newFakeReader([]byte{1 | 0x80, 1}, -1)
	multi := NewMultiReader(fr)
	i, err := multi.ReadCompressedInt32()
	require.NoError(t, err)
	require.Equal(t, int32(0x81), i)
}

func TestMReader_CompressedInt32_LengthErr(t *testing.T) {
	fr := newFakeReader([]byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80}, -1)
	multi := NewMultiReader(fr)
	_, err := multi.ReadCompressedInt32()
	require.Error(t, err, "CompressedInt32 is too long")
}

func TestMReader_CompressedInt32_ReadErr(t *testing.T) {
	fr := newFakeReader(nil, 0)
	multi := NewMultiReader(fr)
	_, err := multi.ReadCompressedInt32()
	require.Error(t, err)
}

func TestMReader_CompressedInt64_Simple(t *testing.T) {
	fr := newFakeReader([]byte{42 | 0x80}, -1)
	multi := NewMultiReader(fr)
	i, err := multi.ReadCompressedInt64()
	require.NoError(t, err)
	require.Equal(t, int64(42), i)
}

func TestMReader_CompressedInt64_MultiByte(t *testing.T) {
	fr := newFakeReader([]byte{1 | 0x80, 1}, -1)
	multi := NewMultiReader(fr)
	i, err := multi.ReadCompressedInt64()
	require.NoError(t, err)
	require.Equal(t, int64(0x81), i)
}

func TestMReader_CompressedInt64_LengthErr(t *testing.T) {
	p := make([]byte, 11)
	for i := range p {
		p[i] = 0x80
	}
	fr := newFakeReader(p, -1)
	multi := NewMultiReader(fr)
	_, err := multi.ReadCompressedInt64()
	require.Error(t, err, "CompressedInt64 is too long")
}

func TestMReader_CompressedInt64_ReadErr(t *testing.T) {
	fr := newFakeReader(nil, 0)
	multi := NewMultiReader(fr)
	_, err := multi.ReadCompressedInt64()
	require.Error(t, err)
}

func TestMReader_SeekReset(t *testing.T) {
	fr := newFakeReader(make([]byte, 6), -1)
	multi := NewMultiReader(fr)
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
	fr := newFakeReader(nil, 0)
	multi := NewMultiReader(fr)
	err := multi.Seek(1)
	require.Error(t, err)
}

func TestMReader_ReadASCII(t *testing.T) {
	fr := newFakeReader([]byte("hello"), -1)
	multi := NewMultiReader(fr)
	msg, err := multi.ReadASCII(5)
	require.NoError(t, err)
	assert.Equal(t, "hello", msg)
}

func TestMReader_ReadASCIIErr(t *testing.T) {
	fr := newFakeReader([]byte("hello"), 0)
	multi := NewMultiReader(fr)
	_, err := multi.ReadASCII(5)
	require.Error(t, err)
}

// fakeReader
type fakeReader struct {
	source     []byte
	pos        int
	readCount  int
	failOnRead int
}

func newFakeReader(source []byte, failOnRead int) *fakeReader {
	return &fakeReader{
		source:     source,
		failOnRead: failOnRead,
	}
}

func (r *fakeReader) Read(p []byte) (n int, err error) {
	defer func() {
		r.readCount++
	}()
	if r.readCount == r.failOnRead {
		return 0, errors.New("")
	}
	copy(p, r.source[r.pos:])
	r.pos += len(p)
	return len(p), nil
}
