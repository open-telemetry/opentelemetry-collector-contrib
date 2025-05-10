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
	conventionsv112 "go.opentelemetry.io/otel/semconv/v1.12.0"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
)

func TestClientSpanWithURLAttribute(t *testing.T) {
	attributes := make(map[string]any)
	attributes[string(conventionsv112.HTTPMethodKey)] = http.MethodGet
	attributes[string(conventionsv112.HTTPURLKey)] = "https://api.example.com/users/junit"
	attributes[string(conventionsv112.HTTPStatusCodeKey)] = 200
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
	attributes[string(conventions.HTTPRequestMethodKey)] = http.MethodGet
	attributes[string(conventions.URLFullKey)] = "https://api.example.com/users/junit"
	attributes[string(conventions.HTTPResponseStatusCodeKey)] = 200
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
	attributes[string(conventionsv112.HTTPMethodKey)] = http.MethodGet
	attributes[string(conventionsv112.HTTPSchemeKey)] = "https"
	attributes[string(conventionsv112.HTTPHostKey)] = "api.example.com"
	attributes[string(conventionsv112.HTTPTargetKey)] = "/users/junit"
	attributes[string(conventionsv112.HTTPStatusCodeKey)] = 200
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
	attributes[string(conventions.HTTPRequestMethodKey)] = "GET"
	attributes[string(conventions.URLSchemeKey)] = "https"
	attributes[string(conventionsv112.HTTPHostKey)] = "api.example.com"
	attributes[string(conventions.URLQueryKey)] = "/users/junit"
	attributes[string(conventions.HTTPResponseStatusCodeKey)] = 200
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
	attributes[string(conventionsv112.HTTPMethodKey)] = http.MethodGet
	attributes[string(conventionsv112.HTTPSchemeKey)] = "http"
	attributes[string(conventionsv112.NetPeerNameKey)] = "kb234.example.com"
	attributes[string(conventionsv112.NetPeerPortKey)] = 8080
	attributes[string(conventionsv112.NetPeerIPKey)] = "10.8.17.36"
	attributes[string(conventionsv112.HTTPTargetKey)] = "/users/junit"
	attributes[string(conventionsv112.HTTPStatusCodeKey)] = 200
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
	attributes[string(conventions.HTTPRequestMethodKey)] = http.MethodGet
	attributes[string(conventions.URLSchemeKey)] = "http"
	attributes[string(conventionsv112.NetPeerNameKey)] = "kb234.example.com"
	attributes[string(conventionsv112.NetPeerPortKey)] = 8080
	attributes[string(conventionsv112.NetPeerIPKey)] = "10.8.17.36"
	attributes[string(conventions.URLQueryKey)] = "/users/junit"
	attributes[string(conventions.HTTPResponseStatusCodeKey)] = 200
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
	attributes[string(conventionsv112.HTTPClientIPKey)] = "1.2.3.4"
	attributes[string(conventionsv112.NetPeerIPKey)] = "10.8.17.36"
	span := constructHTTPClientSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)

	assert.Equal(t, "1.2.3.4", *httpData.Request.ClientIP)
}

func TestClientSpanWithHttpPeerAttributesStable(t *testing.T) {
	attributes := make(map[string]any)
	attributes[string(conventions.URLFullKey)] = "https://api.example.com/users/junit"
	attributes[string(conventions.ClientAddressKey)] = "1.2.3.4"
	attributes[string(conventions.NetworkPeerAddressKey)] = "10.8.17.36"
	span := constructHTTPClientSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)

	assert.Equal(t, "1.2.3.4", *httpData.Request.ClientIP)
}

func TestClientSpanWithPeerIp4Attributes(t *testing.T) {
	attributes := make(map[string]any)
	attributes[string(conventionsv112.HTTPMethodKey)] = http.MethodGet
	attributes[string(conventionsv112.HTTPSchemeKey)] = "http"
	attributes[string(conventionsv112.NetPeerIPKey)] = "10.8.17.36"
	attributes[string(conventionsv112.NetPeerPortKey)] = "8080"
	attributes[string(conventionsv112.HTTPTargetKey)] = "/users/junit"
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
	attributes[string(conventionsv112.HTTPMethodKey)] = http.MethodGet
	attributes[string(conventionsv112.HTTPSchemeKey)] = "https"
	attributes[string(conventionsv112.NetPeerIPKey)] = "2001:db8:85a3::8a2e:370:7334"
	attributes[string(conventionsv112.NetPeerPortKey)] = "443"
	attributes[string(conventionsv112.HTTPTargetKey)] = "/users/junit"
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
	attributes[string(conventionsv112.HTTPMethodKey)] = http.MethodGet
	attributes[string(conventionsv112.HTTPURLKey)] = "https://api.example.com/users/junit"
	attributes[string(conventionsv112.HTTPClientIPKey)] = "192.168.15.32"
	attributes[string(conventionsv112.HTTPUserAgentKey)] = "PostmanRuntime/7.21.0"
	attributes[string(conventionsv112.HTTPStatusCodeKey)] = 200
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
	attributes[string(conventions.HTTPRequestMethodKey)] = http.MethodGet
	attributes[string(conventions.URLFullKey)] = "https://api.example.com/users/junit"
	attributes[string(conventions.ClientAddressKey)] = "192.168.15.32"
	attributes[string(conventions.UserAgentOriginalKey)] = "PostmanRuntime/7.21.0"
	attributes[string(conventions.HTTPResponseStatusCodeKey)] = 200
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
	attributes[string(conventionsv112.HTTPMethodKey)] = http.MethodGet
	attributes[string(conventionsv112.HTTPSchemeKey)] = "https"
	attributes[string(conventionsv112.HTTPHostKey)] = "api.example.com"
	attributes[string(conventionsv112.HTTPTargetKey)] = "/users/junit"
	attributes[string(conventionsv112.HTTPClientIPKey)] = "192.168.15.32"
	attributes[string(conventionsv112.HTTPStatusCodeKey)] = 200
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
	attributes[string(conventions.HTTPRequestMethodKey)] = http.MethodGet
	attributes[string(conventions.URLSchemeKey)] = "https"
	attributes[string(conventions.ServerAddressKey)] = "api.example.com"
	attributes[string(conventions.URLQueryKey)] = "/users/junit"
	attributes[string(conventions.ClientAddressKey)] = "192.168.15.32"
	attributes[string(conventions.HTTPResponseStatusCodeKey)] = 200
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
	attributes[string(conventionsv112.HTTPMethodKey)] = http.MethodGet
	attributes[string(conventionsv112.HTTPSchemeKey)] = "https"
	attributes[string(conventionsv112.HTTPServerNameKey)] = "api.example.com"
	attributes[string(conventionsv112.NetHostPortKey)] = 443
	attributes[string(conventionsv112.HTTPTargetKey)] = "/users/junit"
	attributes[string(conventionsv112.HTTPClientIPKey)] = "192.168.15.32"
	attributes[string(conventionsv112.HTTPStatusCodeKey)] = 200
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
	attributes[string(conventions.HTTPRequestMethodKey)] = http.MethodGet
	attributes[string(conventions.URLSchemeKey)] = "https"
	attributes[string(conventions.ServerAddressKey)] = "api.example.com"
	attributes[string(conventions.ServerPortKey)] = 443
	attributes[string(conventions.URLQueryKey)] = "/users/junit"
	attributes[string(conventions.ClientAddressKey)] = "192.168.15.32"
	attributes[string(conventions.HTTPResponseStatusCodeKey)] = 200
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
	attributes[string(conventionsv112.HTTPMethodKey)] = http.MethodGet
	attributes[string(conventionsv112.HTTPSchemeKey)] = "http"
	attributes[string(conventionsv112.HostNameKey)] = "kb234.example.com"
	attributes[string(conventionsv112.NetHostPortKey)] = 8080
	attributes[string(conventionsv112.HTTPTargetKey)] = "/users/junit"
	attributes[string(conventionsv112.HTTPClientIPKey)] = "192.168.15.32"
	attributes[string(conventionsv112.HTTPStatusCodeKey)] = 200
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
	attributes[string(conventions.HTTPRequestMethodKey)] = http.MethodGet
	attributes[string(conventions.URLSchemeKey)] = "http"
	attributes[string(conventions.ServerAddressKey)] = "kb234.example.com"
	attributes[string(conventions.ServerPortKey)] = 8080
	attributes[string(conventions.URLPathKey)] = "/users/junit"
	attributes[string(conventions.ClientAddressKey)] = "192.168.15.32"
	attributes[string(conventions.HTTPResponseStatusCodeKey)] = 200
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
	attributes[string(conventionsv112.HTTPMethodKey)] = http.MethodGet
	attributes[string(conventionsv112.HTTPSchemeKey)] = "http"
	attributes[string(conventionsv112.HTTPClientIPKey)] = "192.168.15.32"
	attributes[string(conventionsv112.HTTPUserAgentKey)] = "PostmanRuntime/7.21.0"
	attributes[string(conventionsv112.HTTPTargetKey)] = "/users/junit"
	attributes[string(conventionsv112.NetHostPortKey)] = 443
	attributes[string(conventionsv112.NetPeerPortKey)] = 8080
	attributes[string(conventionsv112.HTTPStatusCodeKey)] = 200
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
	attributes[string(conventions.HTTPRequestMethodKey)] = http.MethodGet
	attributes[string(conventions.URLSchemeKey)] = "http"
	attributes[string(conventions.ClientAddressKey)] = "192.168.15.32"
	attributes[string(conventions.UserAgentOriginalKey)] = "PostmanRuntime/7.21.0"
	attributes[string(conventions.URLPathKey)] = "/users/junit"
	attributes[string(conventions.ServerPortKey)] = 443
	attributes[string(conventions.HTTPResponseStatusCodeKey)] = 200
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
	attributes[string(conventionsv112.HTTPMethodKey)] = http.MethodGet
	attributes[string(conventions.HTTPRequestMethodKey)] = http.MethodGet
	attributes[string(conventionsv112.HTTPSchemeKey)] = "http"
	attributes[string(conventions.URLSchemeKey)] = "http"
	attributes[string(conventionsv112.HTTPClientIPKey)] = "192.168.15.32"
	attributes[string(conventions.ClientAddressKey)] = "192.168.15.32"
	attributes[string(conventionsv112.HTTPUserAgentKey)] = "PostmanRuntime/7.21.0"
	attributes[string(conventions.UserAgentOriginalKey)] = "PostmanRuntime/7.21.0"
	attributes[string(conventionsv112.HTTPTargetKey)] = "/users/junit"
	attributes[string(conventions.URLPathKey)] = "/users/junit"
	attributes[string(conventionsv112.NetHostPortKey)] = 443
	attributes[string(conventions.ServerPortKey)] = 443
	attributes[string(conventionsv112.NetPeerPortKey)] = 8080
	attributes[string(conventionsv112.HTTPStatusCodeKey)] = 200
	attributes[string(conventions.HTTPResponseStatusCodeKey)] = 200
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
	attributes[string(conventions.URLFullKey)] = "https://api.example.com/users/junit"
	attributes[string(conventions.ClientAddressKey)] = "192.168.15.32"
	span := constructHTTPServerSpan(attributes)

	_, httpData := makeHTTP(span)

	assert.Equal(t, aws.Bool(true), httpData.Request.XForwardedFor)
}

func TestSpanWithClientAddrAndNetworkPeerAddr(t *testing.T) {
	attributes := make(map[string]any)
	attributes[string(conventions.URLFullKey)] = "https://api.example.com/users/junit"
	attributes[string(conventions.ClientAddressKey)] = "192.168.15.32"
	attributes[string(conventions.NetworkPeerAddressKey)] = "192.168.15.32"
	span := constructHTTPServerSpan(attributes)

	_, httpData := makeHTTP(span)

	assert.Equal(t, "192.168.15.32", *httpData.Request.ClientIP)
	assert.Nil(t, httpData.Request.XForwardedFor)
}

func TestSpanWithClientAddrNotIP(t *testing.T) {
	attributes := make(map[string]any)
	attributes[string(conventions.URLFullKey)] = "https://api.example.com/users/junit"
	attributes[string(conventions.ClientAddressKey)] = "api.example.com"
	attributes[string(conventions.NetworkPeerAddressKey)] = "api.example.com"
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
