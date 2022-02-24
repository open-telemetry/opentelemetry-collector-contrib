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

package tanzuobservabilityexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.6.1"
)

func TestSpanStartTimeIsConvertedToMilliseconds(t *testing.T) {
	inNanos := int64(50000000)
	att := pdata.NewAttributeMap()
	transform := transformerFromAttributes(att)
	span := pdata.NewSpan()
	span.SetSpanID(pdata.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))
	span.SetTraceID(pdata.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	span.SetStartTimestamp(pdata.Timestamp(inNanos))

	actual, err := transform.Span(span)
	require.NoError(t, err, "transforming span to wavefront format")

	assert.Equal(t, inNanos/time.Millisecond.Nanoseconds(), actual.StartMillis)
}

func TestSpanDurationIsCalculatedFromStartAndEndTimes(t *testing.T) {
	startNanos := int64(50000000)
	endNanos := int64(60000000)
	att := pdata.NewAttributeMap()
	transform := transformerFromAttributes(att)
	span := pdata.NewSpan()
	span.SetSpanID(pdata.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))
	span.SetTraceID(pdata.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	span.SetStartTimestamp(pdata.Timestamp(startNanos))
	span.SetEndTimestamp(pdata.Timestamp(endNanos))

	actual, err := transform.Span(span)
	require.NoError(t, err, "transforming span to wavefront format")

	assert.Equal(t, int64(10), actual.DurationMillis)
}

func TestSpanDurationIsZeroIfEndTimeIsUnset(t *testing.T) {
	startNanos := int64(50000000)
	att := pdata.NewAttributeMap()
	transform := transformerFromAttributes(att)
	span := pdata.NewSpan()
	span.SetSpanID(pdata.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))
	span.SetTraceID(pdata.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	span.SetStartTimestamp(pdata.Timestamp(startNanos))

	actual, err := transform.Span(span)
	require.NoError(t, err, "transforming span to wavefront format")

	assert.Equal(t, int64(0), actual.DurationMillis)
}

func TestSpanStatusCodeErrorAddsErrorTag(t *testing.T) {
	transform := transformerFromAttributes(pdata.NewAttributeMap())
	actual, err := transform.Span(spanWithStatus(pdata.StatusCodeError, ""))
	require.NoError(t, err, "transforming span to wavefront format")

	errorTag, ok := actual.Tags["error"]
	assert.True(t, ok)
	assert.Equal(t, "true", errorTag)
}

func TestSpanStatusCodeOkDoesNotAddErrorTag(t *testing.T) {
	transform := transformerFromAttributes(pdata.NewAttributeMap())
	actual, err := transform.Span(spanWithStatus(pdata.StatusCodeOk, ""))
	require.NoError(t, err, "transforming span to wavefront format")

	_, ok := actual.Tags["error"]
	assert.False(t, ok)
}

func TestSpanStatusCodeUnsetDoesNotAddErrorTag(t *testing.T) {
	transform := transformerFromAttributes(pdata.NewAttributeMap())
	actual, err := transform.Span(spanWithStatus(pdata.StatusCodeUnset, ""))
	require.NoError(t, err, "transforming span to wavefront format")

	_, ok := actual.Tags["error"]
	assert.False(t, ok)
}

func TestSpanStatusMessageIsConvertedToTag(t *testing.T) {
	transform := transformerFromAttributes(pdata.NewAttributeMap())
	message := "some error message"
	actual, err := transform.Span(spanWithStatus(pdata.StatusCodeError, message))

	require.NoError(t, err, "transforming span to wavefront format")

	msgVal, ok := actual.Tags["otel.status_description"]
	assert.True(t, ok)
	assert.Equal(t, message, msgVal)
}

func TestSpanStatusMessageIsIgnoredIfStatusIsNotError(t *testing.T) {
	transform := transformerFromAttributes(pdata.NewAttributeMap())
	actual, err := transform.Span(spanWithStatus(pdata.StatusCodeOk, "not a real error message"))

	require.NoError(t, err, "transforming span to wavefront format")

	_, ok := actual.Tags["status.message"]
	assert.False(t, ok)
}

func TestSpanStatusMessageIsTruncatedToValidLength(t *testing.T) {
	/*
	 * Maximum allowed length for a combination of a point tag key and value is 254 characters
	 * (255 including the "=" separating key and value). If the value is longer, the point is rejected and logged.
	 * Keep the number of distinct time series per metric and host to under 1000.
	 * -- https://docs.wavefront.com/wavefront_data_format.html
	 */
	transform := transformerFromAttributes(pdata.NewAttributeMap())
	message := "1234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890"
	message += "1234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890"
	message += "1234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890"
	actual, err := transform.Span(spanWithStatus(pdata.StatusCodeError, message))

	require.NoError(t, err, "transforming span to wavefront format")

	msgVal, ok := actual.Tags["otel.status_description"]
	assert.True(t, ok)
	assert.Equal(t, 255-1-len("otel.status_description"), len(msgVal), "message value truncated")
}

func TestSpanEventsAreTranslatedToSpanLogs(t *testing.T) {
	transform := transformerFromAttributes(pdata.NewAttributeMap())
	now := time.Now()
	span := pdata.NewSpan()
	span.SetSpanID(pdata.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))
	span.SetTraceID(pdata.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	event := pdata.NewSpanEvent()
	event.SetName("eventName")
	event.SetTimestamp(pdata.NewTimestampFromTime(now))
	eventAttrs := pdata.NewAttributeMap()
	eventAttrs.InsertString("attrKey", "attrVal")
	eventAttrs.CopyTo(event.Attributes())
	event.CopyTo(span.Events().AppendEmpty())

	result, err := transform.Span(span)
	require.NoError(t, err, "transforming span to wavefront format")

	require.Equal(t, 1, len(result.SpanLogs))
	actual := result.SpanLogs[0]
	assert.Equal(t, now.UnixNano()/time.Microsecond.Nanoseconds(), actual.Timestamp)
	name, ok := actual.Fields[labelEventName]
	assert.True(t, ok)
	assert.Equal(t, "eventName", name)
	attrVal, ok := actual.Fields["attrKey"]
	assert.True(t, ok)
	assert.Equal(t, "attrVal", attrVal)
}

func TestSpanKindIsTranslatedToTag(t *testing.T) {
	transform := transformerFromAttributes(pdata.NewAttributeMap())

	internalSpan, err := transform.Span(spanWithKind(pdata.SpanKindInternal))
	require.NoError(t, err, "transforming span to wavefront format")
	kind, ok := internalSpan.Tags["span.kind"]
	assert.True(t, ok)
	assert.Equal(t, "internal", kind)

	serverSpan, err := transform.Span(spanWithKind(pdata.SpanKindServer))
	require.NoError(t, err, "transforming span to wavefront format")
	kind, ok = serverSpan.Tags["span.kind"]
	assert.True(t, ok)
	assert.Equal(t, "server", kind)

	clientSpan, err := transform.Span(spanWithKind(pdata.SpanKindClient))
	require.NoError(t, err, "transforming span to wavefront format")
	kind, ok = clientSpan.Tags["span.kind"]
	assert.True(t, ok)
	assert.Equal(t, "client", kind)

	consumerSpan, err := transform.Span(spanWithKind(pdata.SpanKindConsumer))
	require.NoError(t, err, "transforming span to wavefront format")
	kind, ok = consumerSpan.Tags["span.kind"]
	assert.True(t, ok)
	assert.Equal(t, "consumer", kind)

	producerSpan, err := transform.Span(spanWithKind(pdata.SpanKindProducer))
	require.NoError(t, err, "transforming span to wavefront format")
	kind, ok = producerSpan.Tags["span.kind"]
	assert.True(t, ok)
	assert.Equal(t, "producer", kind)

	unspecifiedSpan, err := transform.Span(spanWithKind(pdata.SpanKindUnspecified))
	require.NoError(t, err, "transforming span to wavefront format")
	kind, ok = unspecifiedSpan.Tags["span.kind"]
	assert.True(t, ok)
	assert.Equal(t, "unspecified", kind)
}

func TestTraceStateTranslatedToTag(t *testing.T) {
	transform := transformerFromAttributes(pdata.NewAttributeMap())

	spanWithState, err := transform.Span(spanWithTraceState("key=val"))
	require.NoError(t, err, "transforming span to wavefront format")
	stateVal, ok := spanWithState.Tags["w3c.tracestate"]
	assert.True(t, ok)
	assert.Equal(t, "key=val", stateVal)

	spanWithEmptyState, err := transform.Span(spanWithTraceState(""))
	require.NoError(t, err, "transforming span to wavefront format")
	_, ok = spanWithEmptyState.Tags["w3c.tracestate"]
	assert.False(t, ok)
}

func TestSpanForSourceTag(t *testing.T) {
	inNanos := int64(50000000)

	//TestCase1: default value for source
	resAttrs := pdata.NewAttributeMap()
	transform := transformerFromAttributes(resAttrs)
	span := pdata.NewSpan()
	span.SetSpanID(pdata.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))
	span.SetTraceID(pdata.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	span.SetStartTimestamp(pdata.Timestamp(inNanos))

	actual, err := transform.Span(span)
	require.NoError(t, err, "transforming span to wavefront format")
	assert.Equal(t, "", actual.Source)

	//TestCase2: source value from resAttrs.source
	resAttrs = pdata.NewAttributeMap()
	resAttrs.InsertString(labelSource, "test_source")
	resAttrs.InsertString(conventions.AttributeHostName, "test_host.name")
	transform = transformerFromAttributes(resAttrs)
	span = pdata.NewSpan()
	span.SetSpanID(pdata.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))
	span.SetTraceID(pdata.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	span.SetStartTimestamp(pdata.Timestamp(inNanos))

	actual, err = transform.Span(span)
	require.NoError(t, err, "transforming span to wavefront format")
	assert.Equal(t, "test_source", actual.Source)
	assert.Equal(t, "test_host.name", actual.Tags[conventions.AttributeHostName])
	if value, isFound := actual.Tags[labelSource]; isFound {
		t.Logf("Tag Source with value " + value + " not expected.")
		t.Fail()
	}

	//TestCase2: source value from resAttrs.host.name when source is not present
	resAttrs = pdata.NewAttributeMap()
	resAttrs.InsertString("hostname", "test_hostname")
	resAttrs.InsertString(conventions.AttributeHostName, "test_host.name")
	transform = transformerFromAttributes(resAttrs)
	span = pdata.NewSpan()
	span.SetSpanID(pdata.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))
	span.SetTraceID(pdata.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	span.SetStartTimestamp(pdata.Timestamp(inNanos))

	actual, err = transform.Span(span)
	require.NoError(t, err, "transforming span to wavefront format")
	assert.Equal(t, "test_host.name", actual.Source)
	assert.Equal(t, "test_hostname", actual.Tags["hostname"])
	if value, isFound := actual.Tags[conventions.AttributeHostName]; isFound {
		t.Logf("Tag host.name with value " + value + " not expected.")
		t.Fail()
	}

	//TestCase4: source value from resAttrs.source when spanAttrs.source is present
	resAttrs = pdata.NewAttributeMap()
	span.Attributes().InsertString(labelSource, "source_from_span_attribute")
	resAttrs.InsertString(labelSource, "test_source")
	resAttrs.InsertString(conventions.AttributeHostName, "test_host.name")
	transform = transformerFromAttributes(resAttrs)
	actual, err = transform.Span(span)
	require.NoError(t, err, "transforming span to wavefront format")
	assert.Equal(t, "test_source", actual.Source)
	assert.Equal(t, "test_host.name", actual.Tags[conventions.AttributeHostName])
	if value, isFound := actual.Tags[labelSource]; isFound {
		t.Logf("Tag Source with value " + value + " not expected.")
		t.Fail()
	}
	assert.Equal(t, "source_from_span_attribute", actual.Tags["_source"])
}

func TestGetSourceAndResourceTags(t *testing.T) {
	resAttrs := pdata.NewAttributeMap()
	resAttrs.InsertString(labelSource, "test_source")
	resAttrs.InsertString(conventions.AttributeHostName, "test_host.name")

	actualSource, actualAttrsWithoutSource := getSourceAndResourceTags(resAttrs)
	assert.Equal(t, "test_source", actualSource)
	if value, isFound := actualAttrsWithoutSource[labelSource]; isFound {
		t.Logf("Tag Source with value " + value + " not expected.")
		t.Fail()
	}
}

func spanWithKind(kind pdata.SpanKind) pdata.Span {
	span := pdata.NewSpan()
	span.SetSpanID(pdata.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))
	span.SetTraceID(pdata.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	span.SetKind(kind)
	return span
}

func spanWithTraceState(state pdata.TraceState) pdata.Span {
	span := pdata.NewSpan()
	span.SetSpanID(pdata.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))
	span.SetTraceID(pdata.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	span.SetTraceState(state)
	return span
}

func transformerFromAttributes(attrs pdata.AttributeMap) *traceTransformer {
	return &traceTransformer{
		resAttrs: attrs,
	}
}

func spanWithStatus(statusCode pdata.StatusCode, message string) pdata.Span {
	span := pdata.NewSpan()
	span.SetSpanID(pdata.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))
	span.SetTraceID(pdata.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	status := pdata.NewSpanStatus()
	status.SetCode(statusCode)
	if message != "" {
		status.SetMessage(message)
	}
	status.CopyTo(span.Status())
	return span
}
