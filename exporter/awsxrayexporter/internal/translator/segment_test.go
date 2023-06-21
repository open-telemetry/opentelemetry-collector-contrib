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
	"encoding/binary"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	attributes := make(map[string]interface{})
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

	segment, _ := MakeSegment(span, resource, nil, false, nil)
	assert.Equal(t, "DynamoDB", *segment.Name)
	assert.Equal(t, conventions.AttributeCloudProviderAWS, *segment.Namespace)
	assert.Equal(t, "GetItem", *segment.AWS.Operation)
	assert.Equal(t, "subsegment", *segment.Type)

	jsonStr, err := MakeSegmentDocumentString(span, resource, nil, false, nil)

	assert.NotNil(t, jsonStr)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(jsonStr, "DynamoDB"))
	assert.True(t, strings.Contains(jsonStr, "GetItem"))
	assert.False(t, strings.Contains(jsonStr, user))
	assert.False(t, strings.Contains(jsonStr, "user"))
}

func TestClientSpanWithLegacyAwsSdkClientAttributes(t *testing.T) {
	spanName := "AmazonDynamoDB.getItem"
	parentSpanID := newSegmentID()
	user := "testingT"
	attributes := make(map[string]interface{})
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

	segment, _ := MakeSegment(span, resource, nil, false, nil)
	assert.Equal(t, "DynamoDB", *segment.Name)
	assert.Equal(t, conventions.AttributeCloudProviderAWS, *segment.Namespace)
	assert.Equal(t, "GetItem", *segment.AWS.Operation)
	assert.Equal(t, "subsegment", *segment.Type)

	jsonStr, err := MakeSegmentDocumentString(span, resource, nil, false, nil)

	assert.NotNil(t, jsonStr)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(jsonStr, "DynamoDB"))
	assert.True(t, strings.Contains(jsonStr, "GetItem"))
	assert.False(t, strings.Contains(jsonStr, user))
	assert.False(t, strings.Contains(jsonStr, "user"))
}

func TestClientSpanWithPeerService(t *testing.T) {
	spanName := "AmazonDynamoDB.getItem"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
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

	segment, _ := MakeSegment(span, resource, nil, false, nil)
	assert.Equal(t, "cats-table", *segment.Name)
}

func TestServerSpanWithInternalServerError(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	errorMessage := "java.lang.NullPointerException"
	userAgent := "PostmanRuntime/7.21.0"
	enduser := "go.tester@example.com"
	attributes := make(map[string]interface{})
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

	segment, _ := MakeSegment(span, resource, nil, false, nil)

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
	attributes := make(map[string]interface{})
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

	segment, _ := MakeSegment(span, resource, nil, false, nil)

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

	segment, _ := MakeSegment(span, resource, nil, false, nil)

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
	segment, _ := MakeSegment(span, resource, nil, false, nil)

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
	segment, _ := MakeSegment(span, resource, nil, false, nil)
	assert.NotNil(t, segment)
}

func TestClientSpanWithDbComponent(t *testing.T) {
	spanName := "call update_user_preference( ?, ?, ? )"
	parentSpanID := newSegmentID()
	enterpriseAppID := "25F2E73B-4769-4C79-9DF3-7EBE85D571EA"
	attributes := make(map[string]interface{})
	attributes[conventions.AttributeDBSystem] = "mysql"
	attributes[conventions.AttributeDBName] = "customers"
	attributes[conventions.AttributeDBStatement] = spanName
	attributes[conventions.AttributeDBUser] = "userprefsvc"
	attributes[conventions.AttributeDBConnectionString] = "mysql://db.dev.example.com:3306"
	attributes[conventions.AttributeNetPeerName] = "db.dev.example.com"
	attributes[conventions.AttributeNetPeerPort] = "3306"
	attributes["enterprise.app.id"] = enterpriseAppID
	resource := constructDefaultResource()
	span := constructClientSpan(parentSpanID, spanName, ptrace.StatusCodeUnset, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil)

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
	attributes := make(map[string]interface{})
	attributes[conventions.AttributeHTTPMethod] = "GET"
	attributes[conventions.AttributeHTTPScheme] = "https"
	attributes[conventions.AttributeNetPeerIP] = "2607:f8b0:4000:80c::2004"
	attributes[conventions.AttributeNetPeerPort] = "9443"
	attributes[conventions.AttributeHTTPTarget] = "/"
	attributes[conventions.AttributeHTTPHost] = "foo.com"
	attributes[conventions.AttributeNetPeerName] = "bar.com"
	resource := constructDefaultResource()
	span := constructClientSpan(parentSpanID, spanName, ptrace.StatusCodeUnset, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil)

	assert.NotNil(t, segment)
	assert.Equal(t, "foo.com", *segment.Name)
}

func TestClientSpanWithoutHttpHost(t *testing.T) {
	spanName := "GET /"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	attributes[conventions.AttributeHTTPMethod] = "GET"
	attributes[conventions.AttributeHTTPScheme] = "https"
	attributes[conventions.AttributeNetPeerIP] = "2607:f8b0:4000:80c::2004"
	attributes[conventions.AttributeNetPeerPort] = "9443"
	attributes[conventions.AttributeHTTPTarget] = "/"
	attributes[conventions.AttributeNetPeerName] = "bar.com"
	resource := constructDefaultResource()
	span := constructClientSpan(parentSpanID, spanName, ptrace.StatusCodeUnset, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil)

	assert.NotNil(t, segment)
	assert.Equal(t, "bar.com", *segment.Name)
}

func TestClientSpanWithRpcHost(t *testing.T) {
	spanName := "GET /com.foo.AnimalService/GetCats"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	attributes[conventions.AttributeHTTPMethod] = "GET"
	attributes[conventions.AttributeHTTPScheme] = "https"
	attributes[conventions.AttributeNetPeerIP] = "2607:f8b0:4000:80c::2004"
	attributes[conventions.AttributeNetPeerPort] = "9443"
	attributes[conventions.AttributeHTTPTarget] = "/com.foo.AnimalService/GetCats"
	attributes[conventions.AttributeRPCService] = "com.foo.AnimalService"
	attributes[conventions.AttributeNetPeerName] = "bar.com"
	resource := constructDefaultResource()
	span := constructClientSpan(parentSpanID, spanName, ptrace.StatusCodeUnset, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil)

	assert.NotNil(t, segment)
	assert.Equal(t, "com.foo.AnimalService", *segment.Name)
}

func TestSpanWithInvalidTraceId(t *testing.T) {
	spanName := "platformapi.widgets.searchWidgets"
	attributes := make(map[string]interface{})
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

	_, err := MakeSegmentDocumentString(span, resource, nil, false, nil)

	assert.NotNil(t, err)
}

func TestSpanWithExpiredTraceId(t *testing.T) {
	// First Build expired TraceId
	const maxAge = 60 * 60 * 24 * 30
	ExpiredEpoch := time.Now().Unix() - maxAge - 1

	tempTraceID := newTraceID()
	binary.BigEndian.PutUint32(tempTraceID[0:4], uint32(ExpiredEpoch))

	_, err := convertToAmazonTraceID(tempTraceID)
	assert.NotNil(t, err)
}

func TestFixSegmentName(t *testing.T) {
	validName := "EP @ test_15.testing-d\u00F6main.org#GO"
	fixedName := fixSegmentName(validName)
	assert.Equal(t, validName, fixedName)
	invalidName := "<subDomain>.example.com"
	fixedName = fixSegmentName(invalidName)
	assert.Equal(t, "subDomain.example.com", fixedName)
	fullyInvalidName := "<>"
	fixedName = fixSegmentName(fullyInvalidName)
	assert.Equal(t, defaultSegmentName, fixedName)
}

func TestFixAnnotationKey(t *testing.T) {
	validKey := "Key_1"
	fixedKey := fixAnnotationKey(validKey)
	assert.Equal(t, validKey, fixedKey)
	invalidKey := "Key@1"
	fixedKey = fixAnnotationKey(invalidKey)
	assert.Equal(t, "Key_1", fixedKey)
}

func TestServerSpanWithNilAttributes(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)
	timeEvents := constructTimedEventsWithSentMessageEvent(span.StartTimestamp())
	timeEvents.CopyTo(span.Events())
	pcommon.NewMap().CopyTo(span.Attributes())

	segment, _ := MakeSegment(span, resource, nil, false, nil)

	assert.NotNil(t, segment)
	assert.NotNil(t, segment.Cause)
	assert.Equal(t, "signup_aggregator", *segment.Name)
	assert.True(t, *segment.Fault)
}

func TestSpanWithAttributesDefaultNotIndexed(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	attributes["attr1@1"] = "val1"
	attributes["attr2@2"] = "val2"
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil)

	assert.NotNil(t, segment)
	assert.Equal(t, 0, len(segment.Annotations))
	assert.Equal(t, "val1", segment.Metadata["default"]["attr1@1"])
	assert.Equal(t, "val2", segment.Metadata["default"]["attr2@2"])
	assert.Equal(t, "string", segment.Metadata["default"]["otel.resource.string.key"])
	assert.Equal(t, int64(10), segment.Metadata["default"]["otel.resource.int.key"])
	assert.Equal(t, 5.0, segment.Metadata["default"]["otel.resource.double.key"])
	assert.Equal(t, true, segment.Metadata["default"]["otel.resource.bool.key"])
	expectedMap := make(map[string]interface{})
	expectedMap["key1"] = int64(1)
	expectedMap["key2"] = "value"
	assert.Equal(t, expectedMap, segment.Metadata["default"]["otel.resource.map.key"])
	expectedArr := []interface{}{"foo", "bar"}
	assert.Equal(t, expectedArr, segment.Metadata["default"]["otel.resource.array.key"])
}

func TestSpanWithResourceNotStoredIfSubsegment(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	attributes["attr1@1"] = "val1"
	attributes["attr2@2"] = "val2"
	resource := constructDefaultResource()
	span := constructClientSpan(parentSpanID, spanName, ptrace.StatusCodeError, "ERROR", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil)

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
	attributes := make(map[string]interface{})
	attributes["attr1@1"] = "val1"
	attributes["attr2@2"] = "val2"
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{"attr1@1", "not_exist"}, false, nil)

	assert.NotNil(t, segment)
	assert.Equal(t, 1, len(segment.Annotations))
	assert.Equal(t, "val1", segment.Annotations["attr1_1"])
	assert.Equal(t, "val2", segment.Metadata["default"]["attr2@2"])
}

func TestSpanWithAnnotationsAttribute(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	attributes["attr1@1"] = "val1"
	attributes["attr2@2"] = "val2"
	attributes[awsxray.AWSXraySegmentAnnotationsAttribute] = []string{"attr2@2", "not_exist"}
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil)

	assert.NotNil(t, segment)
	assert.Equal(t, 1, len(segment.Annotations))
	assert.Equal(t, "val2", segment.Annotations["attr2_2"])
	assert.Equal(t, "val1", segment.Metadata["default"]["attr1@1"])
}

func TestSpanWithAttributesAllIndexed(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	attributes["attr1@1"] = "val1"
	attributes["attr2@2"] = "val2"
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeOk, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{"attr1@1", "not_exist"}, true, nil)

	assert.NotNil(t, segment)
	assert.Equal(t, "val1", segment.Annotations["attr1_1"])
	assert.Equal(t, "val2", segment.Annotations["attr2_2"])
}

func TestSpanWithAttributesSegmentMetadata(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	attributes["attr1@1"] = "val1"
	attributes[awsxray.AWSXraySegmentMetadataAttributePrefix+"default"] = "{\"custom_key\": \"custom_value\"}"
	attributes[awsxray.AWSXraySegmentMetadataAttributePrefix+"http"] = "{\"connection\":{\"reused\":false,\"was_idle\":false}}"
	attributes[awsxray.AWSXraySegmentMetadataAttributePrefix+"non-xray-sdk"] = "retain-value"
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil)

	assert.NotNil(t, segment)
	assert.Equal(t, 0, len(segment.Annotations))
	assert.Equal(t, 2, len(segment.Metadata))
	assert.Equal(t, "val1", segment.Metadata["default"]["attr1@1"])
	assert.Equal(t, "custom_value", segment.Metadata["default"]["custom_key"])
	assert.Equal(t, "retain-value", segment.Metadata["default"][awsxray.AWSXraySegmentMetadataAttributePrefix+"non-xray-sdk"])
	assert.Nil(t, segment.Metadata["default"][awsxray.AWSXraySegmentMetadataAttributePrefix+"default"])
	assert.Nil(t, segment.Metadata["default"][awsxray.AWSXraySegmentMetadataAttributePrefix+"http"])
	assert.Equal(t, map[string]interface{}{
		"reused":   false,
		"was_idle": false,
	}, segment.Metadata["http"]["connection"])
}

func TestResourceAttributesCanBeIndexed(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{
		"otel.resource.string.key",
		"otel.resource.int.key",
		"otel.resource.double.key",
		"otel.resource.bool.key",
		"otel.resource.map.key",
		"otel.resource.array.key",
	}, false, nil)

	assert.NotNil(t, segment)
	assert.Equal(t, 4, len(segment.Annotations))
	assert.Equal(t, "string", segment.Annotations["otel_resource_string_key"])
	assert.Equal(t, int64(10), segment.Annotations["otel_resource_int_key"])
	assert.Equal(t, 5.0, segment.Annotations["otel_resource_double_key"])
	assert.Equal(t, true, segment.Annotations["otel_resource_bool_key"])

	expectedMap := make(map[string]interface{})
	expectedMap["key1"] = int64(1)
	expectedMap["key2"] = "value"
	// Maps and arrays are not supported for annotations so still in metadata.
	assert.Equal(t, expectedMap, segment.Metadata["default"]["otel.resource.map.key"])
	expectedArr := []interface{}{"foo", "bar"}
	assert.Equal(t, expectedArr, segment.Metadata["default"]["otel.resource.array.key"])
}

func TestResourceAttributesNotIndexedIfSubsegment(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := constructDefaultResource()
	span := constructClientSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{
		"otel.resource.string.key",
		"otel.resource.int.key",
		"otel.resource.double.key",
		"otel.resource.bool.key",
		"otel.resource.map.key",
		"otel.resource.array.key",
	}, false, nil)

	assert.NotNil(t, segment)
	assert.Empty(t, segment.Annotations)
	assert.Empty(t, segment.Metadata)
}

func TestSpanWithSpecialAttributesAsListed(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	attributes[awsxray.AWSOperationAttribute] = "aws_operation_val"
	attributes[conventions.AttributeRPCMethod] = "rpc_method_val"
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{awsxray.AWSOperationAttribute, conventions.AttributeRPCMethod}, false, nil)

	assert.NotNil(t, segment)
	assert.Equal(t, 2, len(segment.Annotations))
	assert.Equal(t, "aws_operation_val", segment.Annotations["aws_operation"])
	assert.Equal(t, "rpc_method_val", segment.Annotations["rpc_method"])
}

func TestSpanWithSpecialAttributesAsListedAndIndexAll(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	attributes[awsxray.AWSOperationAttribute] = "aws_operation_val"
	attributes[conventions.AttributeRPCMethod] = "rpc_method_val"
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{awsxray.AWSOperationAttribute, conventions.AttributeRPCMethod}, true, nil)

	assert.NotNil(t, segment)
	assert.Equal(t, "aws_operation_val", segment.Annotations["aws_operation"])
	assert.Equal(t, "rpc_method_val", segment.Annotations["rpc_method"])
}

func TestSpanWithSpecialAttributesNotListedAndIndexAll(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	attributes[awsxray.AWSOperationAttribute] = "val1"
	attributes[conventions.AttributeRPCMethod] = "val2"
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, true, nil)

	assert.NotNil(t, segment)
	assert.Nil(t, segment.Annotations["aws_operation"])
	assert.Nil(t, segment.Annotations["rpc_method"])
}

func TestOriginNotAws(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := pcommon.NewResource()
	attrs := resource.Attributes()
	attrs.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderGCP)
	attrs.PutStr(conventions.AttributeHostID, "instance-123")
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false, nil)

	assert.NotNil(t, segment)
	assert.Nil(t, segment.Origin)
}

func TestOriginEc2(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := pcommon.NewResource()
	attrs := resource.Attributes()
	attrs.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.PutStr(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSEC2)
	attrs.PutStr(conventions.AttributeHostID, "instance-123")
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false, nil)

	assert.NotNil(t, segment)
	assert.Equal(t, OriginEC2, *segment.Origin)
}

func TestOriginEcs(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := pcommon.NewResource()
	attrs := resource.Attributes()
	attrs.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.PutStr(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSECS)
	attrs.PutStr(conventions.AttributeHostID, "instance-123")
	attrs.PutStr(conventions.AttributeContainerName, "container-123")
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false, nil)

	assert.NotNil(t, segment)
	assert.Equal(t, OriginECS, *segment.Origin)
}

func TestOriginEcsEc2(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := pcommon.NewResource()
	attrs := resource.Attributes()
	attrs.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.PutStr(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSECS)
	attrs.PutStr(conventions.AttributeAWSECSLaunchtype, conventions.AttributeAWSECSLaunchtypeEC2)
	attrs.PutStr(conventions.AttributeHostID, "instance-123")
	attrs.PutStr(conventions.AttributeContainerName, "container-123")
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false, nil)

	assert.NotNil(t, segment)
	assert.Equal(t, OriginECSEC2, *segment.Origin)
}

func TestOriginEcsFargate(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := pcommon.NewResource()
	attrs := resource.Attributes()
	attrs.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.PutStr(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSECS)
	attrs.PutStr(conventions.AttributeAWSECSLaunchtype, conventions.AttributeAWSECSLaunchtypeFargate)
	attrs.PutStr(conventions.AttributeHostID, "instance-123")
	attrs.PutStr(conventions.AttributeContainerName, "container-123")
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false, nil)

	assert.NotNil(t, segment)
	assert.Equal(t, OriginECSFargate, *segment.Origin)
}

func TestOriginEb(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := pcommon.NewResource()
	attrs := resource.Attributes()
	attrs.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.PutStr(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSElasticBeanstalk)
	attrs.PutStr(conventions.AttributeHostID, "instance-123")
	attrs.PutStr(conventions.AttributeContainerName, "container-123")
	attrs.PutStr(conventions.AttributeServiceInstanceID, "service-123")
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false, nil)

	assert.NotNil(t, segment)
	assert.Equal(t, OriginEB, *segment.Origin)
}

func TestOriginEks(t *testing.T) {
	instanceID := "i-00f7c0bcb26da2a99"
	containerName := "signup_aggregator-x82ufje83"
	containerID := "0123456789A"
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
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

	segment, _ := MakeSegment(span, resource, []string{}, false, nil)

	assert.NotNil(t, segment)
	assert.Equal(t, OriginEKS, *segment.Origin)
}

func TestOriginAppRunner(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := pcommon.NewResource()
	attrs := resource.Attributes()
	attrs.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.PutStr(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSAppRunner)
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false, nil)

	assert.NotNil(t, segment)
	assert.Equal(t, OriginAppRunner, *segment.Origin)
}

func TestOriginBlank(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := pcommon.NewResource()
	attrs := resource.Attributes()
	attrs.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false, nil)

	assert.NotNil(t, segment)
	assert.Nil(t, segment.Origin)
}

func TestOriginPrefersInfraService(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := pcommon.NewResource()
	attrs := resource.Attributes()
	attrs.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.PutStr(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSEC2)
	attrs.PutStr(conventions.AttributeK8SClusterName, "cluster-123")
	attrs.PutStr(conventions.AttributeHostID, "instance-123")
	attrs.PutStr(conventions.AttributeContainerName, "container-123")
	attrs.PutStr(conventions.AttributeServiceInstanceID, "service-123")
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false, nil)

	assert.NotNil(t, segment)
	assert.Equal(t, OriginEC2, *segment.Origin)
}

func TestFilteredAttributesMetadata(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
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

	segment, _ := MakeSegment(span, resource, []string{}, false, nil)

	assert.NotNil(t, segment)
	assert.Nil(t, segment.Metadata["default"]["null_value"])
	assert.Equal(t, "value", segment.Metadata["default"]["string_value"])
	assert.Equal(t, int64(123), segment.Metadata["default"]["int_value"])
	assert.Equal(t, 456.78, segment.Metadata["default"]["float_value"])
	assert.Equal(t, false, segment.Metadata["default"]["bool_value"])
	assert.Equal(t, []interface{}{int64(12), int64(34), int64(56)}, segment.Metadata["default"]["array_value"])
	assert.Equal(t, map[string]interface{}{
		"value1": -987.65,
		"value2": true,
	}, segment.Metadata["default"]["map_value"])
}

func TestSpanWithSingleDynamoDBTableHasTableName(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	attributes[conventions.AttributeAWSDynamoDBTableNames] = []string{"table1"}
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil)

	assert.NotNil(t, segment)
	assert.Equal(t, "table1", *segment.AWS.TableName)
	assert.Nil(t, segment.AWS.TableNames)
	assert.Equal(t, []interface{}{"table1"}, segment.Metadata["default"][conventions.AttributeAWSDynamoDBTableNames])
}

func TestSpanWithMultipleDynamoDBTablesHasTableNames(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	attributes[conventions.AttributeAWSDynamoDBTableNames] = []string{"table1", "table2"}
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil)

	assert.NotNil(t, segment)
	assert.Nil(t, segment.AWS.TableName)
	assert.Equal(t, []string{"table1", "table2"}, segment.AWS.TableNames)
	assert.Equal(t, []interface{}{"table1", "table2"}, segment.Metadata["default"][conventions.AttributeAWSDynamoDBTableNames])
}

func TestSegmentWithLogGroupsFromConfig(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)
	timeEvents := constructTimedEventsWithSentMessageEvent(span.StartTimestamp())
	timeEvents.CopyTo(span.Events())
	pcommon.NewMap().CopyTo(span.Attributes())

	segment, _ := MakeSegment(span, resource, nil, false, []string{"my-logGroup-1"})

	cwl := []awsxray.LogGroupMetadata{{
		LogGroup: awsxray.String("my-logGroup-1"),
	}}

	assert.Equal(t, cwl, segment.AWS.CWLogs)
}

func TestSegmentWith2LogGroupsFromConfig(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeError, "OK", attributes)
	timeEvents := constructTimedEventsWithSentMessageEvent(span.StartTimestamp())
	timeEvents.CopyTo(span.Events())
	pcommon.NewMap().CopyTo(span.Attributes())

	segment, _ := MakeSegment(span, resource, nil, false, []string{"my-logGroup-1", "my-logGroup-2"})

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
	attributes := make(map[string]interface{})
	attributes[conventions.AttributeHTTPMethod] = "POST"
	attributes[conventions.AttributeHTTPScheme] = "https"
	attributes[conventions.AttributeHTTPHost] = "payment.amazonaws.com"
	attributes[conventions.AttributeHTTPTarget] = "/"
	attributes[conventions.AttributeRPCService] = "ABC"
	attributes[awsRemoteService] = "PaymentService"

	resource := constructDefaultResource()
	span := constructClientSpan(parentSpanID, spanName, 0, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil)
	assert.Equal(t, "PaymentService", *segment.Name)
	assert.Equal(t, "subsegment", *segment.Type)

	jsonStr, err := MakeSegmentDocumentString(span, resource, nil, false, nil)

	assert.NotNil(t, jsonStr)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(jsonStr, "PaymentService"))
	assert.False(t, strings.Contains(jsonStr, user))
	assert.False(t, strings.Contains(jsonStr, "user"))
}

func TestProducerSpanWithAwsRemoteServiceName(t *testing.T) {
	spanName := "ABC.payment"
	parentSpanID := newSegmentID()
	user := "testingT"
	attributes := make(map[string]interface{})
	attributes[conventions.AttributeHTTPMethod] = "POST"
	attributes[conventions.AttributeHTTPScheme] = "https"
	attributes[conventions.AttributeHTTPHost] = "payment.amazonaws.com"
	attributes[conventions.AttributeHTTPTarget] = "/"
	attributes[conventions.AttributeRPCService] = "ABC"
	attributes[awsRemoteService] = "ProducerService"

	resource := constructDefaultResource()
	span := constructProducerSpan(parentSpanID, spanName, 0, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil)
	assert.Equal(t, "ProducerService", *segment.Name)
	assert.Equal(t, "subsegment", *segment.Type)

	jsonStr, err := MakeSegmentDocumentString(span, resource, nil, false, nil)

	assert.NotNil(t, jsonStr)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(jsonStr, "ProducerService"))
	assert.False(t, strings.Contains(jsonStr, user))
	assert.False(t, strings.Contains(jsonStr, "user"))
}

func TestConsumerSpanWithAwsRemoteServiceName(t *testing.T) {
	spanName := "ABC.payment"
	parentSpanID := newSegmentID()
	user := "testingT"
	attributes := make(map[string]interface{})
	attributes[conventions.AttributeHTTPMethod] = "POST"
	attributes[conventions.AttributeHTTPScheme] = "https"
	attributes[conventions.AttributeHTTPHost] = "payment.amazonaws.com"
	attributes[conventions.AttributeHTTPTarget] = "/"
	attributes[conventions.AttributeRPCService] = "ABC"
	attributes[awsLocalService] = "ConsumerService"

	resource := constructDefaultResource()
	span := constructConsumerSpan(parentSpanID, spanName, 0, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil)
	assert.Equal(t, "ConsumerService", *segment.Name)

	jsonStr, err := MakeSegmentDocumentString(span, resource, nil, false, nil)

	assert.NotNil(t, jsonStr)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(jsonStr, "ConsumerService"))
	assert.False(t, strings.Contains(jsonStr, user))
	assert.False(t, strings.Contains(jsonStr, "user"))
}

func TestServerSpanWithAwsLocalServiceName(t *testing.T) {
	spanName := "ABC.payment"
	parentSpanID := newSegmentID()
	user := "testingT"
	attributes := make(map[string]interface{})
	attributes[conventions.AttributeHTTPMethod] = "POST"
	attributes[conventions.AttributeHTTPScheme] = "https"
	attributes[conventions.AttributeHTTPHost] = "payment.amazonaws.com"
	attributes[conventions.AttributeHTTPTarget] = "/"
	attributes[conventions.AttributeRPCService] = "ABC"
	attributes[awsLocalService] = "PaymentLocalService"
	attributes[awsRemoteService] = "PaymentService"

	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, 0, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false, nil)
	assert.Equal(t, "PaymentLocalService", *segment.Name)

	jsonStr, err := MakeSegmentDocumentString(span, resource, nil, false, nil)

	assert.NotNil(t, jsonStr)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(jsonStr, "PaymentLocalService"))
	assert.False(t, strings.Contains(jsonStr, user))
	assert.False(t, strings.Contains(jsonStr, "user"))
}

func constructClientSpan(parentSpanID pcommon.SpanID, name string, code ptrace.StatusCode, message string, attributes map[string]interface{}) ptrace.Span {
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

func constructServerSpan(parentSpanID pcommon.SpanID, name string, code ptrace.StatusCode, message string, attributes map[string]interface{}) ptrace.Span {
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

func constructConsumerSpan(parentSpanID pcommon.SpanID, name string, code ptrace.StatusCode, message string, attributes map[string]interface{}) ptrace.Span {
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

func constructProducerSpan(parentSpanID pcommon.SpanID, name string, code ptrace.StatusCode, message string, attributes map[string]interface{}) ptrace.Span {
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

func constructSpanAttributes(attributes map[string]interface{}) pcommon.Map {
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
