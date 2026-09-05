// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonwriter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/jsonwriter"

import (
	"bytes"
	"math"
	"strconv"
	"unicode/utf8"
)

// Writer writes JSON directly to a bytes.Buffer, avoiding the
// interface-dispatch and state-machine overhead of go-structform/json.Visitor.
type Writer struct {
	Buf *bytes.Buffer
}

func New(buf *bytes.Buffer) *Writer {
	return &Writer{Buf: buf}
}

func (w *Writer) StartObject() {
	w.Buf.WriteByte('{')
}

func (w *Writer) EndObject() {
	w.Buf.WriteByte('}')
}

func (w *Writer) StartArray() {
	w.Buf.WriteByte('[')
}

func (w *Writer) EndArray() {
	w.Buf.WriteByte(']')
}

// Key writes a JSON object key with a preceding comma if first is false.
// Returns false (the new value for the caller's "first" tracking variable).
func (w *Writer) Key(k string, first bool) bool {
	if !first {
		w.Buf.WriteByte(',')
	}
	w.JSONString(k)
	w.Buf.WriteByte(':')
	return false
}

func (w *Writer) BoolVal(b bool) {
	if b {
		w.Buf.WriteString("true")
	} else {
		w.Buf.WriteString("false")
	}
}

func (w *Writer) NullVal() {
	w.Buf.WriteString("null")
}

func (w *Writer) Int64Val(n int64) {
	b := strconv.AppendInt(w.Buf.AvailableBuffer(), n, 10)
	w.Buf.Write(b)
}

func (w *Writer) Uint64Val(n uint64) {
	b := strconv.AppendUint(w.Buf.AvailableBuffer(), n, 10)
	w.Buf.Write(b)
}

// Float64Val writes a float64, always including a radix point (e.g. 1.0 not 1)
// to preserve type information for ES dynamic mapping.
func (w *Writer) Float64Val(val float64) {
	if math.IsInf(val, 0) || math.IsNaN(val) {
		w.Buf.WriteString("null")
		return
	}
	b := strconv.AppendFloat(w.Buf.AvailableBuffer(), val, 'g', -1, 64)
	needDot := true
	expIdx := len(b)
	for i, c := range b {
		if c == 'e' {
			expIdx = i
			break
		}
		if c == '.' {
			needDot = false
			break
		}
	}
	if needDot {
		// Insert ".0" before exponent.
		// Copy tail for reuse below. Any write to buf would overwrite the
		// remaining b content, leading to a corruption in the tail part.
		// tail length is based on IEEE 754 max exponent of +308 or min exponent of -324, padded
		// for alignment.
		var tail [8]byte
		n := copy(tail[:], b[expIdx:])
		w.Buf.Write(b[:expIdx])
		w.Buf.WriteString(".0")
		w.Buf.Write(tail[:n])
	} else {
		w.Buf.Write(b)
	}
}

func (w *Writer) ArrayComma(first bool) bool {
	if !first {
		w.Buf.WriteByte(',')
	}
	return false
}

// JSONString writes a JSON-escaped string (with surrounding quotes).
// Uses the same HTML-safe escaping as go-structform and go-fastjson.
func (w *Writer) JSONString(s string) {
	w.Buf.WriteByte('"')
	p := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			if htmlSafeSet[c] {
				i++
				continue
			}
			w.Buf.WriteString(s[p:i])
			switch c {
			case '\\':
				w.Buf.WriteString(`\\`)
			case '"':
				w.Buf.WriteString(`\"`)
			case '\b':
				w.Buf.WriteString(`\b`)
			case '\f':
				w.Buf.WriteString(`\f`)
			case '\n':
				w.Buf.WriteString(`\n`)
			case '\r':
				w.Buf.WriteString(`\r`)
			case '\t':
				w.Buf.WriteString(`\t`)
			default:
				w.Buf.WriteString(`\u00`)
				w.Buf.WriteByte(hexChars[c>>4])
				w.Buf.WriteByte(hexChars[c&0xf])
			}
			i++
			p = i
			continue
		}
		runeValue, runeWidth := utf8.DecodeRuneInString(s[i:])
		if runeValue == utf8.RuneError && runeWidth == 1 {
			w.Buf.WriteString(s[p:i])
			w.Buf.WriteString(`\ufffd`)
			i++
			p = i
			continue
		}
		if runeValue == '\u2028' || runeValue == '\u2029' {
			w.Buf.WriteString(s[p:i])
			w.Buf.WriteString(`\u202`)
			w.Buf.WriteByte(hexChars[runeValue&0xf])
			i += runeWidth
			p = i
			continue
		}
		i += runeWidth
	}
	w.Buf.WriteString(s[p:])
	w.Buf.WriteByte('"')
}

const hexChars = "0123456789abcdef"

// htmlSafeSet matches go-structform's htmlEscapeSet (inverted): true means safe (no escape needed).
var htmlSafeSet [utf8.RuneSelf]bool

func init() {
	for i := range htmlSafeSet {
		htmlSafeSet[i] = true
	}
	for i := range 32 {
		htmlSafeSet[i] = false
	}
	for _, c := range `\"` {
		htmlSafeSet[c] = false
	}
	for _, c := range "&<>" {
		htmlSafeSet[c] = false
	}
}
