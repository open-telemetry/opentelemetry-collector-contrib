// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer"

import (
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/elastic/go-structform"
	"github.com/elastic/go-structform/json"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer"
)

const tsLayout = "2006-01-02T15:04:05.000000000Z"

func writeDataStream(v *json.Visitor, idx elasticsearch.Index) {
	if !idx.IsDataStream() {
		return
	}
	_ = v.OnKey("data_stream")
	_ = v.OnObjectStart(-1, structform.AnyType)
	writeStringFieldSkipDefault(v, "type", idx.Type)
	writeStringFieldSkipDefault(v, "dataset", idx.Dataset)
	writeStringFieldSkipDefault(v, "namespace", idx.Namespace)
	_ = v.OnObjectFinished()
}

func writeResource(v *json.Visitor, resource pcommon.Resource, resourceSchemaURL string, stringifyMapAttributes bool) {
	_ = v.OnKey("resource")
	_ = v.OnObjectStart(-1, structform.AnyType)
	writeStringFieldSkipDefault(v, "schema_url", resourceSchemaURL)
	writeAttributes(v, resource.Attributes(), stringifyMapAttributes)
	writeIntFieldSkipDefault(v, "dropped_attributes_count", int64(resource.DroppedAttributesCount()))
	_ = v.OnObjectFinished()
}

func writeScope(v *json.Visitor, scope pcommon.InstrumentationScope, scopeSchemaURL string, stringifyMapAttributes bool) {
	_ = v.OnKey("scope")
	_ = v.OnObjectStart(-1, structform.AnyType)
	writeStringFieldSkipDefault(v, "schema_url", scopeSchemaURL)
	writeStringFieldSkipDefault(v, "name", scope.Name())
	writeStringFieldSkipDefault(v, "version", scope.Version())
	writeAttributes(v, scope.Attributes(), stringifyMapAttributes)
	writeIntFieldSkipDefault(v, "dropped_attributes_count", int64(scope.DroppedAttributesCount()))
	_ = v.OnObjectFinished()
}

func writeAttributes(v *json.Visitor, attributes pcommon.Map, stringifyMapValues bool) {
	if attributes.Len() == 0 {
		return
	}

	_ = v.OnKey("attributes")
	_ = v.OnObjectStart(-1, structform.AnyType)
	for k, val := range attributes.All() {
		switch k {
		case elasticsearch.DataStreamType, elasticsearch.DataStreamDataset, elasticsearch.DataStreamNamespace, elasticsearch.MappingHintsAttrKey, elasticsearch.DocumentIDAttributeName, elasticsearch.DocumentPipelineAttributeName, elasticsearch.IndexAttributeName:
			continue
		}
		if isGeoAttribute(k, val) {
			continue
		}
		_ = v.OnKey(k)
		serializer.WriteValue(v, val, stringifyMapValues)
	}
	writeGeolocationAttributes(v, attributes)
	_ = v.OnObjectFinished()
}

func isGeoAttribute(k string, val pcommon.Value) bool {
	if val.Type() != pcommon.ValueTypeDouble {
		return false
	}
	switch k {
	case "geo.location.lat", "geo.location.lon":
		return true
	}
	return strings.HasSuffix(k, ".geo.location.lat") || strings.HasSuffix(k, ".geo.location.lon")
}

func writeGeolocationAttributes(v *json.Visitor, attributes pcommon.Map) {
	const (
		lonKey    = "geo.location.lon"
		latKey    = "geo.location.lat"
		mergedKey = "geo.location"
	)
	// Prefix is the attribute name without lonKey or latKey suffix
	// e.g. prefix of "foo.bar.geo.location.lon" is "foo.bar.", prefix of "geo.location.lon" is "".
	prefixToGeo := make(map[string]struct {
		lon, lat       float64
		lonSet, latSet bool
	})
	setLon := func(prefix string, v float64) {
		g := prefixToGeo[prefix]
		g.lon = v
		g.lonSet = true
		prefixToGeo[prefix] = g
	}
	setLat := func(prefix string, v float64) {
		g := prefixToGeo[prefix]
		g.lat = v
		g.latSet = true
		prefixToGeo[prefix] = g
	}
	for key, val := range attributes.All() {
		if val.Type() != pcommon.ValueTypeDouble {
			continue
		}

		if key == lonKey {
			setLon("", val.Double())
		} else if key == latKey {
			setLat("", val.Double())
		} else if namespace, found := strings.CutSuffix(key, "."+lonKey); found {
			prefix := namespace + "."
			setLon(prefix, val.Double())
		} else if namespace, found := strings.CutSuffix(key, "."+latKey); found {
			prefix := namespace + "."
			setLat(prefix, val.Double())
		}
	}

	for prefix, geo := range prefixToGeo {
		if geo.lonSet && geo.latSet {
			key := prefix + mergedKey
			// Geopoint expressed as an array with the format: [lon, lat]
			_ = v.OnKey(key)
			_ = v.OnArrayStart(-1, structform.AnyType)
			_ = v.OnFloat64(geo.lon)
			_ = v.OnFloat64(geo.lat)
			_ = v.OnArrayFinished()
			continue
		}
		// Place the attributes back if lon and lat are not present together
		if geo.lonSet {
			_ = v.OnKey(prefix + lonKey)
			_ = v.OnFloat64(geo.lon)
		}
		if geo.latSet {
			_ = v.OnKey(prefix + latKey)
			_ = v.OnFloat64(geo.lat)
		}
	}
}

func writeTimestampField(v *json.Visitor, key string, timestamp pcommon.Timestamp) {
	_ = v.OnKey(key)
	nsec := uint64(timestamp)
	msec := nsec / 1e6
	nsec -= msec * 1e6
	_ = v.OnString(strconv.FormatUint(msec, 10) + "." + strconv.FormatUint(nsec, 10))
}

func writeUIntField(v *json.Visitor, key string, i uint64) {
	_ = v.OnKey(key)
	_ = v.OnUint64(i)
}

func writeIntFieldSkipDefault(v *json.Visitor, key string, i int64) {
	if i == 0 {
		return
	}
	_ = v.OnKey(key)
	_ = v.OnInt64(i)
}

func writeStringFieldSkipDefault(v *json.Visitor, key, value string) {
	if value == "" {
		return
	}
	_ = v.OnKey(key)
	_ = v.OnString(value)
}

func writeTraceIDField(v *json.Visitor, id pcommon.TraceID) {
	if id.IsEmpty() {
		return
	}
	_ = v.OnKey("trace_id")
	_ = v.OnString(hex.EncodeToString(id[:]))
}

func writeSpanIDField(v *json.Visitor, key string, id pcommon.SpanID) {
	if id.IsEmpty() {
		return
	}
	_ = v.OnKey(key)
	_ = v.OnString(hex.EncodeToString(id[:]))
}
