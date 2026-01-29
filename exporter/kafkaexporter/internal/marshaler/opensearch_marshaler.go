package marshaler

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// OpenSearchLogsMarshaler marshals logs to OpenSearch SS4O JSON format.
type OpenSearchLogsMarshaler struct{ unixTimestamps bool }

func (m *OpenSearchLogsMarshaler) MarshalLogs(logs plog.Logs) ([]Message, error) {
	var total int
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		for j := 0; j < logs.ResourceLogs().At(i).ScopeLogs().Len(); j++ {
			total += logs.ResourceLogs().At(i).ScopeLogs().At(j).LogRecords().Len()
		}
	}
	if total == 0 {
		return nil, nil
	}
	msgs, buf := make([]Message, 0, total), new(bytes.Buffer)
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		rl := logs.ResourceLogs().At(i)
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			for k := 0; k < sl.LogRecords().Len(); k++ {
				rec := sl.LogRecords().At(k)
				buf.Reset()
				m.encode(buf, rec, rl.Resource(), sl.Scope(), sl.SchemaUrl())
				val := make([]byte, buf.Len())
				copy(val, buf.Bytes())
				var key []byte
				if !rec.TraceID().IsEmpty() {
					t := rec.TraceID()
					key = t[:]
				}
				msgs = append(msgs, Message{Key: key, Value: val})
			}
		}
	}
	return msgs, nil
}

func (m *OpenSearchLogsMarshaler) encode(b *bytes.Buffer, r plog.LogRecord, res pcommon.Resource, scope pcommon.InstrumentationScope, schema string) {
	fmt.Fprintf(b, `{"@timestamp":%s,"body":%s`, m.fmtTS(r.Timestamp()), strconv.Quote(r.Body().AsString()))
	if r.ObservedTimestamp() != 0 {
		fmt.Fprintf(b, `,"observedTimestamp":%s`, m.fmtTS(r.ObservedTimestamp()))
	}
	if !r.TraceID().IsEmpty() {
		t := r.TraceID()
		fmt.Fprintf(b, `,"traceId":"%s"`, hex.EncodeToString(t[:]))
	}
	if !r.SpanID().IsEmpty() {
		s := r.SpanID()
		fmt.Fprintf(b, `,"spanId":"%s"`, hex.EncodeToString(s[:]))
	}
	fmt.Fprintf(b, `,"severity":{"text":%s,"number":%d},"attributes":`, strconv.Quote(r.SeverityText()), r.SeverityNumber())
	writeMap(b, r.Attributes(), false)
	b.WriteString(`,"resource":`)
	writeMap(b, res.Attributes(), true)
	if schema != "" {
		fmt.Fprintf(b, `,"schemaUrl":%s`, strconv.Quote(schema))
	}
	fmt.Fprintf(b, `,"instrumentationScope":{"name":%s,"version":%s`, strconv.Quote(scope.Name()), strconv.Quote(scope.Version()))
	if schema != "" {
		fmt.Fprintf(b, `,"schemaUrl":%s`, strconv.Quote(schema))
	}
	if scope.Attributes().Len() > 0 {
		b.WriteString(`,"attributes":`)
		writeMap(b, scope.Attributes(), false)
	}
	b.WriteString("}}")
}

func (m *OpenSearchLogsMarshaler) fmtTS(ts pcommon.Timestamp) string {
	if ts == 0 {
		return "null"
	}
	if m.unixTimestamps {
		return strconv.FormatInt(ts.AsTime().UnixMilli(), 10)
	}
	return strconv.Quote(ts.AsTime().Format(time.RFC3339Nano))
}

func writeMap(b *bytes.Buffer, m pcommon.Map, strVal bool) {
	b.WriteByte('{')
	first := true
	m.Range(func(k string, v pcommon.Value) bool {
		if !first {
			b.WriteByte(',')
		}
		first = false
		b.WriteString(strconv.Quote(k))
		b.WriteByte(':')
		if strVal {
			b.WriteString(strconv.Quote(v.AsString()))
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
		b.WriteString(strconv.Quote(v.Str()))
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
		for i := 0; i < v.Slice().Len(); i++ {
			if i > 0 {
				b.WriteByte(',')
			}
			writeVal(b, v.Slice().At(i))
		}
		b.WriteByte(']')
	case pcommon.ValueTypeBytes:
		fmt.Fprintf(b, `"%s"`, hex.EncodeToString(v.Bytes().AsRaw()))
	default:
		b.WriteString("null")
	}
}

func (m *OpenSearchLogsMarshaler) Encoding() string { return "opensearch_json" }
