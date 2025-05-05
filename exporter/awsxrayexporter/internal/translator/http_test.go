// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventionsv112 "go.opentelemetry.io/collector/semconv/v1.12.0"
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"
)

func TestClientSpanWithURLAttribute(t *testing.T) {
	attributes := make(map[string]any)
	attributes[conventionsv112.AttributeHTTPMethod] = http.MethodGet
	attributes[conventionsv112.AttributeHTTPURL] = "https://api.example.com/users/junit"
	attributes[conventionsv112.AttributeHTTPStatusCode] = 200
	span := constructHTTPClientSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "https://api.example.com/users/junit")
}

func TestClientSpanWithURLAttributeStable(t *testing.T) {
	attributes := make(map[string]any)
	attributes[conventions.AttributeHTTPRequestMethod] = http.MethodGet
	attributes[conventions.AttributeURLFull] = "https://api.example.com/users/junit"
	attributes[conventions.AttributeHTTPResponseStatusCode] = 200
	span := constructHTTPClientSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "https://api.example.com/users/junit")
}

func TestClientSpanWithSchemeHostTargetAttributes(t *testing.T) {
	attributes := make(map[string]any)
	attributes[conventionsv112.AttributeHTTPMethod] = http.MethodGet
	attributes[conventionsv112.AttributeHTTPScheme] = "https"
	attributes[conventionsv112.AttributeHTTPHost] = "api.example.com"
	attributes[conventionsv112.AttributeHTTPTarget] = "/users/junit"
	attributes[conventionsv112.AttributeHTTPStatusCode] = 200
	attributes["user.id"] = "junit"
	span := constructHTTPClientSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "https://api.example.com/users/junit")
}

func TestClientSpanWithSchemeHostTargetAttributesStable(t *testing.T) {
	attributes := make(map[string]any)
	attributes[conventions.AttributeHTTPRequestMethod] = "GET"
	attributes[conventions.AttributeURLScheme] = "https"
	attributes[conventionsv112.AttributeHTTPHost] = "api.example.com"
	attributes[conventions.AttributeURLQuery] = "/users/junit"
	attributes[conventions.AttributeHTTPResponseStatusCode] = 200
	attributes["user.id"] = "junit"
	span := constructHTTPClientSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "https://api.example.com/users/junit")
}

func TestClientSpanWithPeerAttributes(t *testing.T) {
	attributes := make(map[string]any)
	attributes[conventionsv112.AttributeHTTPMethod] = http.MethodGet
	attributes[conventionsv112.AttributeHTTPScheme] = "http"
	attributes[conventionsv112.AttributeNetPeerName] = "kb234.example.com"
	attributes[conventionsv112.AttributeNetPeerPort] = 8080
	attributes[conventionsv112.AttributeNetPeerIP] = "10.8.17.36"
	attributes[conventionsv112.AttributeHTTPTarget] = "/users/junit"
	attributes[conventionsv112.AttributeHTTPStatusCode] = 200
	span := constructHTTPClientSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	assert.NotNil(t, httpData.Request.URL)

	assert.Equal(t, "10.8.17.36", *httpData.Request.ClientIP)

	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "http://kb234.example.com:8080/users/junit")
}

func TestClientSpanWithPeerAttributesStable(t *testing.T) {
	attributes := make(map[string]any)
	attributes[conventions.AttributeHTTPRequestMethod] = http.MethodGet
	attributes[conventions.AttributeURLScheme] = "http"
	attributes[conventionsv112.AttributeNetPeerName] = "kb234.example.com"
	attributes[conventionsv112.AttributeNetPeerPort] = 8080
	attributes[conventionsv112.AttributeNetPeerIP] = "10.8.17.36"
	attributes[conventions.AttributeURLQuery] = "/users/junit"
	attributes[conventions.AttributeHTTPResponseStatusCode] = 200
	span := constructHTTPClientSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)

	assert.Equal(t, "10.8.17.36", *httpData.Request.ClientIP)

	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "http://kb234.example.com:8080/users/junit")
}

func TestClientSpanWithHttpPeerAttributes(t *testing.T) {
	attributes := make(map[string]any)
	attributes[conventionsv112.AttributeHTTPClientIP] = "1.2.3.4"
	attributes[conventionsv112.AttributeNetPeerIP] = "10.8.17.36"
	span := constructHTTPClientSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)

	assert.Equal(t, "1.2.3.4", *httpData.Request.ClientIP)
}

func TestClientSpanWithHttpPeerAttributesStable(t *testing.T) {
	attributes := make(map[string]any)
	attributes[conventions.AttributeURLFull] = "https://api.example.com/users/junit"
	attributes[conventions.AttributeClientAddress] = "1.2.3.4"
	attributes[conventions.AttributeNetworkPeerAddress] = "10.8.17.36"
	span := constructHTTPClientSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)

	assert.Equal(t, "1.2.3.4", *httpData.Request.ClientIP)
}

func TestClientSpanWithPeerIp4Attributes(t *testing.T) {
	attributes := make(map[string]any)
	attributes[conventionsv112.AttributeHTTPMethod] = http.MethodGet
	attributes[conventionsv112.AttributeHTTPScheme] = "http"
	attributes[conventionsv112.AttributeNetPeerIP] = "10.8.17.36"
	attributes[conventionsv112.AttributeNetPeerPort] = "8080"
	attributes[conventionsv112.AttributeHTTPTarget] = "/users/junit"
	span := constructHTTPClientSpan(attributes)

	filtered, httpData := makeHTTP(span)
	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "http://10.8.17.36:8080/users/junit")
}

func TestClientSpanWithPeerIp6Attributes(t *testing.T) {
	attributes := make(map[string]any)
	attributes[conventionsv112.AttributeHTTPMethod] = http.MethodGet
	attributes[conventionsv112.AttributeHTTPScheme] = "https"
	attributes[conventionsv112.AttributeNetPeerIP] = "2001:db8:85a3::8a2e:370:7334"
	attributes[conventionsv112.AttributeNetPeerPort] = "443"
	attributes[conventionsv112.AttributeHTTPTarget] = "/users/junit"
	span := constructHTTPClientSpan(attributes)

	filtered, httpData := makeHTTP(span)
	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "https://2001:db8:85a3::8a2e:370:7334/users/junit")
}

func TestServerSpanWithURLAttribute(t *testing.T) {
	attributes := make(map[string]any)
	attributes[conventionsv112.AttributeHTTPMethod] = http.MethodGet
	attributes[conventionsv112.AttributeHTTPURL] = "https://api.example.com/users/junit"
	attributes[conventionsv112.AttributeHTTPClientIP] = "192.168.15.32"
	attributes[conventionsv112.AttributeHTTPUserAgent] = "PostmanRuntime/7.21.0"
	attributes[conventionsv112.AttributeHTTPStatusCode] = 200
	span := constructHTTPServerSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "https://api.example.com/users/junit")
}

func TestServerSpanWithURLAttributeStable(t *testing.T) {
	attributes := make(map[string]any)
	attributes[conventions.AttributeHTTPRequestMethod] = http.MethodGet
	attributes[conventions.AttributeURLFull] = "https://api.example.com/users/junit"
	attributes[conventions.AttributeClientAddress] = "192.168.15.32"
	attributes[conventions.AttributeUserAgentOriginal] = "PostmanRuntime/7.21.0"
	attributes[conventions.AttributeHTTPResponseStatusCode] = 200
	span := constructHTTPServerSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "https://api.example.com/users/junit")
}

func TestServerSpanWithSchemeHostTargetAttributes(t *testing.T) {
	attributes := make(map[string]any)
	attributes[conventionsv112.AttributeHTTPMethod] = http.MethodGet
	attributes[conventionsv112.AttributeHTTPScheme] = "https"
	attributes[conventionsv112.AttributeHTTPHost] = "api.example.com"
	attributes[conventionsv112.AttributeHTTPTarget] = "/users/junit"
	attributes[conventionsv112.AttributeHTTPClientIP] = "192.168.15.32"
	attributes[conventionsv112.AttributeHTTPStatusCode] = 200
	span := constructHTTPServerSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "https://api.example.com/users/junit")
}

func TestServerSpanWithSchemeHostTargetAttributesStable(t *testing.T) {
	attributes := make(map[string]any)
	attributes[conventions.AttributeHTTPRequestMethod] = http.MethodGet
	attributes[conventions.AttributeURLScheme] = "https"
	attributes[conventions.AttributeServerAddress] = "api.example.com"
	attributes[conventions.AttributeURLQuery] = "/users/junit"
	attributes[conventions.AttributeClientAddress] = "192.168.15.32"
	attributes[conventions.AttributeHTTPResponseStatusCode] = 200
	span := constructHTTPServerSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "https://api.example.com/users/junit")
}

func TestServerSpanWithSchemeServernamePortTargetAttributes(t *testing.T) {
	attributes := make(map[string]any)
	attributes[conventionsv112.AttributeHTTPMethod] = http.MethodGet
	attributes[conventionsv112.AttributeHTTPScheme] = "https"
	attributes[conventionsv112.AttributeHTTPServerName] = "api.example.com"
	attributes[conventionsv112.AttributeNetHostPort] = 443
	attributes[conventionsv112.AttributeHTTPTarget] = "/users/junit"
	attributes[conventionsv112.AttributeHTTPClientIP] = "192.168.15.32"
	attributes[conventionsv112.AttributeHTTPStatusCode] = 200
	span := constructHTTPServerSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "https://api.example.com/users/junit")
}

func TestServerSpanWithSchemeServernamePortTargetAttributesStable(t *testing.T) {
	attributes := make(map[string]any)
	attributes[conventions.AttributeHTTPRequestMethod] = http.MethodGet
	attributes[conventions.AttributeURLScheme] = "https"
	attributes[conventions.AttributeServerAddress] = "api.example.com"
	attributes[conventions.AttributeServerPort] = 443
	attributes[conventions.AttributeURLQuery] = "/users/junit"
	attributes[conventions.AttributeClientAddress] = "192.168.15.32"
	attributes[conventions.AttributeHTTPResponseStatusCode] = 200
	span := constructHTTPServerSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "https://api.example.com/users/junit")
}

func TestServerSpanWithSchemeNamePortTargetAttributes(t *testing.T) {
	attributes := make(map[string]any)
	attributes[conventionsv112.AttributeHTTPMethod] = http.MethodGet
	attributes[conventionsv112.AttributeHTTPScheme] = "http"
	attributes[conventionsv112.AttributeHostName] = "kb234.example.com"
	attributes[conventionsv112.AttributeNetHostPort] = 8080
	attributes[conventionsv112.AttributeHTTPTarget] = "/users/junit"
	attributes[conventionsv112.AttributeHTTPClientIP] = "192.168.15.32"
	attributes[conventionsv112.AttributeHTTPStatusCode] = 200
	span := constructHTTPServerSpan(attributes)
	timeEvents := constructTimedEventsWithReceivedMessageEvent(span.EndTimestamp())
	timeEvents.CopyTo(span.Events())

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "http://kb234.example.com:8080/users/junit")
}

func TestServerSpanWithSchemeNamePortTargetAttributesStable(t *testing.T) {
	attributes := make(map[string]any)
	attributes[conventions.AttributeHTTPRequestMethod] = http.MethodGet
	attributes[conventions.AttributeURLScheme] = "http"
	attributes[conventions.AttributeServerAddress] = "kb234.example.com"
	attributes[conventions.AttributeServerPort] = 8080
	attributes[conventions.AttributeURLPath] = "/users/junit"
	attributes[conventions.AttributeClientAddress] = "192.168.15.32"
	attributes[conventions.AttributeHTTPResponseStatusCode] = 200
	span := constructHTTPServerSpan(attributes)
	timeEvents := constructTimedEventsWithReceivedMessageEvent(span.EndTimestamp())
	timeEvents.CopyTo(span.Events())

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "http://kb234.example.com:8080/users/junit")
}

func TestSpanWithNotEnoughHTTPRequestURLAttributes(t *testing.T) {
	attributes := make(map[string]any)
	attributes[conventionsv112.AttributeHTTPMethod] = http.MethodGet
	attributes[conventionsv112.AttributeHTTPScheme] = "http"
	attributes[conventionsv112.AttributeHTTPClientIP] = "192.168.15.32"
	attributes[conventionsv112.AttributeHTTPUserAgent] = "PostmanRuntime/7.21.0"
	attributes[conventionsv112.AttributeHTTPTarget] = "/users/junit"
	attributes[conventionsv112.AttributeNetHostPort] = 443
	attributes[conventionsv112.AttributeNetPeerPort] = 8080
	attributes[conventionsv112.AttributeHTTPStatusCode] = 200
	span := constructHTTPServerSpan(attributes)
	timeEvents := constructTimedEventsWithReceivedMessageEvent(span.EndTimestamp())
	timeEvents.CopyTo(span.Events())

	filtered, httpData := makeHTTP(span)

	assert.Nil(t, httpData.Request.URL)
	assert.Equal(t, "192.168.15.32", *httpData.Request.ClientIP)
	assert.Equal(t, http.MethodGet, *httpData.Request.Method)
	assert.Equal(t, "PostmanRuntime/7.21.0", *httpData.Request.UserAgent)
	contentLength := *httpData.Response.ContentLength.(*int64)
	assert.Equal(t, int64(12452), contentLength)
	assert.Equal(t, int64(200), *httpData.Response.Status)
	assert.NotNil(t, filtered)
}

func TestSpanWithNotEnoughHTTPRequestURLAttributesStable(t *testing.T) {
	attributes := make(map[string]any)
	attributes[conventions.AttributeHTTPRequestMethod] = http.MethodGet
	attributes[conventions.AttributeURLScheme] = "http"
	attributes[conventions.AttributeClientAddress] = "192.168.15.32"
	attributes[conventions.AttributeUserAgentOriginal] = "PostmanRuntime/7.21.0"
	attributes[conventions.AttributeURLPath] = "/users/junit"
	attributes[conventions.AttributeServerPort] = 443
	attributes[conventions.AttributeHTTPResponseStatusCode] = 200
	span := constructHTTPServerSpan(attributes)
	timeEvents := constructTimedEventsWithReceivedMessageEvent(span.EndTimestamp())
	timeEvents.CopyTo(span.Events())

	filtered, httpData := makeHTTP(span)

	assert.Nil(t, httpData.Request.URL)
	assert.Equal(t, "192.168.15.32", *httpData.Request.ClientIP)
	assert.Equal(t, http.MethodGet, *httpData.Request.Method)
	assert.Equal(t, "PostmanRuntime/7.21.0", *httpData.Request.UserAgent)
	contentLength := *httpData.Response.ContentLength.(*int64)
	assert.Equal(t, int64(12452), contentLength)
	assert.Equal(t, int64(200), *httpData.Response.Status)
	assert.NotNil(t, filtered)
}

func TestSpanWithNotEnoughHTTPRequestURLAttributesDuplicated(t *testing.T) {
	attributes := make(map[string]any)
	attributes[conventionsv112.AttributeHTTPMethod] = http.MethodGet
	attributes[conventions.AttributeHTTPRequestMethod] = http.MethodGet
	attributes[conventionsv112.AttributeHTTPScheme] = "http"
	attributes[conventions.AttributeURLScheme] = "http"
	attributes[conventionsv112.AttributeHTTPClientIP] = "192.168.15.32"
	attributes[conventions.AttributeClientAddress] = "192.168.15.32"
	attributes[conventionsv112.AttributeHTTPUserAgent] = "PostmanRuntime/7.21.0"
	attributes[conventions.AttributeUserAgentOriginal] = "PostmanRuntime/7.21.0"
	attributes[conventionsv112.AttributeHTTPTarget] = "/users/junit"
	attributes[conventions.AttributeURLPath] = "/users/junit"
	attributes[conventionsv112.AttributeNetHostPort] = 443
	attributes[conventions.AttributeServerPort] = 443
	attributes[conventionsv112.AttributeNetPeerPort] = 8080
	attributes[conventionsv112.AttributeHTTPStatusCode] = 200
	attributes[conventions.AttributeHTTPResponseStatusCode] = 200
	span := constructHTTPServerSpan(attributes)
	timeEvents := constructTimedEventsWithReceivedMessageEvent(span.EndTimestamp())
	timeEvents.CopyTo(span.Events())

	filtered, httpData := makeHTTP(span)

	assert.Nil(t, httpData.Request.URL)
	assert.Equal(t, "192.168.15.32", *httpData.Request.ClientIP)
	assert.Equal(t, http.MethodGet, *httpData.Request.Method)
	assert.Equal(t, "PostmanRuntime/7.21.0", *httpData.Request.UserAgent)
	contentLength := *httpData.Response.ContentLength.(*int64)
	assert.Equal(t, int64(12452), contentLength)
	assert.Equal(t, int64(200), *httpData.Response.Status)
	assert.NotNil(t, filtered)
}

func TestSpanWithClientAddrWithoutNetworkPeerAddr(t *testing.T) {
	attributes := make(map[string]any)
	attributes[conventions.AttributeURLFull] = "https://api.example.com/users/junit"
	attributes[conventions.AttributeClientAddress] = "192.168.15.32"
	span := constructHTTPServerSpan(attributes)

	_, httpData := makeHTTP(span)

	assert.Equal(t, aws.Bool(true), httpData.Request.XForwardedFor)
}

func TestSpanWithClientAddrAndNetworkPeerAddr(t *testing.T) {
	attributes := make(map[string]any)
	attributes[conventions.AttributeURLFull] = "https://api.example.com/users/junit"
	attributes[conventions.AttributeClientAddress] = "192.168.15.32"
	attributes[conventions.AttributeNetworkPeerAddress] = "192.168.15.32"
	span := constructHTTPServerSpan(attributes)

	_, httpData := makeHTTP(span)

	assert.Equal(t, "192.168.15.32", *httpData.Request.ClientIP)
	assert.Nil(t, httpData.Request.XForwardedFor)
}

func TestSpanWithClientAddrNotIP(t *testing.T) {
	attributes := make(map[string]any)
	attributes[conventions.AttributeURLFull] = "https://api.example.com/users/junit"
	attributes[conventions.AttributeClientAddress] = "api.example.com"
	attributes[conventions.AttributeNetworkPeerAddress] = "api.example.com"
	span := constructHTTPServerSpan(attributes)

	_, httpData := makeHTTP(span)

	assert.Nil(t, httpData.Request.ClientIP)
	assert.Nil(t, httpData.Request.XForwardedFor)
}

func constructHTTPClientSpan(attributes map[string]any) ptrace.Span {
	endTime := time.Now().Round(time.Second)
	startTime := endTime.Add(-90 * time.Second)
	spanAttributes := constructSpanAttributes(attributes)

	span := ptrace.NewSpan()
	span.SetTraceID(newTraceID())
	span.SetSpanID(newSegmentID())
	span.SetParentSpanID(newSegmentID())
	span.SetName("/users/junit")
	span.SetKind(ptrace.SpanKindClient)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

	status := ptrace.NewStatus()
	status.SetCode(0)
	status.SetMessage("OK")
	status.CopyTo(span.Status())

	spanAttributes.CopyTo(span.Attributes())
	return span
}

func constructHTTPServerSpan(attributes map[string]any) ptrace.Span {
	endTime := time.Now().Round(time.Second)
	startTime := endTime.Add(-90 * time.Second)
	spanAttributes := constructSpanAttributes(attributes)

	span := ptrace.NewSpan()
	span.SetTraceID(newTraceID())
	span.SetSpanID(newSegmentID())
	span.SetParentSpanID(newSegmentID())
	span.SetName("/users/junit")
	span.SetKind(ptrace.SpanKindServer)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

	status := ptrace.NewStatus()
	status.SetCode(0)
	status.SetMessage("OK")
	status.CopyTo(span.Status())

	spanAttributes.CopyTo(span.Attributes())
	return span
}
