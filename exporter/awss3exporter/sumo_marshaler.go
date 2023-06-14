// Copyright 2022, OpenTelemetry Authors
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

package awss3exporter

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type SumoMarshaler struct {
	format string
}

func (marshaler *SumoMarshaler) Format() string {
	return marshaler.format
}

func NewSumoMarshaler() SumoMarshaler {
	return SumoMarshaler{}
}

func logEntry(buf *bytes.Buffer, format string, a ...interface{}) {
	buf.WriteString(fmt.Sprintf(format, a...))
	buf.WriteString("\n")
}

func attributeValueToString(v pcommon.Value) string {
	// TODO: chceck all types and add missing
	switch v.Type() {
	case pcommon.ValueTypeStr:
		return v.Str()
	case pcommon.ValueTypeBool:
		return strconv.FormatBool(v.Bool())
	case pcommon.ValueTypeDouble:
		return strconv.FormatFloat(v.Double(), 'f', -1, 64)
	case pcommon.ValueTypeInt:
		return strconv.FormatInt(v.Int(), 10)
	case pcommon.ValueTypeSlice:
		return sliceToString(v.Slice())
	case pcommon.ValueTypeMap:
		return mapToString(v.Map())
	default:
		return fmt.Sprintf("<Unknown OpenTelemetry attribute value type %q>", v.Type())
	}
}

func sliceToString(s pcommon.Slice) string {
	var b strings.Builder
	b.WriteByte('[')
	for i := 0; i < s.Len(); i++ {
		if i < s.Len()-1 {
			fmt.Fprintf(&b, "%s, ", attributeValueToString(s.At(i)))
		} else {
			b.WriteString(attributeValueToString(s.At(i)))
		}
	}

	b.WriteByte(']')
	return b.String()
}

func mapToString(m pcommon.Map) string {
	var b strings.Builder
	b.WriteString("{\n")

	m.Range(func(k string, v pcommon.Value) bool {
		fmt.Fprintf(&b, "     -> %s: %s(%s)\n", k, v.Type(), v.AsString())
		return true
	})
	b.WriteByte('}')
	return b.String()
}

const (
	SourceCategoryKey = "_sourceCategory"
	SourceHostKey     = "_sourceHost"
	SourceNameKey     = "log.file.path_resolved"
)

func (SumoMarshaler) MarshalLogs(ld plog.Logs) ([]byte, error) {
	buf := bytes.Buffer{}
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		sourceCategory, exists := rl.Resource().Attributes().Get(SourceCategoryKey)
		if !exists {
			return nil, errors.New("_sourceCategory attribute does not exists")
		}
		sourceHost, exists := rl.Resource().Attributes().Get(SourceHostKey)
		if !exists {
			return nil, errors.New("_sourceHost attribute does not exists")
		}
		ills := rl.ScopeLogs()
		for j := 0; j < ills.Len(); j++ {
			ils := ills.At(j)
			logs := ils.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				lr := logs.At(k)
				dateVal := lr.ObservedTimestamp()
				body := attributeValueToString(lr.Body())
				sourceName, exists := lr.Attributes().Get(SourceNameKey)
				if !exists {
					return nil, errors.New("_sourceName attribute does not exists")
				}
				logEntry(&buf, "{\"data\": \"%s\",\"sourceName\":\"%s\",\"sourceHost\":\"%s\",\"sourceCategory\":\"%s\",\"fields\":{},\"message\":\"%s\"}",
					dateVal, attributeValueToString(sourceName), attributeValueToString(sourceHost), attributeValueToString(sourceCategory), body)
			}
		}
	}
	return buf.Bytes(), nil
}

func (SumoMarshaler) MarshalTraces(traces ptrace.Traces) ([]byte, error) {
	return nil, nil
}

func (SumoMarshaler) MarshalMetrics(md pmetric.Metrics) ([]byte, error) {
	return nil, nil
}
