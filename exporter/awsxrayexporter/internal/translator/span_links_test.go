// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/translator"

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestSpanLinkSimple(t *testing.T) {
	spanName := "ProcessingMessage"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeOk, "OK", attributes)

	traceID := newTraceID()

	spanLink := span.Links().AppendEmpty()
	spanLink.SetTraceID(traceID)
	spanLink.SetSpanID(newSegmentID())

	segment, _ := MakeSegment(span, resource, nil, false, nil, false)

	convertedTraceID, _ := convertToAmazonTraceID(traceID, false)

	assert.Len(t, segment.Links, 1)
	assert.Equal(t, spanLink.SpanID().String(), *segment.Links[0].SpanID)
	assert.Equal(t, convertedTraceID, *segment.Links[0].TraceID)
	assert.Empty(t, segment.Links[0].Attributes)

	jsonStr, _ := MakeSegmentDocumentString(span, resource, nil, false, nil, false)

	assert.Contains(t, jsonStr, "links")
	assert.NotContains(t, jsonStr, "attributes")
	assert.Contains(t, jsonStr, convertedTraceID)
	assert.Contains(t, jsonStr, spanLink.SpanID().String())
}

func TestSpanLinkEmpty(t *testing.T) {
	spanName := "ProcessingMessage"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeOk, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil, false)

	assert.Empty(t, segment.Links)

	jsonStr, _ := MakeSegmentDocumentString(span, resource, nil, false, nil, false)

	assert.NotContains(t, jsonStr, "links")
}

func TestOldSpanLinkError(t *testing.T) {
	spanName := "ProcessingMessage"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeOk, "OK", attributes)

	const maxAge = 60 * 60 * 24 * 30
	ExpiredEpoch := time.Now().Unix() - maxAge - 1

	traceID := newTraceID()
	binary.BigEndian.PutUint32(traceID[0:4], uint32(ExpiredEpoch))

	spanLink := span.Links().AppendEmpty()
	spanLink.SetTraceID(traceID)
	spanLink.SetSpanID(newSegmentID())

	_, error1 := MakeSegment(span, resource, nil, false, nil, false)

	assert.Error(t, error1)

	_, error2 := MakeSegmentDocumentString(span, resource, nil, false, nil, false)

	assert.Error(t, error2)
}

func TestTwoSpanLinks(t *testing.T) {
	spanName := "ProcessingMessage"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeOk, "OK", attributes)

	traceID1 := newTraceID()

	spanLink1 := span.Links().AppendEmpty()
	spanLink1.SetTraceID(traceID1)
	spanLink1.SetSpanID(newSegmentID())
	spanLink1.Attributes().PutStr("myKey1", "ABC")

	traceID2 := newTraceID()

	spanLink2 := span.Links().AppendEmpty()
	spanLink2.SetTraceID(traceID2)
	spanLink2.SetSpanID(newSegmentID())
	spanLink2.Attributes().PutInt("myKey2", 1234)

	segment, _ := MakeSegment(span, resource, nil, false, nil, false)

	convertedTraceID1, _ := convertToAmazonTraceID(traceID1, false)
	convertedTraceID2, _ := convertToAmazonTraceID(traceID2, false)

	assert.Len(t, segment.Links, 2)
	assert.Equal(t, spanLink1.SpanID().String(), *segment.Links[0].SpanID)
	assert.Equal(t, convertedTraceID1, *segment.Links[0].TraceID)

	assert.Len(t, segment.Links[0].Attributes, 1)
	assert.Equal(t, "ABC", segment.Links[0].Attributes["myKey1"])

	assert.Equal(t, spanLink2.SpanID().String(), *segment.Links[1].SpanID)
	assert.Equal(t, convertedTraceID2, *segment.Links[1].TraceID)
	assert.Len(t, segment.Links[0].Attributes, 1)
	assert.Equal(t, int64(1234), segment.Links[1].Attributes["myKey2"])

	jsonStr, _ := MakeSegmentDocumentString(span, resource, nil, false, nil, false)

	assert.Contains(t, jsonStr, "attributes")
	assert.Contains(t, jsonStr, "links")
	assert.Contains(t, jsonStr, "myKey1")
	assert.Contains(t, jsonStr, "myKey2")
	assert.Contains(t, jsonStr, "ABC")
	assert.Contains(t, jsonStr, "1234")
	assert.Contains(t, jsonStr, convertedTraceID1)
	assert.Contains(t, jsonStr, convertedTraceID2)
}

func TestSpanLinkComplexAttributes(t *testing.T) {
	spanName := "ProcessingMessage"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeOk, "OK", attributes)

	spanLink := span.Links().AppendEmpty()
	spanLink.SetTraceID(newTraceID())
	spanLink.SetSpanID(newSegmentID())
	spanLink.Attributes().PutStr("myKey1", "myValue")
	spanLink.Attributes().PutBool("myKey2", true)
	spanLink.Attributes().PutInt("myKey3", 112233)
	spanLink.Attributes().PutDouble("myKey4", 3.1415)

	slice1 := spanLink.Attributes().PutEmptySlice("myKey5")
	slice1.AppendEmpty().SetStr("apple")
	slice1.AppendEmpty().SetStr("pear")
	slice1.AppendEmpty().SetStr("banana")

	slice2 := spanLink.Attributes().PutEmptySlice("myKey6")
	slice2.AppendEmpty().SetBool(true)
	slice2.AppendEmpty().SetBool(false)
	slice2.AppendEmpty().SetBool(false)
	slice2.AppendEmpty().SetBool(true)

	slice3 := spanLink.Attributes().PutEmptySlice("myKey7")
	slice3.AppendEmpty().SetInt(1234)
	slice3.AppendEmpty().SetInt(5678)
	slice3.AppendEmpty().SetInt(9012)

	slice4 := spanLink.Attributes().PutEmptySlice("myKey8")
	slice4.AppendEmpty().SetDouble(2.718)
	slice4.AppendEmpty().SetDouble(1.618)

	segment, _ := MakeSegment(span, resource, nil, false, nil, false)

	assert.Len(t, segment.Links, 1)
	assert.Len(t, segment.Links[0].Attributes, 8)

	assert.Equal(t, "myValue", segment.Links[0].Attributes["myKey1"])
	assert.Equal(t, true, segment.Links[0].Attributes["myKey2"])
	assert.Equal(t, int64(112233), segment.Links[0].Attributes["myKey3"])
	assert.Equal(t, 3.1415, segment.Links[0].Attributes["myKey4"])

	assert.Equal(t, "apple", segment.Links[0].Attributes["myKey5"].([]any)[0])
	assert.Equal(t, "pear", segment.Links[0].Attributes["myKey5"].([]any)[1])
	assert.Equal(t, "banana", segment.Links[0].Attributes["myKey5"].([]any)[2])

	assert.Equal(t, true, segment.Links[0].Attributes["myKey6"].([]any)[0])
	assert.Equal(t, false, segment.Links[0].Attributes["myKey6"].([]any)[1])
	assert.Equal(t, false, segment.Links[0].Attributes["myKey6"].([]any)[2])
	assert.Equal(t, true, segment.Links[0].Attributes["myKey6"].([]any)[0])

	assert.Equal(t, int64(1234), segment.Links[0].Attributes["myKey7"].([]any)[0])
	assert.Equal(t, int64(5678), segment.Links[0].Attributes["myKey7"].([]any)[1])
	assert.Equal(t, int64(9012), segment.Links[0].Attributes["myKey7"].([]any)[2])

	assert.Equal(t, 2.718, segment.Links[0].Attributes["myKey8"].([]any)[0])
	assert.Equal(t, 1.618, segment.Links[0].Attributes["myKey8"].([]any)[1])

	jsonStr, _ := MakeSegmentDocumentString(span, resource, nil, false, nil, false)

	assert.Contains(t, jsonStr, "links")

	assert.Contains(t, jsonStr, "myKey1")
	assert.Contains(t, jsonStr, "myValue")

	assert.Contains(t, jsonStr, "myKey2")
	assert.Contains(t, jsonStr, "true")

	assert.Contains(t, jsonStr, "myKey3")
	assert.Contains(t, jsonStr, "112233")

	assert.Contains(t, jsonStr, "myKey4")
	assert.Contains(t, jsonStr, "3.1415")

	assert.Contains(t, jsonStr, "myKey5")
	assert.Contains(t, jsonStr, "apple")
	assert.Contains(t, jsonStr, "pear")
	assert.Contains(t, jsonStr, "banana")

	assert.Contains(t, jsonStr, "myKey6")
	assert.Contains(t, jsonStr, "false")

	assert.Contains(t, jsonStr, "myKey7")
	assert.Contains(t, jsonStr, "1234")
	assert.Contains(t, jsonStr, "5678")
	assert.Contains(t, jsonStr, "9012")

	assert.Contains(t, jsonStr, "myKey8")
	assert.Contains(t, jsonStr, "2.718")
	assert.Contains(t, jsonStr, "1.618")
}
