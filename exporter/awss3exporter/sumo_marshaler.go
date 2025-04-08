// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter"

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const (
	logBodyKey = "log"
)

type sumoMarshaler struct{}

func (*sumoMarshaler) format() string {
	return string(SumoIC)
}

func newSumoICMarshaler() sumoMarshaler {
	return sumoMarshaler{}
}

func logEntry(buf *bytes.Buffer, format string, a ...any) {
	fmt.Fprintf(buf, format, a...)
	buf.WriteString("\n")
}

func attributeValueToString(v pcommon.Value) (string, error) {
	switch v.Type() {
	case pcommon.ValueTypeStr:
		return v.Str(), nil
	case pcommon.ValueTypeBool:
		return strconv.FormatBool(v.Bool()), nil
	case pcommon.ValueTypeBytes:
		return valueToJSON(v.Bytes().AsRaw())
	case pcommon.ValueTypeDouble:
		return strconv.FormatFloat(v.Double(), 'f', -1, 64), nil
	case pcommon.ValueTypeInt:
		return strconv.FormatInt(v.Int(), 10), nil
	case pcommon.ValueTypeSlice:
		return valueToJSON(v.Slice().AsRaw())
	case pcommon.ValueTypeMap:
		return valueToJSON(v.Map().AsRaw())
	case pcommon.ValueTypeEmpty:
		return "", nil
	default:
		return "", fmt.Errorf("unknown OpenTelemetry attribute value type: %q", v.Type())
	}
}

func valueToJSON(m any) (string, error) {
	jsonString := new(bytes.Buffer)
	enc := json.NewEncoder(jsonString)
	err := enc.Encode(m)

	return strings.Trim(jsonString.String(), "\n"), err
}

const (
	SourceCategoryKey = "_sourceCategory"
	SourceHostKey     = "_sourceHost"
	SourceNameKey     = "_sourceName"
)

func (sumoMarshaler) MarshalLogs(ld plog.Logs) ([]byte, error) {
	buf := bytes.Buffer{}
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		ra := rl.Resource().Attributes()
		sourceCategory, exists := ra.Get(SourceCategoryKey)
		if !exists {
			return nil, errors.New("_sourceCategory attribute does not exist")
		}
		sourceHost, exists := ra.Get(SourceHostKey)
		if !exists {
			return nil, errors.New("_sourceHost attribute does not exist")
		}
		sourceName, exists := ra.Get(SourceNameKey)
		if !exists {
			return nil, errors.New("_sourceName attribute does not exist")
		}

		sc, err := attributeValueToString(sourceCategory)
		if err != nil {
			return nil, err
		}
		sh, err := attributeValueToString(sourceHost)
		if err != nil {
			return nil, err
		}
		sn, err := attributeValueToString(sourceName)
		if err != nil {
			return nil, err
		}
		sc = strconv.Quote(sc)
		sh = strconv.Quote(sh)
		sn = strconv.Quote(sn)

		// Remove the source attributes so that they won't be included in "fields" value.
		ra.Remove(SourceCategoryKey)
		ra.Remove(SourceHostKey)
		ra.Remove(SourceNameKey)

		fields, err := valueToJSON(ra.AsRaw())
		if err != nil {
			return nil, err
		}

		ills := rl.ScopeLogs()
		for j := 0; j < ills.Len(); j++ {
			ils := ills.At(j)
			logs := ils.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				lr := logs.At(k)
				dateVal := lr.ObservedTimestamp()

				message, err := getMessageJSON(lr)
				if err != nil {
					return nil, err
				}

				logEntry(&buf, "{\"date\": \"%s\",\"sourceName\":%s,\"sourceHost\":%s,\"sourceCategory\":%s,\"fields\":%s,\"message\":%s}",
					dateVal, sn, sh, sc, fields, message)
			}
		}
	}
	return buf.Bytes(), nil
}

func getMessageJSON(lr plog.LogRecord) (string, error) {
	// The "message" fields is a JSON created from combining the actual log body and log-level attributes,
	// where the log body is stored under "log" key.
	// More info:
	// https://help.sumologic.com/docs/send-data/opentelemetry-collector/data-source-configurations/additional-configurations-reference/#mapping-opentelemetry-concepts-to-sumo-logic
	message := new(bytes.Buffer)
	enc := json.NewEncoder(message)

	lr.Body().CopyTo(lr.Attributes().PutEmpty(logBodyKey))
	err := enc.Encode(lr.Attributes().AsRaw())

	return strings.Trim(message.String(), "\n"), err
}

func (s sumoMarshaler) MarshalTraces(_ ptrace.Traces) ([]byte, error) {
	return nil, fmt.Errorf("traces can't be marshaled into %s format", s.format())
}

func (s sumoMarshaler) MarshalMetrics(_ pmetric.Metrics) ([]byte, error) {
	return nil, fmt.Errorf("metrics can't be marshaled into %s format", s.format())
}
