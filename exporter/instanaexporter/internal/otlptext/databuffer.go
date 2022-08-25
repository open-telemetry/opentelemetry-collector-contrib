// Copyright 2022, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otlptext // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/otlptext"

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type dataBuffer struct {
	buf bytes.Buffer
}

func (b *dataBuffer) logEntry(format string, a ...interface{}) {
	b.buf.WriteString(fmt.Sprintf(format, a...))
	b.buf.WriteString("\n")
}

func (b *dataBuffer) logAttr(label string, value string) {
	b.logEntry("    %-15s: %s", label, value)
}

func (b *dataBuffer) logAttributeMap(label string, am pcommon.Map) {
	if am.Len() == 0 {
		return
	}

	b.logEntry("%s:", label)
	am.Range(func(k string, v pcommon.Value) bool {
		b.logEntry("     -> %s: %s(%s)", k, v.Type().String(), attributeValueToString(v))
		return true
	})
}

func (b *dataBuffer) logInstrumentationLibrary(il pcommon.InstrumentationScope) {
	b.logEntry(
		"InstrumentationLibrary %s %s",
		il.Name(),
		il.Version())
}

func (b *dataBuffer) logEvents(description string, se ptrace.SpanEventSlice) {
	if se.Len() == 0 {
		return
	}

	b.logEntry("%s:", description)
	for i := 0; i < se.Len(); i++ {
		e := se.At(i)
		b.logEntry("SpanEvent #%d", i)
		b.logEntry("     -> Name: %s", e.Name())
		b.logEntry("     -> Timestamp: %s", e.Timestamp())
		b.logEntry("     -> DroppedAttributesCount: %d", e.DroppedAttributesCount())

		if e.Attributes().Len() == 0 {
			continue
		}
		b.logEntry("     -> Attributes:")
		e.Attributes().Range(func(k string, v pcommon.Value) bool {
			b.logEntry("         -> %s: %s(%s)", k, v.Type().String(), attributeValueToString(v))
			return true
		})
	}
}

func (b *dataBuffer) logLinks(description string, sl ptrace.SpanLinkSlice) {
	if sl.Len() == 0 {
		return
	}

	b.logEntry("%s:", description)

	for i := 0; i < sl.Len(); i++ {
		l := sl.At(i)
		b.logEntry("SpanLink #%d", i)
		b.logEntry("     -> Trace ID: %s", l.TraceID().HexString())
		b.logEntry("     -> ID: %s", l.SpanID().HexString())
		b.logEntry("     -> TraceState: %s", l.TraceState())
		b.logEntry("     -> DroppedAttributesCount: %d", l.DroppedAttributesCount())
		if l.Attributes().Len() == 0 {
			continue
		}
		b.logEntry("     -> Attributes:")
		l.Attributes().Range(func(k string, v pcommon.Value) bool {
			b.logEntry("         -> %s: %s(%s)", k, v.Type().String(), attributeValueToString(v))
			return true
		})
	}
}

func attributeValueToString(av pcommon.Value) string {
	switch av.Type() {
	case pcommon.ValueTypeString:
		return av.StringVal()
	case pcommon.ValueTypeBool:
		return strconv.FormatBool(av.BoolVal())
	case pcommon.ValueTypeDouble:
		return strconv.FormatFloat(av.DoubleVal(), 'f', -1, 64)
	case pcommon.ValueTypeInt:
		return strconv.FormatInt(av.IntVal(), 10)
	case pcommon.ValueTypeSlice:
		return attributeValueArrayToString(av.SliceVal())
	case pcommon.ValueTypeMap:
		return attributeMapToString(av.MapVal())
	default:
		return fmt.Sprintf("<Unknown OpenTelemetry attribute value type %q>", av.Type())
	}
}

func attributeValueArrayToString(av pcommon.Slice) string {
	var b strings.Builder
	b.WriteByte('[')
	for i := 0; i < av.Len(); i++ {
		if i < av.Len()-1 {
			fmt.Fprintf(&b, "%s, ", attributeValueToString(av.At(i)))
		} else {
			b.WriteString(attributeValueToString(av.At(i)))
		}
	}

	b.WriteByte(']')
	return b.String()
}

func attributeMapToString(av pcommon.Map) string {
	var b strings.Builder
	b.WriteString("{\n")

	av.Sort().Range(func(k string, v pcommon.Value) bool {
		fmt.Fprintf(&b, "     -> %s: %s(%s)\n", k, v.Type(), v.AsString())
		return true
	})
	b.WriteByte('}')
	return b.String()
}
