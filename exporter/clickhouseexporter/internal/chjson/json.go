// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package chjson // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal/chjson"

import (
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// AppendSpanIDToHex writes a hex encoded byte slice of SpanID.
// If SpanID is empty dst is returned with zero length.
func AppendSpanIDToHex(dst []byte, id pcommon.SpanID) []byte {
	if id.IsEmpty() {
		return dst[:0]
	}

	return hex.AppendEncode(dst, id[:])
}

// AppendTraceIDToHex writes a hex encoded byte slice of TraceID.
// If TraceID is empty dst is returned with zero length.
func AppendTraceIDToHex(dst []byte, id pcommon.TraceID) []byte {
	if id.IsEmpty() {
		return dst[:0]
	}

	return hex.AppendEncode(dst, id[:])
}

func NewJSONBuffer(jsonSize, base64Size int) *JSONBuffer {
	return &JSONBuffer{
		buf:          make([]byte, 0, jsonSize),
		base64Buffer: make([]byte, 0, base64Size),
	}
}

// AttributesToJSON serializes attributes to JSON using a reusable buffer
func AttributesToJSON(b *JSONBuffer, m pcommon.Map) {
	if m.Len() == 0 {
		b.WriteString("{}")
		return
	}

	b.grow(2)
	b.buf = append(b.buf, '{')
	first := true
	m.Range(func(k string, v pcommon.Value) bool {
		if first {
			first = false
		} else {
			b.WriteByte(',')
		}

		b.WriteQuote(k)
		b.WriteByte(':')
		ValueToJSON(b, v)
		return true
	})
	b.buf = append(b.buf, '}')
}

func ValueToJSON(b *JSONBuffer, v pcommon.Value) {
	switch v.Type() {
	case pcommon.ValueTypeEmpty:
		b.WriteString("null")
	case pcommon.ValueTypeStr:
		b.WriteQuote(v.Str())
	case pcommon.ValueTypeBool:
		if v.Bool() {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case pcommon.ValueTypeDouble:
		b.buf = strconv.AppendFloat(b.buf, v.Double(), 'g', -1, 64)
	case pcommon.ValueTypeInt:
		b.buf = strconv.AppendInt(b.buf, v.Int(), 10)
	case pcommon.ValueTypeBytes:
		SerializeBytesBase64(b, v.Bytes())
	case pcommon.ValueTypeMap:
		AttributesToJSON(b, v.Map())
	case pcommon.ValueTypeSlice:
		SerializeSlice(b, v.Slice())
	default:
		b.WriteString("null")
	}
}

func SerializeSlice(b *JSONBuffer, s pcommon.Slice) {
	if s.Len() == 0 {
		b.WriteString("[]")
		return
	}

	b.grow(2)
	b.buf = append(b.buf, '[')
	sLen := s.Len()
	for i := 0; i < sLen; i++ {
		if i > 0 {
			b.WriteByte(',')
		}

		ValueToJSON(b, s.At(i))
	}
	b.buf = append(b.buf, ']')
}

func SerializeBytesBase64(b *JSONBuffer, bs pcommon.ByteSlice) {
	b.base64Buffer = copyByteSlice(b.base64Buffer, bs)
	n := base64.StdEncoding.EncodedLen(len(b.base64Buffer))

	start := len(b.buf)
	b.grow(n + 2)

	b.buf = append(b.buf, '"')

	dst := b.buf[start+1 : start+1+n]
	base64.StdEncoding.Encode(dst, b.base64Buffer)
	b.buf = b.buf[:start+1+n]

	b.buf = append(b.buf, '"')
}

// copyByteSlice copies the data from the ByteSlice into the dst.
// There's no way to simply get the underlying *[]byte, unfortunately
// Removes an allocation in exchange for CPU
func copyByteSlice(dst []byte, bs pcommon.ByteSlice) []byte {
	dst = dst[:0]

	bsLen := bs.Len()
	for i := 0; i < bsLen; i++ {
		dst = append(dst, bs.At(i))
	}

	return dst
}

// JSONBuffer is a reusable buffer for faster JSON serialization
type JSONBuffer struct {
	buf          []byte
	base64Buffer []byte // TODO: this buffer shouldn't go here
}

func (b *JSONBuffer) Reset() {
	b.buf = b.buf[:0]
}

func (b *JSONBuffer) Bytes() []byte {
	return b.buf
}

func (b *JSONBuffer) grow(n int) {
	if cap(b.buf)-len(b.buf) < n {
		buf := make([]byte, len(b.buf), 2*cap(b.buf)+n)
		copy(buf, b.buf)
		b.buf = buf
	}
}

func (b *JSONBuffer) WriteString(s string) {
	b.grow(len(s))
	b.buf = append(b.buf, s...)
}

func (b *JSONBuffer) WriteByte(c byte) {
	b.grow(1)
	b.buf = append(b.buf, c)
}

// WriteQuote writes a quoted string to the buffer
func (b *JSONBuffer) WriteQuote(s string) {
	b.grow(len(s) + 2)

	// This is a quicker way to check whether the given string contains JSON-like data.
	// AppendQuote is expensive vs a simplified slice append.
	// It's a risky optimization, but takes half the CPU time.
	// It's possible a single character could break the
	// server's JSON processor, but this has not happened yet in testing.
	if strings.ContainsAny(s, `"\:`) {
		b.buf = strconv.AppendQuote(b.buf, s)
	} else {
		b.buf = append(b.buf, '"')
		b.buf = append(b.buf, s...)
		b.buf = append(b.buf, '"')
	}
}
