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

package loki // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/loki"

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-logfmt/logfmt"
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

// Encode converts an OTLP log record and its resource attributes into a JSON
// string representing a Loki entry. An error is returned when the record can't
// be marshaled into JSON.
func Encode(lr plog.LogRecord, res pcommon.Resource) (string, error) {
	var logRecord lokiEntry
	var jsonRecord []byte
	var err error
	var body []byte

	body, err = serializeBodyJSON(lr.Body())
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

	jsonRecord, err = json.Marshal(logRecord)
	if err != nil {
		return "", err
	}
	return string(jsonRecord), nil
}

// EncodeLogfmt converts an OTLP log record and its resource attributes into a logfmt
// string representing a Loki entry. An error is returned when the record can't
// be marshaled into logfmt.
func EncodeLogfmt(lr plog.LogRecord, res pcommon.Resource) (string, error) {
	keyvals := bodyToKeyvals(lr.Body())

	traceID := lr.TraceID().HexString()
	if traceID != "" {
		keyvals = keyvalsReplaceOrAppend(keyvals, "traceID", traceID)
	}

	spanID := lr.SpanID().HexString()
	if spanID != "" {
		keyvals = keyvalsReplaceOrAppend(keyvals, "spanID", spanID)
	}

	severity := lr.SeverityText()
	if severity != "" {
		keyvals = keyvalsReplaceOrAppend(keyvals, "severity", severity)
	}

	lr.Attributes().Range(func(k string, v pcommon.Value) bool {
		keyvals = append(keyvals, valueToKeyvals(fmt.Sprintf("attribute_%s", k), v)...)
		return true
	})

	res.Attributes().Range(func(k string, v pcommon.Value) bool {
		// todo handle maps, slices
		keyvals = append(keyvals, valueToKeyvals(fmt.Sprintf("resource_%s", k), v)...)
		return true
	})

	logfmtLine, err := logfmt.MarshalKeyvals(keyvals...)
	if err != nil {
		return "", err
	}
	return string(logfmtLine), nil
}

func serializeBodyJSON(body pcommon.Value) ([]byte, error) {
	var str []byte
	var err error
	switch body.Type() {
	case pcommon.ValueTypeEmpty:
		// no body

	case pcommon.ValueTypeStr:
		str, err = json.Marshal(body.Str())

	case pcommon.ValueTypeInt:
		str, err = json.Marshal(body.Int())

	case pcommon.ValueTypeDouble:
		str, err = json.Marshal(body.Double())

	case pcommon.ValueTypeBool:
		str, err = json.Marshal(body.Bool())

	case pcommon.ValueTypeMap:
		str, err = json.Marshal(body.Map().AsRaw())

	case pcommon.ValueTypeSlice:
		str, err = json.Marshal(body.Slice().AsRaw())

	case pcommon.ValueTypeBytes:
		str, err = json.Marshal(body.Bytes().AsRaw())

	default:
		err = fmt.Errorf("unsuported body type to serialize")
	}
	return str, err
}

func bodyToKeyvals(body pcommon.Value) []interface{} {
	switch body.Type() {
	case pcommon.ValueTypeEmpty:
		return nil
	case pcommon.ValueTypeStr:
		// try to parse record body as logfmt, but failing that assume it's plain text
		value := body.Str()
		keyvals, err := parseLogfmtLine(value)
		if err != nil {
			return []interface{}{"msg", body.Str()}
		}
		return *keyvals
	case pcommon.ValueTypeMap:
		return valueToKeyvals("", body)
	case pcommon.ValueTypeSlice:
		return valueToKeyvals("body", body)
	default:
		return []interface{}{"msg", body.AsRaw()}
	}
}

func valueToKeyvals(key string, value pcommon.Value) []interface{} {
	switch value.Type() {
	case pcommon.ValueTypeEmpty:
		return nil
	case pcommon.ValueTypeStr:
		return []interface{}{key, value.Str()}
	case pcommon.ValueTypeBool:
		return []interface{}{key, value.Bool()}
	case pcommon.ValueTypeInt:
		return []interface{}{key, value.Int()}
	case pcommon.ValueTypeDouble:
		return []interface{}{key, value.Double()}
	case pcommon.ValueTypeMap:
		var keyvals []interface{}
		prefix := ""
		if key != "" {
			prefix = key + "_"
		}
		value.Map().Range(func(k string, v pcommon.Value) bool {

			keyvals = append(keyvals, valueToKeyvals(prefix+k, v)...)
			return true
		})
		return keyvals
	case pcommon.ValueTypeSlice:
		prefix := ""
		if key != "" {
			prefix = key + "_"
		}
		var keyvals []interface{}
		for i := 0; i < value.Slice().Len(); i++ {
			v := value.Slice().At(i)
			keyvals = append(keyvals, valueToKeyvals(fmt.Sprintf("%s%d", prefix, i), v)...)
		}
		return keyvals
	default:
		return []interface{}{key, value.AsRaw()}
	}
}

// if given key:value pair already exists in keyvals, replace value. Otherwise append
func keyvalsReplaceOrAppend(keyvals []interface{}, key string, value interface{}) []interface{} {
	for i := 0; i < len(keyvals); i += 2 {
		if keyvals[i] == key {
			keyvals[i+1] = value
			return keyvals
		}
	}
	return append(keyvals, key, value)
}

func parseLogfmtLine(line string) (*[]interface{}, error) {
	var keyvals []interface{}
	decoder := logfmt.NewDecoder(strings.NewReader(line))
	decoder.ScanRecord()
	for decoder.ScanKeyval() {
		keyvals = append(keyvals, decoder.Key(), decoder.Value())
	}

	err := decoder.Err()
	if err != nil {
		return nil, err
	}
	return &keyvals, nil
}
