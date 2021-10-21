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

	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

// ExceptionEventName the name of the exception event.
// TODO: Remove this when collector defines this semantic convention.
const ExceptionEventName = "exception"

func addCause(seg *awsxray.Segment, span *pdata.Span) {
	if seg.Cause == nil {
		return
	}

	// In the `rawExpectedSegmentForInstrumentedApp` X-Ray segment example in
	// awsxray/tracesegment_test.go, you can see that sometimes the HTTP Response is not
	// set but the Cause field is set for the root segment. And the actual HTTP status
	// can only be found in one of the (nested) subsegmets. So we need to signal that
	// in this case, the status of the span is not otlptrace.Status_Ok by
	// temporarily setting the status to otlptrace.Status_UnknownError. This will be
	// updated to a more specific error in the `segToSpans()` in translator.go once
	// we traverse through all the subsegments.
	if span.Status().Code() == pdata.StatusCodeUnset {
		// StatusCodeUnset is the default value for the span.Status().
		span.Status().SetCode(pdata.StatusCodeError)
	}

	switch seg.Cause.Type {
	case awsxray.CauseTypeExceptionID:
		// Right now the X-Ray exporter does not support the case where
		// 1) CauseData is just a 16-char exception ID,
		// 2) `WorkingDirectory` and `Paths`
		// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/c615d2db351929b99e46f7b427f39c12afe15b54/exporter/awsxrayexporter/translator/cause.go#L107

		// so we can only pass the cause exceptionID as the status message as a fallback mechanism
		span.Status().SetMessage(*seg.Cause.ExceptionID)
	case awsxray.CauseTypeObject:
		evts := span.Events()
		// not sure whether there are existing events, so
		// append new empty events instead
		exceptionEventStartIndex := evts.Len()
		evts.EnsureCapacity(exceptionEventStartIndex + len(seg.Cause.Exceptions))

		for _, excp := range seg.Cause.Exceptions {
			evt := evts.AppendEmpty()
			evt.SetName(ExceptionEventName)
			attrs := evt.Attributes()
			attrs.Clear()
			attrs.EnsureCapacity(8)

			// ID is a required field
			attrs.UpsertString(awsxray.AWSXrayExceptionIDAttribute, *excp.ID)
			addString(excp.Message, conventions.AttributeExceptionMessage, &attrs)
			addString(excp.Type, conventions.AttributeExceptionType, &attrs)
			addBool(excp.Remote, awsxray.AWSXrayExceptionRemoteAttribute, &attrs)
			addInt64(excp.Truncated, awsxray.AWSXrayExceptionTruncatedAttribute, &attrs)
			addInt64(excp.Skipped, awsxray.AWSXrayExceptionSkippedAttribute, &attrs)
			addString(excp.Cause, awsxray.AWSXrayExceptionCauseAttribute, &attrs)

			if len(excp.Stack) > 0 {
				stackTrace := convertStackFramesToStackTraceStr(excp)
				attrs.UpsertString(conventions.AttributeExceptionStacktrace, stackTrace)
			}
		}
	}
}

func convertStackFramesToStackTraceStr(excp awsxray.Exception) string {
	// resulting stacktrace looks like:
	// "<*excp.Type>: <*excp.Message>\n" +
	// "\tat <*frameN.Label>(<*frameN.Path>: <*frameN.Line>)\n"
	var b strings.Builder
	b.Grow(len(*excp.Type) + len(": ") + len(*excp.Message) + len("\n"))
	b.WriteString(*excp.Type)
	b.WriteString(": ")
	b.WriteString(*excp.Message)
	b.WriteString("\n")
	for _, frame := range excp.Stack {
		line := strconv.Itoa(*frame.Line)
		// the string representation of a frame looks like:
		// <*frame.Label>(<*frame.Path>):line\n
		b.Grow(4 + len(*frame.Label) + 2 + len(*frame.Path) + len(": ") + len(line) + len("\n"))
		b.WriteString("\tat ")
		b.WriteString(*frame.Label)
		b.WriteString("(")
		b.WriteString(*frame.Path)
		b.WriteString(": ")
		b.WriteString(line)
		b.WriteString(")")
		b.WriteString("\n")
	}
	return b.String()
}
