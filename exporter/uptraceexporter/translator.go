// Copyright OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package uptraceexporter

import (
	"encoding/binary"
	"encoding/json"

	"github.com/uptrace/uptrace-go/spanexp"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

func asUint64(b [8]byte) uint64 {
	return binary.LittleEndian.Uint64(b[:])
}

func spanKind(kind pdata.SpanKind) string {
	switch kind {
	case pdata.SpanKindClient:
		return "client"
	case pdata.SpanKindServer:
		return "server"
	case pdata.SpanKindProducer:
		return "producer"
	case pdata.SpanKindConsumer:
		return "consumer"
	}
	return "internal"
}

func statusCode(status pdata.StatusCode) string {
	switch status {
	case pdata.StatusCodeOk:
		return "ok"
	case pdata.StatusCodeError:
		return "error"
	}
	return "unset"
}

func (e *traceExporter) keyValueSlice(attrs pdata.AttributeMap) spanexp.KeyValueSlice {
	out := make(spanexp.KeyValueSlice, 0, attrs.Len())

	attrs.Range(func(key string, value pdata.AttributeValue) bool {
		switch value.Type() {
		case pdata.AttributeValueTypeString:
			out = append(out, attribute.String(key, value.StringVal()))
		case pdata.AttributeValueTypeBool:
			out = append(out, attribute.Bool(key, value.BoolVal()))
		case pdata.AttributeValueTypeInt:
			out = append(out, attribute.Int64(key, value.IntVal()))
		case pdata.AttributeValueTypeDouble:
			out = append(out, attribute.Float64(key, value.DoubleVal()))
		case pdata.AttributeValueTypeMap:
			if value, ok := mapLabelValue(value.MapVal()); ok {
				out = append(out, attribute.KeyValue{
					Key:   attribute.Key(key),
					Value: value,
				})
			}
		case pdata.AttributeValueTypeArray:
			if value, ok := arrayLabelValue(value.ArrayVal()); ok {
				out = append(out, attribute.KeyValue{
					Key:   attribute.Key(key),
					Value: value,
				})
			}
		case pdata.AttributeValueTypeNull:
			// Ignore. Uptrace does not support nulls.
		default:
			e.logger.Warn("uptraceexporter: unsupported attribute value type",
				zap.String("type", value.Type().String()))
		}
		return true
	})

	return out
}

func (e *traceExporter) uptraceEvents(events pdata.SpanEventSlice) []spanexp.Event {
	if events.Len() == 0 {
		return nil
	}

	outEvents := make([]spanexp.Event, events.Len())
	for i := 0; i < events.Len(); i++ {
		event := events.At(i)

		out := &outEvents[i]
		out.Name = event.Name()
		out.Attrs = e.keyValueSlice(event.Attributes())
		out.Time = int64(event.Timestamp())
	}
	return outEvents
}

func (e *traceExporter) uptraceLinks(links pdata.SpanLinkSlice) []spanexp.Link {
	if links.Len() == 0 {
		return nil
	}

	outLinks := make([]spanexp.Link, links.Len())
	for i := 0; i < links.Len(); i++ {
		link := links.At(i)

		out := &outLinks[i]
		out.TraceID = link.TraceID().Bytes()
		out.SpanID = asUint64(link.SpanID().Bytes())
		out.Attrs = e.keyValueSlice(link.Attributes())
	}
	return outLinks
}

//------------------------------------------------------------------------------

func mapLabelValue(m pdata.AttributeMap) (attribute.Value, bool) {
	out := make(map[string]interface{}, m.Len())
	m.Range(func(key string, val pdata.AttributeValue) bool {
		out[key] = attrAsInterface(val)
		return true
	})
	return jsonLabelValue(out)
}

func arrayLabelValue(arr pdata.AnyValueArray) (attribute.Value, bool) {
	if arr.Len() == 0 {
		return attribute.Value{}, false
	}

	switch arrType := arr.At(0).Type(); arrType {
	case pdata.AttributeValueTypeString:
		out := make([]string, 0, arr.Len())
		for i := 0; i < arr.Len(); i++ {
			val := arr.At(i)
			if val.Type() != arrType {
				return attribute.Value{}, false
			}
			out = append(out, val.StringVal())
		}
		return attribute.ArrayValue(out), true

	case pdata.AttributeValueTypeBool:
		out := make([]bool, 0, arr.Len())
		for i := 0; i < arr.Len(); i++ {
			val := arr.At(i)
			if val.Type() != arrType {
				return attribute.Value{}, false
			}
			out = append(out, val.BoolVal())
		}
		return attribute.ArrayValue(out), true

	case pdata.AttributeValueTypeInt:
		out := make([]int64, 0, arr.Len())
		for i := 0; i < arr.Len(); i++ {
			val := arr.At(i)
			if val.Type() != arrType {
				return attribute.Value{}, false
			}
			out = append(out, val.IntVal())
		}
		return attribute.ArrayValue(out), true

	case pdata.AttributeValueTypeDouble:
		out := make([]float64, 0, arr.Len())
		for i := 0; i < arr.Len(); i++ {
			val := arr.At(i)
			if val.Type() != arrType {
				return attribute.Value{}, false
			}
			out = append(out, val.DoubleVal())
		}
		return attribute.ArrayValue(out), true

	case pdata.AttributeValueTypeNull:
		// Ignore. Uptrace does not support nulls.
		return attribute.Value{}, false
	}

	out := make([]interface{}, arr.Len())
	for i := 0; i < arr.Len(); i++ {
		out[i] = attrAsInterface(arr.At(i))
	}
	return jsonLabelValue(out)
}

func jsonLabelValue(v interface{}) (attribute.Value, bool) {
	if b, err := json.Marshal(v); err == nil {
		return attribute.StringValue(string(b)), true
	}
	return attribute.Value{}, false
}

func attrAsInterface(val pdata.AttributeValue) interface{} {
	switch val.Type() {
	case pdata.AttributeValueTypeString:
		return val.StringVal()
	case pdata.AttributeValueTypeInt:
		return val.IntVal()
	case pdata.AttributeValueTypeDouble:
		return val.DoubleVal()
	case pdata.AttributeValueTypeBool:
		return val.BoolVal()
	case pdata.AttributeValueTypeMap:
		out := map[string]interface{}{}
		val.MapVal().Range(func(key string, val pdata.AttributeValue) bool {
			out[key] = attrAsInterface(val)
			return true
		})
		return out
	case pdata.AttributeValueTypeArray:
		arr := val.ArrayVal()
		out := make([]interface{}, arr.Len())
		for i := 0; i < arr.Len(); i++ {
			out[i] = attrAsInterface(arr.At(i))
		}
		return out
	}
	return nil
}
