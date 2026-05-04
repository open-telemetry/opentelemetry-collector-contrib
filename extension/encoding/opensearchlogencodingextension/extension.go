// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchlogencodingextension // import "github.com/cloudoperators/opentelemetry-collector-contrib/extension/encoding/opensearchlogencodingextension"

import (
	"bytes"
	"context"
	"encoding/hex"
	"strconv"
	"time"
	"unicode/utf8"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

var _ encoding.LogsMarshalerExtension = (*opensearchLogExtension)(nil)

// estimatedBytesPerRecord is a rough estimate for pre-sizing the output buffer.
const estimatedBytesPerRecord = 256

type opensearchLogExtension struct{}

// MarshalLogs encodes logs as NDJSON following the SS4O schema.
// Each log record becomes one JSON line. Returns nil for empty input.
func (e *opensearchLogExtension) MarshalLogs(ld plog.Logs) ([]byte, error) {
	var total int
	for i := range ld.ResourceLogs().Len() {
		for j := range ld.ResourceLogs().At(i).ScopeLogs().Len() {
			total += ld.ResourceLogs().At(i).ScopeLogs().At(j).LogRecords().Len()
		}
	}
	if total == 0 {
		return nil, nil
	}

	out := bytes.NewBuffer(make([]byte, 0, total*estimatedBytesPerRecord))
	first := true
	for i := range ld.ResourceLogs().Len() {
		rl := ld.ResourceLogs().At(i)
		for j := range rl.ScopeLogs().Len() {
			sl := rl.ScopeLogs().At(j)
			for k := range sl.LogRecords().Len() {
				if !first {
					out.WriteByte('\n')
				}
				first = false
				encodeRecord(out, sl.LogRecords().At(k), rl.Resource(), sl.Scope(), sl.SchemaUrl())
			}
		}
	}
	return out.Bytes(), nil
}

func encodeRecord(b *bytes.Buffer, r plog.LogRecord, res pcommon.Resource, scope pcommon.InstrumentationScope, schemaURL string) {
	b.WriteString(`{"@timestamp":`)
	writeTimestamp(b, r.Timestamp())
	b.WriteString(`,"body":`)
	writeJSONString(b, r.Body().AsString())
	if r.ObservedTimestamp() != 0 {
		b.WriteString(`,"observedTimestamp":`)
		writeTimestamp(b, r.ObservedTimestamp())
	}
	if !r.TraceID().IsEmpty() {
		tid := r.TraceID()
		b.WriteString(`,"traceId":"`)
		b.WriteString(hex.EncodeToString(tid[:]))
		b.WriteByte('"')
	}
	if !r.SpanID().IsEmpty() {
		sid := r.SpanID()
		b.WriteString(`,"spanId":"`)
		b.WriteString(hex.EncodeToString(sid[:]))
		b.WriteByte('"')
	}
	b.WriteString(`,"severity":{"text":`)
	writeJSONString(b, r.SeverityText())
	b.WriteString(`,"number":`)
	b.WriteString(strconv.FormatInt(int64(r.SeverityNumber()), 10))
	b.WriteString(`},"attributes":`)
	writeMap(b, r.Attributes(), false)
	b.WriteString(`,"resource":`)
	writeMap(b, res.Attributes(), true)
	if schemaURL != "" {
		b.WriteString(`,"schemaUrl":`)
		writeJSONString(b, schemaURL)
	}
	b.WriteString(`,"instrumentationScope":{"name":`)
	writeJSONString(b, scope.Name())
	b.WriteString(`,"version":`)
	writeJSONString(b, scope.Version())
	if schemaURL != "" {
		b.WriteString(`,"schemaUrl":`)
		writeJSONString(b, schemaURL)
	}
	if scope.Attributes().Len() > 0 {
		b.WriteString(`,"attributes":`)
		writeMap(b, scope.Attributes(), false)
	}
	b.WriteString("}}")
}

func writeTimestamp(b *bytes.Buffer, ts pcommon.Timestamp) {
	if ts == 0 {
		b.WriteString("null")
		return
	}
	b.WriteByte('"')
	b.WriteString(ts.AsTime().Format(time.RFC3339Nano))
	b.WriteByte('"')
}

func writeMap(b *bytes.Buffer, m pcommon.Map, strVal bool) {
	b.WriteByte('{')
	first := true
	m.Range(func(k string, v pcommon.Value) bool {
		if !first {
			b.WriteByte(',')
		}
		first = false
		writeJSONString(b, k)
		b.WriteByte(':')
		if strVal {
			writeJSONString(b, v.AsString())
		} else {
			writeVal(b, v)
		}
		return true
	})
	b.WriteByte('}')
}

func writeVal(b *bytes.Buffer, v pcommon.Value) {
	switch v.Type() {
	case pcommon.ValueTypeStr:
		writeJSONString(b, v.Str())
	case pcommon.ValueTypeBool:
		b.WriteString(strconv.FormatBool(v.Bool()))
	case pcommon.ValueTypeInt:
		b.WriteString(strconv.FormatInt(v.Int(), 10))
	case pcommon.ValueTypeDouble:
		b.WriteString(strconv.FormatFloat(v.Double(), 'g', -1, 64))
	case pcommon.ValueTypeMap:
		writeMap(b, v.Map(), false)
	case pcommon.ValueTypeSlice:
		b.WriteByte('[')
		for i := range v.Slice().Len() {
			if i > 0 {
				b.WriteByte(',')
			}
			writeVal(b, v.Slice().At(i))
		}
		b.WriteByte(']')
	case pcommon.ValueTypeBytes:
		b.WriteByte('"')
		b.WriteString(hex.EncodeToString(v.Bytes().AsRaw()))
		b.WriteByte('"')
	default:
		b.WriteString("null")
	}
}

// hexTable is a lookup table for hex encoding JSON unicode escapes.
const hexChars = "0123456789abcdef"

// writeJSONString writes s as a JSON-escaped quoted string directly to b,
// avoiding the allocation that json.Marshal would incur.
func writeJSONString(b *bytes.Buffer, s string) {
	b.WriteByte('"')
	start := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c >= utf8.RuneSelf {
			// Multi-byte UTF-8: valid JSON, pass through.
			_, size := utf8.DecodeRuneInString(s[i:])
			i += size
			continue
		}
		// Single-byte: check if it needs escaping.
		var esc string
		switch c {
		case '"':
			esc = `\"`
		case '\\':
			esc = `\\`
		case '\b':
			esc = `\b`
		case '\f':
			esc = `\f`
		case '\n':
			esc = `\n`
		case '\r':
			esc = `\r`
		case '\t':
			esc = `\t`
		default:
			if c < 0x20 {
				// Control character: \u00XX
				b.WriteString(s[start:i])
				b.WriteString(`\u00`)
				b.WriteByte(hexChars[c>>4])
				b.WriteByte(hexChars[c&0xf])
				i++
				start = i
				continue
			}
			i++
			continue
		}
		b.WriteString(s[start:i])
		b.WriteString(esc)
		i++
		start = i
	}
	b.WriteString(s[start:])
	b.WriteByte('"')
}

func (*opensearchLogExtension) Start(context.Context, component.Host) error {
	return nil
}

func (*opensearchLogExtension) Shutdown(context.Context) error {
	return nil
}
