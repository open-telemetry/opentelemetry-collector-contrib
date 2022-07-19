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

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
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

func serializeBody(body pcommon.Value) ([]byte, error) {
	var str []byte
	var err error
	switch body.Type() {
	case pcommon.ValueTypeEmpty:
		// no body

	case pcommon.ValueTypeString:
		str, err = json.Marshal(body.StringVal())

	case pcommon.ValueTypeInt:
		str, err = json.Marshal(body.IntVal())

	case pcommon.ValueTypeDouble:
		str, err = json.Marshal(body.DoubleVal())

	case pcommon.ValueTypeBool:
		str, err = json.Marshal(body.BoolVal())

	case pcommon.ValueTypeMap:
		str, err = json.Marshal(body.MapVal().AsRaw())

	case pcommon.ValueTypeSlice:
		str, err = json.Marshal(body.SliceVal().AsRaw())

	case pcommon.ValueTypeBytes:
		str, err = json.Marshal(body.MBytesVal())

	default:
		err = fmt.Errorf("unsuported body type to serialize")
	}
	return str, err
}

func encodeJSON(lr plog.LogRecord, res pcommon.Resource) (string, error) {
	var logRecord lokiEntry
	var jsonRecord []byte
	var err error
	var body []byte

	body, err = serializeBody(lr.Body())
	if err != nil {
		return "", err
	}
	logRecord = lokiEntry{
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
