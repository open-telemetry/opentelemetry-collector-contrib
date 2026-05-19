// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package msgpackvalidator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/msgpackvalidator"

import (
	"encoding/binary"
	"fmt"

	"github.com/tinylib/msgp/msgp"
)

const (
	maxMsgpackContainerElements = 1 << 20
	maxMsgpackBinStringLen      = 20 * 1024 * 1024
	maxMsgpackExtLen            = maxMsgpackBinStringLen
	maxMsgpackDepth             = 64
)

// Validate walks a single MessagePack object without materializing it.
func Validate(b []byte) error {
	remain, err := validateValue(b, 0)
	if err != nil {
		return err
	}
	if len(remain) != 0 {
		return fmt.Errorf("msgpack has %d trailing bytes", len(remain))
	}
	return nil
}

func validateValue(b []byte, depth int) ([]byte, error) {
	if depth > maxMsgpackDepth {
		return b, fmt.Errorf("msgpack nesting exceeds depth limit %d", maxMsgpackDepth)
	}
	if len(b) == 0 {
		return b, msgp.ErrShortBytes
	}

	if isExtensionPrefix(b[0]) {
		return validateExtension(b)
	}

	switch msgp.NextType(b) {
	case msgp.ArrayType:
		return validateArray(b, depth)
	case msgp.MapType:
		return validateMap(b, depth)
	case msgp.BinType:
		return validateBin(b)
	case msgp.StrType:
		return validateString(b)
	default:
		return msgp.Skip(b)
	}
}

func validateArray(b []byte, depth int) ([]byte, error) {
	size, remain, err := msgp.ReadArrayHeaderBytes(b)
	if err != nil {
		return b, err
	}
	err = validateElementCount("array", size, len(remain), 1)
	if err != nil {
		return b, err
	}
	for range size {
		remain, err = validateValue(remain, depth+1)
		if err != nil {
			return remain, err
		}
	}
	return remain, nil
}

func validateMap(b []byte, depth int) ([]byte, error) {
	size, remain, err := msgp.ReadMapHeaderBytes(b)
	if err != nil {
		return b, err
	}
	err = validateElementCount("map", size, len(remain), 2)
	if err != nil {
		return b, err
	}
	for range size {
		remain, err = validateValue(remain, depth+1)
		if err != nil {
			return remain, err
		}
		remain, err = validateValue(remain, depth+1)
		if err != nil {
			return remain, err
		}
	}
	return remain, nil
}

func validateElementCount(kind string, size uint32, remaining, objectsPerElement int) error {
	if size > maxMsgpackContainerElements {
		return fmt.Errorf("msgpack %s length %d exceeds limit %d", kind, size, maxMsgpackContainerElements)
	}
	if uint64(size)*uint64(objectsPerElement) > uint64(remaining) {
		return fmt.Errorf("msgpack %s length %d exceeds remaining payload size %d", kind, size, remaining)
	}
	return nil
}

func validateBin(b []byte) ([]byte, error) {
	size, remain, err := msgp.ReadBytesHeader(b)
	if err != nil {
		return b, err
	}
	if size > maxMsgpackBinStringLen {
		return b, fmt.Errorf("msgpack bin length %d exceeds limit %d", size, maxMsgpackBinStringLen)
	}
	if uint64(size) > uint64(len(remain)) {
		return b, msgp.ErrShortBytes
	}
	return remain[size:], nil
}

func validateString(b []byte) ([]byte, error) {
	size, remain, err := readStringHeader(b)
	if err != nil {
		return b, err
	}
	if size > maxMsgpackBinStringLen {
		return b, fmt.Errorf("msgpack string length %d exceeds limit %d", size, maxMsgpackBinStringLen)
	}
	if uint64(size) > uint64(len(remain)) {
		return b, msgp.ErrShortBytes
	}
	return remain[size:], nil
}

func validateExtension(b []byte) ([]byte, error) {
	size, remain, err := readExtensionHeader(b)
	if err != nil {
		return b, err
	}
	if size > maxMsgpackExtLen {
		return b, fmt.Errorf("msgpack extension length %d exceeds limit %d", size, maxMsgpackExtLen)
	}
	if uint64(size) > uint64(len(remain)) {
		return b, msgp.ErrShortBytes
	}
	return remain[size:], nil
}

func isExtensionPrefix(prefix byte) bool {
	switch prefix {
	case 0xc7, 0xc8, 0xc9, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8:
		return true
	default:
		return false
	}
}

func readStringHeader(b []byte) (uint32, []byte, error) {
	if len(b) == 0 {
		return 0, b, msgp.ErrShortBytes
	}
	lead := b[0]
	if lead&0xe0 == 0xa0 {
		return uint32(lead & 0x1f), b[1:], nil
	}
	switch lead {
	case 0xd9:
		if len(b) < 2 {
			return 0, b, msgp.ErrShortBytes
		}
		return uint32(b[1]), b[2:], nil
	case 0xda:
		if len(b) < 3 {
			return 0, b, msgp.ErrShortBytes
		}
		return uint32(binary.BigEndian.Uint16(b[1:3])), b[3:], nil
	case 0xdb:
		if len(b) < 5 {
			return 0, b, msgp.ErrShortBytes
		}
		return binary.BigEndian.Uint32(b[1:5]), b[5:], nil
	default:
		return 0, b, msgp.TypeError{Method: msgp.StrType, Encoded: msgp.NextType(b)}
	}
}

func readExtensionHeader(b []byte) (uint32, []byte, error) {
	if len(b) == 0 {
		return 0, b, msgp.ErrShortBytes
	}
	switch b[0] {
	case 0xd4:
		if len(b) < 2 {
			return 0, b, msgp.ErrShortBytes
		}
		return 1, b[2:], nil
	case 0xd5:
		if len(b) < 2 {
			return 0, b, msgp.ErrShortBytes
		}
		return 2, b[2:], nil
	case 0xd6:
		if len(b) < 2 {
			return 0, b, msgp.ErrShortBytes
		}
		return 4, b[2:], nil
	case 0xd7:
		if len(b) < 2 {
			return 0, b, msgp.ErrShortBytes
		}
		return 8, b[2:], nil
	case 0xd8:
		if len(b) < 2 {
			return 0, b, msgp.ErrShortBytes
		}
		return 16, b[2:], nil
	case 0xc7:
		if len(b) < 3 {
			return 0, b, msgp.ErrShortBytes
		}
		return uint32(b[1]), b[3:], nil
	case 0xc8:
		if len(b) < 4 {
			return 0, b, msgp.ErrShortBytes
		}
		return uint32(binary.BigEndian.Uint16(b[1:3])), b[4:], nil
	case 0xc9:
		if len(b) < 6 {
			return 0, b, msgp.ErrShortBytes
		}
		return binary.BigEndian.Uint32(b[1:5]), b[6:], nil
	default:
		return 0, b, fmt.Errorf("msgpack prefix 0x%x is not an extension", b[0])
	}
}
