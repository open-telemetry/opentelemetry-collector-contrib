// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loki // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/loki"

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-logfmt/logfmt"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

// JSON representation of the LogRecord as described by https://developers.google.com/protocol-buffers/docs/proto3#json
type lokiEntry struct {
	Name                 string                 `json:"name,omitempty"`
	Body                 json.RawMessage        `json:"body,omitempty"`
	TraceID              string                 `json:"traceid,omitempty"`
	SpanID               string                 `json:"spanid,omitempty"`
	Severity             string                 `json:"severity,omitempty"`
	Flags                uint32                 `json:"flags,omitempty"`
	Attributes           map[string]interface{} `json:"attributes,omitempty"`
	Resources            map[string]interface{} `json:"resources,omitempty"`
	InstrumentationScope *instrumentationScope  `json:"instrumentation_scope,omitempty"`
}

type instrumentationScope struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

// Encode converts an OTLP log record and its resource attributes into a JSON
// string representing a Loki entry. An error is returned when the record can't
// be marshaled into JSON.
func Encode(lr plog.LogRecord, res pcommon.Resource, scope pcommon.InstrumentationScope) (string, error) {
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
		TraceID:    traceutil.TraceIDToHexOrEmptyString(lr.TraceID()),
		SpanID:     traceutil.SpanIDToHexOrEmptyString(lr.SpanID()),
		Severity:   lr.SeverityText(),
		Attributes: lr.Attributes().AsRaw(),
		Resources:  res.Attributes().AsRaw(),
		Flags:      uint32(lr.Flags()),
	}

	scopeName := scope.Name()
	scopeVersion := scope.Version()
	if scopeName != "" {
		logRecord.InstrumentationScope = &instrumentationScope{
			Name: scopeName,
		}
		if scopeVersion != "" {
			logRecord.InstrumentationScope.Version = scopeVersion
		}
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
func EncodeLogfmt(lr plog.LogRecord, res pcommon.Resource, scope pcommon.InstrumentationScope) (string, error) {
	keyvals := bodyToKeyvals(lr.Body())

	if traceID := lr.TraceID(); !traceID.IsEmpty() {
		keyvals = keyvalsReplaceOrAppend(keyvals, "traceID", hex.EncodeToString(traceID[:]))
	}

	if spanID := lr.SpanID(); !spanID.IsEmpty() {
		keyvals = keyvalsReplaceOrAppend(keyvals, "spanID", hex.EncodeToString(spanID[:]))
	}

	severity := lr.SeverityText()
	if severity != "" {
		keyvals = keyvalsReplaceOrAppend(keyvals, "severity", severity)
	}

	flags := lr.Flags()
	if flags != 0 {
		keyvals = keyvalsReplaceOrAppend(keyvals, "flags", lr.Flags())
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

	scopeName := scope.Name()
	scopeVersion := scope.Version()
	if scopeName != "" {
		keyvals = append(keyvals, "instrumentation_scope_name", scopeName)
		if scopeVersion != "" {
			keyvals = append(keyvals, "instrumentation_scope_version", scopeVersion)
		}
	}

	logfmtLine, err := logfmt.MarshalKeyvals(keyvals...)
	if err != nil {
		return "", err
	}
	return string(logfmtLine), nil
}

func serializeBodyJSON(body pcommon.Value) ([]byte, error) {
	if body.Type() == pcommon.ValueTypeEmpty {
		// no body
		return nil, nil
	}

	return json.Marshal(body.AsRaw())
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
