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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

func TestSpanStartTimeIsConvertedToMilliseconds(t *testing.T) {
	inNanos := int64(50000000)
	att := pcommon.NewMap()
	transform := transformerFromAttributes(att)
	span := ptrace.NewSpan()
	span.SetSpanID(pcommon.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))
	span.SetTraceID(pcommon.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	span.SetStartTimestamp(pcommon.Timestamp(inNanos))

	actual, err := transform.Span(span)
	require.NoError(t, err, "transforming span to wavefront format")

	assert.Equal(t, inNanos/time.Millisecond.Nanoseconds(), actual.StartMillis)
}

func TestSpanDurationIsCalculatedFromStartAndEndTimes(t *testing.T) {
	startNanos := int64(50000000)
	endNanos := int64(60000000)
	att := pcommon.NewMap()
	transform := transformerFromAttributes(att)
	span := ptrace.NewSpan()
	span.SetSpanID(pcommon.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))
	span.SetTraceID(pcommon.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	span.SetStartTimestamp(pcommon.Timestamp(startNanos))
	span.SetEndTimestamp(pcommon.Timestamp(endNanos))

	actual, err := transform.Span(span)
	require.NoError(t, err, "transforming span to wavefront format")

	assert.Equal(t, int64(10), actual.DurationMillis)
}

func TestSpanDurationIsZeroIfEndTimeIsUnset(t *testing.T) {
	startNanos := int64(50000000)
	att := pcommon.NewMap()
	transform := transformerFromAttributes(att)
	span := ptrace.NewSpan()
	span.SetSpanID(pcommon.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))
	span.SetTraceID(pcommon.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	span.SetStartTimestamp(pcommon.Timestamp(startNanos))

	actual, err := transform.Span(span)
	require.NoError(t, err, "transforming span to wavefront format")

	assert.Equal(t, int64(0), actual.DurationMillis)
}

func TestSpanStatusCodeErrorAddsErrorTag(t *testing.T) {
	transform := transformerFromAttributes(pcommon.NewMap())
	actual, err := transform.Span(spanWithStatus(ptrace.StatusCodeError, ""))
	require.NoError(t, err, "transforming span to wavefront format")

	errorTag, ok := actual.Tags["error"]
	assert.True(t, ok)
	assert.Equal(t, "true", errorTag)
}

func TestSpanStatusCodeOkDoesNotAddErrorTag(t *testing.T) {
	transform := transformerFromAttributes(pcommon.NewMap())
	actual, err := transform.Span(spanWithStatus(ptrace.StatusCodeOk, ""))
	require.NoError(t, err, "transforming span to wavefront format")

	_, ok := actual.Tags["error"]
	assert.False(t, ok)
}

func TestSpanStatusCodeUnsetDoesNotAddErrorTag(t *testing.T) {
	transform := transformerFromAttributes(pcommon.NewMap())
	actual, err := transform.Span(spanWithStatus(ptrace.StatusCodeUnset, ""))
	require.NoError(t, err, "transforming span to wavefront format")

	_, ok := actual.Tags["error"]
	assert.False(t, ok)
}

func TestSpanStatusMessageIsConvertedToTag(t *testing.T) {
	transform := transformerFromAttributes(pcommon.NewMap())
	message := "some error message"
	actual, err := transform.Span(spanWithStatus(ptrace.StatusCodeError, message))

	require.NoError(t, err, "transforming span to wavefront format")

	msgVal, ok := actual.Tags["otel.status_description"]
	assert.True(t, ok)
	assert.Equal(t, message, msgVal)
}

func TestSpanStatusMessageIsIgnoredIfStatusIsNotError(t *testing.T) {
	transform := transformerFromAttributes(pcommon.NewMap())
	actual, err := transform.Span(spanWithStatus(ptrace.StatusCodeOk, "not a real error message"))

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
	transform := transformerFromAttributes(pcommon.NewMap())
	message := "1234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890"
	message += "1234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890"
	message += "1234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890"
	actual, err := transform.Span(spanWithStatus(ptrace.StatusCodeError, message))

	require.NoError(t, err, "transforming span to wavefront format")

	msgVal, ok := actual.Tags["otel.status_description"]
	assert.True(t, ok)
	assert.Equal(t, 255-1-len("otel.status_description"), len(msgVal), "message value truncated")
}

func TestSpanEventsAreTranslatedToSpanLogs(t *testing.T) {
	transform := transformerFromAttributes(pcommon.NewMap())
	now := time.Now()
	span := ptrace.NewSpan()
	span.SetSpanID(pcommon.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))
	span.SetTraceID(pcommon.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	event := ptrace.NewSpanEvent()
	event.SetName("eventName")
	event.SetTimestamp(pcommon.NewTimestampFromTime(now))
	eventAttrs := pcommon.NewMap()
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
	transform := transformerFromAttributes(pcommon.NewMap())

	internalSpan, err := transform.Span(spanWithKind(ptrace.SpanKindInternal))
	require.NoError(t, err, "transforming span to wavefront format")
	kind, ok := internalSpan.Tags["span.kind"]
	assert.True(t, ok)
	assert.Equal(t, "internal", kind)

	serverSpan, err := transform.Span(spanWithKind(ptrace.SpanKindServer))
	require.NoError(t, err, "transforming span to wavefront format")
	kind, ok = serverSpan.Tags["span.kind"]
	assert.True(t, ok)
	assert.Equal(t, "server", kind)

	clientSpan, err := transform.Span(spanWithKind(ptrace.SpanKindClient))
	require.NoError(t, err, "transforming span to wavefront format")
	kind, ok = clientSpan.Tags["span.kind"]
	assert.True(t, ok)
	assert.Equal(t, "client", kind)

	consumerSpan, err := transform.Span(spanWithKind(ptrace.SpanKindConsumer))
	require.NoError(t, err, "transforming span to wavefront format")
	kind, ok = consumerSpan.Tags["span.kind"]
	assert.True(t, ok)
	assert.Equal(t, "consumer", kind)

	producerSpan, err := transform.Span(spanWithKind(ptrace.SpanKindProducer))
	require.NoError(t, err, "transforming span to wavefront format")
	kind, ok = producerSpan.Tags["span.kind"]
	assert.True(t, ok)
	assert.Equal(t, "producer", kind)

	unspecifiedSpan, err := transform.Span(spanWithKind(ptrace.SpanKindUnspecified))
	require.NoError(t, err, "transforming span to wavefront format")
	kind, ok = unspecifiedSpan.Tags["span.kind"]
	assert.True(t, ok)
	assert.Equal(t, "unspecified", kind)
}

func TestTraceStateTranslatedToTag(t *testing.T) {
	transform := transformerFromAttributes(pcommon.NewMap())

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
	resAttrs := pcommon.NewMap()
	transform := transformerFromAttributes(resAttrs)
	span := ptrace.NewSpan()
	span.SetSpanID(pcommon.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))
	span.SetTraceID(pcommon.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	span.SetStartTimestamp(pcommon.Timestamp(inNanos))

	actual, err := transform.Span(span)
	require.NoError(t, err, "transforming span to wavefront format")
	assert.Equal(t, "", actual.Source)

	//TestCase2: source value from resAttrs.source
	resAttrs = pcommon.NewMap()
	resAttrs.InsertString(labelSource, "test_source")
	resAttrs.InsertString(conventions.AttributeHostName, "test_host.name")
	transform = transformerFromAttributes(resAttrs)
	span = ptrace.NewSpan()
	span.SetSpanID(pcommon.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))
	span.SetTraceID(pcommon.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	span.SetStartTimestamp(pcommon.Timestamp(inNanos))

	actual, err = transform.Span(span)
	require.NoError(t, err, "transforming span to wavefront format")
	assert.Equal(t, "test_source", actual.Source)
	assert.Equal(t, "test_host.name", actual.Tags[conventions.AttributeHostName])
	if value, isFound := actual.Tags[labelSource]; isFound {
		t.Logf("Tag Source with value " + value + " not expected.")
		t.Fail()
	}

	//TestCase2: source value from resAttrs.host.name when source is not present
	resAttrs = pcommon.NewMap()
	resAttrs.InsertString("hostname", "test_hostname")
	resAttrs.InsertString(conventions.AttributeHostName, "test_host.name")
	transform = transformerFromAttributes(resAttrs)
	span = ptrace.NewSpan()
	span.SetSpanID(pcommon.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))
	span.SetTraceID(pcommon.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	span.SetStartTimestamp(pcommon.Timestamp(inNanos))

	actual, err = transform.Span(span)
	require.NoError(t, err, "transforming span to wavefront format")
	assert.Equal(t, "test_host.name", actual.Source)
	assert.Equal(t, "test_hostname", actual.Tags["hostname"])
	if value, isFound := actual.Tags[conventions.AttributeHostName]; isFound {
		t.Logf("Tag host.name with value " + value + " not expected.")
		t.Fail()
	}

	//TestCase4: source value from resAttrs.source when spanAttrs.source is present
	resAttrs = pcommon.NewMap()
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

func TestSpanForDroppedCount(t *testing.T) {
	inNanos := int64(50000000)

	//TestCase: 1 count tags are not set
	resAttrs := pcommon.NewMap()
	transform := transformerFromAttributes(resAttrs)
	span := ptrace.NewSpan()
	span.SetSpanID(pcommon.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))
	span.SetTraceID(pcommon.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	span.SetStartTimestamp(pcommon.Timestamp(inNanos))

	actual, err := transform.Span(span)
	require.NoError(t, err, "transforming span to wavefront format")
	assert.NotContains(t, actual.Tags, "otel.dropped_events_count")
	assert.NotContains(t, actual.Tags, "otel.dropped_links_count")
	assert.NotContains(t, actual.Tags, "otel.dropped_attributes_count")

	//TestCase2: count tags are set
	span.SetDroppedEventsCount(123)
	span.SetDroppedLinksCount(456)
	span.SetDroppedAttributesCount(789)

	actual, err = transform.Span(span)
	require.NoError(t, err, "transforming span to wavefront format")
	assert.Equal(t, "123", actual.Tags["otel.dropped_events_count"])
	assert.Equal(t, "456", actual.Tags["otel.dropped_links_count"])
	assert.Equal(t, "789", actual.Tags["otel.dropped_attributes_count"])
}

func TestGetSourceAndResourceTags(t *testing.T) {
	resAttrs := pcommon.NewMap()
	resAttrs.InsertString(labelSource, "test_source")
	resAttrs.InsertString(conventions.AttributeHostName, "test_host.name")

	actualSource, actualAttrsWithoutSource := getSourceAndResourceTags(resAttrs)
	assert.Equal(t, "test_source", actualSource)
	if value, isFound := actualAttrsWithoutSource[labelSource]; isFound {
		t.Logf("Tag Source with value " + value + " not expected.")
		t.Fail()
	}
}

func TestGetSourceAndKey(t *testing.T) {
	resAttrs := pcommon.NewMap()
	resAttrs.InsertString(labelSource, "some_source")
	resAttrs.InsertString(conventions.AttributeHostName, "test_host.name")

	source, sourceKey := getSourceAndKey(resAttrs)
	assert.Equal(t, "some_source", source)
	assert.Equal(t, labelSource, sourceKey)
}

func TestGetSourceAndKeyNotFound(t *testing.T) {
	resAttrs := pcommon.NewMap()
	resAttrs.InsertString("foo", "some_source")
	resAttrs.InsertString("bar", "test_host.name")

	source, sourceKey := getSourceAndKey(resAttrs)
	assert.Equal(t, "", source)
	assert.Equal(t, "", sourceKey)
}

func TestAttributesToTagsReplaceSource(t *testing.T) {
	attrMap1 := newMap(map[string]string{"customer": "aws", "env": "dev"})
	attrMap2 := newMap(map[string]string{"env": "prod", "source": "ethernet"})
	result := attributesToTagsReplaceSource(attrMap1, attrMap2)

	// attrMap2 takes precedence because it is last, so "env"->"prod" not "dev"
	assert.Equal(
		t,
		map[string]string{"env": "prod", "customer": "aws", "_source": "ethernet"},
		result)
}

func spanWithKind(kind ptrace.SpanKind) ptrace.Span {
	span := ptrace.NewSpan()
	span.SetSpanID(pcommon.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))
	span.SetTraceID(pcommon.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	span.SetKind(kind)
	return span
}

func spanWithTraceState(state ptrace.TraceState) ptrace.Span {
	span := ptrace.NewSpan()
	span.SetSpanID(pcommon.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))
	span.SetTraceID(pcommon.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	span.SetTraceState(state)
	return span
}

func transformerFromAttributes(attrs pcommon.Map) *traceTransformer {
	return &traceTransformer{
		resAttrs: attrs,
	}
}

func spanWithStatus(statusCode ptrace.StatusCode, message string) ptrace.Span {
	span := ptrace.NewSpan()
	span.SetSpanID(pcommon.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))
	span.SetTraceID(pcommon.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	status := ptrace.NewSpanStatus()
	status.SetCode(statusCode)
	if message != "" {
		status.SetMessage(message)
	}
	status.CopyTo(span.Status())
	return span
}
