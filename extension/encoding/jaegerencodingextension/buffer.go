// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jaegerencodingextension"

import (
	"bytes"
	"encoding/binary"
	"errors"
)

var invalidBuff = errors.New("invalid buffer")
var header = []byte{'o', 't', 'l', 'p'}

type readonlyLengthFieldBuffer struct {
	buff      *bytes.Buffer
	hasHeader bool
}

func newReadableBuffer(bts []byte) *readonlyLengthFieldBuffer {
	hasHeader := checkHeader(bts)
	buffer := bytes.NewBuffer(bts)
	// Skip the first 4 bytes.
	if hasHeader {
		buffer.Next(4)
	}
	return &readonlyLengthFieldBuffer{buff: buffer, hasHeader: hasHeader}
}

func checkHeader(bts []byte) bool {
	return len(bts) > 4 && bts[0] == header[0] && bts[1] == header[1] && bts[2] == header[2] && bts[3] == header[3]
}

// slices Return slices
func (b *readonlyLengthFieldBuffer) slices() ([][]byte, error) {
	arr := make([][]byte, 0)
	if !b.hasHeader {
		arr = append(arr, b.buff.Bytes())
		return arr, nil
	}

	for {
		if b.buff.Len() == 0 {
			break
		}
		size, err := b.readInt()
		if err != nil {
			return nil, invalidBuff
		}
		bts := make([]byte, size)
		n, err := b.buff.Read(bts)
		if n != size {
			return nil, invalidBuff
		}
		if err != nil {
			return nil, err
		}
		arr = append(arr, bts)
	}
	return arr, nil
}

// readInt Read an int number from the buffer.
func (b *readonlyLengthFieldBuffer) readInt() (int, error) {
	var v int32
	err := binary.Read(b.buff, binary.BigEndian, &v)
	if err != nil {
		return 0, err
	}
	return int(v), nil
}

// Write only length filed byte buffer.
type writeOnlyLengthFieldBuffer struct {
	buff *bytes.Buffer
}

func newWritableBuffer(initCap int) (*writeOnlyLengthFieldBuffer, error) {
	arr := make([]byte, 0, initCap)
	buff := bytes.NewBuffer(arr)
	// write head to buff first.
	n, err := buff.Write(header)
	if n != len(header) || err != nil {
		return nil, errors.New("unable to create buffer")
	}
	return &writeOnlyLengthFieldBuffer{buff: buff}, nil
}

func calculateCap(bts [][]byte) int {
	c := 4
	for _, bt := range bts {
		c = len(bt) + 4
	}
	return c
}

// writeBytes write a byte array to the buffer.
func (b *writeOnlyLengthFieldBuffer) writeBytes(bts []byte) error {
	length := len(bts)
	err := b.writeInt(length)
	if err != nil {
		return err
	}
	n, err0 := b.buff.Write(bts)
	if n != length {
		return invalidBuff
	}
	if err0 != nil {
		return err0
	}
	return nil
}

// writeInt Write an int number to the buffer.
func (b *writeOnlyLengthFieldBuffer) writeInt(value int) error {
	v := int32(value)
	return binary.Write(b.buff, binary.BigEndian, v)
}

// bytes Return the byte array.
func (b *writeOnlyLengthFieldBuffer) bytes() []byte {
	return b.buff.Bytes()
}
