// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer"

import (
	"bytes"
	"encoding/hex"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/jsonwriter"
)

type jsonWriter struct {
	*jsonwriter.Writer
}

func newJSONWriter(buf *bytes.Buffer) jsonWriter {
	return jsonWriter{Writer: jsonwriter.New(buf)}
}

// writeTimestampField writes "@timestamp" or similar with msec.nsec format.
func (w *jsonWriter) writeTimestampField(key string, timestamp pcommon.Timestamp, first bool) bool {
	nsec := uint64(timestamp)
	msec := nsec / 1e6
	nsec -= msec * 1e6
	first = w.Key(key, first)
	w.Buf.WriteByte('"')
	b := strconv.AppendUint(w.Buf.AvailableBuffer(), msec, 10)
	w.Buf.Write(b)
	w.Buf.WriteByte('.')
	b = strconv.AppendUint(w.Buf.AvailableBuffer(), nsec, 10)
	w.Buf.Write(b)
	w.Buf.WriteByte('"')
	return first
}

func (w *jsonWriter) writeTimestampEpochMillisField(key string, timestamp pcommon.Timestamp, first bool) bool {
	first = w.Key(key, first)
	w.Uint64Val(uint64(timestamp) / 1e6)
	return first
}

func (w *jsonWriter) writeUIntField(key string, val uint64, first bool) bool {
	first = w.Key(key, first)
	w.Uint64Val(val)
	return first
}

func (w *jsonWriter) writeStringFieldSkipDefault(key, value string, first bool) bool {
	if value == "" {
		return first
	}
	first = w.Key(key, first)
	w.JSONString(value)
	return first
}

func (w *jsonWriter) writeIntFieldSkipDefault(key string, val int64, first bool) bool {
	if val == 0 {
		return first
	}
	first = w.Key(key, first)
	w.Int64Val(val)
	return first
}

func (w *jsonWriter) writeTraceIDField(id pcommon.TraceID, first bool) bool {
	if id.IsEmpty() {
		return first
	}
	first = w.Key("trace_id", first)
	w.Buf.WriteByte('"')
	b := hex.AppendEncode(w.Buf.AvailableBuffer(), id[:])
	w.Buf.Write(b)
	w.Buf.WriteByte('"')
	return first
}

func (w *jsonWriter) writeSpanIDField(key string, id pcommon.SpanID, first bool) bool {
	if id.IsEmpty() {
		return first
	}
	first = w.Key(key, first)
	w.Buf.WriteByte('"')
	b := hex.AppendEncode(w.Buf.AvailableBuffer(), id[:])
	w.Buf.Write(b)
	w.Buf.WriteByte('"')
	return first
}

func (w *jsonWriter) writeDataStream(idx elasticsearch.Index, first bool) bool {
	if !idx.IsDataStream() {
		return first
	}
	first = w.Key("data_stream", first)
	w.StartObject()
	firstField := true
	firstField = w.writeStringFieldSkipDefault("type", idx.Type, firstField)
	firstField = w.writeStringFieldSkipDefault("dataset", idx.Dataset, firstField)
	_ = w.writeStringFieldSkipDefault("namespace", idx.Namespace, firstField)
	w.EndObject()
	return first
}

func (w *jsonWriter) writeResource(resource pcommon.Resource, resourceSchemaURL string, stringifyMapAttributes, first bool) bool {
	first = w.Key("resource", first)
	w.StartObject()
	firstField := true
	firstField = w.writeStringFieldSkipDefault("schema_url", resourceSchemaURL, firstField)
	firstField = w.writeAttributes(resource.Attributes(), stringifyMapAttributes, firstField)
	_ = w.writeIntFieldSkipDefault("dropped_attributes_count", int64(resource.DroppedAttributesCount()), firstField)
	w.EndObject()
	return first
}

func (w *jsonWriter) writeScope(scope pcommon.InstrumentationScope, scopeSchemaURL string, stringifyMapAttributes, first bool) bool {
	first = w.Key("scope", first)
	w.StartObject()
	firstField := true
	firstField = w.writeStringFieldSkipDefault("schema_url", scopeSchemaURL, firstField)
	firstField = w.writeStringFieldSkipDefault("name", scope.Name(), firstField)
	firstField = w.writeStringFieldSkipDefault("version", scope.Version(), firstField)
	firstField = w.writeAttributes(scope.Attributes(), stringifyMapAttributes, firstField)
	_ = w.writeIntFieldSkipDefault("dropped_attributes_count", int64(scope.DroppedAttributesCount()), firstField)
	w.EndObject()
	return first
}

func (w *jsonWriter) writeAttributes(attributes pcommon.Map, stringifyMapValues, first bool) bool {
	if attributes.Len() == 0 {
		return first
	}

	first = w.Key("attributes", first)
	w.StartObject()
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
		firstAttr = w.Key(k, firstAttr)
		w.writeValue(val, stringifyMapValues)
	}
	w.writeGeolocationAttributes(attributes, firstAttr)
	w.EndObject()
	return first
}

func (w *jsonWriter) writeValue(val pcommon.Value, stringifyMaps bool) {
	switch val.Type() {
	case pcommon.ValueTypeEmpty:
		w.NullVal()
	case pcommon.ValueTypeStr:
		w.JSONString(val.Str())
	case pcommon.ValueTypeBool:
		w.BoolVal(val.Bool())
	case pcommon.ValueTypeDouble:
		w.Float64Val(val.Double())
	case pcommon.ValueTypeInt:
		w.Int64Val(val.Int())
	case pcommon.ValueTypeBytes:
		w.Buf.WriteByte('"')
		b := hex.AppendEncode(w.Buf.AvailableBuffer(), val.Bytes().AsRaw())
		w.Buf.Write(b)
		w.Buf.WriteByte('"')
	case pcommon.ValueTypeMap:
		if stringifyMaps {
			w.JSONString(val.AsString())
		} else {
			w.writeMap(val.Map())
		}
	case pcommon.ValueTypeSlice:
		w.StartArray()
		firstElem := true
		for _, item := range val.Slice().All() {
			firstElem = w.ArrayComma(firstElem)
			w.writeValue(item, stringifyMaps)
		}
		w.EndArray()
	}
}

func (w *jsonWriter) writeMap(m pcommon.Map) {
	w.StartObject()
	first := true
	for k, val := range m.All() {
		first = w.Key(k, first)
		w.writeValue(val, false)
	}
	w.EndObject()
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
			first = w.Key(k, first)
			w.StartArray()
			w.Float64Val(geo.lon)
			w.Buf.WriteByte(',')
			w.Float64Val(geo.lat)
			w.EndArray()
			continue
		}
		if geo.lonSet {
			first = w.Key(prefix+lonKey, first)
			w.Float64Val(geo.lon)
		}
		if geo.latSet {
			first = w.Key(prefix+latKey, first)
			w.Float64Val(geo.lat)
		}
	}
	return first
}
