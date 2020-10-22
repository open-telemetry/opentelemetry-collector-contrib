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
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
	semconventions "go.opentelemetry.io/collector/translator/conventions"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/awsxray"
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
	attributes[semconventions.AttributeComponent] = semconventions.ComponentTypeHTTP
	attributes[semconventions.AttributeHTTPMethod] = "POST"
	attributes[semconventions.AttributeHTTPScheme] = "https"
	attributes[semconventions.AttributeHTTPHost] = "dynamodb.us-east-1.amazonaws.com"
	attributes[semconventions.AttributeHTTPTarget] = "/"
	attributes[awsxray.AWSServiceAttribute] = "DynamoDB"
	attributes[awsxray.AWSOperationAttribute] = "GetItem"
	attributes[awsxray.AWSRequestIDAttribute] = "18BO1FEPJSSAOGNJEDPTPCMIU7VV4KQNSO5AEMVJF66Q9ASUAAJG"
	attributes[awsxray.AWSTableNameAttribute] = "otel-dev-Testing"
	resource := constructDefaultResource()
	span := constructClientSpan(parentSpanID, spanName, 0, "OK", attributes)

	segment := MakeSegment(span, resource, nil, false)
	assert.Equal(t, "DynamoDB", *segment.Name)
	assert.Equal(t, "aws", *segment.Namespace)
	assert.Equal(t, "subsegment", *segment.Type)

	jsonStr, err := MakeSegmentDocumentString(span, resource, nil, false)

	assert.NotNil(t, jsonStr)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(jsonStr, "DynamoDB"))
	assert.False(t, strings.Contains(jsonStr, user))
	assert.False(t, strings.Contains(jsonStr, semconventions.AttributeComponent))
	assert.False(t, strings.Contains(jsonStr, "user"))
}

func TestClientSpanWithPeerService(t *testing.T) {
	spanName := "AmazonDynamoDB.getItem"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	attributes[semconventions.AttributeComponent] = semconventions.ComponentTypeHTTP
	attributes[semconventions.AttributeHTTPMethod] = "POST"
	attributes[semconventions.AttributeHTTPScheme] = "https"
	attributes[semconventions.AttributeHTTPHost] = "dynamodb.us-east-1.amazonaws.com"
	attributes[semconventions.AttributeHTTPTarget] = "/"
	attributes[semconventions.AttributePeerService] = "cats-table"
	attributes[awsxray.AWSServiceAttribute] = "DynamoDB"
	attributes[awsxray.AWSOperationAttribute] = "GetItem"
	attributes[awsxray.AWSRequestIDAttribute] = "18BO1FEPJSSAOGNJEDPTPCMIU7VV4KQNSO5AEMVJF66Q9ASUAAJG"
	attributes[awsxray.AWSTableNameAttribute] = "otel-dev-Testing"
	resource := constructDefaultResource()
	span := constructClientSpan(parentSpanID, spanName, 0, "OK", attributes)

	segment := MakeSegment(span, resource, nil, false)
	assert.Equal(t, "cats-table", *segment.Name)
}

func TestServerSpanWithInternalServerError(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	errorMessage := "java.lang.NullPointerException"
	userAgent := "PostmanRuntime/7.21.0"
	enduser := "go.tester@example.com"
	attributes := make(map[string]interface{})
	attributes[semconventions.AttributeComponent] = semconventions.ComponentTypeHTTP
	attributes[semconventions.AttributeHTTPMethod] = "POST"
	attributes[semconventions.AttributeHTTPURL] = "https://api.example.org/api/locations"
	attributes[semconventions.AttributeHTTPTarget] = "/api/locations"
	attributes[semconventions.AttributeHTTPStatusCode] = 500
	attributes[semconventions.AttributeHTTPStatusText] = "java.lang.NullPointerException"
	attributes[semconventions.AttributeHTTPUserAgent] = userAgent
	attributes[semconventions.AttributeEnduserID] = enduser
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, tracetranslator.OCInternal, errorMessage, attributes)
	timeEvents := constructTimedEventsWithSentMessageEvent(span.StartTime())
	timeEvents.CopyTo(span.Events())

	segment := MakeSegment(span, resource, nil, false)

	assert.NotNil(t, segment)
	assert.NotNil(t, segment.Cause)
	assert.Equal(t, "signup_aggregator", *segment.Name)
	assert.True(t, *segment.Fault)
	w := testWriters.borrow()
	if err := w.Encode(segment); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	testWriters.release(w)
	assert.True(t, strings.Contains(jsonStr, spanName))
	assert.True(t, strings.Contains(jsonStr, errorMessage))
	assert.True(t, strings.Contains(jsonStr, userAgent))
	assert.True(t, strings.Contains(jsonStr, enduser))
}

func TestServerSpanNoParentId(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := pdata.NewSpanID([]byte{0, 0, 0, 0, 0, 0, 0, 0})
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, 0, "OK", nil)

	segment := MakeSegment(span, resource, nil, false)

	assert.Empty(t, segment.ParentID)
}

func TestSpanWithNoStatus(t *testing.T) {
	span := pdata.NewSpan()
	span.InitEmpty()
	span.SetTraceID(newTraceID())
	span.SetSpanID(newSegmentID())
	span.SetParentSpanID(newSegmentID())
	span.SetKind(pdata.SpanKindSERVER)
	span.SetStartTime(pdata.TimestampUnixNano(time.Now().UnixNano()))
	span.SetEndTime(pdata.TimestampUnixNano(time.Now().Add(10).UnixNano()))

	resource := pdata.NewResource()
	resource.InitEmpty()
	segment := MakeSegment(span, resource, nil, false)
	assert.NotNil(t, segment)
}

func TestClientSpanWithDbComponent(t *testing.T) {
	spanName := "call update_user_preference( ?, ?, ? )"
	parentSpanID := newSegmentID()
	enterpriseAppID := "25F2E73B-4769-4C79-9DF3-7EBE85D571EA"
	attributes := make(map[string]interface{})
	attributes[semconventions.AttributeDBSystem] = "mysql"
	attributes[semconventions.AttributeDBName] = "customers"
	attributes[semconventions.AttributeDBStatement] = spanName
	attributes[semconventions.AttributeDBUser] = "userprefsvc"
	attributes[semconventions.AttributeDBConnectionString] = "mysql://db.dev.example.com:3306"
	attributes[semconventions.AttributeNetPeerName] = "db.dev.example.com"
	attributes[semconventions.AttributeNetPeerPort] = "3306"
	attributes["enterprise.app.id"] = enterpriseAppID
	resource := constructDefaultResource()
	span := constructClientSpan(parentSpanID, spanName, 0, "OK", attributes)

	segment := MakeSegment(span, resource, nil, false)

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
	attributes[semconventions.AttributeHTTPMethod] = "GET"
	attributes[semconventions.AttributeHTTPScheme] = "https"
	attributes[semconventions.AttributeNetPeerIP] = "2607:f8b0:4000:80c::2004"
	attributes[semconventions.AttributeNetPeerPort] = "9443"
	attributes[semconventions.AttributeHTTPTarget] = "/"
	attributes[semconventions.AttributeHTTPHost] = "foo.com"
	attributes[semconventions.AttributeNetPeerName] = "bar.com"
	resource := constructDefaultResource()
	span := constructClientSpan(parentSpanID, spanName, 0, "OK", attributes)

	segment := MakeSegment(span, resource, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, "foo.com", *segment.Name)
}

func TestClientSpanWithoutHttpHost(t *testing.T) {
	spanName := "GET /"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	attributes[semconventions.AttributeHTTPMethod] = "GET"
	attributes[semconventions.AttributeHTTPScheme] = "https"
	attributes[semconventions.AttributeNetPeerIP] = "2607:f8b0:4000:80c::2004"
	attributes[semconventions.AttributeNetPeerPort] = "9443"
	attributes[semconventions.AttributeHTTPTarget] = "/"
	attributes[semconventions.AttributeNetPeerName] = "bar.com"
	resource := constructDefaultResource()
	span := constructClientSpan(parentSpanID, spanName, 0, "OK", attributes)

	segment := MakeSegment(span, resource, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, "bar.com", *segment.Name)
}

func TestClientSpanWithRpcHost(t *testing.T) {
	spanName := "GET /com.foo.AnimalService/GetCats"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	attributes[semconventions.AttributeHTTPMethod] = "GET"
	attributes[semconventions.AttributeHTTPScheme] = "https"
	attributes[semconventions.AttributeNetPeerIP] = "2607:f8b0:4000:80c::2004"
	attributes[semconventions.AttributeNetPeerPort] = "9443"
	attributes[semconventions.AttributeHTTPTarget] = "/com.foo.AnimalService/GetCats"
	attributes[semconventions.AttributeRPCService] = "com.foo.AnimalService"
	attributes[semconventions.AttributeNetPeerName] = "bar.com"
	resource := constructDefaultResource()
	span := constructClientSpan(parentSpanID, spanName, 0, "OK", attributes)

	segment := MakeSegment(span, resource, nil, false)

	assert.NotNil(t, segment)
	assert.Equal(t, "com.foo.AnimalService", *segment.Name)
}

func TestSpanWithInvalidTraceId(t *testing.T) {
	spanName := "platformapi.widgets.searchWidgets"
	attributes := make(map[string]interface{})
	attributes[semconventions.AttributeComponent] = semconventions.ComponentTypeGRPC
	attributes[semconventions.AttributeHTTPMethod] = "GET"
	attributes[semconventions.AttributeHTTPScheme] = "ipv6"
	attributes[semconventions.AttributeNetPeerIP] = "2607:f8b0:4000:80c::2004"
	attributes[semconventions.AttributeNetPeerPort] = "9443"
	attributes[semconventions.AttributeHTTPTarget] = spanName
	resource := constructDefaultResource()
	span := constructClientSpan(pdata.NewSpanID(nil), spanName, 0, "OK", attributes)
	timeEvents := constructTimedEventsWithSentMessageEvent(span.StartTime())
	timeEvents.CopyTo(span.Events())
	traceID := span.TraceID().Bytes()
	traceID[0] = 0x11
	span.SetTraceID(pdata.NewTraceID(traceID))

	jsonStr, err := MakeSegmentDocumentString(span, resource, nil, false)

	assert.NotNil(t, jsonStr)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(jsonStr, spanName))
	assert.False(t, strings.Contains(jsonStr, "1-11"))
	assert.False(t, strings.Contains(jsonStr, "user"))
}

func TestSpanWithExpiredTraceId(t *testing.T) {
	// First Build expired TraceId
	const maxAge = 60 * 60 * 24 * 30
	ExpiredEpoch := time.Now().Unix() - maxAge - 1

	TempTraceID := newTraceID()
	binary.BigEndian.PutUint32(TempTraceID.Bytes()[0:4], uint32(ExpiredEpoch))

	PrevEpoch := uint32(time.Now().Unix())

	ResTraceID := convertToAmazonTraceID(TempTraceID)
	BinaryCurEpoch, err := hex.DecodeString(ResTraceID[2:10])
	if err != nil {
		panic(err)
	}
	CurEpoch := binary.BigEndian.Uint32(BinaryCurEpoch)

	assert.GreaterOrEqual(t, CurEpoch, PrevEpoch)
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
	span := constructServerSpan(parentSpanID, spanName, tracetranslator.OCInternal, "OK", attributes)
	timeEvents := constructTimedEventsWithSentMessageEvent(span.StartTime())
	timeEvents.CopyTo(span.Events())
	pdata.NewAttributeMap().CopyTo(span.Attributes())

	segment := MakeSegment(span, resource, nil, false)

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
	span := constructServerSpan(parentSpanID, spanName, tracetranslator.OCInternal, "OK", attributes)

	segment := MakeSegment(span, resource, nil, false)

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
	span := constructClientSpan(parentSpanID, spanName, tracetranslator.OCInternal, "OK", attributes)

	segment := MakeSegment(span, resource, nil, false)

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
	span := constructServerSpan(parentSpanID, spanName, tracetranslator.OCInternal, "OK", attributes)

	segment := MakeSegment(span, resource, []string{"attr1@1", "not_exist"}, false)

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
	span := constructServerSpan(parentSpanID, spanName, tracetranslator.OCInternal, "OK", attributes)

	segment := MakeSegment(span, resource, []string{"attr1@1", "not_exist"}, true)

	assert.NotNil(t, segment)
	assert.Equal(t, "val1", segment.Annotations["attr1_1"])
	assert.Equal(t, "val2", segment.Annotations["attr2_2"])
}

func TestResourceAttributesCanBeIndexed(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, tracetranslator.OCInternal, "OK", attributes)

	segment := MakeSegment(span, resource, []string{
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
	span := constructClientSpan(parentSpanID, spanName, tracetranslator.OCInternal, "OK", attributes)

	segment := MakeSegment(span, resource, []string{
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
	resource.InitEmpty()
	attrs := pdata.NewAttributeMap()
	attrs.InsertString(semconventions.AttributeCloudProvider, semconventions.AttributeCloudProviderGCP)
	attrs.InsertString(semconventions.AttributeHostID, "instance-123")
	attrs.CopyTo(resource.Attributes())
	span := constructServerSpan(parentSpanID, spanName, tracetranslator.OCInternal, "OK", attributes)

	segment := MakeSegment(span, resource, []string{}, false)

	assert.NotNil(t, segment)
	assert.Nil(t, segment.Origin)
}

func TestOriginEc2(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := pdata.NewResource()
	resource.InitEmpty()
	attrs := pdata.NewAttributeMap()
	attrs.InsertString(semconventions.AttributeCloudProvider, semconventions.AttributeCloudProviderAWS)
	attrs.InsertString(semconventions.AttributeHostID, "instance-123")
	attrs.CopyTo(resource.Attributes())
	span := constructServerSpan(parentSpanID, spanName, tracetranslator.OCInternal, "OK", attributes)

	segment := MakeSegment(span, resource, []string{}, false)

	assert.NotNil(t, segment)
	assert.Equal(t, OriginEC2, *segment.Origin)
}

func TestOriginEcs(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := pdata.NewResource()
	resource.InitEmpty()
	attrs := pdata.NewAttributeMap()
	attrs.InsertString(semconventions.AttributeCloudProvider, semconventions.AttributeCloudProviderAWS)
	attrs.InsertString(semconventions.AttributeHostID, "instance-123")
	attrs.InsertString(semconventions.AttributeContainerName, "container-123")
	attrs.CopyTo(resource.Attributes())
	span := constructServerSpan(parentSpanID, spanName, tracetranslator.OCInternal, "OK", attributes)

	segment := MakeSegment(span, resource, []string{}, false)

	assert.NotNil(t, segment)
	assert.Equal(t, OriginECS, *segment.Origin)
}

func TestOriginEb(t *testing.T) {
	spanName := "/test"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := pdata.NewResource()
	resource.InitEmpty()
	attrs := pdata.NewAttributeMap()
	attrs.InsertString(semconventions.AttributeCloudProvider, semconventions.AttributeCloudProviderAWS)
	attrs.InsertString(semconventions.AttributeHostID, "instance-123")
	attrs.InsertString(semconventions.AttributeContainerName, "container-123")
	attrs.InsertString(semconventions.AttributeServiceInstance, "service-123")
	attrs.CopyTo(resource.Attributes())
	span := constructServerSpan(parentSpanID, spanName, tracetranslator.OCInternal, "OK", attributes)

	segment := MakeSegment(span, resource, []string{}, false)

	assert.NotNil(t, segment)
	assert.Equal(t, OriginEB, *segment.Origin)
}

func constructClientSpan(parentSpanID pdata.SpanID, name string, code int32, message string, attributes map[string]interface{}) pdata.Span {
	var (
		traceID        = newTraceID()
		spanID         = newSegmentID()
		endTime        = time.Now()
		startTime      = endTime.Add(-215 * time.Millisecond)
		spanAttributes = constructSpanAttributes(attributes)
	)

	span := pdata.NewSpan()
	span.InitEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID(spanID)
	span.SetParentSpanID(parentSpanID)
	span.SetName(name)
	span.SetKind(pdata.SpanKindCLIENT)
	span.SetStartTime(pdata.TimestampUnixNano(startTime.UnixNano()))
	span.SetEndTime(pdata.TimestampUnixNano(endTime.UnixNano()))

	status := pdata.NewSpanStatus()
	status.InitEmpty()
	status.SetCode(pdata.StatusCode(code))
	status.SetMessage(message)
	status.CopyTo(span.Status())

	spanAttributes.CopyTo(span.Attributes())
	return span
}

func constructServerSpan(parentSpanID pdata.SpanID, name string, code int32, message string, attributes map[string]interface{}) pdata.Span {
	var (
		traceID        = newTraceID()
		spanID         = newSegmentID()
		endTime        = time.Now()
		startTime      = endTime.Add(-215 * time.Millisecond)
		spanAttributes = constructSpanAttributes(attributes)
	)

	span := pdata.NewSpan()
	span.InitEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID(spanID)
	span.SetParentSpanID(parentSpanID)
	span.SetName(name)
	span.SetKind(pdata.SpanKindSERVER)
	span.SetStartTime(pdata.TimestampUnixNano(startTime.UnixNano()))
	span.SetEndTime(pdata.TimestampUnixNano(endTime.UnixNano()))

	status := pdata.NewSpanStatus()
	status.InitEmpty()
	status.SetCode(pdata.StatusCode(code))
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
	resource.InitEmpty()
	attrs := pdata.NewAttributeMap()
	attrs.InsertString(semconventions.AttributeServiceName, "signup_aggregator")
	attrs.InsertString(semconventions.AttributeServiceVersion, "semver:1.1.4")
	attrs.InsertString(semconventions.AttributeContainerName, "signup_aggregator")
	attrs.InsertString(semconventions.AttributeContainerImage, "otel/signupaggregator")
	attrs.InsertString(semconventions.AttributeContainerTag, "v1")
	attrs.InsertString(semconventions.AttributeK8sCluster, "production")
	attrs.InsertString(semconventions.AttributeK8sNamespace, "default")
	attrs.InsertString(semconventions.AttributeK8sDeployment, "signup_aggregator")
	attrs.InsertString(semconventions.AttributeK8sPod, "signup_aggregator-x82ufje83")
	attrs.InsertString(semconventions.AttributeCloudProvider, semconventions.AttributeCloudProviderAWS)
	attrs.InsertString(semconventions.AttributeCloudAccount, "123456789")
	attrs.InsertString(semconventions.AttributeCloudRegion, "us-east-1")
	attrs.InsertString(semconventions.AttributeCloudZone, "us-east-1c")
	attrs.InsertString(resourceStringKey, "string")
	attrs.InsertInt(resourceIntKey, 10)
	attrs.InsertDouble(resourceDoubleKey, 5.0)
	attrs.InsertBool(resourceBoolKey, true)
	resourceMap := pdata.NewAttributeMap()
	resourceMap.InitEmptyWithCapacity(2)
	resourceMap.InsertInt("key1", 1)
	resourceMap.InsertString("key2", "value")
	resourceMapVal := pdata.NewAttributeValue()
	resourceMapVal.InitEmpty()
	resourceMapVal.SetMapVal(resourceMap)
	attrs.Insert(resourceMapKey, resourceMapVal)
	resourceArray := pdata.NewAnyValueArray()
	val1 := pdata.NewAttributeValue()
	val1.InitEmpty()
	val1.SetStringVal("foo")
	val2 := pdata.NewAttributeValue()
	val2.InitEmpty()
	val2.SetStringVal("bar")
	resourceArray.Append(val1)
	resourceArray.Append(val2)
	resourceArrayVal := pdata.NewAttributeValue()
	resourceArrayVal.InitEmpty()
	resourceArrayVal.SetArrayVal(resourceArray)
	attrs.Insert(resourceArrayKey, resourceArrayVal)
	attrs.CopyTo(resource.Attributes())
	return resource
}

func constructTimedEventsWithReceivedMessageEvent(tm pdata.TimestampUnixNano) pdata.SpanEventSlice {
	eventAttr := pdata.NewAttributeMap()
	eventAttr.InsertString(semconventions.AttributeMessageType, "RECEIVED")
	eventAttr.InsertInt(semconventions.AttributeMessageID, 1)
	eventAttr.InsertInt(semconventions.AttributeMessageCompressedSize, 6478)
	eventAttr.InsertInt(semconventions.AttributeMessageUncompressedSize, 12452)

	event := pdata.NewSpanEvent()
	event.InitEmpty()
	event.SetTimestamp(tm)
	eventAttr.CopyTo(event.Attributes())
	event.SetDroppedAttributesCount(0)

	events := pdata.NewSpanEventSlice()
	events.Resize(1)
	event.CopyTo(events.At(0))
	return events
}

func constructTimedEventsWithSentMessageEvent(tm pdata.TimestampUnixNano) pdata.SpanEventSlice {
	eventAttr := pdata.NewAttributeMap()
	eventAttr.InsertString(semconventions.AttributeMessageType, "SENT")
	eventAttr.InsertInt(semconventions.AttributeMessageID, 1)
	eventAttr.InsertInt(semconventions.AttributeMessageUncompressedSize, 7480)

	event := pdata.NewSpanEvent()
	event.InitEmpty()
	event.SetTimestamp(tm)
	eventAttr.CopyTo(event.Attributes())
	event.SetDroppedAttributesCount(0)

	events := pdata.NewSpanEventSlice()
	events.Resize(1)
	event.CopyTo(events.At(0))
	return events
}
