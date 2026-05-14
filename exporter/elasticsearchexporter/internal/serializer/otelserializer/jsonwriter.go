// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer"

import (
	"bytes"
	"encoding/hex"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

// jsonWriter is a low-overhead JSON writer that writes directly to a
// bytes.Buffer, avoiding the interface-dispatch and state-machine overhead
// of go-structform/json.Visitor.
type jsonWriter struct {
	buf *bytes.Buffer
}

func newJSONWriter(buf *bytes.Buffer) jsonWriter {
	return jsonWriter{buf: buf}
}

func (w *jsonWriter) startObject() {
	w.buf.WriteByte('{')
}

func (w *jsonWriter) endObject() {
	w.buf.WriteByte('}')
}

func (w *jsonWriter) startArray() {
	w.buf.WriteByte('[')
}

func (w *jsonWriter) endArray() {
	w.buf.WriteByte(']')
}

// key writes a JSON object key with a preceding comma if first is false.
// Returns false (the new value for the caller's "first" tracking variable).
func (w *jsonWriter) key(k string, first bool) bool {
	if !first {
		w.buf.WriteByte(',')
	}
	w.jsonString(k)
	w.buf.WriteByte(':')
	return false
}

func (w *jsonWriter) boolVal(b bool) {
	if b {
		w.buf.WriteString("true")
	} else {
		w.buf.WriteString("false")
	}
}

func (w *jsonWriter) nullVal() {
	w.buf.WriteString("null")
}

func (w *jsonWriter) int64Val(n int64) {
	b := strconv.AppendInt(w.buf.AvailableBuffer(), n, 10)
	w.buf.Write(b)
}

func (w *jsonWriter) uint64Val(n uint64) {
	b := strconv.AppendUint(w.buf.AvailableBuffer(), n, 10)
	w.buf.Write(b)
}

// float64Val writes a float64, always including a radix point (e.g. 1.0 not 1)
// to preserve type information for ES dynamic mapping.
func (w *jsonWriter) float64Val(val float64) {
	if math.IsInf(val, 0) || math.IsNaN(val) {
		w.buf.WriteString("null")
		return
	}
	b := strconv.AppendFloat(w.buf.AvailableBuffer(), val, 'g', -1, 64)
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
		w.buf.Write(b[:expIdx])
		w.buf.WriteString(".0")
		w.buf.Write(tail[:n])
	} else {
		w.buf.Write(b)
	}
}

func (w *jsonWriter) arrayComma(first bool) bool {
	if !first {
		w.buf.WriteByte(',')
	}
	return false
}

// jsonString writes a JSON-escaped string (with surrounding quotes).
// Uses the same HTML-safe escaping as go-structform and go-fastjson.
func (w *jsonWriter) jsonString(s string) {
	w.buf.WriteByte('"')
	p := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			if htmlSafeSet[c] {
				i++
				continue
			}
			w.buf.WriteString(s[p:i])
			switch c {
			case '\\':
				w.buf.WriteString(`\\`)
			case '"':
				w.buf.WriteString(`\"`)
			case '\b':
				w.buf.WriteString(`\b`)
			case '\f':
				w.buf.WriteString(`\f`)
			case '\n':
				w.buf.WriteString(`\n`)
			case '\r':
				w.buf.WriteString(`\r`)
			case '\t':
				w.buf.WriteString(`\t`)
			default:
				w.buf.WriteString(`\u00`)
				w.buf.WriteByte(hexChars[c>>4])
				w.buf.WriteByte(hexChars[c&0xf])
			}
			i++
			p = i
			continue
		}
		runeValue, runeWidth := utf8.DecodeRuneInString(s[i:])
		if runeValue == utf8.RuneError && runeWidth == 1 {
			w.buf.WriteString(s[p:i])
			w.buf.WriteString(`\ufffd`)
			i++
			p = i
			continue
		}
		if runeValue == '\u2028' || runeValue == '\u2029' {
			w.buf.WriteString(s[p:i])
			w.buf.WriteString(`\u202`)
			w.buf.WriteByte(hexChars[runeValue&0xf])
			i += runeWidth
			p = i
			continue
		}
		i += runeWidth
	}
	w.buf.WriteString(s[p:])
	w.buf.WriteByte('"')
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

// writeTimestampField writes "@timestamp" or similar with msec.nsec format.
func (w *jsonWriter) writeTimestampField(key string, timestamp pcommon.Timestamp, first bool) bool {
	nsec := uint64(timestamp)
	msec := nsec / 1e6
	nsec -= msec * 1e6
	first = w.key(key, first)
	w.buf.WriteByte('"')
	b := strconv.AppendUint(w.buf.AvailableBuffer(), msec, 10)
	w.buf.Write(b)
	w.buf.WriteByte('.')
	b = strconv.AppendUint(w.buf.AvailableBuffer(), nsec, 10)
	w.buf.Write(b)
	w.buf.WriteByte('"')
	return first
}

func (w *jsonWriter) writeTimestampEpochMillisField(key string, timestamp pcommon.Timestamp, first bool) bool {
	first = w.key(key, first)
	w.uint64Val(uint64(timestamp) / 1e6)
	return first
}

func (w *jsonWriter) writeUIntField(key string, val uint64, first bool) bool {
	first = w.key(key, first)
	w.uint64Val(val)
	return first
}

func (w *jsonWriter) writeStringFieldSkipDefault(key, value string, first bool) bool {
	if value == "" {
		return first
	}
	first = w.key(key, first)
	w.jsonString(value)
	return first
}

func (w *jsonWriter) writeIntFieldSkipDefault(key string, val int64, first bool) bool {
	if val == 0 {
		return first
	}
	first = w.key(key, first)
	w.int64Val(val)
	return first
}

func (w *jsonWriter) writeTraceIDField(id pcommon.TraceID, first bool) bool {
	if id.IsEmpty() {
		return first
	}
	first = w.key("trace_id", first)
	w.buf.WriteByte('"')
	b := hex.AppendEncode(w.buf.AvailableBuffer(), id[:])
	w.buf.Write(b)
	w.buf.WriteByte('"')
	return first
}

func (w *jsonWriter) writeSpanIDField(key string, id pcommon.SpanID, first bool) bool {
	if id.IsEmpty() {
		return first
	}
	first = w.key(key, first)
	w.buf.WriteByte('"')
	b := hex.AppendEncode(w.buf.AvailableBuffer(), id[:])
	w.buf.Write(b)
	w.buf.WriteByte('"')
	return first
}

func (w *jsonWriter) writeDataStream(idx elasticsearch.Index, first bool) bool {
	if !idx.IsDataStream() {
		return first
	}
	first = w.key("data_stream", first)
	w.startObject()
	firstField := true
	firstField = w.writeStringFieldSkipDefault("type", idx.Type, firstField)
	firstField = w.writeStringFieldSkipDefault("dataset", idx.Dataset, firstField)
	_ = w.writeStringFieldSkipDefault("namespace", idx.Namespace, firstField)
	w.endObject()
	return first
}

func (w *jsonWriter) writeResource(resource pcommon.Resource, resourceSchemaURL string, stringifyMapAttributes, first bool) bool {
	first = w.key("resource", first)
	w.startObject()
	firstField := true
	firstField = w.writeStringFieldSkipDefault("schema_url", resourceSchemaURL, firstField)
	firstField = w.writeAttributes(resource.Attributes(), stringifyMapAttributes, firstField)
	_ = w.writeIntFieldSkipDefault("dropped_attributes_count", int64(resource.DroppedAttributesCount()), firstField)
	w.endObject()
	return first
}

func (w *jsonWriter) writeScope(scope pcommon.InstrumentationScope, scopeSchemaURL string, stringifyMapAttributes, first bool) bool {
	first = w.key("scope", first)
	w.startObject()
	firstField := true
	firstField = w.writeStringFieldSkipDefault("schema_url", scopeSchemaURL, firstField)
	firstField = w.writeStringFieldSkipDefault("name", scope.Name(), firstField)
	firstField = w.writeStringFieldSkipDefault("version", scope.Version(), firstField)
	firstField = w.writeAttributes(scope.Attributes(), stringifyMapAttributes, firstField)
	_ = w.writeIntFieldSkipDefault("dropped_attributes_count", int64(scope.DroppedAttributesCount()), firstField)
	w.endObject()
	return first
}

func (w *jsonWriter) writeAttributes(attributes pcommon.Map, stringifyMapValues, first bool) bool {
	if attributes.Len() == 0 {
		return first
	}

	first = w.key("attributes", first)
	w.startObject()
	firstAttr := true

	seenKeys := make(map[string]struct{})

	for k, val := range attributes.All() {
		if _, exists := seenKeys[k]; exists {
			continue
		}
		seenKeys[k] = struct{}{}

		switch k {
		case elasticsearch.DataStreamType,
			elasticsearch.DataStreamDataset,
			elasticsearch.DataStreamNamespace,
			elasticsearch.MappingHintsAttrKey,
			elasticsearch.MappingModeAttributeName,
			elasticsearch.DocumentIDAttributeName,
			elasticsearch.DocumentPipelineAttributeName,
			elasticsearch.IndexAttributeName:
			continue
		}
		if isGeoAttribute(k, val) {
			continue
		}
		firstAttr = w.key(k, firstAttr)
		w.writeValue(val, stringifyMapValues)
	}
	w.writeGeolocationAttributes(attributes, firstAttr)
	w.endObject()
	return first
}

func (w *jsonWriter) writeValue(val pcommon.Value, stringifyMaps bool) {
	switch val.Type() {
	case pcommon.ValueTypeEmpty:
		w.nullVal()
	case pcommon.ValueTypeStr:
		w.jsonString(val.Str())
	case pcommon.ValueTypeBool:
		w.boolVal(val.Bool())
	case pcommon.ValueTypeDouble:
		w.float64Val(val.Double())
	case pcommon.ValueTypeInt:
		w.int64Val(val.Int())
	case pcommon.ValueTypeBytes:
		w.buf.WriteByte('"')
		b := hex.AppendEncode(w.buf.AvailableBuffer(), val.Bytes().AsRaw())
		w.buf.Write(b)
		w.buf.WriteByte('"')
	case pcommon.ValueTypeMap:
		if stringifyMaps {
			w.jsonString(val.AsString())
		} else {
			w.writeMap(val.Map())
		}
	case pcommon.ValueTypeSlice:
		w.startArray()
		firstElem := true
		for _, item := range val.Slice().All() {
			firstElem = w.arrayComma(firstElem)
			w.writeValue(item, stringifyMaps)
		}
		w.endArray()
	}
}

func (w *jsonWriter) writeMap(m pcommon.Map) {
	w.startObject()
	first := true
	for k, val := range m.All() {
		first = w.key(k, first)
		w.writeValue(val, false)
	}
	w.endObject()
}

func (w *jsonWriter) writeGeolocationAttributes(attributes pcommon.Map, first bool) bool {
	const (
		lonKey    = "geo.location.lon"
		latKey    = "geo.location.lat"
		mergedKey = "geo.location"
	)
	type geoEntry struct {
		lon, lat       float64
		lonSet, latSet bool
	}
	var prefixToGeo map[string]*geoEntry
	getOrCreate := func(prefix string) *geoEntry {
		if prefixToGeo == nil {
			prefixToGeo = make(map[string]*geoEntry)
		}
		if g, ok := prefixToGeo[prefix]; ok {
			return g
		}
		g := &geoEntry{}
		prefixToGeo[prefix] = g
		return g
	}

	for key, val := range attributes.All() {
		if val.Type() != pcommon.ValueTypeDouble {
			continue
		}
		if key == lonKey {
			g := getOrCreate("")
			g.lon = val.Double()
			g.lonSet = true
		} else if key == latKey {
			g := getOrCreate("")
			g.lat = val.Double()
			g.latSet = true
		} else if namespace, found := strings.CutSuffix(key, "."+lonKey); found {
			g := getOrCreate(namespace + ".")
			g.lon = val.Double()
			g.lonSet = true
		} else if namespace, found := strings.CutSuffix(key, "."+latKey); found {
			g := getOrCreate(namespace + ".")
			g.lat = val.Double()
			g.latSet = true
		}
	}

	for prefix, geo := range prefixToGeo {
		if geo.lonSet && geo.latSet {
			k := prefix + mergedKey
			first = w.key(k, first)
			w.startArray()
			w.float64Val(geo.lon)
			w.buf.WriteByte(',')
			w.float64Val(geo.lat)
			w.endArray()
			continue
		}
		if geo.lonSet {
			first = w.key(prefix+lonKey, first)
			w.float64Val(geo.lon)
		}
		if geo.latSet {
			first = w.key(prefix+latKey, first)
			w.float64Val(geo.lat)
		}
	}
	return first
}
