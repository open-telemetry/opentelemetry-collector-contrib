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
	"errors"
	"fmt"
	"io"
	"unicode/utf16"
)

// MultiReader provides an abstraction to make reading from the diagnostics
// stream easier for callers.
type MultiReader interface {
	// ReadByte reads and returns a single byte from the stream
	ReadByte() (byte, error)
	// ReadCompressedInt32 reads a compressed int32 from the stream
	ReadCompressedInt32() (int32, error)
	// ReadCompressedUInt32 reads a compressed uint32 from the stream
	ReadCompressedUInt32() (uint32, error)
	// ReadCompressedInt64 reads a compressed int64 from the stream
	ReadCompressedInt64() (int64, error)
	// ReadCompressedUInt64 reads a compressed uint64 from the stream
	ReadCompressedUInt64() (uint64, error)
	// Read reads bytes from the underlying stream into the passed in reference
	Read(interface{}) error
	// ReadUTF16 reads and returns a zero-terminated UTF16 string from the stream
	ReadUTF16() (string, error)
	// ReadASCII reads an ASCII string of the given length from the underlying stream
	ReadASCII(int) (string, error)
	// AssertNextByteEquals reads the next byte and returns an error if it doesn't
	// equal the passed in byte
	AssertNextByteEquals(byte) error
	// Seek moves the current position forward by reading and throwing away the
	// specified number of bytes
	Seek(i int) error
	// Align moves the current position forward for 4-byte alignment
	// https://github.com/Microsoft/perfview/blob/main/src/TraceEvent/EventPipe/EventPipeFormat.md
	Align() error
	// Pos returns the position (byte offset) from the underlying PositionalReader
	Pos() int
	// Reset resets the underlying PositionalReader
	Reset()
	// Flush flushes the underlying BlobWriter
	Flush()
}

// mReader is an implementation of MultiReader. It wraps a PositionalReader,
// which keeps track of the byte count.
type mReader struct {
	pr PositionalReader
}

func NewMultiReader(r io.Reader, bw BlobWriter) MultiReader {
	pr := NewPositionalReader(r, bw)
	return &mReader{pr: pr}
}

var _ MultiReader = (*mReader)(nil)

var singleByte = make([]byte, 1)

// ReadByte reads and returns a single byte from the stream
func (r *mReader) ReadByte() (byte, error) {
	err := r.Read(singleByte)
	if err != nil {
		return 0, err
	}
	return singleByte[0], nil
}

// From https://github.com/Microsoft/perfview/blob/master/src/TraceEvent/EventPipe/EventPipeFormat.md
// > Variable length encodings of int and long. This is a standard technique where
// > the least significant 7 bits are encoded in a byte. If all remaining bits in the
// > starting number are zero then the 8th bit is set to zero to mark the end of the
// > encoding. Otherwise the 8th bit is set to 1 to indicate another byte is
// > forthcoming containing the next 7 least significant bits. This pattern repeats
// > until there are no more non-zero bits to encode, at most 5 bytes for a 32 bit
// > int and 10 bytes for a 64 bit long.

const shiftWidth = 7
const valueMask = 0x7f
const continueFlag = 0x80

// ReadCompressedInt32 reads a compressed int32 from the stream
func (r *mReader) ReadCompressedInt32() (int32, error) {
	uInt32, err := r.ReadCompressedUInt32()
	return int32(uInt32), err
}

// ReadCompressedUInt32 reads a compressed uint32 from the stream
func (r *mReader) ReadCompressedUInt32() (out uint32, err error) {
	const maxLen = 5
	for i := 0; ; i++ {
		if i > maxLen {
			return 0, errors.New("CompressedInt32 is too long")
		}
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		out |= uint32(b&valueMask) << (i * shiftWidth)
		if (b & continueFlag) == 0 {
			break
		}
	}
	return out, nil
}

// ReadCompressedInt64 reads a compressed int64 from the stream
func (r *mReader) ReadCompressedInt64() (int64, error) {
	uInt64, err := r.ReadCompressedUInt64()
	return int64(uInt64), err
}

// ReadCompressedUInt64 reads a compressed uint64 from the stream
func (r *mReader) ReadCompressedUInt64() (out uint64, err error) {
	const maxLen = 10
	for i := 0; ; i++ {
		if i > maxLen {
			return 0, errors.New("CompressedInt64 is too long")
		}
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		out |= uint64(b&valueMask) << (i * shiftWidth)
		if (b & continueFlag) == 0 {
			break
		}
	}
	return out, nil
}

// Read reads bytes from the underlying stream into v
func (r *mReader) Read(v interface{}) error {
	return binary.Read(r.pr, ByteOrder, v)
}

// ReadUTF16 reads and returns a zero-terminated UTF16 string from the stream
func (r *mReader) ReadUTF16() (s string, err error) {
	var a []uint16
	for {
		var c uint16
		err = r.Read(&c)
		if err != nil {
			return "", err
		}
		if c == 0 {
			break
		}
		a = append(a, c)
	}
	return string(utf16.Decode(a)), nil
}

// ReadASCII reads an ASCII string of the given length from the underlying stream
func (r *mReader) ReadASCII(strlen int) (string, error) {
	b := make([]byte, strlen)
	_, err := r.pr.Read(b)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// AssertNextByteEquals reads the next byte and returns an error if it doesn't
// equal the passed in byte
func (r *mReader) AssertNextByteEquals(b byte) error {
	found, err := r.ReadByte()
	if err != nil {
		return err
	}
	if found != b {
		return fmt.Errorf("next byte assertion failed, expected %d, got %d", b, found)
	}
	return nil
}

// Seek moves the current position forward by reading and throwing away the
// specified number of bytes
func (r *mReader) Seek(i int) error {
	_, err := r.pr.Read(make([]byte, i))
	if err != nil {
		return err
	}
	return nil
}

// Align moves the current position forward for 4-byte alignment
// https://github.com/Microsoft/perfview/blob/main/src/TraceEvent/EventPipe/EventPipeFormat.md
func (r *mReader) Align() error {
	mod := r.Pos() % 4
	if mod > 0 {
		err := r.Seek(4 - mod)
		if err != nil {
			return err
		}
	}
	return nil
}

// Pos returns the position (byte offset) from the underlying PositionalReader
func (r *mReader) Pos() int {
	return r.pr.Position()
}

// Reset resets the underlying PositionalReader
func (r *mReader) Reset() {
	r.pr.Reset()
}

// Flush flushes the underlying BlobWriter
func (r *mReader) Flush() {
	r.pr.Flush()
}

// PositionalReader is an interface extracted from posReader so it can be
// swapped for testing. It implements io.Reader, can return the number of bytes
// read, and can reset the current position.
type PositionalReader interface {
	Read(p []byte) (n int, err error)
	Position() int
	Flush()
	Reset()
}

// posReader wraps an io.Reader, and keeps track of the number of bytes read.
// Implements PositionalReader.
type posReader struct {
	r   io.Reader
	pos int
	w   BlobWriter
}

// NewPositionalReader creates a PositionalReader, wrapping the passed-in
// io.Reader, and keeping track of the number of bytes read.
func NewPositionalReader(r io.Reader, w BlobWriter) PositionalReader {
	return &posReader{r: r, w: w}
}

// Read implements io.Reader incrementing the current position by the number of
// bytes read.
func (r *posReader) Read(p []byte) (n int, err error) {
	n, err = r.r.Read(p)
	r.w.append(p, n)
	r.pos += n
	return n, err
}

// Position returns the number of bytes read from the underlying io.Reader.
func (r *posReader) Position() int {
	return r.pos
}

// Reset resets the position to as close to zero as possible without altering
// the current 4-byte alignment. This is to prevent integer overflow during
// long-running usage.
func (r *posReader) Reset() {
	r.pos = r.pos % 4
}

func (r *posReader) Flush() {
	r.w.flush()
}
