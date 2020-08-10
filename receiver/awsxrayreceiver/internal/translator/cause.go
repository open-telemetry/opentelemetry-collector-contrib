// Copyright The OpenTelemetry Authors
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

package translator

import (
	"strconv"
	"strings"

	otlptrace "github.com/open-telemetry/opentelemetry-proto/gen/go/trace/v1"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/tracesegment"
)

const (
	newLine   = "\n"
	separator = ":"
)

func addCause(seg *tracesegment.Segment, span *pdata.Span) {
	if seg.Cause == nil {
		return
	}

	// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/master/exporter/awsxrayexporter/translator/cause.go#L51
	// In the `rawExpectedSegmentForInstrumentedApp` X-Ray segment example in
	// tracesegment_test.go, you can see that sometimes the HTTP Response is not
	// set but the Cause field is set for the root segment. And the actual HTTP status
	// can only be found in one of the (nested) subsegmets. So we need to signal that
	// in this case, the status of the span is not otlptrace.Status_Ok by
	// temporarily setting the status to otlptrace.Status_UnknownError. This will be
	// updated to a more specific error in the `segToSpans()` in translator.go once
	// we traverse through all the subsegments.
	if span.Status().Code() == pdata.StatusCode(otlptrace.Status_Ok) {
		// otlptrace.Status_Ok is the default value after span.Status().InitEmpty()
		// is called
		span.Status().SetCode(pdata.StatusCode(otlptrace.Status_UnknownError))
	}

	switch seg.Cause.Type {
	case tracesegment.CauseTypeExceptionID:
		// Right now the X-Ray exporter always genearates a new ID:
		// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/master/exporter/awsxrayexporter/translator/cause.go#L74
		// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/master/exporter/awsxrayexporter/translator/cause.go#L112
		// so we can only pass this as part of the status message as a fallback mechanism
		span.Status().SetMessage(*seg.Cause.ExceptionID)
	case tracesegment.CauseTypeObject:
		evts := span.Events()
		// not sure whether there are existing events, so
		// append new empty events instead
		exceptionEventStartIndex := evts.Len()
		evts.Resize(exceptionEventStartIndex + len(seg.Cause.Exceptions))

		for i, excp := range seg.Cause.Exceptions {
			evt := evts.At(exceptionEventStartIndex + i)
			evt.InitEmpty()
			evt.SetName(conventions.AttributeExceptionEventName)
			attrs := evt.Attributes()
			attrs.InitEmptyWithCapacity(2)
			if excp.Type != nil {
				attrs.UpsertString(conventions.AttributeExceptionType, *excp.Type)
				attrs.UpsertString(conventions.AttributeExceptionMessage, *excp.Message)
				stackTrace := convertStackFramesToStackTraceStr(excp.Stack)
				attrs.UpsertString(conventions.AttributeExceptionStacktrace, stackTrace)
				// For now X-Ray's exception data model is not fully supported in the OpenTelemetry
				// spec, so some information is lost here.
				// For example, the "cause" ,"remote", ... and some fields within each exception
				// are dropped.
			}
		}
	}
}

func convertStackFramesToStackTraceStr(stack []tracesegment.StackFrame) string {
	var b strings.Builder
	for _, frame := range stack {
		line := strconv.Itoa(*frame.Line)
		// the string representation of a frame looks like:
		// <*frame.Label>\n<*frame.Path>:line\n
		b.Grow(len(*frame.Label) + len(*frame.Path) + len(line) + 3)
		b.WriteString(*frame.Label)
		b.WriteString(newLine)
		b.WriteString(*frame.Path)
		b.WriteString(separator)
		b.WriteString(line)
		b.WriteString(newLine)
	}
	return b.String()
}
