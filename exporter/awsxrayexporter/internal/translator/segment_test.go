// Copyright 2019, OpenTelemetry Authors
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
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"

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

func TestClientSpanWithAwsSdkClient(t *testing.T) {
	spanName := "AmazonDynamoDB.getItem"
	parentSpanID := newSegmentID()
	user := "testingT"
	attributes := make(map[string]interface{})
	attributes[conventions.AttributeHTTPMethod] = "POST"
	attributes[conventions.AttributeHTTPScheme] = "https"
	attributes[conventions.AttributeHTTPHost] = "dynamodb.us-east-1.amazonaws.com"
	attributes[conventions.AttributeHTTPTarget] = "/"
	attributes[awsxray.AWSServiceAttribute] = "DynamoDB"
	attributes[awsxray.AWSOperationAttribute] = "GetItem"
	attributes[awsxray.AWSRequestIDAttribute] = "18BO1FEPJSSAOGNJEDPTPCMIU7VV4KQNSO5AEMVJF66Q9ASUAAJG"
	attributes[awsxray.AWSTableNameAttribute] = "otel-dev-Testing"
	resource := constructDefaultResource()
	span := constructClientSpan(parentSpanID, spanName, 0, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false)
	assert.Equal(t, "DynamoDB", *segment.Name)
	assert.Equal(t, "aws", *segment.Namespace)
	assert.Equal(t, "subsegment", *segment.Type)

	jsonStr, err := MakeSegmentDocumentString(span, resource, nil, false)

	assert.NotNil(t, jsonStr)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(jsonStr, "DynamoDB"))
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

	segment, _ := MakeSegment(span, resource, nil, false)
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
	attributes[conventions.AttributeHTTPStatusText] = "java.lang.NullPointerException"
	attributes[conventions.AttributeHTTPUserAgent] = userAgent
	attributes[conventions.AttributeEnduserID] = enduser
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, pdata.StatusCodeError, errorMessage, attributes)
	timeEvents := constructTimedEventsWithSentMessageEvent(span.StartTimestamp())
	timeEvents.CopyTo(span.Events())

	segment, _ := MakeSegment(span, resource, nil, false)

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
	attributes[conventions.AttributeHTTPStatusText] = "java.lang.NullPointerException"
	attributes[conventions.AttributeHTTPUserAgent] = userAgent
	attributes[conventions.AttributeEnduserID] = enduser
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, pdata.StatusCodeError, errorMessage, attributes)
	timeEvents := constructTimedEventsWithSentMessageEvent(span.StartTimestamp())
	timeEvents.CopyTo(span.Events())

	segment, _ := MakeSegment(span, resource, nil, false)

	assert.NotNil(t, segment)
	assert.NotNil(t, segment.Cause)
	assert.Equal(t, "signup_aggregator", *segment.Name)
	assert.False(t, *segment.Fault)
	assert.True(t, *segment.Error)
	assert.True(t, *segment.Throttle)
}

func TestServerSpanNoParentId(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := pdata.InvalidSpanID()
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, pdata.StatusCodeOk, "OK", nil)

	segment, _ := MakeSegment(span, resource, nil, false)

	assert.Empty(t, segment.ParentID)
}

func TestSpanNoParentId(t *testing.T) {
	span := pdata.NewSpan()
	span.SetName("my-topic send")
	span.SetTraceID(newTraceID())
	span.SetSpanID(newSegmentID())
	span.SetParentSpanID(pdata.InvalidSpanID())
	span.SetKind(pdata.SpanKindProducer)
	span.SetStartTimestamp(pdata.NewTimestampFromTime(time.Now()))
	span.SetEndTimestamp(pdata.NewTimestampFromTime(time.Now().Add(10)))
	resource := pdata.NewResource()
	segment, _ := MakeSegment(span, resource, nil, false)

	assert.Empty(t, segment.ParentID)
	assert.Nil(t, segment.Type)
}

func TestSpanWithNoStatus(t *testing.T) {
	span := pdata.NewSpan()
	span.SetTraceID(newTraceID())
	span.SetSpanID(newSegmentID())
	span.SetParentSpanID(newSegmentID())
	span.SetKind(pdata.SpanKindServer)
	span.SetStartTimestamp(pdata.NewTimestampFromTime(time.Now()))
	span.SetEndTimestamp(pdata.NewTimestampFromTime(time.Now().Add(10)))

	resource := pdata.NewResource()
	segment, _ := MakeSegment(span, resource, nil, false)
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
	span := constructClientSpan(parentSpanID, spanName, pdata.StatusCodeUnset, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false)

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
	if err := w.Encode(segment); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	testWriters.release(w)
	fmt.Println(jsonStr)
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
	span := constructClientSpan(parentSpanID, spanName, pdata.StatusCodeUnset, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false)

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
	span := constructClientSpan(parentSpanID, spanName, pdata.StatusCodeUnset, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false)

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
	span := constructClientSpan(parentSpanID, spanName, pdata.StatusCodeUnset, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false)

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
	span := constructClientSpan(pdata.InvalidSpanID(), spanName, pdata.StatusCodeUnset, "OK", attributes)
	timeEvents := constructTimedEventsWithSentMessageEvent(span.StartTimestamp())
	timeEvents.CopyTo(span.Events())
	traceID := span.TraceID().Bytes()
	traceID[0] = 0x11
	span.SetTraceID(pdata.NewTraceID(traceID))

	_, err := MakeSegmentDocumentString(span, resource, nil, false)

	assert.NotNil(t, err)
}

func TestSpanWithExpiredTraceId(t *testing.T) {
	// First Build expired TraceId
	const maxAge = 60 * 60 * 24 * 30
	ExpiredEpoch := time.Now().Unix() - maxAge - 1

	tempTraceID := newTraceID().Bytes()
	binary.BigEndian.PutUint32(tempTraceID[0:4], uint32(ExpiredEpoch))

	_, err := convertToAmazonTraceID(pdata.NewTraceID(tempTraceID))
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
	span := constructServerSpan(parentSpanID, spanName, pdata.StatusCodeError, "OK", attributes)
	timeEvents := constructTimedEventsWithSentMessageEvent(span.StartTimestamp())
	timeEvents.CopyTo(span.Events())
	pdata.NewAttributeMap().CopyTo(span.Attributes())

	segment, _ := MakeSegment(span, resource, nil, false)

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
	span := constructServerSpan(parentSpanID, spanName, pdata.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, nil, false)

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
	span := constructClientSpan(parentSpanID, spanName, pdata.StatusCodeError, "ERROR", attributes)

	segment, _ := MakeSegment(span, resource, nil, false)

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
	span := constructServerSpan(parentSpanID, spanName, pdata.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{"attr1@1", "not_exist"}, false)

	assert.NotNil(t, segment)
	assert.Equal(t, 1, len(segment.Annotations))
	assert.Equal(t, "val1", segment.Annotations["attr1_1"])
	assert.Equal(t, "val2", segment.Metadata["default"]["attr2@2"])
}

func TestSpanWithAttributesAllIndexed(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	attributes["attr1@1"] = "val1"
	attributes["attr2@2"] = "val2"
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, pdata.StatusCodeOk, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{"attr1@1", "not_exist"}, true)

	assert.NotNil(t, segment)
	assert.Equal(t, "val1", segment.Annotations["attr1_1"])
	assert.Equal(t, "val2", segment.Annotations["attr2_2"])
}

func TestResourceAttributesCanBeIndexed(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, pdata.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{
		"otel.resource.string.key",
		"otel.resource.int.key",
		"otel.resource.double.key",
		"otel.resource.bool.key",
		"otel.resource.map.key",
		"otel.resource.array.key",
	}, false)

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
	span := constructClientSpan(parentSpanID, spanName, pdata.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{
		"otel.resource.string.key",
		"otel.resource.int.key",
		"otel.resource.double.key",
		"otel.resource.bool.key",
		"otel.resource.map.key",
		"otel.resource.array.key",
	}, false)

	assert.NotNil(t, segment)
	assert.Empty(t, segment.Annotations)
	assert.Empty(t, segment.Metadata)
}

func TestOriginNotAws(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := pdata.NewResource()
	attrs := pdata.NewAttributeMap()
	attrs.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderGCP)
	attrs.InsertString(conventions.AttributeHostID, "instance-123")
	attrs.CopyTo(resource.Attributes())
	span := constructServerSpan(parentSpanID, spanName, pdata.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false)

	assert.NotNil(t, segment)
	assert.Nil(t, segment.Origin)
}

func TestOriginEc2(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := pdata.NewResource()
	attrs := pdata.NewAttributeMap()
	attrs.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.InsertString(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSEC2)
	attrs.InsertString(conventions.AttributeHostID, "instance-123")
	attrs.CopyTo(resource.Attributes())
	span := constructServerSpan(parentSpanID, spanName, pdata.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false)

	assert.NotNil(t, segment)
	assert.Equal(t, OriginEC2, *segment.Origin)
}

func TestOriginEcs(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := pdata.NewResource()
	attrs := pdata.NewAttributeMap()
	attrs.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.InsertString(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSECS)
	attrs.InsertString(conventions.AttributeHostID, "instance-123")
	attrs.InsertString(conventions.AttributeContainerName, "container-123")
	attrs.CopyTo(resource.Attributes())
	span := constructServerSpan(parentSpanID, spanName, pdata.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false)

	assert.NotNil(t, segment)
	assert.Equal(t, OriginECS, *segment.Origin)
}

func TestOriginEcsEc2(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := pdata.NewResource()
	attrs := pdata.NewAttributeMap()
	attrs.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.InsertString(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSECS)
	attrs.InsertString(conventions.AttributeAWSECSLaunchtype, conventions.AttributeAWSECSLaunchtypeEC2)
	attrs.InsertString(conventions.AttributeHostID, "instance-123")
	attrs.InsertString(conventions.AttributeContainerName, "container-123")
	attrs.CopyTo(resource.Attributes())
	span := constructServerSpan(parentSpanID, spanName, pdata.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false)

	assert.NotNil(t, segment)
	assert.Equal(t, OriginECSEC2, *segment.Origin)
}

func TestOriginEcsFargate(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := pdata.NewResource()
	attrs := pdata.NewAttributeMap()
	attrs.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.InsertString(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSECS)
	attrs.InsertString(conventions.AttributeAWSECSLaunchtype, conventions.AttributeAWSECSLaunchtypeFargate)
	attrs.InsertString(conventions.AttributeHostID, "instance-123")
	attrs.InsertString(conventions.AttributeContainerName, "container-123")
	attrs.CopyTo(resource.Attributes())
	span := constructServerSpan(parentSpanID, spanName, pdata.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false)

	assert.NotNil(t, segment)
	assert.Equal(t, OriginECSFargate, *segment.Origin)
}

func TestOriginEb(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := pdata.NewResource()
	attrs := pdata.NewAttributeMap()
	attrs.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.InsertString(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSElasticBeanstalk)
	attrs.InsertString(conventions.AttributeHostID, "instance-123")
	attrs.InsertString(conventions.AttributeContainerName, "container-123")
	attrs.InsertString(conventions.AttributeServiceInstanceID, "service-123")
	attrs.CopyTo(resource.Attributes())
	span := constructServerSpan(parentSpanID, spanName, pdata.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false)

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
	resource := pdata.NewResource()
	attrs := pdata.NewAttributeMap()
	attrs.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.InsertString(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSEKS)
	attrs.InsertString(conventions.AttributeCloudAccountID, "123456789")
	attrs.InsertString(conventions.AttributeCloudAvailabilityZone, "us-east-1c")
	attrs.InsertString(conventions.AttributeContainerImageName, "otel/signupaggregator")
	attrs.InsertString(conventions.AttributeContainerImageTag, "v1")
	attrs.InsertString(conventions.AttributeK8SClusterName, "production")
	attrs.InsertString(conventions.AttributeK8SNamespaceName, "default")
	attrs.InsertString(conventions.AttributeK8SDeploymentName, "signup_aggregator")
	attrs.InsertString(conventions.AttributeK8SPodName, "my-deployment-65dcf7d447-ddjnl")
	attrs.InsertString(conventions.AttributeContainerName, containerName)
	attrs.InsertString(conventions.AttributeContainerID, containerID)
	attrs.InsertString(conventions.AttributeHostID, instanceID)
	attrs.InsertString(conventions.AttributeHostType, "m5.xlarge")
	attrs.CopyTo(resource.Attributes())
	span := constructServerSpan(parentSpanID, spanName, pdata.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false)

	assert.NotNil(t, segment)
	assert.Equal(t, OriginEKS, *segment.Origin)
}

func TestOriginBlank(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := pdata.NewResource()
	attrs := pdata.NewAttributeMap()
	attrs.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.CopyTo(resource.Attributes())
	span := constructServerSpan(parentSpanID, spanName, pdata.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false)

	assert.NotNil(t, segment)
	assert.Nil(t, segment.Origin)
}

func TestOriginPrefersInfraService(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := pdata.NewResource()
	attrs := pdata.NewAttributeMap()
	attrs.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.InsertString(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSEC2)
	attrs.InsertString(conventions.AttributeK8SClusterName, "cluster-123")
	attrs.InsertString(conventions.AttributeHostID, "instance-123")
	attrs.InsertString(conventions.AttributeContainerName, "container-123")
	attrs.InsertString(conventions.AttributeServiceInstanceID, "service-123")
	attrs.CopyTo(resource.Attributes())
	span := constructServerSpan(parentSpanID, spanName, pdata.StatusCodeError, "OK", attributes)

	segment, _ := MakeSegment(span, resource, []string{}, false)

	assert.NotNil(t, segment)
	assert.Equal(t, OriginEC2, *segment.Origin)
}

func TestFilteredAttributesMetadata(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := pdata.NewResource()

	attrs := pdata.NewAttributeMap()
	attrs.InsertString("string_value", "value")
	attrs.InsertInt("int_value", 123)
	attrs.InsertDouble("float_value", 456.78)
	attrs.InsertBool("bool_value", false)
	attrs.InsertNull("null_value")

	arrayValue := pdata.NewAttributeValueArray()
	arrayValue.ArrayVal().AppendEmpty().SetIntVal(12)
	arrayValue.ArrayVal().AppendEmpty().SetIntVal(34)
	arrayValue.ArrayVal().AppendEmpty().SetIntVal(56)
	attrs.Insert("array_value", arrayValue)

	mapValue := pdata.NewAttributeValueMap()
	mapValue.MapVal().InsertDouble("value1", -987.65)
	mapValue.MapVal().InsertBool("value2", true)
	attrs.Insert("map_value", mapValue)

	span := constructServerSpan(parentSpanID, spanName, pdata.StatusCodeError, "OK", attributes)
	attrs.CopyTo(span.Attributes())

	segment, _ := MakeSegment(span, resource, []string{}, false)

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

func constructClientSpan(parentSpanID pdata.SpanID, name string, code pdata.StatusCode, message string, attributes map[string]interface{}) pdata.Span {
	var (
		traceID        = newTraceID()
		spanID         = newSegmentID()
		endTime        = time.Now()
		startTime      = endTime.Add(-215 * time.Millisecond)
		spanAttributes = constructSpanAttributes(attributes)
	)

	span := pdata.NewSpan()
	span.SetTraceID(traceID)
	span.SetSpanID(spanID)
	span.SetParentSpanID(parentSpanID)
	span.SetName(name)
	span.SetKind(pdata.SpanKindClient)
	span.SetStartTimestamp(pdata.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pdata.NewTimestampFromTime(endTime))

	status := pdata.NewSpanStatus()
	status.SetCode(code)
	status.SetMessage(message)
	status.CopyTo(span.Status())

	spanAttributes.CopyTo(span.Attributes())
	return span
}

func constructServerSpan(parentSpanID pdata.SpanID, name string, code pdata.StatusCode, message string, attributes map[string]interface{}) pdata.Span {
	var (
		traceID        = newTraceID()
		spanID         = newSegmentID()
		endTime        = time.Now()
		startTime      = endTime.Add(-215 * time.Millisecond)
		spanAttributes = constructSpanAttributes(attributes)
	)

	span := pdata.NewSpan()
	span.SetTraceID(traceID)
	span.SetSpanID(spanID)
	span.SetParentSpanID(parentSpanID)
	span.SetName(name)
	span.SetKind(pdata.SpanKindServer)
	span.SetStartTimestamp(pdata.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pdata.NewTimestampFromTime(endTime))

	status := pdata.NewSpanStatus()
	status.SetCode(code)
	status.SetMessage(message)
	status.CopyTo(span.Status())

	spanAttributes.CopyTo(span.Attributes())
	return span
}

func constructSpanAttributes(attributes map[string]interface{}) pdata.AttributeMap {
	attrs := pdata.NewAttributeMap()
	for key, value := range attributes {
		if cast, ok := value.(int); ok {
			attrs.InsertInt(key, int64(cast))
		} else if cast, ok := value.(int64); ok {
			attrs.InsertInt(key, cast)
		} else {
			attrs.InsertString(key, fmt.Sprintf("%v", value))
		}
	}
	return attrs
}

func constructDefaultResource() pdata.Resource {
	resource := pdata.NewResource()
	attrs := pdata.NewAttributeMap()
	attrs.InsertString(conventions.AttributeServiceName, "signup_aggregator")
	attrs.InsertString(conventions.AttributeServiceVersion, "semver:1.1.4")
	attrs.InsertString(conventions.AttributeContainerName, "signup_aggregator")
	attrs.InsertString(conventions.AttributeContainerImageName, "otel/signupaggregator")
	attrs.InsertString(conventions.AttributeContainerImageTag, "v1")
	attrs.InsertString(conventions.AttributeK8SClusterName, "production")
	attrs.InsertString(conventions.AttributeK8SNamespaceName, "default")
	attrs.InsertString(conventions.AttributeK8SDeploymentName, "signup_aggregator")
	attrs.InsertString(conventions.AttributeK8SPodName, "signup_aggregator-x82ufje83")
	attrs.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.InsertString(conventions.AttributeCloudAccountID, "123456789")
	attrs.InsertString(conventions.AttributeCloudRegion, "us-east-1")
	attrs.InsertString(conventions.AttributeCloudAvailabilityZone, "us-east-1c")
	attrs.InsertString(resourceStringKey, "string")
	attrs.InsertInt(resourceIntKey, 10)
	attrs.InsertDouble(resourceDoubleKey, 5.0)
	attrs.InsertBool(resourceBoolKey, true)

	resourceMapVal := pdata.NewAttributeValueMap()
	resourceMap := resourceMapVal.MapVal()
	resourceMap.InsertInt("key1", 1)
	resourceMap.InsertString("key2", "value")
	attrs.Insert(resourceMapKey, resourceMapVal)

	resourceArrayVal := pdata.NewAttributeValueArray()
	resourceArray := resourceArrayVal.ArrayVal()
	resourceArray.AppendEmpty().SetStringVal("foo")
	resourceArray.AppendEmpty().SetStringVal("bar")
	attrs.Insert(resourceArrayKey, resourceArrayVal)
	attrs.CopyTo(resource.Attributes())
	return resource
}

func constructTimedEventsWithReceivedMessageEvent(tm pdata.Timestamp) pdata.SpanEventSlice {
	eventAttr := pdata.NewAttributeMap()
	eventAttr.InsertString(conventions.AttributeMessageType, "RECEIVED")
	eventAttr.InsertInt(conventions.AttributeMessagingMessageID, 1)
	eventAttr.InsertInt(conventions.AttributeMessagingMessagePayloadCompressedSizeBytes, 6478)
	eventAttr.InsertInt(conventions.AttributeMessagingMessagePayloadSizeBytes, 12452)

	event := pdata.NewSpanEvent()
	event.SetTimestamp(tm)
	eventAttr.CopyTo(event.Attributes())
	event.SetDroppedAttributesCount(0)

	events := pdata.NewSpanEventSlice()
	event.CopyTo(events.AppendEmpty())
	return events
}

func constructTimedEventsWithSentMessageEvent(tm pdata.Timestamp) pdata.SpanEventSlice {
	eventAttr := pdata.NewAttributeMap()
	eventAttr.InsertString(conventions.AttributeMessageType, "SENT")
	eventAttr.InsertInt(conventions.AttributeMessagingMessageID, 1)
	eventAttr.InsertInt(conventions.AttributeMessagingMessagePayloadSizeBytes, 7480)

	event := pdata.NewSpanEvent()
	event.SetTimestamp(tm)
	eventAttr.CopyTo(event.Attributes())
	event.SetDroppedAttributesCount(0)

	events := pdata.NewSpanEventSlice()
	event.CopyTo(events.AppendEmpty())
	return events
}

// newTraceID generates a new valid X-Ray TraceID
func newTraceID() pdata.TraceID {
	var r [16]byte
	epoch := time.Now().Unix()
	binary.BigEndian.PutUint32(r[0:4], uint32(epoch))
	_, err := rand.Read(r[4:])
	if err != nil {
		panic(err)
	}
	return pdata.NewTraceID(r)
}
