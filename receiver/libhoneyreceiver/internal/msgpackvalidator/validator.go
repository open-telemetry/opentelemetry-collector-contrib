// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package msgpackvalidator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/msgpackvalidator"

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	// These caps are intentionally above normal libhoney payload sizes; they
	// bound allocations that would otherwise be driven only by msgpack headers.
	maxMsgpackContainerElements = 1 << 20
	maxMsgpackBinStringLen      = 20 * 1024 * 1024
	maxMsgpackExtLen            = maxMsgpackBinStringLen
	maxMsgpackDepth             = 64
)

var errShortBytes = errors.New("msgpack: short bytes")

// Validate walks a single MessagePack object without materializing it.
func Validate(b []byte) error {
	scanner := msgpackScanner{b: b}
	if err := scanner.validateValue(0); err != nil {
		return err
	}
	if scanner.remaining() != 0 {
		return fmt.Errorf("msgpack has %d trailing bytes", scanner.remaining())
	}
	return nil
}

type msgpackScanner struct {
	b   []byte
	off int
}

func (s *msgpackScanner) validateValue(depth int) error {
	if depth > maxMsgpackDepth {
		return fmt.Errorf("msgpack nesting exceeds depth limit %d", maxMsgpackDepth)
	}

	c, err := s.readByte()
	if err != nil {
		return err
	}

	switch {
	case c <= 0x7f || c >= 0xe0:
		return nil
	case c >= 0x80 && c <= 0x8f:
		return s.validateMap(uint64(c&0x0f), depth)
	case c >= 0x90 && c <= 0x9f:
		return s.validateArray(uint64(c&0x0f), depth)
	case c >= 0xa0 && c <= 0xbf:
		return s.validateBytes("string", uint64(c&0x1f), maxMsgpackBinStringLen)
	}

	switch c {
	case 0xc0, 0xc2, 0xc3:
		return nil
	case 0xc1:
		return fmt.Errorf("msgpack: unknown code 0x%x", c)
	case 0xc4:
		n, err := s.readUint8()
		if err != nil {
			return err
		}
		return s.validateBytes("bin", n, maxMsgpackBinStringLen)
	case 0xc5:
		n, err := s.readUint16()
		if err != nil {
			return err
		}
		return s.validateBytes("bin", n, maxMsgpackBinStringLen)
	case 0xc6:
		n, err := s.readUint32()
		if err != nil {
			return err
		}
		return s.validateBytes("bin", n, maxMsgpackBinStringLen)
	case 0xc7:
		n, err := s.readUint8()
		if err != nil {
			return err
		}
		return s.validateExt(n)
	case 0xc8:
		n, err := s.readUint16()
		if err != nil {
			return err
		}
		return s.validateExt(n)
	case 0xc9:
		n, err := s.readUint32()
		if err != nil {
			return err
		}
		return s.validateExt(n)
	case 0xca, 0xce, 0xd2:
		return s.skip(4)
	case 0xcb, 0xcf, 0xd3:
		return s.skip(8)
	case 0xcc, 0xd0:
		return s.skip(1)
	case 0xcd, 0xd1:
		return s.skip(2)
	case 0xd4:
		return s.validateExt(1)
	case 0xd5:
		return s.validateExt(2)
	case 0xd6:
		return s.validateExt(4)
	case 0xd7:
		return s.validateExt(8)
	case 0xd8:
		return s.validateExt(16)
	case 0xd9:
		n, err := s.readUint8()
		if err != nil {
			return err
		}
		return s.validateBytes("string", n, maxMsgpackBinStringLen)
	case 0xda:
		n, err := s.readUint16()
		if err != nil {
			return err
		}
		return s.validateBytes("string", n, maxMsgpackBinStringLen)
	case 0xdb:
		n, err := s.readUint32()
		if err != nil {
			return err
		}
		return s.validateBytes("string", n, maxMsgpackBinStringLen)
	case 0xdc:
		n, err := s.readUint16()
		if err != nil {
			return err
		}
		return s.validateArray(n, depth)
	case 0xdd:
		n, err := s.readUint32()
		if err != nil {
			return err
		}
		return s.validateArray(n, depth)
	case 0xde:
		n, err := s.readUint16()
		if err != nil {
			return err
		}
		return s.validateMap(n, depth)
	case 0xdf:
		n, err := s.readUint32()
		if err != nil {
			return err
		}
		return s.validateMap(n, depth)
	default:
		return fmt.Errorf("msgpack: unknown code 0x%x", c)
	}
}

func (s *msgpackScanner) validateArray(size uint64, depth int) error {
	if err := validateElementCount("array", size, s.remaining(), 1); err != nil {
		return err
	}
	for range size {
		if err := s.validateValue(depth + 1); err != nil {
			return err
		}
	}
	return nil
}

func (s *msgpackScanner) validateMap(size uint64, depth int) error {
	if err := validateElementCount("map", size, s.remaining(), 2); err != nil {
		return err
	}
	for range size {
		if err := s.validateValue(depth + 1); err != nil {
			return err
		}
		if err := s.validateValue(depth + 1); err != nil {
			return err
		}
	}
	return nil
}

func validateElementCount(kind string, size uint64, remaining, objectsPerElement int) error {
	if size > maxMsgpackContainerElements {
		return fmt.Errorf("msgpack %s length %d exceeds limit %d", kind, size, maxMsgpackContainerElements)
	}
	if size > uint64(remaining)/uint64(objectsPerElement) {
		return fmt.Errorf("msgpack %s length %d exceeds remaining payload size %d", kind, size, remaining)
	}
	return nil
}

func (s *msgpackScanner) validateBytes(kind string, size, limit uint64) error {
	if size > limit {
		return fmt.Errorf("msgpack %s length %d exceeds limit %d", kind, size, limit)
	}
	if size > uint64(s.remaining()) {
		return fmt.Errorf("msgpack %s length %d exceeds remaining payload size %d", kind, size, s.remaining())
	}
	return s.skip(size)
}

func (s *msgpackScanner) validateExt(size uint64) error {
	if _, err := s.readByte(); err != nil {
		return err
	}
	return s.validateBytes("extension", size, maxMsgpackExtLen)
}

func (s *msgpackScanner) readByte() (byte, error) {
	if s.remaining() < 1 {
		return 0, errShortBytes
	}
	c := s.b[s.off]
	s.off++
	return c, nil
}

func (s *msgpackScanner) readUint8() (uint64, error) {
	c, err := s.readByte()
	if err != nil {
		return 0, err
	}
	return uint64(c), nil
}

func (s *msgpackScanner) readUint16() (uint64, error) {
	if s.remaining() < 2 {
		return 0, errShortBytes
	}
	n := binary.BigEndian.Uint16(s.b[s.off : s.off+2])
	s.off += 2
	return uint64(n), nil
}

func (s *msgpackScanner) readUint32() (uint64, error) {
	if s.remaining() < 4 {
		return 0, errShortBytes
	}
	n := binary.BigEndian.Uint32(s.b[s.off : s.off+4])
	s.off += 4
	return uint64(n), nil
}

func (s *msgpackScanner) skip(n uint64) error {
	if n > uint64(s.remaining()) {
		return errShortBytes
	}
	s.off += int(n)
	return nil
}

func (s *msgpackScanner) remaining() int {
	return len(s.b) - s.off
}
