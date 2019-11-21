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

package azuremonitorexporter

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"

	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/stretchr/testify/mock"

	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	timestamp "github.com/golang/protobuf/ptypes/timestamp"
)

var (
	defaultTraceID      = []byte{35, 191, 77, 229, 162, 242, 217, 75, 148, 170, 81, 99, 227, 163, 145, 25}
	defaultTraceIDAsHex = toHex(defaultTraceID)
	defaultSpanID       = []byte{35, 191, 77, 229, 162, 242, 217, 75, 148, 170, 81, 99, 227, 163, 145, 26}
	defaultSpanIDAsHex  = toHex(defaultSpanID)
	defaultParentSpanID = []byte{35, 191, 77, 229, 162, 242, 217, 75, 148, 170, 81, 99, 227, 163, 145, 27}
)

func TestToHex(t *testing.T) {
	assert.Equal(t, "", toHex(nil))

	bytes := []byte{35, 191, 77, 229, 162, 242, 217, 75, 148, 170, 81, 99, 227, 163, 145, 25}
	hex := toHex(bytes)

	assert.Equal(t, "23bf4de5a2f2d94b94aa5163e3a39119", hex)
}

func TestFormatParentChild(t *testing.T) {
	assert.Equal(t, "|0000000000000000.00000000.", formatParentChild("", ""))
	assert.Equal(t, "|foo.bar.", formatParentChild("foo", "bar"))
}

func TestSanitize(t *testing.T) {
	sanitizeFunc := func() []string {
		warnings := [4]string{
			"John",
			"Paul",
			"George",
			"Ringo",
		}

		return warnings[:]
	}

	warningCounter := 0
	warningCallback := func(string) {
		warningCounter++
	}

	sanitizeWithCallback(sanitizeFunc, warningCallback)
	assert.Equal(t, 4, warningCounter)
}

// Tests proper assignment for unknown server spans
func TestSpanToRequestData_UnknownType(t *testing.T) {
	wireFormatSpan := getDefaultServerSpan("foo")

	// attach some random attributres
	attributes := map[string]string{
		"unknown_1": "a",
		"unknown_2": "b",
	}

	wireFormatSpan.Attributes = initializeAttributes(attributes)

	data := spanToRequestData(&wireFormatSpan)

	assert.Equal(t, "|"+defaultTraceIDAsHex+"."+defaultSpanIDAsHex+".", data.Id)
	assert.Equal(t, "foo", data.Name)
	assert.Equal(t, "00.00:00:01.000000", data.Duration)
	assert.Equal(t, 2, len(data.Properties))
	assert.True(t, data.Success)
	assert.Equal(t, "0", data.ResponseCode)
}

// Tests proper assignment for HTTP span with attribute set
// http.scheme, http.host, http.target
// along with a specific status code
func TestSpanToRequestDataHTTPAttributeSet1(t *testing.T) {
	wireFormatSpan, baseAttributes := getDefaultServerSpanHTTP()
	attributes := map[string]string{
		spanAttributeKeyHTTPScheme:     "https",
		spanAttributeKeyHTTPHost:       "foo",
		spanAttributeKeyHTTPTarget:     "/bar",
		spanAttributeKeyHTTPStatusCode: "400",
		spanAttributeKeyHTTPClientIP:   "127.0.0.1",
		"foo":                          "bar",
	}

	appendMap(attributes, baseAttributes)
	wireFormatSpan.Attributes = initializeAttributes(attributes)
	data := spanToRequestData(&wireFormatSpan)

	assert.Equal(t, "GET /bar", data.Name)
	assert.Equal(t, "https://foo/bar", data.Url)
	assert.Equal(t, 1, len(data.Properties))
	assert.False(t, data.Success)
	assert.Equal(t, "400", data.ResponseCode)
	assert.Equal(t, "127.0.0.1", data.Source)
}

// Tests proper assignment for HTTP span with attribute set
// http.scheme, http.server_name, host.port, http.target
func TestSpanToRequestDataHTTPAttributeSet2(t *testing.T) {
	wireFormatSpan, baseAttributes := getDefaultServerSpanHTTP()
	attributes := map[string]string{
		spanAttributeKeyHTTPScheme:     "https",
		spanAttributeKeyHTTPServerName: "foo",
		spanAttributeKeyHostPort:       "81",
		spanAttributeKeyHTTPTarget:     "/bar",
	}

	appendMap(attributes, baseAttributes)
	wireFormatSpan.Attributes = initializeAttributes(attributes)
	data := spanToRequestData(&wireFormatSpan)

	assert.Equal(t, "GET /bar", data.Name)
	assert.Equal(t, "https://foo:81/bar", data.Url)
	assert.Equal(t, 0, len(data.Properties))
	assert.True(t, data.Success)
	assert.Equal(t, "0", data.ResponseCode)
}

// Tests proper assignment for HTTP span with attribute set
// http.scheme, host.name, host.port, http.target
func TestSpanToRequestDataHTTPAttributeSet3(t *testing.T) {
	wireFormatSpan, baseAttributes := getDefaultServerSpanHTTP()
	attributes := map[string]string{
		spanAttributeKeyHTTPScheme: "https",
		spanAttributeKeyHostName:   "foo",
		spanAttributeKeyHostPort:   "81",
		spanAttributeKeyHTTPTarget: "/bar",
	}

	appendMap(attributes, baseAttributes)
	wireFormatSpan.Attributes = initializeAttributes(attributes)
	data := spanToRequestData(&wireFormatSpan)

	assert.Equal(t, "GET /bar", data.Name)
	assert.Equal(t, "https://foo:81/bar", data.Url)
	assert.Equal(t, 0, len(data.Properties))
	assert.True(t, data.Success)
	assert.Equal(t, "0", data.ResponseCode)
}

// Tests proper assignment for HTTP span with attribute set
// http.url
func TestSpanToRequestDataHTTPAttributeSet4(t *testing.T) {
	wireFormatSpan, baseAttributes := getDefaultServerSpanHTTP()
	attributes := map[string]string{
		spanAttributeKeyHTTPUrl: "https://foo:81/bar",
	}

	appendMap(attributes, baseAttributes)
	wireFormatSpan.Attributes = initializeAttributes(attributes)
	data := spanToRequestData(&wireFormatSpan)

	assert.Equal(t, "GET /bar", data.Name)
	assert.Equal(t, "https://foo:81/bar", data.Url)
	assert.Equal(t, 0, len(data.Properties))
	assert.True(t, data.Success)
	assert.Equal(t, "0", data.ResponseCode)
}

// Tests proper assignment for gRPC span with attribute set
func TestSpanToRequestDataGrpcAttributeSet(t *testing.T) {
	spanName := "foopackage.barservice/methodX"
	wireFormatSpan := getDefaultServerSpan(spanName)
	baseAttributes := getRequiredGrpcAttributes()

	attributes := map[string]string{
		"foo": "bar",
	}

	appendMap(attributes, baseAttributes)

	wireFormatSpan.Attributes = initializeAttributes(attributes)
	data := spanToRequestData(&wireFormatSpan)

	assert.Equal(t, spanName, data.Name)
	assert.Equal(t, 1, len(data.Properties))
	assert.False(t, data.Success)
	assert.Equal(t, strconv.Itoa(int(codes.ResourceExhausted)), data.ResponseCode)
}

// Tests proper assignment for unknown client spans
func TestSpanToRemoteDependencyData_UnknownType(t *testing.T) {
	wireFormatSpan := getDefaultClientSpan("foo")

	// attach some random attributres
	attributes := map[string]string{
		"unknown_1": "a",
		"unknown_2": "b",
	}

	wireFormatSpan.Attributes = initializeAttributes(attributes)

	data := spanToRemoteDependencyData(&wireFormatSpan)

	assert.Equal(t, "|"+defaultTraceIDAsHex+"."+defaultSpanIDAsHex+".", data.Id)
	assert.Equal(t, "foo", data.Name)
	assert.Equal(t, "00.00:00:01.000000", data.Duration)
	assert.Equal(t, 2, len(data.Properties))
	assert.True(t, data.Success)
	assert.Equal(t, "0", data.ResultCode)
	assert.Equal(t, "InProc", data.Type)
}

// Tests proper assignment for HTTP span with attribute set
// http.url
// along with a specific status code
func TestSpanToRemoteDependencyDataHTTPAttributeSet1(t *testing.T) {
	wireFormatSpan, baseAttributes := getDefaultClientSpanHTTP()
	url := "https://foo:81/bar"
	statusCode := "400"

	attributes := map[string]string{
		spanAttributeKeyHTTPUrl:        url,
		spanAttributeKeyHTTPStatusCode: statusCode,
		"foo":                          "bar",
	}

	appendMap(attributes, baseAttributes)
	wireFormatSpan.Attributes = initializeAttributes(attributes)
	data := spanToRemoteDependencyData(&wireFormatSpan)

	assert.Equal(t, "GET /bar", data.Name)
	assert.Equal(t, url, data.Data)
	assert.Equal(t, "foo:81", data.Target)
	assert.Equal(t, 1, len(data.Properties))
	assert.False(t, data.Success)
	assert.Equal(t, statusCode, data.ResultCode)
}

// Tests proper assignment for HTTP span with attribute set
// http.scheme, http.host, http.target
func TestSpanToRemoteDependencyDataHTTPAttributeSet2(t *testing.T) {
	wireFormatSpan, baseAttributes := getDefaultClientSpanHTTP()

	attributes := map[string]string{
		spanAttributeKeyHTTPScheme: "https",
		spanAttributeKeyHTTPHost:   "foo",
		spanAttributeKeyHTTPTarget: "/bar",
	}

	appendMap(attributes, baseAttributes)
	wireFormatSpan.Attributes = initializeAttributes(attributes)
	data := spanToRemoteDependencyData(&wireFormatSpan)

	assert.Equal(t, "GET /bar", data.Name)
	assert.Equal(t, "https://foo/bar", data.Data)
	assert.Equal(t, "foo", data.Target)
	assert.Equal(t, 0, len(data.Properties))
	assert.True(t, data.Success)
	assert.Equal(t, "200", data.ResultCode)
}

// Tests proper assignment for HTTP span with attribute set
// http.scheme, peer.hostname, peer.port, http.target
func TestSpanToRemoteDependencyDataHTTPAttributeSet3(t *testing.T) {
	wireFormatSpan, baseAttributes := getDefaultClientSpanHTTP()

	attributes := map[string]string{
		spanAttributeKeyHTTPScheme:   "https",
		spanAttributeKeyPeerHostname: "foo",
		spanAttributeKeyPeerPort:     "81",
		spanAttributeKeyHTTPTarget:   "/bar",
	}

	appendMap(attributes, baseAttributes)
	wireFormatSpan.Attributes = initializeAttributes(attributes)
	data := spanToRemoteDependencyData(&wireFormatSpan)

	assert.Equal(t, "GET /bar", data.Name)
	assert.Equal(t, "https://foo:81/bar", data.Data)
	assert.Equal(t, "foo:81", data.Target)
	assert.Equal(t, 0, len(data.Properties))
	assert.True(t, data.Success)
	assert.Equal(t, "200", data.ResultCode)
}

// Tests proper assignment for HTTP span with attribute set
// http.scheme, peer.ip, peer.port, http.target
func TestSpanToRemoteDependencyDataHTTPAttributeSet4(t *testing.T) {
	wireFormatSpan, baseAttributes := getDefaultClientSpanHTTP()

	attributes := map[string]string{
		spanAttributeKeyHTTPScheme: "https",
		spanAttributeKeyPeerIP:     "10.0.0.1",
		spanAttributeKeyPeerPort:   "81",
		spanAttributeKeyHTTPTarget: "/bar",
	}

	appendMap(attributes, baseAttributes)
	wireFormatSpan.Attributes = initializeAttributes(attributes)
	data := spanToRemoteDependencyData(&wireFormatSpan)

	assert.Equal(t, "GET /bar", data.Name)
	assert.Equal(t, "https://10.0.0.1:81/bar", data.Data)
	assert.Equal(t, "10.0.0.1:81", data.Target)
	assert.Equal(t, 0, len(data.Properties))
	assert.True(t, data.Success)
	assert.Equal(t, "200", data.ResultCode)
}

// Tests proper assignment for gRPC span with attribute set
func TestSpanToRemoteDependencyDataGrpcAttributeSet(t *testing.T) {
	spanName := "/foopackage.barservice/methodX"
	wireFormatSpan := getDefaultClientSpan(spanName)
	baseAttributes := getRequiredGrpcAttributes()

	attributes := map[string]string{
		spanAttributeKeyPeerService:  "/foopackage.barservice/methodX",
		spanAttributeKeyPeerHostname: "localhost",
		spanAttributeKeyPeerPort:     "5001",
		"foo":                        "bar",
	}

	appendMap(attributes, baseAttributes)

	wireFormatSpan.Attributes = initializeAttributes(attributes)
	data := spanToRemoteDependencyData(&wireFormatSpan)

	assert.Equal(t, spanName, data.Name)
	assert.Equal(t, 1, len(data.Properties))
	assert.False(t, data.Success)
	assert.Equal(t, strconv.Itoa(int(codes.ResourceExhausted)), data.ResultCode)
	assert.Equal(t, "localhost:5001", data.Target)
	assert.Equal(t, "localhost:5001/foopackage.barservice/methodX", data.Data)
}

// Tests proper assignment for database span with attribute set
func TestSpanToRemoteDependencyDataDatabaseAttributeSet(t *testing.T) {
	spanName := "somestoredproc"
	wireFormatSpan := getDefaultClientSpan(spanName)

	dbType := "sql"
	dbStatement := "select * from foobar"
	peerHostname := "db.example.com"
	peerAddress := "mysql://" + peerHostname + ":3306"

	attributes := map[string]string{
		spanAttributeKeyComponent:    "odbc",
		spanAttributeKeyDbType:       dbType,
		spanAttributeKeyDbInstance:   "foo",
		spanAttributeKeyDbStatement:  dbStatement,
		spanAttributeKeyPeerAddress:  peerAddress,
		spanAttributeKeyPeerHostname: peerHostname,
	}

	wireFormatSpan.Attributes = initializeAttributes(attributes)
	data := spanToRemoteDependencyData(&wireFormatSpan)

	assert.Equal(t, spanName, data.Name)
	assert.Equal(t, dbType, data.Type)
	assert.Equal(t, dbStatement, data.Data)
	assert.Equal(t, peerAddress, data.Target)
	assert.Equal(t, 6, len(data.Properties))
	assert.True(t, data.Success)
	assert.Equal(t, "0", data.ResultCode)
}

// Tests the exporter's pushTraceData callback method
func TestExporterPushTraceDataCallback(t *testing.T) {
	factory := Factory{}

	// mock channel
	transportChannelMock := mockTransportChannel{}
	transportChannelMock.On("Send", mock.Anything)

	exporter := &traceExporter{
		config:           factory.CreateDefaultConfig().(*Config),
		transportChannel: &transportChannelMock,
	}

	// No spans
	droppedSpans, err := exporter.pushTraceData(context.TODO(), consumerdata.TraceData{})
	assert.Equal(t, 0, droppedSpans)
	assert.Nil(t, err)
	transportChannelMock.AssertNumberOfCalls(t, "Send", 0)

	// Some spans
	span1 := getDefaultServerSpan("foo")
	span2 := getDefaultClientSpan("bar")

	traceData := consumerdata.TraceData{
		Spans: []*tracepb.Span{
			&span1,
			&span2,
		},
	}

	droppedSpans, err = exporter.pushTraceData(context.TODO(), traceData)
	assert.Equal(t, 0, droppedSpans)
	assert.Nil(t, err)

	// transport channel should have received 2 envelopes
	transportChannelMock.AssertNumberOfCalls(t, "Send", 2)
}

func getDefaultSpan(spanName string, spanKind tracepb.Span_SpanKind) tracepb.Span {
	return tracepb.Span{
		TraceId:      defaultTraceID,
		SpanId:       defaultSpanID,
		ParentSpanId: defaultParentSpanID,
		Name:         &tracepb.TruncatableString{Value: spanName},
		Kind:         spanKind,
		StartTime:    &timestamp.Timestamp{},
		EndTime:      &timestamp.Timestamp{Seconds: 1, Nanos: 0},
	}
}

// Returns a default server span
func getDefaultServerSpan(spanName string) tracepb.Span {
	return getDefaultSpan(spanName, tracepb.Span_SERVER)
}

// Returns a default client span
func getDefaultClientSpan(spanName string) tracepb.Span {
	return getDefaultSpan(spanName, tracepb.Span_CLIENT)
}

// Returns the set of required HTTP attributes
func getRequiredHTTPAttributes() map[string]string {
	attributes := map[string]string{
		spanAttributeKeyComponent:  "http",
		spanAttributeKeyHTTPMethod: "GET",
	}

	return attributes
}

// Returns the set of required gRPC attributes
func getRequiredGrpcAttributes() map[string]string {
	attributes := map[string]string{
		spanAttributeKeyComponent:     "grpc",
		spanAttributeKeyRPCStatusCode: strconv.Itoa(int(codes.ResourceExhausted)),
	}

	return attributes
}

// Returns a default server span along with the set of required HTTP attributes
func getDefaultServerSpanHTTP() (tracepb.Span, map[string]string) {
	baseSpan := getDefaultServerSpan("foo")
	return baseSpan, getRequiredHTTPAttributes()
}

// Returns a default client span along with the set of required HTTP attributes
func getDefaultClientSpanHTTP() (tracepb.Span, map[string]string) {
	baseSpan := getDefaultClientSpan("foo")
	return baseSpan, getRequiredHTTPAttributes()
}

// Creates and initializes a wire format Span attributes struct with some default string key/value pairs
func initializeAttributes(values map[string]string) *tracepb.Span_Attributes {
	attributes := &tracepb.Span_Attributes{
		AttributeMap: make(map[string]*tracepb.AttributeValue),
	}

	for k, v := range values {
		attributes.AttributeMap[k] = &tracepb.AttributeValue{
			Value: &tracepb.AttributeValue_StringValue{
				StringValue: &tracepb.TruncatableString{
					Value: v,
				},
			},
		}
	}

	return attributes
}

// Adds the key/values from one map to another
func appendMap(target map[string]string, values map[string]string) {
	for k, v := range values {
		target[k] = v
	}
}
