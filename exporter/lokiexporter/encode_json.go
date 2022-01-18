// Copyright The OpenTelemetry Authors
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

package lokiexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter"

import (
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/collector/model/pdata"
)

// JSON representation of the LogRecord as described by https://developers.google.com/protocol-buffers/docs/proto3#json

type lokiEntry struct {
	Name       string                 `json:"name,omitempty"`
	Body       json.RawMessage        `json:"body,omitempty"`
	TraceID    string                 `json:"traceid,omitempty"`
	SpanID     string                 `json:"spanid,omitempty"`
	Severity   string                 `json:"severity,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
	Resources  map[string]interface{} `json:"resources,omitempty"`
}

func serializeBody(body pdata.AttributeValue) ([]byte, error) {
	var str []byte
	var err error
	switch body.Type() {
	case pdata.AttributeValueTypeEmpty:
		// no body

	case pdata.AttributeValueTypeString:
		str, err = json.Marshal(body.StringVal())

	case pdata.AttributeValueTypeInt:
		str, err = json.Marshal(body.IntVal())

	case pdata.AttributeValueTypeDouble:
		str, err = json.Marshal(body.DoubleVal())

	case pdata.AttributeValueTypeBool:
		str, err = json.Marshal(body.BoolVal())

	case pdata.AttributeValueTypeMap:
		str, err = json.Marshal(body.MapVal().AsRaw())

	case pdata.AttributeValueTypeArray:
		str, err = json.Marshal(attributeValueSliceAsRaw(body.SliceVal()))

	case pdata.AttributeValueTypeBytes:
		str, err = json.Marshal(body.BytesVal())

	default:
		err = fmt.Errorf("unsuported body type to serialize")
	}
	return str, err
}

func encodeJSON(lr pdata.LogRecord, res pdata.Resource) (string, error) {
	var logRecord lokiEntry
	var jsonRecord []byte
	var err error
	var body []byte

	body, err = serializeBody(lr.Body())
	if err != nil {
		return "", err
	}
	logRecord = lokiEntry{
		Name:       lr.Name(),
		Body:       body,
		TraceID:    lr.TraceID().HexString(),
		SpanID:     lr.SpanID().HexString(),
		Severity:   lr.SeverityText(),
		Attributes: lr.Attributes().AsRaw(),
		Resources:  res.Attributes().AsRaw(),
	}
	lr.Body().Type()

	jsonRecord, err = json.Marshal(logRecord)
	if err != nil {
		return "", err
	}
	return string(jsonRecord), nil
}

// Copied from pdata (es AttributeValueSlice) asRaw() since its not exported
func attributeValueSliceAsRaw(es pdata.AttributeValueSlice) []interface{} {
	rawSlice := make([]interface{}, 0, es.Len())
	for i := 0; i < es.Len(); i++ {
		v := es.At(i)
		switch v.Type() {
		case pdata.AttributeValueTypeString:
			rawSlice = append(rawSlice, v.StringVal())
		case pdata.AttributeValueTypeInt:
			rawSlice = append(rawSlice, v.IntVal())
		case pdata.AttributeValueTypeDouble:
			rawSlice = append(rawSlice, v.DoubleVal())
		case pdata.AttributeValueTypeBool:
			rawSlice = append(rawSlice, v.BoolVal())
		case pdata.AttributeValueTypeBytes:
			rawSlice = append(rawSlice, v.BytesVal())
		case pdata.AttributeValueTypeEmpty:
			rawSlice = append(rawSlice, nil)
		default:
			rawSlice = append(rawSlice, "<Invalid array value>")
		}
	}
	return rawSlice
}
