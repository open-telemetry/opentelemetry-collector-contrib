package elasticsearchexporter

import (
	"bytes"
	"encoding/hex"
	"github.com/elastic/go-structform"
	"github.com/elastic/go-structform/json"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"strings"
)

const tsLayout = "2006-01-02T15:04:05.000000000Z"

func serializeLog(resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, record plog.LogRecord) ([]byte, error) {
	var buf bytes.Buffer

	v := json.NewVisitor(&buf)
	// Enable ExplicitRadixPoint such that 1.0 is encoded as 1.0 instead of 1.
	// This is required to generate the correct dynamic mapping in ES.
	v.SetExplicitRadixPoint(true)
	if err := v.OnObjectStart(-1, structform.AnyType); err != nil {
		return nil, err
	}
	docTimeStamp := record.Timestamp()
	if docTimeStamp.AsTime().UnixNano() == 0 {
		docTimeStamp = record.ObservedTimestamp()
	}
	if err := writeTimestampField(v, "@timestamp", docTimeStamp); err != nil {
		return nil, err
	}
	if err := writeTimestampField(v, "observed_timestamp", record.ObservedTimestamp()); err != nil {
		return nil, err
	}
	if err := writeDataStream(v, record.Attributes()); err != nil {
		return nil, err
	}
	if err := writeStringFieldSkipDefault(v, "severity_text", record.SeverityText()); err != nil {
		return nil, err
	}
	if err := writeIntFieldSkipDefault(v, "severity_number", int64(record.SeverityNumber())); err != nil {
		return nil, err
	}
	if err := writeTraceIdField(v, record.TraceID()); err != nil {
		return nil, err
	}
	if err := writeSpanIdField(v, record.SpanID()); err != nil {
		return nil, err
	}
	if err := writeAttributes(v, record.Attributes(), false); err != nil {
		return nil, err
	}
	if err := writeIntFieldSkipDefault(v, "dropped_attributes_count", int64(record.DroppedAttributesCount())); err != nil {
		return nil, err
	}
	if err := writeResource(v, resource, resourceSchemaURL); err != nil {
		return nil, err
	}
	if err := writeScope(v, scope, scopeSchemaURL); err != nil {
		return nil, err
	}
	if err := writeLogBody(v, record); err != nil {
		return nil, err
	}
	if err := v.OnObjectFinished(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeDataStream(v *json.Visitor, attributes pcommon.Map) error {
	if err := v.OnKey("data_stream"); err != nil {
		return err
	}
	if err := v.OnObjectStart(-1, structform.AnyType); err != nil {
		return err
	}
	var err error
	attributes.Range(func(k string, val pcommon.Value) bool {
		if strings.HasPrefix(k, "data_stream.") && val.Type() == pcommon.ValueTypeStr {
			if err = writeStringFieldSkipDefault(v, k[12:], val.Str()); err != nil {
				return false
			}
		}
		return true
	})
	if err != nil {
		return err
	}

	if err := v.OnObjectFinished(); err != nil {
		return err
	}
	return nil
}

func writeLogBody(v *json.Visitor, record plog.LogRecord) error {
	if record.Body().Type() == pcommon.ValueTypeEmpty {
		return nil
	}
	if err := v.OnKey("body"); err != nil {
		return err
	}
	if err := v.OnObjectStart(-1, structform.AnyType); err != nil {
		return err
	}

	// Determine if this log record is an event, as they are mapped differently
	// https://github.com/open-telemetry/semantic-conventions/blob/main/docs/general/events.md
	var bodyType string
	if _, hasEventNameAttribute := record.Attributes().Get("event.name"); hasEventNameAttribute || record.EventName() != "" {
		bodyType = "structured"
	} else {
		bodyType = "flattened"
	}
	body := record.Body()
	switch body.Type() {
	case pcommon.ValueTypeMap:
	case pcommon.ValueTypeSlice:
		// output must be an array of objects due to ES limitations
		// otherwise, wrap the array in an object
		s := body.Slice()
		allMaps := true
		for i := 0; i < s.Len(); i++ {
			if s.At(i).Type() != pcommon.ValueTypeMap {
				allMaps = false
			}
		}

		if !allMaps {
			body = pcommon.NewValueMap()
			m := body.SetEmptyMap()
			record.Body().Slice().CopyTo(m.PutEmptySlice("value"))
		}
	default:
		bodyType = "text"
	}
	if err := v.OnKey(bodyType); err != nil {
		return err
	}
	if err := writeValue(v, body, false); err != nil {
		return err
	}
	if err := v.OnObjectFinished(); err != nil {
		return err
	}
	return nil
}

func writeResource(v *json.Visitor, resource pcommon.Resource, resourceSchemaURL string) error {
	if err := v.OnKey("resource"); err != nil {
		return err
	}
	if err := v.OnObjectStart(-1, structform.AnyType); err != nil {
		return err
	}
	if err := writeStringFieldSkipDefault(v, "schema_url", resourceSchemaURL); err != nil {
		return err
	}
	if err := writeAttributes(v, resource.Attributes(), true); err != nil {
		return err
	}
	if err := writeIntFieldSkipDefault(v, "dropped_attributes_count", int64(resource.DroppedAttributesCount())); err != nil {
		return err
	}
	if err := v.OnObjectFinished(); err != nil {
		return err
	}
	return nil
}

func writeScope(v *json.Visitor, scope pcommon.InstrumentationScope, scopeSchemaURL string) error {
	if err := v.OnKey("scope"); err != nil {
		return err
	}
	if err := v.OnObjectStart(-1, structform.AnyType); err != nil {
		return err
	}
	if err := writeStringFieldSkipDefault(v, "schema_url", scopeSchemaURL); err != nil {
		return err
	}
	if err := writeStringFieldSkipDefault(v, "name", scope.Name()); err != nil {
		return err
	}
	if err := writeStringFieldSkipDefault(v, "version", scope.Version()); err != nil {
		return err
	}
	if err := writeAttributes(v, scope.Attributes(), true); err != nil {
		return err
	}
	if err := writeIntFieldSkipDefault(v, "dropped_attributes_count", int64(scope.DroppedAttributesCount())); err != nil {
		return err
	}
	if err := v.OnObjectFinished(); err != nil {
		return err
	}
	return nil
}

func writeAttributes(v *json.Visitor, attributes pcommon.Map, stringifyMapValues bool) error {
	if attributes.Len() == 0 {
		return nil
	}
	if err := v.OnKey("attributes"); err != nil {
		return err
	}
	attrCopy := pcommon.NewMap()
	attributes.CopyTo(attrCopy)
	attrCopy.RemoveIf(func(key string, _ pcommon.Value) bool {
		switch key {
		case dataStreamType, dataStreamDataset, dataStreamNamespace:
			return true
		}
		return false
	})
	mergeGeolocation(attrCopy)
	if err := writeMap(v, attrCopy, stringifyMapValues); err != nil {
		return err
	}
	return nil
}

func writeMap(v *json.Visitor, attributes pcommon.Map, stringifyMapValues bool) error {
	if err := v.OnObjectStart(-1, structform.AnyType); err != nil {
		return err
	}
	var err error
	attributes.Range(func(k string, val pcommon.Value) bool {
		if err = v.OnKey(k); err != nil {
			return false
		}
		err = writeValue(v, val, stringifyMapValues)
		return err == nil
	})
	if err != nil {
		return err
	}
	if err := v.OnObjectFinished(); err != nil {
		return err
	}
	return nil
}

func writeValue(v *json.Visitor, val pcommon.Value, stringifyMaps bool) error {
	switch val.Type() {
	case pcommon.ValueTypeEmpty:
		if err := v.OnNil(); err != nil {
			return err
		}
	case pcommon.ValueTypeStr:
		if err := v.OnString(val.Str()); err != nil {
			return err
		}
	case pcommon.ValueTypeBool:
		if err := v.OnBool(val.Bool()); err != nil {
			return err
		}
	case pcommon.ValueTypeDouble:
		if err := v.OnFloat64(val.Double()); err != nil {
			return err
		}
	case pcommon.ValueTypeInt:
		if err := v.OnInt64(val.Int()); err != nil {
			return err
		}
	case pcommon.ValueTypeBytes:
		if err := v.OnString(hex.EncodeToString(val.Bytes().AsRaw())); err != nil {
			return err
		}
	case pcommon.ValueTypeMap:
		if stringifyMaps {
			if err := v.OnString(val.AsString()); err != nil {
				return err
			}
		} else {
			if err := writeMap(v, val.Map(), false); err != nil {
				return err
			}
		}
	case pcommon.ValueTypeSlice:
		if err := v.OnArrayStart(-1, structform.AnyType); err != nil {
			return err
		}
		slice := val.Slice()
		for i := 0; i < slice.Len(); i++ {
			if err := writeValue(v, slice.At(i), stringifyMaps); err != nil {
				return err
			}
		}
		if err := v.OnArrayFinished(); err != nil {
			return err
		}
	}
	return nil
}

func writeTimestampField(v *json.Visitor, key string, timestamp pcommon.Timestamp) error {
	if err := v.OnKey(key); err != nil {
		return err
	}
	if err := v.OnString(timestamp.AsTime().UTC().Format(tsLayout)); err != nil {
		return err
	}
	return nil
}

func writeIntFieldSkipDefault(v *json.Visitor, key string, i int64) error {
	if i == 0 {
		return nil
	}
	if err := v.OnKey(key); err != nil {
		return err
	}
	if err := v.OnInt64(i); err != nil {
		return err
	}
	return nil
}

func writeStringFieldSkipDefault(v *json.Visitor, key, value string) error {
	if value == "" {
		return nil
	}
	if err := v.OnKey(key); err != nil {
		return err
	}
	if err := v.OnString(value); err != nil {
		return err
	}
	return nil
}

func writeTraceIdField(v *json.Visitor, id pcommon.TraceID) error {
	if id.IsEmpty() {
		return nil
	}
	if err := v.OnKey("trace_id"); err != nil {
		return err
	}
	if err := v.OnString(hex.EncodeToString(id[:])); err != nil {
		return err
	}
	return nil
}

func writeSpanIdField(v *json.Visitor, id pcommon.SpanID) error {
	if id.IsEmpty() {
		return nil
	}
	if err := v.OnKey("span_id"); err != nil {
		return err
	}
	if err := v.OnString(hex.EncodeToString(id[:])); err != nil {
		return err
	}
	return nil
}
