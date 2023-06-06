// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/translator"

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestMessagingSimple(t *testing.T) {
	spanName := "ProcessingMessage"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeOk, "OK", attributes)

	span.Attributes().PutStr("messaging.operation", "process")
	span.Attributes().PutStr("messaging.system", "AmazonSQS")
	span.Attributes().PutStr("notMessaging", "myValue")

	segment, _ := MakeSegment(span, resource, nil, false, nil)

	assert.Equal(t, 2, len(segment.Messaging))
	assert.Equal(t, "process", segment.Messaging["operation"])
	assert.Equal(t, "AmazonSQS", segment.Messaging["system"])
	assert.Equal(t, "myValue", segment.Metadata["default"]["notMessaging"])
	assert.Equal(t, 0, len(segment.Annotations))

	jsonStr, _ := MakeSegmentDocumentString(span, resource, nil, false, nil)

	assert.True(t, strings.Contains(jsonStr, "messaging"))

	assert.True(t, strings.Contains(jsonStr, "operation"))
	assert.True(t, strings.Contains(jsonStr, "process"))

	assert.True(t, strings.Contains(jsonStr, "system"))
	assert.True(t, strings.Contains(jsonStr, "AmazonSQS"))

	assert.True(t, strings.Contains(jsonStr, "notMessaging"))
	assert.True(t, strings.Contains(jsonStr, "myValue"))
}

func TestMessagingEmpty(t *testing.T) {
	spanName := "ProcessingMessage"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeOk, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil)

	assert.Equal(t, 0, len(segment.Messaging))

	jsonStr, _ := MakeSegmentDocumentString(span, resource, nil, false, nil)

	assert.False(t, strings.Contains(jsonStr, "messaging"))
}

func TestMessagingComplex(t *testing.T) {
	spanName := "ProcessingMessage"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeOk, "OK", attributes)

	span.Attributes().PutStr("messaging.operation", "process")
	span.Attributes().PutStr("messaging.system", "AmazonSQS")
	span.Attributes().PutInt("messaging.message_count", 7)
	span.Attributes().PutInt("messaging.payload_size_bytes", 2048)
	span.Attributes().PutInt("messaging.payload_compressed_size_bytes", 1024)
	span.Attributes().PutStr("messaging.conversation_id", "MyConversationId")
	span.Attributes().PutStr("messaging.id", "452a7c7c7c7048c2f887f61572b18fc2")
	var slice = span.Attributes().PutEmptySlice("messaging.sliceData")
	slice.AppendEmpty().SetStr("alpha")
	slice.AppendEmpty().SetStr("beta")

	segment, _ := MakeSegment(span, resource, nil, false, nil)

	assert.Equal(t, 8, len(segment.Messaging))
	assert.Equal(t, "process", segment.Messaging["operation"])
	assert.Equal(t, "AmazonSQS", segment.Messaging["system"])
	assert.Equal(t, int64(7), segment.Messaging["message_count"])
	assert.Equal(t, int64(2048), segment.Messaging["payload_size_bytes"])
	assert.Equal(t, int64(1024), segment.Messaging["payload_compressed_size_bytes"])
	assert.Equal(t, "MyConversationId", segment.Messaging["conversation_id"])
	assert.Equal(t, "452a7c7c7c7048c2f887f61572b18fc2", segment.Messaging["id"])
	assert.Equal(t, "alpha", segment.Messaging["sliceData"].([]interface{})[0])
	assert.Equal(t, "beta", segment.Messaging["sliceData"].([]interface{})[1])

	jsonStr, _ := MakeSegmentDocumentString(span, resource, nil, false, nil)

	assert.True(t, strings.Contains(jsonStr, "messaging"))

	assert.True(t, strings.Contains(jsonStr, "operation"))
	assert.True(t, strings.Contains(jsonStr, "process"))

	assert.True(t, strings.Contains(jsonStr, "system"))
	assert.True(t, strings.Contains(jsonStr, "AmazonSQS"))

	assert.True(t, strings.Contains(jsonStr, "message_count"))
	assert.True(t, strings.Contains(jsonStr, "7"))

	assert.True(t, strings.Contains(jsonStr, "payload_size_bytes"))
	assert.True(t, strings.Contains(jsonStr, "2048"))

	assert.True(t, strings.Contains(jsonStr, "payload_compressed_size_bytes"))
	assert.True(t, strings.Contains(jsonStr, "1024"))

	assert.True(t, strings.Contains(jsonStr, "conversation_id"))
	assert.True(t, strings.Contains(jsonStr, "MyConversationId"))

	assert.True(t, strings.Contains(jsonStr, "id"))
	assert.True(t, strings.Contains(jsonStr, "452a7c7c7c7048c2f887f61572b18fc2"))

	assert.True(t, strings.Contains(jsonStr, "sliceData"))
	assert.True(t, strings.Contains(jsonStr, "alpha"))
	assert.True(t, strings.Contains(jsonStr, "beta"))
}

func TestMessagingWithIndexedAttributes(t *testing.T) {
	spanName := "ProcessingMessage"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeOk, "OK", attributes)

	span.Attributes().PutStr("messaging.operation", "process")
	span.Attributes().PutStr("messaging.system", "AmazonSQS")

	segment, _ := MakeSegment(span, resource, []string{"messaging.system"}, false, nil)

	assert.Equal(t, 2, len(segment.Messaging))
	assert.Equal(t, "process", segment.Messaging["operation"])
	assert.Equal(t, "AmazonSQS", segment.Messaging["system"])
	assert.Equal(t, 1, len(segment.Annotations))
	assert.Equal(t, "AmazonSQS", segment.Annotations["messaging_system"])

	jsonStr, _ := MakeSegmentDocumentString(span, resource, []string{"messaging.system"}, false, nil)

	assert.True(t, strings.Contains(jsonStr, "messaging"))

	assert.True(t, strings.Contains(jsonStr, "operation"))
	assert.True(t, strings.Contains(jsonStr, "process"))

	assert.True(t, strings.Contains(jsonStr, "system"))
	assert.True(t, strings.Contains(jsonStr, "AmazonSQS"))
	assert.True(t, strings.Contains(jsonStr, "annotations"))
	assert.True(t, strings.Contains(jsonStr, "messaging_system"))
}
