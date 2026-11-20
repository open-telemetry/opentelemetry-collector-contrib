// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package xconfigunit provides configuration types for unit-based values.
package xconfigunit // import "github.com/open-telemetry/opentelemetry-collector-contrib/config/xconfigunit"

import (
	"bytes"
	"encoding"
	"errors"
	"fmt"
	"math"
	"strconv"
	"unicode"
)

// ByteSize represents a signed byte count. Its text form has an optional leading
// minus sign followed by one or more concatenated integer units, such as
// "100MB20KB" or "-100MB20KB". It accepts B, K/KB, M/MB, and G/GB; KB, MB, and
// GB use decimal multipliers. A leading minus sign applies to the entire value.
type ByteSize int64

const (
	// ByteUnit is the multiplier for bytes.
	ByteUnit = 1
	// KiloByteUnit is the decimal multiplier for kilobytes.
	KiloByteUnit = 1000
	// MegaByteUnit is the decimal multiplier for megabytes.
	MegaByteUnit = 1000 * KiloByteUnit
	// GigaByteUnit is the decimal multiplier for gigabytes.
	GigaByteUnit = 1000 * MegaByteUnit
)

var (
	bytesUnits = map[rune]ByteSize{
		'B': ByteUnit,
		'K': KiloByteUnit,
		'M': MegaByteUnit,
		'G': GigaByteUnit,
	}
	bytesSymbols = map[ByteSize]rune{
		ByteUnit:     'B',
		KiloByteUnit: 'K',
		MegaByteUnit: 'M',
		GigaByteUnit: 'G',
	}
)

var (
	_ encoding.TextUnmarshaler = (*ByteSize)(nil)
	_ encoding.TextAppender    = (*ByteSize)(nil)
	_ encoding.TextMarshaler   = (*ByteSize)(nil)
	_ fmt.Stringer             = (*ByteSize)(nil)
)

// AppendText appends ByteSize as concatenated byte units to b.
func (bs ByteSize) AppendText(b []byte) ([]byte, error) {
	if bs == 0 {
		return append(b, '0', 'B'), nil
	}
	if bs < 0 {
		bs, b = -bs, append(b, '-')
	}
	for _, u := range []ByteSize{GigaByteUnit, MegaByteUnit, KiloByteUnit, ByteUnit} {
		if n := int64(bs / u); n > 0 {
			b = strconv.AppendInt(b, n, 10)
			b = append(b, byte(bytesSymbols[u]))
			if u != ByteUnit {
				b = append(b, 'B')
			}
			bs -= ByteSize(n) * u
		}
	}
	return b, nil
}

// MarshalText returns ByteSize as concatenated byte units.
func (bs ByteSize) MarshalText() ([]byte, error) {
	return bs.AppendText(nil)
}

// String returns ByteSize as concatenated byte units.
func (bs ByteSize) String() string {
	v, _ := bs.AppendText(nil)
	return string(v)
}

// UnmarshalText parses a signed value expressed as concatenated byte units.
func (bs *ByteSize) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		return errors.New("empty byte size")
	}
	negative := text[0] == '-'
	if negative {
		text = text[1:]
	}

	var total ByteSize
	for len(text) > 0 {
		end := bytes.IndexFunc(text, func(r rune) bool { return !unicode.IsDigit(r) })
		if end <= 0 {
			return fmt.Errorf("invalid byte size %q", text)
		}

		value, err := strconv.ParseInt(string(text[:end]), 10, 64)
		if err != nil {
			return err
		}

		text = text[end:]
		unit := bytesUnits[rune(text[0])]
		if unit == 0 {
			return fmt.Errorf("invalid byte unit in %q", text)
		}
		text = text[1:]
		if unit != ByteUnit && len(text) > 0 && text[0] == 'B' {
			text = text[1:]
		}

		factor := int64(unit)
		if value > (math.MaxInt64-int64(total))/factor {
			return errors.New("byte size overflows int64")
		}
		total += ByteSize(value * factor)
	}
	if negative {
		total = -total
	}
	*bs = total
	return nil
}
