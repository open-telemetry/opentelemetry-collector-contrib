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
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/otel/label"
	"go.uber.org/zap"
)

func asUint64(b [8]byte) uint64 {
	return binary.LittleEndian.Uint64(b[:])
}

func spanKind(kind pdata.SpanKind) string {
	switch kind {
	case pdata.SpanKindCLIENT:
		return "client"
	case pdata.SpanKindSERVER:
		return "server"
	case pdata.SpanKindPRODUCER:
		return "producer"
	case pdata.SpanKindCONSUMER:
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

	attrs.ForEach(func(key string, value pdata.AttributeValue) {
		switch value.Type() {
		case pdata.AttributeValueSTRING:
			out = append(out, label.String(key, value.StringVal()))
		case pdata.AttributeValueBOOL:
			out = append(out, label.Bool(key, value.BoolVal()))
		case pdata.AttributeValueINT:
			out = append(out, label.Int64(key, value.IntVal()))
		case pdata.AttributeValueDOUBLE:
			out = append(out, label.Float64(key, value.DoubleVal()))
		case pdata.AttributeValueMAP:
			if value, ok := mapLabelValue(value.MapVal()); ok {
				out = append(out, label.KeyValue{
					Key:   label.Key(key),
					Value: value,
				})
			}
		case pdata.AttributeValueARRAY:
			if value, ok := arrayLabelValue(value.ArrayVal()); ok {
				out = append(out, label.KeyValue{
					Key:   label.Key(key),
					Value: value,
				})
			}
		case pdata.AttributeValueNULL:
			// Ignore. Uptrace does not support nulls.
		default:
			e.logger.Warn("uptraceexporter: unsupported attribute value type",
				zap.String("type", value.Type().String()))
		}
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

func mapLabelValue(m pdata.AttributeMap) (label.Value, bool) {
	out := make(map[string]interface{}, m.Len())
	m.ForEach(func(key string, val pdata.AttributeValue) {
		out[key] = attrAsInterface(val)
	})
	return jsonLabelValue(out)
}

func arrayLabelValue(arr pdata.AnyValueArray) (label.Value, bool) {
	if arr.Len() == 0 {
		return label.Value{}, false
	}

	switch arrType := arr.At(0).Type(); arrType {
	case pdata.AttributeValueSTRING:
		out := make([]string, 0, arr.Len())
		for i := 0; i < arr.Len(); i++ {
			val := arr.At(i)
			if val.Type() != arrType {
				return label.Value{}, false
			}
			out = append(out, val.StringVal())
		}
		return label.ArrayValue(out), true

	case pdata.AttributeValueBOOL:
		out := make([]bool, 0, arr.Len())
		for i := 0; i < arr.Len(); i++ {
			val := arr.At(i)
			if val.Type() != arrType {
				return label.Value{}, false
			}
			out = append(out, val.BoolVal())
		}
		return label.ArrayValue(out), true

	case pdata.AttributeValueINT:
		out := make([]int64, 0, arr.Len())
		for i := 0; i < arr.Len(); i++ {
			val := arr.At(i)
			if val.Type() != arrType {
				return label.Value{}, false
			}
			out = append(out, val.IntVal())
		}
		return label.ArrayValue(out), true

	case pdata.AttributeValueDOUBLE:
		out := make([]float64, 0, arr.Len())
		for i := 0; i < arr.Len(); i++ {
			val := arr.At(i)
			if val.Type() != arrType {
				return label.Value{}, false
			}
			out = append(out, val.DoubleVal())
		}
		return label.ArrayValue(out), true

	case pdata.AttributeValueNULL:
		// Ignore. Uptrace does not support nulls.
		return label.Value{}, false
	}

	out := make([]interface{}, arr.Len())
	for i := 0; i < arr.Len(); i++ {
		out[i] = attrAsInterface(arr.At(i))
	}
	return jsonLabelValue(out)
}

func jsonLabelValue(v interface{}) (label.Value, bool) {
	if b, err := json.Marshal(v); err == nil {
		return label.StringValue(string(b)), true
	}
	return label.Value{}, false
}

func attrAsInterface(val pdata.AttributeValue) interface{} {
	switch val.Type() {
	case pdata.AttributeValueSTRING:
		return val.StringVal()
	case pdata.AttributeValueINT:
		return val.IntVal()
	case pdata.AttributeValueDOUBLE:
		return val.DoubleVal()
	case pdata.AttributeValueBOOL:
		return val.BoolVal()
	case pdata.AttributeValueMAP:
		out := map[string]interface{}{}
		val.MapVal().ForEach(func(key string, val pdata.AttributeValue) {
			out[key] = attrAsInterface(val)
		})
		return out
	case pdata.AttributeValueARRAY:
		arr := val.ArrayVal()
		out := make([]interface{}, arr.Len())
		for i := 0; i < arr.Len(); i++ {
			out[i] = attrAsInterface(arr.At(i))
		}
		return out
	}
	return nil
}
