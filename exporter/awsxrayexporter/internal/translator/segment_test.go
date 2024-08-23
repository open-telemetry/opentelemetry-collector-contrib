// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.8.0"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

const (
	resourceStringKey = "string.key"
	resourceIntKey    = "int.key"
	resourceDoubleKey = "double.key"
	resourceBoolKey   = "bool.key"
	resourceMapKey    = "map.key"
	resourceArrayKey  = "array.key"
)

var (
	testWriters = newWriterPool(2048)
)

func TestClientSpanWithRpcAwsSdkClientAttributes(t *testing.T) {
	spanName := "AmazonDynamoDB.getItem"
	parentSpanID := newSegmentID()
	user := "testingT"
	attributes := make(map[string]any)
	attributes[conventions.AttributeHTTPMethod] = "POST"
	attributes[conventions.AttributeHTTPScheme] = "https"
	attributes[conventions.AttributeHTTPHost] = "dynamodb.us-east-1.amazonaws.com"
	attributes[conventions.AttributeHTTPTarget] = "/"
	attributes[conventions.AttributeRPCService] = "DynamoDB"
	attributes[conventions.AttributeRPCMethod] = "GetItem"
	attributes[conventions.AttributeRPCSystem] = "aws-api"
	attributes[awsxray.AWSRequestIDAttribute] = "18BO1FEPJSSAOGNJEDPTPCMIU7VV4KQNSO5AEMVJF66Q9ASUAAJG"
	attributes[awsxray.AWSTableNameAttribute] = "otel-dev-Testing"
	resource := constructDefaultResource()
	span := constructClientSpan(parentSpanID, spanName, 0, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil, false)
	assert.Equal(t, "DynamoDB", *segment.Name)
	assert.Equal(t, conventions.AttributeCloudProviderAWS, *segment.Namespace)
	assert.Equal(t, "GetItem", *segment.AWS.Operation)
	assert.Equal(t, "subsegment", *segment.Type)

	jsonStr, err := MakeSegmentDocumentString(span, resource, nil, false, nil, false)

	assert.NotNil(t, jsonStr)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(jsonStr, "DynamoDB"))
	assert.True(t, strings.Contains(jsonStr, "GetItem"))
	assert.False(t, strings.Contains(jsonStr, user))
	assert.False(t, strings.Contains(jsonStr, "user"))
}

func TestClientSpanWithLegacyAwsSdkClientAttributes(t *testing.T) {
	spanName := "AmazonDynamoDB.getItem"
	parentSpanID := newSegmentID()
	user := "testingT"
	attributes := make(map[string]any)
	attributes[conventions.AttributeHTTPMethod] = "POST"
	attributes[conventions.AttributeHTTPScheme] = "https"
	attributes[conventions.AttributeHTTPHost] = "dynamodb.us-east-1.amazonaws.com"
	attributes[conventions.AttributeHTTPTarget] = "/"
	attributes[awsxray.AWSServiceAttribute] = "DynamoDB"
	attributes[conventions.AttributeRPCMethod] = "IncorrectAWSSDKOperation"
	attributes[awsxray.AWSOperationAttribute] = "GetItem"
	attributes[awsxray.AWSRequestIDAttribute] = "18BO1FEPJSSAOGNJEDPTPCMIU7VV4KQNSO5AEMVJF66Q9ASUAAJG"
	attributes[awsxray.AWSTableNameAttribute] = "otel-dev-Testing"
	resource := constructDefaultResource()
	span := constructClientSpan(parentSpanID, spanName, 0, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil, false)
	assert.Equal(t, "DynamoDB", *segment.Name)
	assert.Equal(t, conventions.AttributeCloudProviderAWS, *segment.Namespace)
	assert.Equal(t, "GetItem", *segment.AWS.Operation)
	assert.Equal(t, "subsegment", *segment.Type)

	jsonStr, err := MakeSegmentDocumentString(span, resource, nil, false, nil, false)

	assert.NotNil(t, jsonStr)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(jsonStr, "DynamoDB"))
	assert.True(t, strings.Contains(jsonStr, "GetItem"))
	assert.False(t, strings.Contains(jsonStr, user))
	assert.False(t, strings.Contains(jsonStr, "user"))
}

func TestClientSpanWithPeerService(t *testing.T) {
	spanName := "AmazonDynamoDB.getItem"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	attributes[conventions.AttributeHTTPMethod] = "POST"
	attributes[conventions.AttributeHTTPScheme] = "https"
	attributes[conventions.AttributeHTTPHost] = "dynamodb.us-east-1.amazonaws.com"
	attributes[conventions.AttributeHTTPTarget] = "/"
	attributes[conventions.AttributePeerService] = "cats-table"
	attributes[awsxray.AWSServiceAttribute] = "DynamoDB"
	attributes[awsxray.AWSOperationAttribute] = "GetItem"
	attributes[awsxray.AWSRequestIDAttribute] = "18BO1FEPJSSAOGNJEDPTPCMIU7VV4KQNSO5AEMVJF66Q9ASUAAJG"
	attributes[awsxray.AWSTableNameAttribute] = "otel-dev-Testing"
	resource := constructDefaultResource()
	span := constructClientSpan(parentSpanID, spanName, 0, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil, false)
	assert.Equal(t, "cats-table", *segment.Name)
}

func TestServerSpanWithInternalServerError(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	errorMessage := "java.lang.NullPointerException"
	userAgent := "PostmanRuntime/7.21.0"
	enduser := "go.tester@example.com"
	attributes := make(map[string]any)
	attributes[conventions.AttributeHTTPMethod] = "POST"
	attributes[conventions.AttributeHTTPURL] = "https://api.example.org/api/locations"
	attributes[conventions.AttributeHTTPTarget] = "/api/locations"
	attributes[conventions.AttributeHTTPStatusCode] = 500
	attributes["http.status_text"] = "java.lang.NullPointerException"
	attributes[conventions.AttributeHTTPUserAgent] = userAgent
	attributes[conventions.AttributeEnduserID] = enduser
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, errorMessage, attributes)
	timeEvents := constructTimedEventsWithSentMessageEvent(span.StartTimestamp())
	timeEvents.CopyTo(span.Events())

	segment, _ := MakeSegment(span, resource, nil, false, nil, false)

	assert.NotNil(t, segment)
	assert.NotNil(t, segment.Cause)
	assert.Equal(t, "signup_aggregator", *segment.Name)
	assert.True(t, *segment.Fault)
}

func TestServerSpanWithThrottle(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	errorMessage := "java.lang.NullPointerException"
	userAgent := "PostmanRuntime/7.21.0"
	enduser := "go.tester@example.com"
	attributes := make(map[string]any)
	attributes[conventions.AttributeHTTPMethod] = "POST"
	attributes[conventions.AttributeHTTPURL] = "https://api.example.org/api/locations"
	attributes[conventions.AttributeHTTPTarget] = "/api/locations"
	attributes[conventions.AttributeHTTPStatusCode] = 429
	attributes["http.status_text"] = "java.lang.NullPointerException"
	attributes[conventions.AttributeHTTPUserAgent] = userAgent
	attributes[conventions.AttributeEnduserID] = enduser
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, errorMessage, attributes)
	timeEvents := constructTimedEventsWithSentMessageEvent(span.StartTimestamp())
	timeEvents.CopyTo(span.Events())

	segment, _ := MakeSegment(span, resource, nil, false, nil, false)

	assert.NotNil(t, segment)
	assert.NotNil(t, segment.Cause)
	assert.Equal(t, "signup_aggregator", *segment.Name)
	assert.False(t, *segment.Fault)
	assert.True(t, *segment.Error)
	assert.True(t, *segment.Throttle)
}

func TestServerSpanNoParentId(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := pcommon.NewSpanIDEmpty()
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeOk, "OK", nil)

	segment, _ := MakeSegment(span, resource, nil, false, nil, false)

	assert.Empty(t, segment.ParentID)
}

func TestSpanNoParentId(t *testing.T) {
	span := ptrace.NewSpan()
	span.SetName("my-topic send")
	span.SetTraceID(newTraceID())
	span.SetSpanID(newSegmentID())
	span.SetParentSpanID(pcommon.NewSpanIDEmpty())
	span.SetKind(ptrace.SpanKindProducer)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(10)))
	resource := pcommon.NewResource()
	segment, _ := MakeSegment(span, resource, nil, false, nil, false)

	assert.Empty(t, segment.ParentID)
	assert.Nil(t, segment.Type)
}

func TestSpanWithNoStatus(t *testing.T) {
	span := ptrace.NewSpan()
	span.SetTraceID(newTraceID())
	span.SetSpanID(newSegmentID())
	span.SetParentSpanID(newSegmentID())
	span.SetKind(ptrace.SpanKindServer)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(10)))

	resource := pcommon.NewResource()
	segment, _ := MakeSegment(span, resource, nil, false, nil, false)
	assert.NotNil(t, segment)
}

func TestClientSpanWithDbComponent(t *testing.T) {
	spanName := "call update_user_preference( ?, ?, ? )"
	parentSpanID := newSegmentID()
	enterpriseAppID := "25F2E73B-4769-4C79-9DF3-7EBE85D571EA"
	attributes := make(map[string]any)
	attributes[conventions.AttributeDBSystem] = "mysql"
	attributes[conventions.AttributeDBName] = "customers"
	attributes[conventions.AttributeDBStatement] = spanName
	attributes[conventions.AttributeDBUser] = "userprefsvc"
	attributes[conventions.AttributeDBConnectionString] = "jdbc:mysql://db.dev.example.com:3306"
	attributes[conventions.AttributeNetPeerName] = "db.dev.example.com"
	attributes[conventions.AttributeNetPeerPort] = "3306"
	attributes["enterprise.app.id"] = enterpriseAppID
	resource := constructDefaultResource()
	span := constructClientSpan(parentSpanID, spanName, ptrace.StatusCodeUnset, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil, false)

	assert.NotNil(t, segment)
	assert.NotNil(t, segment.SQL)
	assert.NotNil(t, segment.Service)
	assert.NotNil(t, segment.AWS)
	assert.NotNil(t, segment.Metadata)
	assert.Equal(t, 0, len(segment.Annotations))
	assert.Equal(t, enterpriseAppID, segment.Metadata["default"]["enterprise.app.id"])
	assert.Nil(t, segment.Cause)
	assert.Nil(t, segment.HTTP)
	assert.Equal(t, "customers@db.dev.example.com", *segment.Name)
	assert.False(t, *segment.Fault)
	assert.False(t, *segment.Error)
	assert.Equal(t, "remote", *segment.Namespace)

	w := testWriters.borrow()
	require.NoError(t, w.Encode(segment))
	jsonStr := w.String()
	testWriters.release(w)
	assert.True(t, strings.Contains(jsonStr, spanName))
	assert.True(t, strings.Contains(jsonStr, enterpriseAppID))
}

func TestClientSpanWithHttpHost(t *testing.T) {
	spanName := "GET /"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	attributes[conventions.AttributeHTTPMethod] = "GET"
	attributes[conventions.AttributeHTTPScheme] = "https"
	attributes[conventions.AttributeNetPeerIP] = "2607:f8b0:4000:80c::2004"
	attributes[conventions.AttributeNetPeerPort] = "9443"
	attributes[conventions.AttributeHTTPTarget] = "/"
	attributes[conventions.AttributeHTTPHost] = "foo.com"
	attributes[conventions.AttributeNetPeerName] = "bar.com"
	resource := constructDefaultResource()
	span := constructClientSpan(parentSpanID, spanName, ptrace.StatusCodeUnset, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, "foo.com", *segment.Name)
}

func TestClientSpanWithoutHttpHost(t *testing.T) {
	spanName := "GET /"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	attributes[conventions.AttributeHTTPMethod] = "GET"
	attributes[conventions.AttributeHTTPScheme] = "https"
	attributes[conventions.AttributeNetPeerIP] = "2607:f8b0:4000:80c::2004"
	attributes[conventions.AttributeNetPeerPort] = "9443"
	attributes[conventions.AttributeHTTPTarget] = "/"
	attributes[conventions.AttributeNetPeerName] = "bar.com"
	resource := constructDefaultResource()
	span := constructClientSpan(parentSpanID, spanName, ptrace.StatusCodeUnset, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, "bar.com", *segment.Name)
}

func TestClientSpanWithRpcHost(t *testing.T) {
	spanName := "GET /com.foo.AnimalService/GetCats"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	attributes[conventions.AttributeHTTPMethod] = "GET"
	attributes[conventions.AttributeHTTPScheme] = "https"
	attributes[conventions.AttributeNetPeerIP] = "2607:f8b0:4000:80c::2004"
	attributes[conventions.AttributeNetPeerPort] = "9443"
	attributes[conventions.AttributeHTTPTarget] = "/com.foo.AnimalService/GetCats"
	attributes[conventions.AttributeRPCService] = "com.foo.AnimalService"
	attributes[conventions.AttributeNetPeerName] = "bar.com"
	resource := constructDefaultResource()
	span := constructClientSpan(parentSpanID, spanName, ptrace.StatusCodeUnset, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, "com.foo.AnimalService", *segment.Name)
}

func TestSpanWithInvalidTraceId(t *testing.T) {
	spanName := "platformapi.widgets.searchWidgets"
	attributes := make(map[string]any)
	attributes[conventions.AttributeHTTPMethod] = "GET"
	attributes[conventions.AttributeHTTPScheme] = "ipv6"
	attributes[conventions.AttributeNetPeerIP] = "2607:f8b0:4000:80c::2004"
	attributes[conventions.AttributeNetPeerPort] = "9443"
	attributes[conventions.AttributeHTTPTarget] = spanName
	resource := constructDefaultResource()
	span := constructClientSpan(pcommon.NewSpanIDEmpty(), spanName, ptrace.StatusCodeUnset, "OK", attributes)
	timeEvents := constructTimedEventsWithSentMessageEvent(span.StartTimestamp())
	timeEvents.CopyTo(span.Events())
	traceID := span.TraceID()
	traceID[0] = 0x11
	span.SetTraceID(traceID)

	_, err := MakeSegmentDocumentString(span, resource, nil, false, nil, false)

	assert.Error(t, err)
}

func TestSpanWithExpiredTraceId(t *testing.T) {
	// First Build expired TraceId
	const maxAge = 60 * 60 * 24 * 30
	ExpiredEpoch := time.Now().Unix() - maxAge - 1

	tempTraceID := newTraceID()
	binary.BigEndian.PutUint32(tempTraceID[0:4], uint32(ExpiredEpoch))

	_, err := convertToAmazonTraceID(tempTraceID, false)
	assert.Error(t, err)
}

func TestSpanWithInvalidTraceIdWithoutTimestampValidation(t *testing.T) {
	spanName := "ABC.payment"
	parentSpanID := newSegmentID()
	user := "testingT"
	attributes := make(map[string]any)
	attributes[conventions.AttributeHTTPMethod] = "POST"
	attributes[conventions.AttributeHTTPScheme] = "https"
	attributes[conventions.AttributeHTTPHost] = "payment.amazonaws.com"
	attributes[conventions.AttributeHTTPTarget] = "/"
	attributes[conventions.AttributeRPCService] = "ABC"
	attributes[awsRemoteService] = "ProducerService"

	resource := constructDefaultResource()
	span := constructProducerSpan(parentSpanID, spanName, 0, "OK", attributes)
	traceID := span.TraceID()
	traceID[0] = 0x11
	span.SetTraceID(traceID)

	segment, err := MakeSegment(span, resource, nil, false, nil, true)
	require.NoError(t, err)
	assert.Equal(t, "ProducerService", *segment.Name)
	assert.Equal(t, "subsegment", *segment.Type)

	jsonStr, err := MakeSegmentDocumentString(span, resource, nil, false, nil, true)

	require.NoError(t, err)
	assert.NotNil(t, jsonStr)
	assert.True(t, strings.Contains(jsonStr, "ProducerService"))
	assert.False(t, strings.Contains(jsonStr, user))
	assert.False(t, strings.Contains(jsonStr, "user"))
}

func TestSpanWithExpiredTraceIdWithoutTimestampValidation(t *testing.T) {
	// First Build expired TraceId
	const maxAge = 60 * 60 * 24 * 30
	ExpiredEpoch := time.Now().Unix() - maxAge - 1

	tempTraceID := newTraceID()
	binary.BigEndian.PutUint32(tempTraceID[0:4], uint32(ExpiredEpoch))

	amazonTraceID, err := convertToAmazonTraceID(tempTraceID, true)
	assert.NoError(t, err)
	expectedTraceID := "1-" + fmt.Sprintf("%x", tempTraceID[0:4]) + "-" + fmt.Sprintf("%x", tempTraceID[4:16])
	assert.Equal(t, expectedTraceID, amazonTraceID)
}

func TestFixSegmentName(t *testing.T) {
	validName := "EP @ test_15.testing-d\u00F6main.org#GO"
	fixedName := fixSegmentName(validName)
	assert.Equal(t, validName, fixedName)
	invalidName := "<subDomain>.example.com,1413"
	fixedName = fixSegmentName(invalidName)
	assert.Equal(t, "subDomain.example.com1413", fixedName)
	fullyInvalidName := "<>"
	fixedName = fixSegmentName(fullyInvalidName)
	assert.Equal(t, defaultSegmentName, fixedName)
}

func TestFixAnnotationKey(t *testing.T) {
	err := featuregate.GlobalRegistry().Set("exporter.xray.allowDot", false)
	assert.Nil(t, err)

	validKey := "Key_1"
	fixedKey := fixAnnotationKey(validKey)
	assert.Equal(t, validKey, fixedKey)
	validDotKey := "Key.1"
	fixedDotKey := fixAnnotationKey(validDotKey)
	assert.Equal(t, "Key_1", fixedDotKey)
	invalidKey := "Key@1"
	fixedKey = fixAnnotationKey(invalidKey)
	assert.Equal(t, "Key_1", fixedKey)
}

func TestFixAnnotationKeyWithAllowDot(t *testing.T) {
	err := featuregate.GlobalRegistry().Set("exporter.xray.allowDot", true)
	assert.Nil(t, err)

	validKey := "Key_1"
	fixedKey := fixAnnotationKey(validKey)
	assert.Equal(t, validKey, fixedKey)
	validDotKey := "Key.1"
	fixedDotKey := fixAnnotationKey(validDotKey)
	assert.Equal(t, validDotKey, fixedDotKey)
	invalidKey := "Key@1"
	fixedKey = fixAnnotationKey(invalidKey)
	assert.Equal(t, "Key_1", fixedKey)
}

func TestServerSpanWithNilAttributes(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)
	timeEvents := constructTimedEventsWithSentMessageEvent(span.StartTimestamp())
	timeEvents.CopyTo(span.Events())
	pcommon.NewMap().CopyTo(span.Attributes())

	segment, _ := MakeSegment(span, resource, nil, false, nil, false)

	assert.NotNil(t, segment)
	assert.NotNil(t, segment.Cause)
	assert.Equal(t, "signup_aggregator", *segment.Name)
	assert.True(t, *segment.Fault)
}

func TestSpanWithAttributesDefaultNotIndexed(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	attributes["attr1@1"] = "val1"
	attributes["attr2@2"] = "val2"
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, 0, len(segment.Annotations))
	assert.Equal(t, "val1", segment.Metadata["default"]["attr1@1"])
	assert.Equal(t, "val2", segment.Metadata["default"]["attr2@2"])
	assert.Equal(t, "string", segment.Metadata["default"]["otel.resource.string.key"])
	assert.Equal(t, int64(10), segment.Metadata["default"]["otel.resource.int.key"])
	assert.Equal(t, 5.0, segment.Metadata["default"]["otel.resource.double.key"])
	assert.Equal(t, true, segment.Metadata["default"]["otel.resource.bool.key"])
	expectedMap := make(map[string]any)
	expectedMap["key1"] = int64(1)
	expectedMap["key2"] = "value"
	assert.Equal(t, expectedMap, segment.Metadata["default"]["otel.resource.map.key"])
	expectedArr := []any{"foo", "bar"}
	assert.Equal(t, expectedArr, segment.Metadata["default"]["otel.resource.array.key"])
}

func TestSpanWithResourceNotStoredIfSubsegment(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	attributes["attr1@1"] = "val1"
	attributes["attr2@2"] = "val2"
	resource := constructDefaultResource()
	span := constructClientSpan(parentSpanID, spanName, ptrace.StatusCodeError, "ERROR", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, 0, len(segment.Annotations))
	assert.Equal(t, "val1", segment.Metadata["default"]["attr1@1"])
	assert.Equal(t, "val2", segment.Metadata["default"]["attr2@2"])
	assert.Nil(t, segment.Metadata["default"]["otel.resource.string.key"])
	assert.Nil(t, segment.Metadata["default"]["otel.resource.int.key"])
	assert.Nil(t, segment.Metadata["default"]["otel.resource.double.key"])
	assert.Nil(t, segment.Metadata["default"]["otel.resource.bool.key"])
	assert.Nil(t, segment.Metadata["default"]["otel.resource.map.key"])
	assert.Nil(t, segment.Metadata["default"]["otel.resource.array.key"])
}

func TestSpanWithAttributesPartlyIndexed(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	attributes["attr1@1"] = "val1"
	attributes["attr2@2"] = "val2"
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{"attr1@1", "not_exist"}, false, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, 1, len(segment.Annotations))
	assert.Equal(t, "val1", segment.Annotations["attr1_1"])
	assert.Equal(t, "val2", segment.Metadata["default"]["attr2@2"])
}

func TestSpanWithAnnotationsAttribute(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	attributes["attr1@1"] = "val1"
	attributes["attr2@2"] = "val2"
	attributes[awsxray.AWSXraySegmentAnnotationsAttribute] = []string{"attr2@2", "not_exist"}
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, 1, len(segment.Annotations))
	assert.Equal(t, "val2", segment.Annotations["attr2_2"])
	assert.Equal(t, "val1", segment.Metadata["default"]["attr1@1"])
}

func TestSpanWithAttributesAllIndexed(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	attributes["attr1@1"] = "val1"
	attributes["attr2@2"] = "val2"
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeOk, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{"attr1@1", "not_exist"}, true, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, "val1", segment.Annotations["attr1_1"])
	assert.Equal(t, "val2", segment.Annotations["attr2_2"])
}

func TestSpanWithAttributesSegmentMetadata(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	attributes["attr1@1"] = "val1"
	attributes[awsxray.AWSXraySegmentMetadataAttributePrefix+"default"] = "{\"custom_key\": \"custom_value\"}"
	attributes[awsxray.AWSXraySegmentMetadataAttributePrefix+"http"] = "{\"connection\":{\"reused\":false,\"was_idle\":false}}"
	attributes[awsxray.AWSXraySegmentMetadataAttributePrefix+"non-xray-sdk"] = "retain-value"
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, 0, len(segment.Annotations))
	assert.Equal(t, 2, len(segment.Metadata))
	assert.Equal(t, "val1", segment.Metadata["default"]["attr1@1"])
	assert.Equal(t, "custom_value", segment.Metadata["default"]["custom_key"])
	assert.Equal(t, "retain-value", segment.Metadata["default"][awsxray.AWSXraySegmentMetadataAttributePrefix+"non-xray-sdk"])
	assert.Nil(t, segment.Metadata["default"][awsxray.AWSXraySegmentMetadataAttributePrefix+"default"])
	assert.Nil(t, segment.Metadata["default"][awsxray.AWSXraySegmentMetadataAttributePrefix+"http"])
	assert.Equal(t, map[string]any{
		"reused":   false,
		"was_idle": false,
	}, segment.Metadata["http"]["connection"])
}

func TestResourceAttributesCanBeIndexed(t *testing.T) {
	err := featuregate.GlobalRegistry().Set("exporter.xray.allowDot", false)
	assert.Nil(t, err)

	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)
	segment, _ := MakeSegment(span, resource, []string{
		"otel.resource.string.key",
		"otel.resource.int.key",
		"otel.resource.double.key",
		"otel.resource.bool.key",
		"otel.resource.map.key",
		"otel.resource.array.key",
	}, false, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, 4, len(segment.Annotations))
	assert.Equal(t, "string", segment.Annotations["otel_resource_string_key"])
	assert.Equal(t, int64(10), segment.Annotations["otel_resource_int_key"])
	assert.Equal(t, 5.0, segment.Annotations["otel_resource_double_key"])
	assert.Equal(t, true, segment.Annotations["otel_resource_bool_key"])
	expectedMap := make(map[string]any)
	expectedMap["key1"] = int64(1)
	expectedMap["key2"] = "value"
	// Maps and arrays are not supported for annotations so still in metadata.
	assert.Equal(t, expectedMap, segment.Metadata["default"]["otel.resource.map.key"])
	expectedArr := []any{"foo", "bar"}
	assert.Equal(t, expectedArr, segment.Metadata["default"]["otel.resource.array.key"])
}

func TestResourceAttributesCanBeIndexedWithAllowDot(t *testing.T) {
	err := featuregate.GlobalRegistry().Set("exporter.xray.allowDot", true)
	assert.Nil(t, err)

	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)
	segment, _ := MakeSegment(span, resource, []string{
		"otel.resource.string.key",
		"otel.resource.int.key",
		"otel.resource.double.key",
		"otel.resource.bool.key",
		"otel.resource.map.key",
		"otel.resource.array.key",
	}, false, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, 4, len(segment.Annotations))
	assert.Equal(t, "string", segment.Annotations["otel.resource.string.key"])
	assert.Equal(t, int64(10), segment.Annotations["otel.resource.int.key"])
	assert.Equal(t, 5.0, segment.Annotations["otel.resource.double.key"])
	assert.Equal(t, true, segment.Annotations["otel.resource.bool.key"])
	expectedMap := make(map[string]any)
	expectedMap["key1"] = int64(1)
	expectedMap["key2"] = "value"
	// Maps and arrays are not supported for annotations so still in metadata.
	assert.Equal(t, expectedMap, segment.Metadata["default"]["otel.resource.map.key"])
	expectedArr := []any{"foo", "bar"}
	assert.Equal(t, expectedArr, segment.Metadata["default"]["otel.resource.array.key"])
}

func TestResourceAttributesNotIndexedIfSubsegment(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	resource := constructDefaultResource()
	span := constructClientSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{
		"otel.resource.string.key",
		"otel.resource.int.key",
		"otel.resource.double.key",
		"otel.resource.bool.key",
		"otel.resource.map.key",
		"otel.resource.array.key",
	}, false, nil, false)

	assert.NotNil(t, segment)
	assert.Empty(t, segment.Annotations)
	assert.Empty(t, segment.Metadata)
}

func TestSpanWithSpecialAttributesAsListed(t *testing.T) {
	err := featuregate.GlobalRegistry().Set("exporter.xray.allowDot", false)
	assert.Nil(t, err)

	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	attributes[awsxray.AWSOperationAttribute] = "aws_operation_val"
	attributes[conventions.AttributeRPCMethod] = "rpc_method_val"
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{awsxray.AWSOperationAttribute, conventions.AttributeRPCMethod}, false, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, 2, len(segment.Annotations))
	assert.Equal(t, "aws_operation_val", segment.Annotations["aws_operation"])
	assert.Equal(t, "rpc_method_val", segment.Annotations["rpc_method"])
}

func TestSpanWithSpecialAttributesAsListedWithAllowDot(t *testing.T) {
	err := featuregate.GlobalRegistry().Set("exporter.xray.allowDot", true)
	assert.Nil(t, err)

	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	attributes[awsxray.AWSOperationAttribute] = "aws_operation_val"
	attributes[conventions.AttributeRPCMethod] = "rpc_method_val"
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{awsxray.AWSOperationAttribute, conventions.AttributeRPCMethod}, false, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, 2, len(segment.Annotations))
	assert.Equal(t, "aws_operation_val", segment.Annotations[awsxray.AWSOperationAttribute])
	assert.Equal(t, "rpc_method_val", segment.Annotations[conventions.AttributeRPCMethod])
}

func TestSpanWithSpecialAttributesAsListedAndIndexAll(t *testing.T) {
	err := featuregate.GlobalRegistry().Set("exporter.xray.allowDot", false)
	assert.Nil(t, err)

	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	attributes[awsxray.AWSOperationAttribute] = "aws_operation_val"
	attributes[conventions.AttributeRPCMethod] = "rpc_method_val"
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{awsxray.AWSOperationAttribute, conventions.AttributeRPCMethod}, true, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, "aws_operation_val", segment.Annotations["aws_operation"])
	assert.Equal(t, "rpc_method_val", segment.Annotations["rpc_method"])
}

func TestSpanWithSpecialAttributesAsListedAndIndexAllWithAllowDot(t *testing.T) {
	err := featuregate.GlobalRegistry().Set("exporter.xray.allowDot", true)
	assert.Nil(t, err)

	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	attributes[awsxray.AWSOperationAttribute] = "aws_operation_val"
	attributes[conventions.AttributeRPCMethod] = "rpc_method_val"
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{awsxray.AWSOperationAttribute, conventions.AttributeRPCMethod}, true, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, "aws_operation_val", segment.Annotations[awsxray.AWSOperationAttribute])
	assert.Equal(t, "rpc_method_val", segment.Annotations[conventions.AttributeRPCMethod])
}

func TestSpanWithSpecialAttributesNotListedAndIndexAll(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	attributes[awsxray.AWSOperationAttribute] = "val1"
	attributes[conventions.AttributeRPCMethod] = "val2"
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, true, nil, false)

	assert.NotNil(t, segment)
	assert.Nil(t, segment.Annotations["aws_operation"])
	assert.Nil(t, segment.Annotations["rpc_method"])
}

func TestOriginNotAws(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	resource := pcommon.NewResource()
	attrs := resource.Attributes()
	attrs.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderGCP)
	attrs.PutStr(conventions.AttributeHostID, "instance-123")
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false, nil, false)

	assert.NotNil(t, segment)
	assert.Nil(t, segment.Origin)
}

func TestOriginEc2(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	resource := pcommon.NewResource()
	attrs := resource.Attributes()
	attrs.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.PutStr(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSEC2)
	attrs.PutStr(conventions.AttributeHostID, "instance-123")
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, OriginEC2, *segment.Origin)
}

func TestOriginEcs(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	resource := pcommon.NewResource()
	attrs := resource.Attributes()
	attrs.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.PutStr(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSECS)
	attrs.PutStr(conventions.AttributeHostID, "instance-123")
	attrs.PutStr(conventions.AttributeContainerName, "container-123")
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, OriginECS, *segment.Origin)
}

func TestOriginEcsEc2(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	resource := pcommon.NewResource()
	attrs := resource.Attributes()
	attrs.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.PutStr(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSECS)
	attrs.PutStr(conventions.AttributeAWSECSLaunchtype, conventions.AttributeAWSECSLaunchtypeEC2)
	attrs.PutStr(conventions.AttributeHostID, "instance-123")
	attrs.PutStr(conventions.AttributeContainerName, "container-123")
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, OriginECSEC2, *segment.Origin)
}

func TestOriginEcsFargate(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	resource := pcommon.NewResource()
	attrs := resource.Attributes()
	attrs.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.PutStr(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSECS)
	attrs.PutStr(conventions.AttributeAWSECSLaunchtype, conventions.AttributeAWSECSLaunchtypeFargate)
	attrs.PutStr(conventions.AttributeHostID, "instance-123")
	attrs.PutStr(conventions.AttributeContainerName, "container-123")
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, OriginECSFargate, *segment.Origin)
}

func TestOriginEb(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	resource := pcommon.NewResource()
	attrs := resource.Attributes()
	attrs.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.PutStr(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSElasticBeanstalk)
	attrs.PutStr(conventions.AttributeHostID, "instance-123")
	attrs.PutStr(conventions.AttributeContainerName, "container-123")
	attrs.PutStr(conventions.AttributeServiceInstanceID, "service-123")
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, OriginEB, *segment.Origin)
}

func TestOriginEks(t *testing.T) {
	instanceID := "i-00f7c0bcb26da2a99"
	containerName := "signup_aggregator-x82ufje83"
	containerID := "0123456789A"
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	resource := pcommon.NewResource()
	attrs := resource.Attributes()
	attrs.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.PutStr(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSEKS)
	attrs.PutStr(conventions.AttributeCloudAccountID, "123456789")
	attrs.PutStr(conventions.AttributeCloudAvailabilityZone, "us-east-1c")
	attrs.PutStr(conventions.AttributeContainerImageName, "otel/signupaggregator")
	attrs.PutStr(conventions.AttributeContainerImageTag, "v1")
	attrs.PutStr(conventions.AttributeK8SClusterName, "production")
	attrs.PutStr(conventions.AttributeK8SNamespaceName, "default")
	attrs.PutStr(conventions.AttributeK8SDeploymentName, "signup_aggregator")
	attrs.PutStr(conventions.AttributeK8SPodName, "my-deployment-65dcf7d447-ddjnl")
	attrs.PutStr(conventions.AttributeContainerName, containerName)
	attrs.PutStr(conventions.AttributeContainerID, containerID)
	attrs.PutStr(conventions.AttributeHostID, instanceID)
	attrs.PutStr(conventions.AttributeHostType, "m5.xlarge")
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, OriginEKS, *segment.Origin)
}

func TestOriginAppRunner(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	resource := pcommon.NewResource()
	attrs := resource.Attributes()
	attrs.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.PutStr(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSAppRunner)
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, OriginAppRunner, *segment.Origin)
}

func TestOriginBlank(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	resource := pcommon.NewResource()
	attrs := resource.Attributes()
	attrs.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false, nil, false)

	assert.NotNil(t, segment)
	assert.Nil(t, segment.Origin)
}

func TestOriginPrefersInfraService(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	resource := pcommon.NewResource()
	attrs := resource.Attributes()
	attrs.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.PutStr(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSEC2)
	attrs.PutStr(conventions.AttributeK8SClusterName, "cluster-123")
	attrs.PutStr(conventions.AttributeHostID, "instance-123")
	attrs.PutStr(conventions.AttributeContainerName, "container-123")
	attrs.PutStr(conventions.AttributeServiceInstanceID, "service-123")
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, OriginEC2, *segment.Origin)
}

func TestFilteredAttributesMetadata(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	resource := pcommon.NewResource()

	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	attrs := span.Attributes()
	attrs.PutStr("string_value", "value")
	attrs.PutInt("int_value", 123)
	attrs.PutDouble("float_value", 456.78)
	attrs.PutBool("bool_value", false)
	attrs.PutEmpty("null_value")

	arrayValue := attrs.PutEmptySlice("array_value")
	arrayValue.AppendEmpty().SetInt(12)
	arrayValue.AppendEmpty().SetInt(34)
	arrayValue.AppendEmpty().SetInt(56)

	mapValue := attrs.PutEmptyMap("map_value")
	mapValue.PutDouble("value1", -987.65)
	mapValue.PutBool("value2", true)

	segment, _ := MakeSegment(span, resource, []string{}, false, nil, false)

	assert.NotNil(t, segment)
	assert.Nil(t, segment.Metadata["default"]["null_value"])
	assert.Equal(t, "value", segment.Metadata["default"]["string_value"])
	assert.Equal(t, int64(123), segment.Metadata["default"]["int_value"])
	assert.Equal(t, 456.78, segment.Metadata["default"]["float_value"])
	assert.Equal(t, false, segment.Metadata["default"]["bool_value"])
	assert.Equal(t, []any{int64(12), int64(34), int64(56)}, segment.Metadata["default"]["array_value"])
	assert.Equal(t, map[string]any{
		"value1": -987.65,
		"value2": true,
	}, segment.Metadata["default"]["map_value"])
}

func TestSpanWithSingleDynamoDBTableHasTableName(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	attributes[conventions.AttributeAWSDynamoDBTableNames] = []string{"table1"}
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, "table1", *segment.AWS.TableName)
	assert.Nil(t, segment.AWS.TableNames)
	assert.Equal(t, []any{"table1"}, segment.Metadata["default"][conventions.AttributeAWSDynamoDBTableNames])
}

func TestSpanWithMultipleDynamoDBTablesHasTableNames(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	attributes[conventions.AttributeAWSDynamoDBTableNames] = []string{"table1", "table2"}
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil, false)

	assert.NotNil(t, segment)
	assert.Nil(t, segment.AWS.TableName)
	assert.Equal(t, []string{"table1", "table2"}, segment.AWS.TableNames)
	assert.Equal(t, []any{"table1", "table2"}, segment.Metadata["default"][conventions.AttributeAWSDynamoDBTableNames])
}

func TestSegmentWithLogGroupsFromConfig(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)
	timeEvents := constructTimedEventsWithSentMessageEvent(span.StartTimestamp())
	timeEvents.CopyTo(span.Events())
	pcommon.NewMap().CopyTo(span.Attributes())

	segment, _ := MakeSegment(span, resource, nil, false, []string{"my-logGroup-1"}, false)

	cwl := []awsxray.LogGroupMetadata{{
		LogGroup: awsxray.String("my-logGroup-1"),
	}}

	assert.Equal(t, cwl, segment.AWS.CWLogs)
}

func TestSegmentWith2LogGroupsFromConfig(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]any)
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)
	timeEvents := constructTimedEventsWithSentMessageEvent(span.StartTimestamp())
	timeEvents.CopyTo(span.Events())
	pcommon.NewMap().CopyTo(span.Attributes())

	segment, _ := MakeSegment(span, resource, nil, false, []string{"my-logGroup-1", "my-logGroup-2"}, false)

	cwl := []awsxray.LogGroupMetadata{{
		LogGroup: awsxray.String("my-logGroup-1"),
	}, {
		LogGroup: awsxray.String("my-logGroup-2"),
	}}

	assert.Equal(t, cwl, segment.AWS.CWLogs)
}

func TestClientSpanWithAwsRemoteServiceName(t *testing.T) {
	spanName := "ABC.payment"
	parentSpanID := newSegmentID()
	user := "testingT"
	attributes := make(map[string]any)
	attributes[conventions.AttributeHTTPMethod] = "POST"
	attributes[conventions.AttributeHTTPScheme] = "https"
	attributes[conventions.AttributeHTTPHost] = "payment.amazonaws.com"
	attributes[conventions.AttributeHTTPTarget] = "/"
	attributes[conventions.AttributeRPCService] = "ABC"
	attributes[awsRemoteService] = "PaymentService"

	resource := constructDefaultResource()
	span := constructClientSpan(parentSpanID, spanName, 0, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil, false)
	assert.Equal(t, "PaymentService", *segment.Name)
	assert.Equal(t, "subsegment", *segment.Type)

	jsonStr, err := MakeSegmentDocumentString(span, resource, nil, false, nil, false)

	assert.NotNil(t, jsonStr)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(jsonStr, "PaymentService"))
	assert.False(t, strings.Contains(jsonStr, user))
	assert.False(t, strings.Contains(jsonStr, "user"))
}

func TestAwsSdkSpanWithDeprecatedAwsRemoteServiceName(t *testing.T) {
	spanName := "DynamoDB.PutItem"
	parentSpanID := newSegmentID()
	user := "testingT"
	attributes := make(map[string]any)
	attributes[conventions.AttributeRPCSystem] = "aws-api"
	attributes[conventions.AttributeHTTPMethod] = "POST"
	attributes[conventions.AttributeHTTPScheme] = "https"
	attributes[conventions.AttributeRPCService] = "DynamoDb"
	attributes[awsRemoteService] = "AWS.SDK.DynamoDb"

	resource := constructDefaultResource()
	span := constructClientSpan(parentSpanID, spanName, 0, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil, false)
	assert.Equal(t, "DynamoDb", *segment.Name)
	assert.Equal(t, "subsegment", *segment.Type)

	jsonStr, err := MakeSegmentDocumentString(span, resource, nil, false, nil, false)

	assert.NotNil(t, jsonStr)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(jsonStr, "DynamoDb"))
	assert.False(t, strings.Contains(jsonStr, "DynamoDb.PutItem"))
	assert.False(t, strings.Contains(jsonStr, user))
	assert.False(t, strings.Contains(jsonStr, "user"))
}

func TestAwsSdkSpanWithAwsRemoteServiceName(t *testing.T) {
	spanName := "DynamoDB.PutItem"
	parentSpanID := newSegmentID()
	user := "testingT"
	attributes := make(map[string]any)
	attributes[conventions.AttributeRPCSystem] = "aws-api"
	attributes[conventions.AttributeHTTPMethod] = "POST"
	attributes[conventions.AttributeHTTPScheme] = "https"
	attributes[conventions.AttributeRPCService] = "DynamoDb"
	attributes[awsRemoteService] = "AWS::DynamoDB"

	resource := constructDefaultResource()
	span := constructClientSpan(parentSpanID, spanName, 0, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil, false)
	assert.Equal(t, "DynamoDB", *segment.Name)
	assert.Equal(t, "subsegment", *segment.Type)

	jsonStr, err := MakeSegmentDocumentString(span, resource, nil, false, nil, false)

	assert.NotNil(t, jsonStr)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(jsonStr, "DynamoDb"))
	assert.False(t, strings.Contains(jsonStr, "DynamoDb.PutItem"))
	assert.False(t, strings.Contains(jsonStr, user))
	assert.False(t, strings.Contains(jsonStr, "user"))
}

func TestProducerSpanWithAwsRemoteServiceName(t *testing.T) {
	spanName := "ABC.payment"
	parentSpanID := newSegmentID()
	user := "testingT"
	attributes := make(map[string]any)
	attributes[conventions.AttributeHTTPMethod] = "POST"
	attributes[conventions.AttributeHTTPScheme] = "https"
	attributes[conventions.AttributeHTTPHost] = "payment.amazonaws.com"
	attributes[conventions.AttributeHTTPTarget] = "/"
	attributes[conventions.AttributeRPCService] = "ABC"
	attributes[awsRemoteService] = "ProducerService"

	resource := constructDefaultResource()
	span := constructProducerSpan(parentSpanID, spanName, 0, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil, false)
	assert.Equal(t, "ProducerService", *segment.Name)
	assert.Equal(t, "subsegment", *segment.Type)

	jsonStr, err := MakeSegmentDocumentString(span, resource, nil, false, nil, false)

	assert.NotNil(t, jsonStr)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(jsonStr, "ProducerService"))
	assert.False(t, strings.Contains(jsonStr, user))
	assert.False(t, strings.Contains(jsonStr, "user"))
}

func TestConsumerSpanWithAwsRemoteServiceName(t *testing.T) {
	spanName := "ABC.payment"
	parentSpanID := newSegmentID()
	user := "testingT"
	attributes := getBasicAttributes()
	attributes[awsRemoteService] = "ConsumerService"

	resource := constructDefaultResource()
	span := constructConsumerSpan(parentSpanID, spanName, 0, "Ok", attributes)

	jsonStr, err := MakeSegmentDocumentString(span, resource, nil, false, nil, false)

	assert.NotNil(t, jsonStr)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(jsonStr, "ConsumerService"))
	assert.False(t, strings.Contains(jsonStr, user))
	assert.False(t, strings.Contains(jsonStr, "user"))
}

func TestServerSpanWithAwsLocalServiceName(t *testing.T) {
	spanName := "ABC.payment"
	parentSpanID := newSegmentID()
	user := "testingT"
	attributes := make(map[string]any)
	attributes[conventions.AttributeHTTPMethod] = "POST"
	attributes[conventions.AttributeHTTPScheme] = "https"
	attributes[conventions.AttributeHTTPHost] = "payment.amazonaws.com"
	attributes[conventions.AttributeHTTPTarget] = "/"
	attributes[conventions.AttributeRPCService] = "ABC"
	attributes[awsLocalService] = "PaymentLocalService"
	attributes[awsRemoteService] = "PaymentService"

	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, 0, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil, false)
	assert.Equal(t, "PaymentLocalService", *segment.Name)

	jsonStr, err := MakeSegmentDocumentString(span, resource, nil, false, nil, false)

	assert.NotNil(t, jsonStr)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(jsonStr, "PaymentLocalService"))
	assert.False(t, strings.Contains(jsonStr, user))
	assert.False(t, strings.Contains(jsonStr, "user"))
}

func validateLocalRootDependencySubsegment(t *testing.T, segment *awsxray.Segment, span ptrace.Span, parentID string) {
	tempTraceID := span.TraceID()
	expectedTraceID := "1-" + fmt.Sprintf("%x", tempTraceID[0:4]) + "-" + fmt.Sprintf("%x", tempTraceID[4:16])

	assert.Equal(t, "subsegment", *segment.Type)
	assert.Equal(t, "myRemoteService", *segment.Name)
	assert.Equal(t, span.SpanID().String(), *segment.ID)
	assert.Equal(t, parentID, *segment.ParentID)
	assert.Equal(t, expectedTraceID, *segment.TraceID)
	assert.NotNil(t, segment.HTTP)
	assert.Equal(t, "POST", *segment.HTTP.Request.Method)
	assert.Equal(t, 2, len(segment.Annotations))
	assert.Nil(t, segment.Annotations[awsRemoteService])
	assert.Nil(t, segment.Annotations[remoteTarget])
	assert.Equal(t, "myAnnotationValue", segment.Annotations["myAnnotationKey"])

	assert.Equal(t, 8, len(segment.Metadata["default"]))
	assert.Equal(t, "receive", segment.Metadata["default"][conventions.AttributeMessagingOperation])
	assert.Equal(t, "LOCAL_ROOT", segment.Metadata["default"][awsSpanKind])
	assert.Equal(t, "myRemoteOperation", segment.Metadata["default"][awsRemoteOperation])
	assert.Equal(t, "myTarget", segment.Metadata["default"][remoteTarget])
	assert.Equal(t, "k8sRemoteNamespace", segment.Metadata["default"][k8sRemoteNamespace])
	assert.Equal(t, "myLocalService", segment.Metadata["default"][awsLocalService])
	assert.Equal(t, "awsLocalOperation", segment.Metadata["default"][awsLocalOperation])
	assert.Equal(t, "service.name=myTest", segment.Metadata["default"]["otel.resource.attributes"])

	assert.Equal(t, "MySDK", *segment.AWS.XRay.SDK)
	assert.Equal(t, "1.20.0", *segment.AWS.XRay.SDKVersion)
	assert.Equal(t, true, *segment.AWS.XRay.AutoInstrumentation)

	assert.Equal(t, "UpdateItem", *segment.AWS.Operation)
	assert.Equal(t, "AWSAccountAttribute", *segment.AWS.AccountID)
	assert.Equal(t, "AWSRegionAttribute", *segment.AWS.RemoteRegion)
	assert.Equal(t, "AWSRequestIDAttribute", *segment.AWS.RequestID)
	assert.Equal(t, "AWSQueueURLAttribute", *segment.AWS.QueueURL)
	assert.Equal(t, "TableName", *segment.AWS.TableName)

	assert.Equal(t, "remote", *segment.Namespace)
}

func validateLocalRootServiceSegment(t *testing.T, segment *awsxray.Segment, span ptrace.Span) {
	tempTraceID := span.TraceID()
	expectedTraceID := "1-" + fmt.Sprintf("%x", tempTraceID[0:4]) + "-" + fmt.Sprintf("%x", tempTraceID[4:16])

	assert.Nil(t, segment.Type)
	assert.Equal(t, "myLocalService", *segment.Name)
	assert.Equal(t, expectedTraceID, *segment.TraceID)
	assert.Nil(t, segment.HTTP)
	assert.Equal(t, 1, len(segment.Annotations))
	assert.Equal(t, "myAnnotationValue", segment.Annotations["myAnnotationKey"])
	assert.Equal(t, 1, len(segment.Metadata["default"]))
	assert.Equal(t, "service.name=myTest", segment.Metadata["default"]["otel.resource.attributes"])
	assert.Equal(t, "MySDK", *segment.AWS.XRay.SDK)
	assert.Equal(t, "1.20.0", *segment.AWS.XRay.SDKVersion)
	assert.Equal(t, true, *segment.AWS.XRay.AutoInstrumentation)
	assert.Nil(t, segment.AWS.Operation)
	assert.Nil(t, segment.AWS.AccountID)
	assert.Nil(t, segment.AWS.RemoteRegion)
	assert.Nil(t, segment.AWS.RequestID)
	assert.Nil(t, segment.AWS.QueueURL)
	assert.Nil(t, segment.AWS.TableName)
	assert.Nil(t, segment.Namespace)

	assert.Nil(t, segment.Namespace)
}

func getBasicAttributes() map[string]any {
	attributes := make(map[string]any)

	attributes[conventions.AttributeHTTPMethod] = "POST"
	attributes[conventions.AttributeMessagingOperation] = "receive"

	attributes["otel.resource.attributes"] = "service.name=myTest"

	attributes[awsSpanKind] = "LOCAL_ROOT"
	attributes[awsRemoteService] = "myRemoteService"
	attributes[awsRemoteOperation] = "myRemoteOperation"
	attributes[remoteTarget] = "myTarget"
	attributes[k8sRemoteNamespace] = "k8sRemoteNamespace"
	attributes[awsLocalService] = "myLocalService"
	attributes[awsLocalOperation] = "awsLocalOperation"

	attributes["myAnnotationKey"] = "myAnnotationValue"

	attributes[awsxray.AWSOperationAttribute] = "UpdateItem"
	attributes[awsxray.AWSAccountAttribute] = "AWSAccountAttribute"
	attributes[awsxray.AWSRegionAttribute] = "AWSRegionAttribute"
	attributes[awsxray.AWSRequestIDAttribute] = "AWSRequestIDAttribute"
	attributes[awsxray.AWSQueueURLAttribute] = "AWSQueueURLAttribute"
	attributes[awsxray.AWSTableNameAttribute] = "TableName"

	return attributes
}

func getBasicResource() pcommon.Resource {
	resource := constructDefaultResource()

	resource.Attributes().PutStr(conventions.AttributeTelemetrySDKName, "MySDK")
	resource.Attributes().PutStr(conventions.AttributeTelemetrySDKVersion, "1.20.0")
	resource.Attributes().PutStr(conventions.AttributeTelemetryAutoVersion, "1.2.3")

	return resource
}

func addSpanLink(span ptrace.Span) {
	spanLink := span.Links().AppendEmpty()
	spanLink.SetTraceID(newTraceID())
	spanLink.SetSpanID(newSegmentID())
}

func TestLocalRootConsumer(t *testing.T) {
	err := featuregate.GlobalRegistry().Set("exporter.xray.allowDot", false)
	assert.Nil(t, err)

	spanName := "destination operation"
	resource := getBasicResource()
	parentSpanID := newSegmentID()

	attributes := getBasicAttributes()

	span := constructConsumerSpan(parentSpanID, spanName, 200, "OK", attributes)

	addSpanLink(span)

	segments, err := MakeSegmentsFromSpan(span, resource, []string{awsRemoteService, "myAnnotationKey"}, false, nil, false)

	assert.NotNil(t, segments)
	assert.Equal(t, 2, len(segments))
	assert.Nil(t, err)

	validateLocalRootDependencySubsegment(t, segments[0], span, *segments[1].ID)
	assert.Nil(t, segments[0].Links)

	validateLocalRootServiceSegment(t, segments[1], span)
	assert.Equal(t, 1, len(segments[1].Links))

	// Checks these values are the same for both
	assert.Equal(t, segments[0].StartTime, segments[1].StartTime)
	assert.Equal(t, segments[0].EndTime, segments[1].EndTime)
}

func TestNonLocalRootConsumerProcess(t *testing.T) {
	spanName := "destination operation"
	resource := getBasicResource()
	parentSpanID := newSegmentID()

	attributes := getBasicAttributes()
	delete(attributes, awsRemoteService)
	delete(attributes, awsRemoteOperation)
	attributes[awsSpanKind] = "Consumer"

	span := constructConsumerSpan(parentSpanID, spanName, 200, "OK", attributes)

	addSpanLink(span)

	segments, err := MakeSegmentsFromSpan(span, resource, []string{awsRemoteService, "myAnnotationKey"}, false, nil, false)

	assert.NotNil(t, segments)
	assert.Equal(t, 1, len(segments))
	assert.Nil(t, err)

	tempTraceID := span.TraceID()
	expectedTraceID := "1-" + fmt.Sprintf("%x", tempTraceID[0:4]) + "-" + fmt.Sprintf("%x", tempTraceID[4:16])

	// Validate segment 1 (dependency subsegment)
	assert.Equal(t, "subsegment", *segments[0].Type)
	assert.Equal(t, "destination operation", *segments[0].Name)
	assert.NotEqual(t, parentSpanID.String(), *segments[0].ID)
	assert.Equal(t, span.SpanID().String(), *segments[0].ID)
	assert.Equal(t, 1, len(segments[0].Links))
	assert.Equal(t, expectedTraceID, *segments[0].TraceID)
	assert.NotNil(t, segments[0].HTTP)
	assert.Equal(t, "POST", *segments[0].HTTP.Request.Method)
	assert.Equal(t, 1, len(segments[0].Annotations))
	assert.Equal(t, "myAnnotationValue", segments[0].Annotations["myAnnotationKey"])
	assert.Equal(t, 7, len(segments[0].Metadata["default"]))
	assert.Equal(t, "Consumer", segments[0].Metadata["default"][awsSpanKind])
	assert.Equal(t, "myLocalService", segments[0].Metadata["default"][awsLocalService])
	assert.Equal(t, "receive", segments[0].Metadata["default"][conventions.AttributeMessagingOperation])
	assert.Equal(t, "service.name=myTest", segments[0].Metadata["default"]["otel.resource.attributes"])
	assert.Equal(t, "MySDK", *segments[0].AWS.XRay.SDK)
	assert.Equal(t, "1.20.0", *segments[0].AWS.XRay.SDKVersion)
	assert.Equal(t, true, *segments[0].AWS.XRay.AutoInstrumentation)
	assert.Equal(t, "UpdateItem", *segments[0].AWS.Operation)
	assert.Nil(t, segments[0].Namespace)
}

func TestLocalRootConsumerAWSNamespace(t *testing.T) {
	spanName := "destination receive"
	resource := getBasicResource()
	parentSpanID := newSegmentID()

	attributes := getBasicAttributes()
	attributes[conventions.AttributeRPCSystem] = "aws-api"

	span := constructConsumerSpan(parentSpanID, spanName, 200, "OK", attributes)

	spanLink := span.Links().AppendEmpty()
	spanLink.SetTraceID(newTraceID())
	spanLink.SetSpanID(newSegmentID())

	segments, err := MakeSegmentsFromSpan(span, resource, []string{awsRemoteService, "myAnnotationKey"}, false, nil, false)

	assert.NotNil(t, segments)
	assert.Equal(t, 2, len(segments))
	assert.Nil(t, err)

	// Ensure that AWS namespace is not overwritten to remote
	assert.Equal(t, "aws", *segments[0].Namespace)
}

func TestLocalRootClient(t *testing.T) {
	err := featuregate.GlobalRegistry().Set("exporter.xray.allowDot", false)
	assert.Nil(t, err)

	spanName := "SQS Get"
	resource := getBasicResource()
	parentSpanID := newSegmentID()

	attributes := getBasicAttributes()

	span := constructClientSpan(parentSpanID, spanName, 200, "OK", attributes)

	spanLink := span.Links().AppendEmpty()
	spanLink.SetTraceID(newTraceID())
	spanLink.SetSpanID(newSegmentID())

	segments, err := MakeSegmentsFromSpan(span, resource, []string{awsRemoteService, "myAnnotationKey"}, false, nil, false)

	assert.NotNil(t, segments)
	assert.Equal(t, 2, len(segments))
	assert.Nil(t, err)

	validateLocalRootDependencySubsegment(t, segments[0], span, *segments[1].ID)
	assert.Equal(t, 1, len(segments[0].Links))

	validateLocalRootServiceSegment(t, segments[1], span)
	assert.Nil(t, segments[1].Links)

	// Checks these values are the same for both
	assert.Equal(t, segments[0].StartTime, segments[1].StartTime)
	assert.Equal(t, segments[0].EndTime, segments[1].EndTime)
}

func TestLocalRootClientAwsServiceMetrics(t *testing.T) {
	spanName := "SQS ReceiveMessage"
	resource := getBasicResource()

	parentSpanID := newSegmentID()

	attributes := getBasicAttributes()
	attributes[awsSpanKind] = "LOCAL_ROOT"
	attributes[conventions.AttributeRPCSystem] = "aws-api"
	attributes[conventions.AttributeHTTPMethod] = "POST"
	attributes[conventions.AttributeHTTPScheme] = "https"
	attributes[conventions.AttributeRPCService] = "SQS"
	attributes[awsRemoteService] = "AWS.SDK.SQS"

	span := constructClientSpan(parentSpanID, spanName, 200, "OK", attributes)

	spanLink := span.Links().AppendEmpty()
	spanLink.SetTraceID(newTraceID())
	spanLink.SetSpanID(newSegmentID())

	segments, err := MakeSegmentsFromSpan(span, resource, []string{awsRemoteService, "myAnnotationKey"}, false, nil, false)

	assert.NotNil(t, segments)
	assert.Equal(t, 2, len(segments))
	assert.Nil(t, err)

	subsegment := segments[0]

	assert.Equal(t, "subsegment", *subsegment.Type)
	assert.Equal(t, "SQS", *subsegment.Name)
	assert.Equal(t, "aws", *subsegment.Namespace)
}

func TestLocalRootProducer(t *testing.T) {
	spanName := "destination operation"
	resource := getBasicResource()
	parentSpanID := newSegmentID()

	attributes := getBasicAttributes()

	span := constructProducerSpan(parentSpanID, spanName, 200, "Ok", attributes)

	addSpanLink(span)

	segments, err := MakeSegmentsFromSpan(span, resource, []string{awsRemoteService, "myAnnotationKey"}, false, nil, false)

	assert.NotNil(t, segments)
	assert.Equal(t, 2, len(segments))
	assert.Nil(t, err)

	validateLocalRootDependencySubsegment(t, segments[0], span, *segments[1].ID)
	assert.Equal(t, 1, len(segments[0].Links))

	validateLocalRootServiceSegment(t, segments[1], span)
	assert.Nil(t, segments[1].Links)

	// Checks these values are the same for both
	assert.Equal(t, segments[0].StartTime, segments[1].StartTime)
	assert.Equal(t, segments[0].EndTime, segments[1].EndTime)
}

func validateLocalRootWithoutDependency(t *testing.T, segment *awsxray.Segment, span ptrace.Span) {
	tempTraceID := span.TraceID()
	expectedTraceID := "1-" + fmt.Sprintf("%x", tempTraceID[0:4]) + "-" + fmt.Sprintf("%x", tempTraceID[4:16])

	// Validate segment
	assert.Nil(t, segment.Type)
	assert.Equal(t, "myLocalService", *segment.Name)
	assert.Equal(t, span.ParentSpanID().String(), *segment.ParentID)
	assert.Equal(t, 1, len(segment.Links))
	assert.Equal(t, expectedTraceID, *segment.TraceID)
	assert.Equal(t, "POST", *segment.HTTP.Request.Method)
	assert.Equal(t, 2, len(segment.Annotations))
	assert.Equal(t, "myRemoteService", segment.Annotations["aws_remote_service"])
	assert.Equal(t, "myAnnotationValue", segment.Annotations["myAnnotationKey"])

	var numberOfMetadataKeys = 8

	if span.Kind() == ptrace.SpanKindServer {
		numberOfMetadataKeys = 30
	}

	assert.Equal(t, numberOfMetadataKeys, len(segment.Metadata["default"]))
	assert.Equal(t, "receive", segment.Metadata["default"][conventions.AttributeMessagingOperation])
	assert.Equal(t, "LOCAL_ROOT", segment.Metadata["default"][awsSpanKind])
	assert.Equal(t, "myRemoteOperation", segment.Metadata["default"][awsRemoteOperation])
	assert.Equal(t, "myTarget", segment.Metadata["default"][remoteTarget])
	assert.Equal(t, "k8sRemoteNamespace", segment.Metadata["default"][k8sRemoteNamespace])
	assert.Equal(t, "myLocalService", segment.Metadata["default"][awsLocalService])
	assert.Equal(t, "awsLocalOperation", segment.Metadata["default"][awsLocalOperation])
	assert.Equal(t, "service.name=myTest", segment.Metadata["default"]["otel.resource.attributes"])

	assert.Equal(t, "service.name=myTest", segment.Metadata["default"]["otel.resource.attributes"])
	assert.Equal(t, "MySDK", *segment.AWS.XRay.SDK)
	assert.Equal(t, "1.20.0", *segment.AWS.XRay.SDKVersion)
	assert.Equal(t, true, *segment.AWS.XRay.AutoInstrumentation)

	assert.Equal(t, "UpdateItem", *segment.AWS.Operation)
	assert.Equal(t, "AWSAccountAttribute", *segment.AWS.AccountID)
	assert.Equal(t, "AWSRegionAttribute", *segment.AWS.RemoteRegion)
	assert.Equal(t, "AWSRequestIDAttribute", *segment.AWS.RequestID)
	assert.Equal(t, "AWSQueueURLAttribute", *segment.AWS.QueueURL)
	assert.Equal(t, "TableName", *segment.AWS.TableName)

	assert.Nil(t, segment.Namespace)
}

func TestLocalRootServer(t *testing.T) {
	err := featuregate.GlobalRegistry().Set("exporter.xray.allowDot", false)
	assert.Nil(t, err)

	spanName := "MyService"
	resource := getBasicResource()
	parentSpanID := newSegmentID()

	attributes := getBasicAttributes()

	span := constructServerSpan(parentSpanID, spanName, 200, "OK", attributes)

	addSpanLink(span)

	segments, err := MakeSegmentsFromSpan(span, resource, []string{awsRemoteService, "myAnnotationKey"}, false, nil, false)

	assert.NotNil(t, segments)
	assert.Equal(t, 1, len(segments))
	assert.Nil(t, err)

	validateLocalRootWithoutDependency(t, segments[0], span)
}

func TestLocalRootInternal(t *testing.T) {
	err := featuregate.GlobalRegistry().Set("exporter.xray.allowDot", false)
	assert.Nil(t, err)

	spanName := "MyInternalService"
	resource := getBasicResource()
	parentSpanID := newSegmentID()

	attributes := getBasicAttributes()

	span := constructInternalSpan(parentSpanID, spanName, 200, "OK", attributes)

	addSpanLink(span)

	segments, err := MakeSegmentsFromSpan(span, resource, []string{awsRemoteService, "myAnnotationKey"}, false, nil, false)

	assert.NotNil(t, segments)
	assert.Equal(t, 1, len(segments))
	assert.Nil(t, err)

	validateLocalRootWithoutDependency(t, segments[0], span)
}

func TestNotLocalRootInternal(t *testing.T) {
	spanName := "MyService"
	resource := getBasicResource()
	parentSpanID := newSegmentID()

	attributes := getBasicAttributes()
	attributes[awsSpanKind] = "Internal"

	span := constructInternalSpan(parentSpanID, spanName, 200, "OK", attributes)

	addSpanLink(span)

	segments, err := MakeSegmentsFromSpan(span, resource, []string{awsRemoteService, "myAnnotationKey"}, false, nil, false)

	assert.NotNil(t, segments)
	assert.Equal(t, 1, len(segments))
	assert.Nil(t, err)

	// Validate segment
	assert.Equal(t, "subsegment", *segments[0].Type)
	assert.Nil(t, segments[0].Namespace)
	assert.Equal(t, "MyService", *segments[0].Name)
}

func TestNotLocalRootConsumer(t *testing.T) {
	spanName := "MyService"
	resource := getBasicResource()
	parentSpanID := newSegmentID()

	attributes := getBasicAttributes()
	attributes[awsSpanKind] = "Consumer"

	span := constructConsumerSpan(parentSpanID, spanName, 200, "OK", attributes)

	addSpanLink(span)

	segments, err := MakeSegmentsFromSpan(span, resource, []string{awsRemoteService, "myAnnotationKey"}, false, nil, false)

	assert.NotNil(t, segments)
	assert.Equal(t, 1, len(segments))
	assert.Nil(t, err)

	// Validate segment
	assert.Equal(t, "subsegment", *segments[0].Type)
	assert.Equal(t, "remote", *segments[0].Namespace)
	assert.Equal(t, "myRemoteService", *segments[0].Name)
}

func TestNotLocalRootClient(t *testing.T) {
	spanName := "MyService"
	resource := getBasicResource()
	parentSpanID := newSegmentID()

	attributes := getBasicAttributes()
	attributes[awsSpanKind] = "Client"

	span := constructClientSpan(parentSpanID, spanName, 200, "OK", attributes)

	addSpanLink(span)

	segments, err := MakeSegmentsFromSpan(span, resource, []string{awsRemoteService, "myAnnotationKey"}, false, nil, false)

	assert.NotNil(t, segments)
	assert.Equal(t, 1, len(segments))
	assert.Nil(t, err)

	// Validate segment
	assert.Equal(t, "subsegment", *segments[0].Type)
	assert.Equal(t, "remote", *segments[0].Namespace)
	assert.Equal(t, "myRemoteService", *segments[0].Name)
}

func TestNotLocalRootProducer(t *testing.T) {
	spanName := "MyService"
	resource := getBasicResource()
	parentSpanID := newSegmentID()

	attributes := getBasicAttributes()
	attributes[awsSpanKind] = "Producer"

	span := constructProducerSpan(parentSpanID, spanName, 200, "OK", attributes)

	addSpanLink(span)

	segments, err := MakeSegmentsFromSpan(span, resource, []string{awsRemoteService, "myAnnotationKey"}, false, nil, false)

	assert.NotNil(t, segments)
	assert.Equal(t, 1, len(segments))
	assert.Nil(t, err)

	// Validate segment
	assert.Equal(t, "subsegment", *segments[0].Type)
	assert.Equal(t, "remote", *segments[0].Namespace)
	assert.Equal(t, "myRemoteService", *segments[0].Name)
}

func TestNotLocalRootServer(t *testing.T) {
	spanName := "MyInternalService"
	resource := getBasicResource()
	parentSpanID := newSegmentID()

	attributes := getBasicAttributes()
	attributes[awsSpanKind] = "Server"
	delete(attributes, awsRemoteService)
	delete(attributes, awsRemoteOperation)

	span := constructServerSpan(parentSpanID, spanName, 200, "OK", attributes)

	addSpanLink(span)

	segments, err := MakeSegmentsFromSpan(span, resource, []string{awsRemoteService, "myAnnotationKey"}, false, nil, false)

	assert.NotNil(t, segments)
	assert.Equal(t, 1, len(segments))
	assert.Nil(t, err)

	// Validate segment
	assert.Nil(t, segments[0].Type)
	assert.Nil(t, segments[0].Namespace)
	assert.Equal(t, "myLocalService", *segments[0].Name)
}

func constructClientSpan(parentSpanID pcommon.SpanID, name string, code ptrace.StatusCode, message string, attributes map[string]any) ptrace.Span {
	var (
		traceID        = newTraceID()
		spanID         = newSegmentID()
		endTime        = time.Now()
		startTime      = endTime.Add(-215 * time.Millisecond)
		spanAttributes = constructSpanAttributes(attributes)
	)

	span := ptrace.NewSpan()
	span.SetTraceID(traceID)
	span.SetSpanID(spanID)
	span.SetParentSpanID(parentSpanID)
	span.SetName(name)
	span.SetKind(ptrace.SpanKindClient)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

	status := ptrace.NewStatus()
	status.SetCode(code)
	status.SetMessage(message)
	status.CopyTo(span.Status())

	spanAttributes.CopyTo(span.Attributes())
	return span
}

func constructServerSpan(parentSpanID pcommon.SpanID, name string, code ptrace.StatusCode, message string, attributes map[string]any) ptrace.Span {
	var (
		traceID        = newTraceID()
		spanID         = newSegmentID()
		endTime        = time.Now()
		startTime      = endTime.Add(-215 * time.Millisecond)
		spanAttributes = constructSpanAttributes(attributes)
	)

	span := ptrace.NewSpan()
	span.SetTraceID(traceID)
	span.SetSpanID(spanID)
	span.SetParentSpanID(parentSpanID)
	span.SetName(name)
	span.SetKind(ptrace.SpanKindServer)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

	status := ptrace.NewStatus()
	status.SetCode(code)
	status.SetMessage(message)
	status.CopyTo(span.Status())

	spanAttributes.CopyTo(span.Attributes())
	return span
}

func constructInternalSpan(parentSpanID pcommon.SpanID, name string, code ptrace.StatusCode, message string, attributes map[string]any) ptrace.Span {
	var (
		traceID        = newTraceID()
		spanID         = newSegmentID()
		endTime        = time.Now()
		startTime      = endTime.Add(-215 * time.Millisecond)
		spanAttributes = constructSpanAttributes(attributes)
	)

	span := ptrace.NewSpan()
	span.SetTraceID(traceID)
	span.SetSpanID(spanID)
	span.SetParentSpanID(parentSpanID)
	span.SetName(name)
	span.SetKind(ptrace.SpanKindInternal)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

	status := ptrace.NewStatus()
	status.SetCode(code)
	status.SetMessage(message)
	status.CopyTo(span.Status())

	spanAttributes.CopyTo(span.Attributes())
	return span
}

func constructConsumerSpan(parentSpanID pcommon.SpanID, name string, code ptrace.StatusCode, message string, attributes map[string]any) ptrace.Span {
	var (
		traceID        = newTraceID()
		spanID         = newSegmentID()
		endTime        = time.Now()
		startTime      = endTime.Add(-215 * time.Millisecond)
		spanAttributes = constructSpanAttributes(attributes)
	)

	span := ptrace.NewSpan()
	span.SetTraceID(traceID)
	span.SetSpanID(spanID)
	span.SetParentSpanID(parentSpanID)
	span.SetName(name)
	span.SetKind(ptrace.SpanKindConsumer)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

	status := ptrace.NewStatus()
	status.SetCode(code)
	status.SetMessage(message)
	status.CopyTo(span.Status())

	spanAttributes.CopyTo(span.Attributes())
	return span
}

func constructProducerSpan(parentSpanID pcommon.SpanID, name string, code ptrace.StatusCode, message string, attributes map[string]any) ptrace.Span {
	var (
		traceID        = newTraceID()
		spanID         = newSegmentID()
		endTime        = time.Now()
		startTime      = endTime.Add(-215 * time.Millisecond)
		spanAttributes = constructSpanAttributes(attributes)
	)

	span := ptrace.NewSpan()
	span.SetTraceID(traceID)
	span.SetSpanID(spanID)
	span.SetParentSpanID(parentSpanID)
	span.SetName(name)
	span.SetKind(ptrace.SpanKindProducer)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

	status := ptrace.NewStatus()
	status.SetCode(code)
	status.SetMessage(message)
	status.CopyTo(span.Status())

	spanAttributes.CopyTo(span.Attributes())
	return span
}

func constructSpanAttributes(attributes map[string]any) pcommon.Map {
	attrs := pcommon.NewMap()
	for key, value := range attributes {
		if cast, ok := value.(int); ok {
			attrs.PutInt(key, int64(cast))
		} else if cast, ok := value.(int64); ok {
			attrs.PutInt(key, cast)
		} else if cast, ok := value.([]string); ok {
			slice := attrs.PutEmptySlice(key)
			for _, v := range cast {
				slice.AppendEmpty().SetStr(v)
			}
		} else {
			attrs.PutStr(key, fmt.Sprintf("%v", value))
		}
	}
	return attrs
}

func constructDefaultResource() pcommon.Resource {
	resource := pcommon.NewResource()
	attrs := resource.Attributes()
	attrs.PutStr(conventions.AttributeServiceName, "signup_aggregator")
	attrs.PutStr(conventions.AttributeServiceVersion, "semver:1.1.4")
	attrs.PutStr(conventions.AttributeContainerName, "signup_aggregator")
	attrs.PutStr(conventions.AttributeContainerImageName, "otel/signupaggregator")
	attrs.PutStr(conventions.AttributeContainerImageTag, "v1")
	attrs.PutStr(conventions.AttributeK8SClusterName, "production")
	attrs.PutStr(conventions.AttributeK8SNamespaceName, "default")
	attrs.PutStr(conventions.AttributeK8SDeploymentName, "signup_aggregator")
	attrs.PutStr(conventions.AttributeK8SPodName, "signup_aggregator-x82ufje83")
	attrs.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.PutStr(conventions.AttributeCloudAccountID, "123456789")
	attrs.PutStr(conventions.AttributeCloudRegion, "us-east-1")
	attrs.PutStr(conventions.AttributeCloudAvailabilityZone, "us-east-1c")
	attrs.PutStr(resourceStringKey, "string")
	attrs.PutInt(resourceIntKey, 10)
	attrs.PutDouble(resourceDoubleKey, 5.0)
	attrs.PutBool(resourceBoolKey, true)

	resourceMap := attrs.PutEmptyMap(resourceMapKey)
	resourceMap.PutInt("key1", 1)
	resourceMap.PutStr("key2", "value")

	resourceArray := attrs.PutEmptySlice(resourceArrayKey)
	resourceArray.AppendEmpty().SetStr("foo")
	resourceArray.AppendEmpty().SetStr("bar")
	return resource
}

func constructTimedEventsWithReceivedMessageEvent(tm pcommon.Timestamp) ptrace.SpanEventSlice {
	events := ptrace.NewSpanEventSlice()
	event := events.AppendEmpty()

	eventAttr := event.Attributes()
	eventAttr.PutStr("message.type", "RECEIVED")
	eventAttr.PutInt(conventions.AttributeMessagingMessageID, 1)
	eventAttr.PutInt(conventions.AttributeMessagingMessagePayloadCompressedSizeBytes, 6478)
	eventAttr.PutInt(conventions.AttributeMessagingMessagePayloadSizeBytes, 12452)

	event.SetTimestamp(tm)
	event.SetDroppedAttributesCount(0)

	return events
}

func constructTimedEventsWithSentMessageEvent(tm pcommon.Timestamp) ptrace.SpanEventSlice {
	events := ptrace.NewSpanEventSlice()
	event := events.AppendEmpty()

	eventAttr := event.Attributes()
	eventAttr.PutStr("message.type", "SENT")
	eventAttr.PutInt(conventions.AttributeMessagingMessageID, 1)
	eventAttr.PutInt(conventions.AttributeMessagingMessagePayloadSizeBytes, 7480)

	event.SetTimestamp(tm)
	event.SetDroppedAttributesCount(0)

	return events
}

// newTraceID generates a new valid X-Ray TraceID
func newTraceID() pcommon.TraceID {
	var r [16]byte
	epoch := time.Now().Unix()
	binary.BigEndian.PutUint32(r[0:4], uint32(epoch))
	_, err := rand.Read(r[4:])
	if err != nil {
		panic(err)
	}
	return r
}
