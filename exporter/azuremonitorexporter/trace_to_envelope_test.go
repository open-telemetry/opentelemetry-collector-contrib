// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.12.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

const (
	defaultRequestDataEnvelopeName          = "Microsoft.ApplicationInsights.Request"
	defaultRemoteDependencyDataEnvelopeName = "Microsoft.ApplicationInsights.RemoteDependency"
	defaultMessageDataEnvelopeName          = "Microsoft.ApplicationInsights.Message"
	defaultExceptionDataEnvelopeName        = "Microsoft.ApplicationInsights.Exception"
	defaultServiceName                      = "foo"
	defaultServiceNamespace                 = "ns1"
	defaultServiceInstance                  = "112345"
	defaultScopeName                        = "myinstrumentationlib"
	defaultScopeVersion                     = "1.0"
	defaultHTTPMethod                       = http.MethodGet
	defaultHTTPServerSpanName               = "/bar"
	defaultHTTPClientSpanName               = defaultHTTPMethod
	defaultHTTPStatusCode                   = 200
	defaultRPCSystem                        = "grpc"
	defaultRPCSpanName                      = "com.example.ExampleRmiService/exampleMethod"
	defaultRPCStatusCode                    = 0
	defaultDBSystem                         = "mssql"
	defaultDBName                           = "adventureworks"
	defaultDBSpanName                       = "astoredproc"
	defaultDBStatement                      = "exec astoredproc1"
	defaultDBOperation                      = "exec astoredproc2"
	defaultMessagingSpanName                = "MyQueue"
	defaultMessagingSystem                  = "kafka"
	defaultMessagingDestination             = "MyQueue"
	defaultMessagingURL                     = "https://queue.amazonaws.com/80398EXAMPLE/MyQueue"
	defaultInternalSpanName                 = "MethodX"
)

var (
	defaultTraceID                = [16]byte{35, 191, 77, 229, 162, 242, 217, 75, 148, 170, 81, 99, 227, 163, 145, 25}
	defaultTraceIDAsHex           = fmt.Sprintf("%02x", defaultTraceID)
	defaultSpanID                 = [8]byte{35, 191, 77, 229, 162, 242, 217, 76}
	defaultSpanIDAsHex            = fmt.Sprintf("%02x", defaultSpanID)
	defaultParentSpanID           = [8]byte{35, 191, 77, 229, 162, 242, 217, 77}
	defaultParentSpanIDAsHex      = fmt.Sprintf("%02x", defaultParentSpanID)
	defaultSpanStartTime          = pcommon.Timestamp(0)
	defaultSpanEndTme             = pcommon.Timestamp(60000000000)
	defaultSpanDuration           = formatDuration(toTime(defaultSpanEndTme).Sub(toTime(defaultSpanStartTime)))
	defaultSpanEventTime          = pcommon.Timestamp(0)
	defaultHTTPStatusCodeAsString = strconv.FormatInt(defaultHTTPStatusCode, 10)
	defaultRPCStatusCodeAsString  = strconv.FormatInt(defaultRPCStatusCode, 10)

	// Same as RPC codes?
	defaultDatabaseStatusCodeAsString  = strconv.FormatInt(defaultRPCStatusCode, 10)
	defaultMessagingStatusCodeAsString = strconv.FormatInt(defaultRPCStatusCode, 10)

	// Required attribute for any HTTP Span
	requiredHTTPAttributes = map[string]any{
		conventions.AttributeHTTPMethod: defaultHTTPMethod,
	}

	// Required attribute for any RPC Span
	requiredRPCAttributes = map[string]any{
		conventions.AttributeRPCSystem: defaultRPCSystem,
	}

	requiredDatabaseAttributes = map[string]any{
		conventions.AttributeDBSystem: defaultDBSystem,
		conventions.AttributeDBName:   defaultDBName,
	}

	requiredMessagingAttributes = map[string]any{
		conventions.AttributeMessagingSystem:      defaultMessagingSystem,
		conventions.AttributeMessagingDestination: defaultMessagingDestination,
	}

	defaultResource               = getResource()
	defaultInstrumentationLibrary = getScope()
)

/*
	https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/http.md#http-server-semantic-conventions
	We need to test the following attribute sets for HTTP Server Spans:
	http.scheme, http.host, http.target
	http.scheme, http.server_name, net.host.port, http.target
	http.scheme, net.host.name, net.host.port, http.target
	http.url
*/

// Tests proper assignment for a HTTP server span
// http.scheme, http.host, http.target => data.Url
// Also sets a few other things to increase code coverage:
// - a specific SpanStatus as opposed to none
// - an error http.status_code
// - http.route is specified which should replace Span name as part of the RequestData name
// - no http.client_ip or net.peer.ip specified which causes data.Source to be empty
// - adds a few different types of attributes
func TestHTTPServerSpanToRequestDataAttributeSet1(t *testing.T) {
	span := getDefaultHTTPServerSpan()
	span.Status().SetCode(ptrace.StatusCodeError)
	span.Status().SetMessage("Fubar")
	spanAttributes := span.Attributes()

	// http.scheme, http.host, http.target => data.Url
	spanAttributes.PutStr(conventions.AttributeHTTPScheme, "https")
	spanAttributes.PutStr(conventions.AttributeHTTPHost, "foo")
	spanAttributes.PutStr(conventions.AttributeHTTPTarget, "/bar?biz=baz")

	// A non 2xx status code
	spanAttributes.PutInt(conventions.AttributeHTTPStatusCode, 400)

	// A specific http route
	spanAttributes.PutStr(conventions.AttributeHTTPRoute, "bizzle")

	// Unused but should get copied to the RequestData .Properties and .Measurements
	spanAttributes.PutBool("somebool", false)
	spanAttributes.PutDouble("somedouble", 0.1)

	envelopes, _ := spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, zap.NewNop())
	envelope := envelopes[0]
	commonEnvelopeValidations(t, span, envelope, defaultRequestDataEnvelopeName)
	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.RequestData)

	// set verification
	commonRequestDataValidations(t, span, data)

	assert.Equal(t, "400", data.ResponseCode)
	assert.False(t, data.Success)
	assert.Empty(t, data.Source)
	assert.Equal(t, "GET /bizzle", data.Name)
	assert.Equal(t, "https://foo/bar?biz=baz", data.Url)
	assert.Equal(t, span.Status().Message(), data.Properties[attributeOtelStatusDescription])
}

// Tests proper assignment for a HTTP server span
// http.scheme, http.server_name, net.host.port, http.target
// Also tests:
// - net.peer.ip => data.Source
func TestHTTPServerSpanToRequestDataAttributeSet2(t *testing.T) {
	span := getDefaultHTTPServerSpan()
	spanAttributes := span.Attributes()

	spanAttributes.PutInt(conventions.AttributeHTTPStatusCode, defaultHTTPStatusCode)
	spanAttributes.PutStr(conventions.AttributeHTTPScheme, "https")
	spanAttributes.PutStr(conventions.AttributeHTTPServerName, "foo")
	spanAttributes.PutInt(conventions.AttributeNetHostPort, 81)
	spanAttributes.PutStr(conventions.AttributeHTTPTarget, "/bar?biz=baz")

	spanAttributes.PutStr(conventions.AttributeNetPeerIP, "127.0.0.1")

	envelopes, _ := spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, zap.NewNop())
	envelope := envelopes[0]
	commonEnvelopeValidations(t, span, envelope, defaultRequestDataEnvelopeName)
	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.RequestData)

	defaultHTTPRequestDataValidations(t, span, data)
	assert.Equal(t, "127.0.0.1", data.Source)
	assert.Equal(t, "https://foo:81/bar?biz=baz", data.Url)
}

// Tests proper assignment for a HTTP server span
// http.scheme, net.host.name, net.host.port, http.target
// Also tests:
// - http.client_ip and net.peer.ip specified => data.Source from http.client_ip
func TestHTTPServerSpanToRequestDataAttributeSet3(t *testing.T) {
	span := getDefaultHTTPServerSpan()
	spanAttributes := span.Attributes()

	spanAttributes.PutInt(conventions.AttributeHTTPStatusCode, defaultHTTPStatusCode)
	spanAttributes.PutStr(conventions.AttributeHTTPScheme, "https")
	spanAttributes.PutStr(conventions.AttributeNetHostName, "foo")
	spanAttributes.PutInt(conventions.AttributeNetHostPort, 81)
	spanAttributes.PutStr(conventions.AttributeHTTPTarget, "/bar?biz=baz")

	spanAttributes.PutStr(conventions.AttributeHTTPClientIP, "127.0.0.2")
	spanAttributes.PutStr(conventions.AttributeNetPeerIP, "127.0.0.1")

	envelopes, _ := spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, zap.NewNop())
	envelope := envelopes[0]
	commonEnvelopeValidations(t, span, envelope, defaultRequestDataEnvelopeName)
	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.RequestData)
	defaultHTTPRequestDataValidations(t, span, data)
	assert.Equal(t, "127.0.0.2", data.Source)
	assert.Equal(t, "https://foo:81/bar?biz=baz", data.Url)
}

// Tests proper assignment for a HTTP server span
// http.url
func TestHTTPServerSpanToRequestDataAttributeSet4(t *testing.T) {
	span := getDefaultHTTPServerSpan()
	spanAttributes := span.Attributes()

	spanAttributes.PutInt(conventions.AttributeHTTPStatusCode, defaultHTTPStatusCode)
	spanAttributes.PutStr(conventions.AttributeHTTPURL, "https://foo:81/bar?biz=baz")

	envelopes, _ := spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, zap.NewNop())
	envelope := envelopes[0]
	commonEnvelopeValidations(t, span, envelope, defaultRequestDataEnvelopeName)
	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.RequestData)
	defaultHTTPRequestDataValidations(t, span, data)
	assert.Equal(t, "https://foo:81/bar?biz=baz", data.Url)
}

/*
	https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/http.md#http-server-semantic-conventions
	We need to test the following attribute sets for HTTP Client Spans:
	http.url
	http.scheme, http.host, http.target
	http.scheme, net.peer.name, net.peer.port, http.target
	http.scheme, net.peer.ip, net.peer.port, http.target
*/

// Tests proper assignment for a HTTP client span
// http.url
// Also tests:
// - a non 2xx http.status_code
func TestHTTPClientSpanToRemoteDependencyAttributeSet1(t *testing.T) {
	span := getDefaultHTTPClientSpan()
	spanAttributes := span.Attributes()

	spanAttributes.PutStr(conventions.AttributeHTTPURL, "https://foo:81/bar?biz=baz")
	spanAttributes.PutInt(conventions.AttributeHTTPStatusCode, 400)

	envelopes, _ := spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, zap.NewNop())
	envelope := envelopes[0]
	commonEnvelopeValidations(t, span, envelope, defaultRemoteDependencyDataEnvelopeName)
	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)
	commonRemoteDependencyDataValidations(t, span, data)

	assert.Equal(t, "400", data.ResultCode)
	assert.False(t, data.Success)
	assert.Equal(t, defaultHTTPMethod, data.Name)
	assert.Equal(t, "https://foo:81/bar?biz=baz", data.Data)
	assert.Equal(t, "HTTP", data.Type)
}

// Tests proper assignment for a HTTP client span
// http.scheme, http.host, http.target
// Also tests:
// - a specific http.route is specified => data.Name
func TestHTTPClientSpanToRemoteDependencyAttributeSet2(t *testing.T) {
	span := getDefaultHTTPClientSpan()
	spanAttributes := span.Attributes()

	// http.scheme, http.host, http.target => data.Url
	spanAttributes.PutInt(conventions.AttributeHTTPStatusCode, defaultHTTPStatusCode)
	spanAttributes.PutStr(conventions.AttributeHTTPScheme, "https")
	spanAttributes.PutStr(conventions.AttributeHTTPHost, "foo")
	spanAttributes.PutStr(conventions.AttributeHTTPTarget, "bar/12345?biz=baz")

	// A specific http.route
	spanAttributes.PutStr(conventions.AttributeHTTPRoute, "/bar/:baz_id")

	envelopes, _ := spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, zap.NewNop())
	envelope := envelopes[0]
	commonEnvelopeValidations(t, span, envelope, defaultRemoteDependencyDataEnvelopeName)
	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)
	commonRemoteDependencyDataValidations(t, span, data)

	assert.Equal(t, "200", data.ResultCode)
	assert.True(t, data.Success)
	assert.Equal(t, defaultHTTPMethod+" /bar/:baz_id", data.Name)
	assert.Equal(t, "https://foo/bar/12345?biz=baz", data.Data)
	assert.Equal(t, "HTTP", data.Type)
}

// Tests proper assignment for a HTTP client span
// http.scheme, net.peer.name, net.peer.port, http.target
func TestHTTPClientSpanToRemoteDependencyAttributeSet3(t *testing.T) {
	span := getDefaultHTTPClientSpan()
	spanAttributes := span.Attributes()

	spanAttributes.PutInt(conventions.AttributeHTTPStatusCode, defaultHTTPStatusCode)
	spanAttributes.PutStr(conventions.AttributeHTTPScheme, "https")
	spanAttributes.PutStr(conventions.AttributeNetPeerName, "foo")
	spanAttributes.PutInt(conventions.AttributeNetPeerPort, 81)
	spanAttributes.PutStr(conventions.AttributeHTTPTarget, "/bar?biz=baz")

	envelopes, _ := spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, zap.NewNop())
	envelope := envelopes[0]
	commonEnvelopeValidations(t, span, envelope, defaultRemoteDependencyDataEnvelopeName)
	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)
	defaultHTTPRemoteDependencyDataValidations(t, span, data)
	assert.Equal(t, "https://foo:81/bar?biz=baz", data.Data)
}

// Tests proper assignment for a HTTP client span
// http.scheme, net.peer.ip, net.peer.port, http.target, enduser.id
func TestHTTPClientSpanToRemoteDependencyAttributeSet4(t *testing.T) {
	span := getDefaultHTTPClientSpan()
	spanAttributes := span.Attributes()

	spanAttributes.PutInt(conventions.AttributeHTTPStatusCode, defaultHTTPStatusCode)
	spanAttributes.PutStr(conventions.AttributeHTTPScheme, "https")
	spanAttributes.PutStr(conventions.AttributeNetPeerIP, "127.0.0.1")
	spanAttributes.PutInt(conventions.AttributeNetPeerPort, 81)
	spanAttributes.PutStr(conventions.AttributeHTTPTarget, "/bar?biz=baz")
	spanAttributes.PutStr(conventions.AttributeEnduserID, "12345")

	envelopes, _ := spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, zap.NewNop())
	envelope := envelopes[0]
	commonEnvelopeValidations(t, span, envelope, defaultRemoteDependencyDataEnvelopeName)
	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)
	defaultHTTPRemoteDependencyDataValidations(t, span, data)
	assert.Equal(t, "https://127.0.0.1:81/bar?biz=baz", data.Data)
	assert.Equal(t, "12345", envelope.Tags[contracts.UserId])
}

// Tests proper assignment for an RPC server span
func TestRPCServerSpanToRequestData(t *testing.T) {
	span := getDefaultRPCServerSpan()
	spanAttributes := span.Attributes()

	spanAttributes.PutStr(conventions.AttributeNetPeerName, "foo")
	spanAttributes.PutStr(conventions.AttributeNetPeerIP, "127.0.0.1")
	spanAttributes.PutInt(conventions.AttributeNetPeerPort, 81)

	envelopes, _ := spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, zap.NewNop())
	envelope := envelopes[0]
	commonEnvelopeValidations(t, span, envelope, defaultRequestDataEnvelopeName)
	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.RequestData)
	defaultRPCRequestDataValidations(t, span, data, "foo:81")

	// test fallback to peerip
	spanAttributes.PutStr(conventions.AttributeNetPeerName, "")
	spanAttributes.PutStr(conventions.AttributeNetPeerIP, "127.0.0.1")

	envelopes, _ = spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, zap.NewNop())
	envelope = envelopes[0]
	data = envelope.Data.(*contracts.Data).BaseData.(*contracts.RequestData)
	defaultRPCRequestDataValidations(t, span, data, "127.0.0.1:81")
}

// Tests proper assignment for an RPC client span
func TestRPCClientSpanToRemoteDependencyData(t *testing.T) {
	span := getDefaultRPCClientSpan()
	spanAttributes := span.Attributes()

	spanAttributes.PutStr(conventions.AttributeNetPeerName, "foo")
	spanAttributes.PutInt(conventions.AttributeNetPeerPort, 81)
	spanAttributes.PutStr(conventions.AttributeNetPeerIP, "127.0.0.1")

	envelopes, _ := spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, zap.NewNop())
	envelope := envelopes[0]
	commonEnvelopeValidations(t, span, envelope, defaultRemoteDependencyDataEnvelopeName)
	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)
	defaultRPCRemoteDependencyDataValidations(t, span, data, "foo:81")

	// test fallback to peerip
	spanAttributes.PutStr(conventions.AttributeNetPeerName, "")
	spanAttributes.PutStr(conventions.AttributeNetPeerIP, "127.0.0.1")

	envelopes, _ = spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, zap.NewNop())
	envelope = envelopes[0]
	data = envelope.Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)
	defaultRPCRemoteDependencyDataValidations(t, span, data, "127.0.0.1:81")

	// test RPC error using the new rpc.grpc.status_code attribute
	span.Status().SetCode(ptrace.StatusCodeError)
	span.Status().SetMessage("Resource exhausted")
	spanAttributes.PutInt(conventions.AttributeRPCGRPCStatusCode, 8)

	envelopes, _ = spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, zap.NewNop())
	envelope = envelopes[0]
	data = envelope.Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)

	assert.Equal(t, "8", data.ResultCode)
	assert.Equal(t, traceutil.StatusCodeStr(span.Status().Code()), data.Properties[attributeOtelStatusCode])
	assert.Equal(t, span.Status().Message(), data.Properties[attributeOtelStatusDescription])
}

// Tests proper assignment for a Database client span
func TestDatabaseClientSpanToRemoteDependencyData(t *testing.T) {
	span := getDefaultDatabaseClientSpan()
	spanAttributes := span.Attributes()

	spanAttributes.PutStr(conventions.AttributeDBStatement, defaultDBStatement)
	spanAttributes.PutStr(conventions.AttributeNetPeerName, "foo")
	spanAttributes.PutInt(conventions.AttributeNetPeerPort, 81)

	envelopes, _ := spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, zap.NewNop())
	envelope := envelopes[0]
	commonEnvelopeValidations(t, span, envelopes[0], defaultRemoteDependencyDataEnvelopeName)
	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)
	defaultDatabaseRemoteDependencyDataValidations(t, span, data)

	assert.Equal(t, "foo:81", data.Target)
	assert.Equal(t, defaultDBStatement, data.Data)

	// Test the fallback to data.Data fallback to DBOperation
	spanAttributes.PutStr(conventions.AttributeDBStatement, "")
	spanAttributes.PutStr(conventions.AttributeDBOperation, defaultDBOperation)

	envelopes, _ = spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, zap.NewNop())
	envelope = envelopes[0]
	data = envelope.Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)
	assert.Equal(t, defaultDBOperation, data.Data)
}

// Tests proper assignment for a Messaging consumer span
func TestMessagingConsumerSpanToRequestData(t *testing.T) {
	span := getDefaultMessagingConsumerSpan()
	spanAttributes := span.Attributes()

	spanAttributes.PutStr(conventions.AttributeMessagingURL, defaultMessagingURL)
	spanAttributes.PutStr(conventions.AttributeNetPeerName, "foo")
	spanAttributes.PutInt(conventions.AttributeNetPeerPort, 81)

	envelopes, _ := spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, zap.NewNop())
	envelope := envelopes[0]
	commonEnvelopeValidations(t, span, envelope, defaultRequestDataEnvelopeName)
	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.RequestData)
	defaultMessagingRequestDataValidations(t, span, data)

	assert.Equal(t, defaultMessagingURL, data.Source)

	// test fallback from MessagingURL to net.* properties
	spanAttributes.PutStr(conventions.AttributeMessagingURL, "")

	envelopes, _ = spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, zap.NewNop())
	envelope = envelopes[0]
	data = envelope.Data.(*contracts.Data).BaseData.(*contracts.RequestData)

	assert.Equal(t, "foo:81", data.Source)
}

// Tests proper assignment for a Messaging producer span
func TestMessagingProducerSpanToRequestData(t *testing.T) {
	span := getDefaultMessagingProducerSpan()
	spanAttributes := span.Attributes()

	spanAttributes.PutStr(conventions.AttributeMessagingURL, defaultMessagingURL)
	spanAttributes.PutStr(conventions.AttributeNetPeerName, "foo")
	spanAttributes.PutInt(conventions.AttributeNetPeerPort, 81)

	envelopes, _ := spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, zap.NewNop())
	envelope := envelopes[0]
	commonEnvelopeValidations(t, span, envelope, defaultRemoteDependencyDataEnvelopeName)
	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)
	defaultMessagingRemoteDependencyDataValidations(t, span, data)

	assert.Equal(t, defaultMessagingURL, data.Target)

	// test fallback from MessagingURL to net.* properties
	spanAttributes.PutStr(conventions.AttributeMessagingURL, "")

	envelopes, _ = spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, zap.NewNop())
	envelope = envelopes[0]
	data = envelope.Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)

	assert.Equal(t, "foo:81", data.Target)
}

// Tests proper assignment for an internal span and enduser.id
func TestUnknownInternalSpanToRemoteDependencyData(t *testing.T) {
	span := getDefaultInternalSpan()
	spanAttributes := span.Attributes()

	spanAttributes.PutStr("foo", "bar")
	spanAttributes.PutStr(conventions.AttributeEnduserID, "4567")

	envelopes, _ := spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, zap.NewNop())
	envelope := envelopes[0]
	commonEnvelopeValidations(t, span, envelope, defaultRemoteDependencyDataEnvelopeName)
	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)
	defaultInternalRemoteDependencyDataValidations(t, span, data)
	assert.Equal(t, "4567", envelope.Tags[contracts.UserId])
}

// Tests that spans with unspecified kind are treated similar to internal spans
func TestUnspecifiedSpanToInProcRemoteDependencyData(t *testing.T) {
	span := getDefaultInternalSpan()
	span.SetKind(ptrace.SpanKindUnspecified)

	envelopes, _ := spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, zap.NewNop())
	envelope := envelopes[0]
	commonEnvelopeValidations(t, span, envelope, defaultRemoteDependencyDataEnvelopeName)
	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)
	defaultInternalRemoteDependencyDataValidations(t, span, data)
}

func TestSpanWithEventsToEnvelopes(t *testing.T) {
	span := getDefaultRPCClientSpan()

	spanEvent := getSpanEvent("foo", map[string]any{"bar": "baz"})
	spanEvent.CopyTo(span.Events().AppendEmpty())

	exceptionType := "foo"
	exceptionMessage := "bar"
	exceptionStackTrace := "baz"

	exceptionEvent := getSpanEvent("exception", map[string]any{
		conventions.AttributeExceptionType:       exceptionType,
		conventions.AttributeExceptionMessage:    exceptionMessage,
		conventions.AttributeExceptionStacktrace: exceptionStackTrace,
	})

	exceptionEvent.CopyTo(span.Events().AppendEmpty())

	envelopes, _ := spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, zap.NewNop())

	assert.NotNil(t, envelopes)
	assert.Len(t, envelopes, 3)

	validateEnvelope := func(spanEvent ptrace.SpanEvent, envelope *contracts.Envelope, targetEnvelopeName string) {
		assert.Equal(t, targetEnvelopeName, envelope.Name)
		assert.Equal(t, toTime(spanEvent.Timestamp()).Format(time.RFC3339Nano), envelope.Time)
		assert.Equal(t, defaultTraceIDAsHex, envelope.Tags[contracts.OperationId])
		assert.Equal(t, defaultSpanIDAsHex, envelope.Tags[contracts.OperationParentId])
		assert.Equal(t, defaultServiceNamespace+"."+defaultServiceName, envelope.Tags[contracts.CloudRole])
		assert.Equal(t, defaultServiceInstance, envelope.Tags[contracts.CloudRoleInstance])
		assert.NotNil(t, envelope.Data)
	}

	// We are ignoring the first envelope which is the span. Tested elsewhere
	validateEnvelope(spanEvent, envelopes[1], defaultMessageDataEnvelopeName)
	messageData := envelopes[1].Data.(*contracts.Data).BaseData.(*contracts.MessageData)
	assert.Equal(t, "foo", messageData.Message)

	validateEnvelope(exceptionEvent, envelopes[2], defaultExceptionDataEnvelopeName)
	exceptionData := envelopes[2].Data.(*contracts.Data).BaseData.(*contracts.ExceptionData)
	exceptionDetails := exceptionData.Exceptions[0]
	assert.Equal(t, exceptionType, exceptionDetails.TypeName)
	assert.Equal(t, exceptionMessage, exceptionDetails.Message)
	assert.Equal(t, exceptionStackTrace, exceptionDetails.Stack)
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

	sanitizeWithCallback(sanitizeFunc, warningCallback, zap.NewNop())
	assert.Equal(t, 4, warningCounter)
}

// Tests proper conversion of span links to envelope tags
func TestApplyLinksToEnvelope(t *testing.T) {
	properties := make(map[string]string)

	span := ptrace.NewSpan()

	link := span.Links().AppendEmpty()
	link.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	link.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	link2 := span.Links().AppendEmpty()
	link2.SetTraceID([16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1})
	link2.SetSpanID([8]byte{8, 7, 6, 5, 4, 3, 2, 1})

	applyLinksToDataProperties(properties, span.Links(), zap.NewNop())

	expectedLinks := []msLink{
		{
			OperationID: "0102030405060708090a0b0c0d0e0f10",
			ID:          "0102030405060708",
		},
		{
			OperationID: "100f0e0d0c0b0a090807060504030201",
			ID:          "0807060504030201",
		},
	}

	linksJSON := properties[msLinks]
	assert.NotEmpty(t, linksJSON)

	var actualLinks []msLink
	err := json.Unmarshal([]byte(linksJSON), &actualLinks)
	assert.NoError(t, err)
	assert.Equal(t, expectedLinks, actualLinks)
}

// Tests handling of empty span links
func TestApplyLinksToEnvelopeWithNoLinks(t *testing.T) {
	properties := make(map[string]string)

	span := ptrace.NewSpan()
	applyLinksToDataProperties(properties, span.Links(), zap.NewNop())

	_, exists := properties[msLinks]
	assert.False(t, exists)
}

/*
These methods are for handling some common validations
*/
func commonEnvelopeValidations(
	t *testing.T,
	span ptrace.Span,
	envelope *contracts.Envelope,
	expectedEnvelopeName string,
) {
	assert.NotNil(t, envelope)
	assert.Equal(t, expectedEnvelopeName, envelope.Name)
	assert.Equal(t, toTime(span.StartTimestamp()).Format(time.RFC3339Nano), envelope.Time)
	assert.Equal(t, defaultTraceIDAsHex, envelope.Tags[contracts.OperationId])
	assert.Equal(t, defaultParentSpanIDAsHex, envelope.Tags[contracts.OperationParentId])
	assert.Equal(t, defaultServiceNamespace+"."+defaultServiceName, envelope.Tags[contracts.CloudRole])
	assert.Equal(t, defaultServiceInstance, envelope.Tags[contracts.CloudRoleInstance])
	assert.Contains(t, envelope.Tags[contracts.InternalSdkVersion], "otelc-")
	assert.NotNil(t, envelope.Data)

	if expectedEnvelopeName == defaultRequestDataEnvelopeName {
		requestData := envelope.Data.(*contracts.Data).BaseData.(*contracts.RequestData)
		assert.Equal(t, requestData.Name, envelope.Tags[contracts.OperationName])
	}
}

// Validate common stuff across any Span -> RequestData translation
func commonRequestDataValidations(
	t *testing.T,
	span ptrace.Span,
	data *contracts.RequestData,
) {
	assertAttributesCopiedToProperties(t, span.Attributes(), data.Properties)
	assert.Equal(t, defaultSpanIDAsHex, data.Id)
	assert.Equal(t, defaultSpanDuration, data.Duration)

	assert.Equal(t, defaultScopeName, data.Properties[instrumentationLibraryName])
	assert.Equal(t, defaultScopeVersion, data.Properties[instrumentationLibraryVersion])
}

// Validate common RequestData values for HTTP Spans created using the default test values
func defaultHTTPRequestDataValidations(
	t *testing.T,
	span ptrace.Span,
	data *contracts.RequestData,
) {
	commonRequestDataValidations(t, span, data)

	assert.Equal(t, defaultHTTPStatusCodeAsString, data.ResponseCode)
	assert.Equal(t, defaultHTTPStatusCode <= 399, data.Success)
	assert.Equal(t, defaultHTTPMethod+" "+defaultHTTPServerSpanName, data.Name)
}

// Validate common stuff across any Span -> RemoteDependencyData translation
func commonRemoteDependencyDataValidations(
	t *testing.T,
	span ptrace.Span,
	data *contracts.RemoteDependencyData,
) {
	assertAttributesCopiedToProperties(t, span.Attributes(), data.Properties)
	assert.Equal(t, defaultSpanIDAsHex, data.Id)
	assert.Equal(t, defaultSpanDuration, data.Duration)
}

// Validate common RemoteDependencyData values for HTTP Spans created using the default test values
func defaultHTTPRemoteDependencyDataValidations(
	t *testing.T,
	span ptrace.Span,
	data *contracts.RemoteDependencyData,
) {
	commonRemoteDependencyDataValidations(t, span, data)

	assert.Equal(t, defaultHTTPStatusCodeAsString, data.ResultCode)
	assert.Equal(t, defaultHTTPStatusCode >= 200 && defaultHTTPStatusCode <= 299, data.Success)
	assert.Equal(t, defaultHTTPClientSpanName, data.Name)
	assert.Equal(t, "HTTP", data.Type)
}

func defaultRPCRequestDataValidations(
	t *testing.T,
	span ptrace.Span,
	data *contracts.RequestData,
	expectedDataSource string,
) {
	commonRequestDataValidations(t, span, data)

	assert.Equal(t, defaultRPCStatusCodeAsString, data.ResponseCode)
	assert.True(t, data.Success)
	assert.Equal(t, defaultRPCSystem+" "+defaultRPCSpanName, data.Name)
	assert.Equal(t, data.Name, data.Url)
	assert.Equal(t, expectedDataSource, data.Source)
}

func defaultRPCRemoteDependencyDataValidations(
	t *testing.T,
	span ptrace.Span,
	data *contracts.RemoteDependencyData,
	expectedDataTarget string,
) {
	commonRemoteDependencyDataValidations(t, span, data)

	assert.Equal(t, defaultRPCStatusCodeAsString, data.ResultCode)
	assert.True(t, data.Success)
	assert.Equal(t, defaultRPCSpanName, data.Name)
	assert.Equal(t, defaultRPCSystem, data.Type)

	// .Data should be set to .Name. .Name contains the full RPC method name
	assert.Equal(t, data.Name, data.Data)
	assert.Equal(t, expectedDataTarget, data.Target)
}

func defaultDatabaseRemoteDependencyDataValidations(
	t *testing.T,
	span ptrace.Span,
	data *contracts.RemoteDependencyData,
) {
	commonRemoteDependencyDataValidations(t, span, data)

	assert.Equal(t, defaultDatabaseStatusCodeAsString, data.ResultCode)
	assert.True(t, data.Success)
	assert.Equal(t, defaultDBSpanName, data.Name)
	assert.Equal(t, defaultDBSystem, data.Type)
}

func defaultMessagingRequestDataValidations(
	t *testing.T,
	span ptrace.Span,
	data *contracts.RequestData,
) {
	commonRequestDataValidations(t, span, data)

	assert.Equal(t, defaultMessagingStatusCodeAsString, data.ResponseCode)
	assert.True(t, data.Success)
	assert.Equal(t, defaultMessagingDestination, data.Name)
}

func defaultMessagingRemoteDependencyDataValidations(
	t *testing.T,
	span ptrace.Span,
	data *contracts.RemoteDependencyData,
) {
	commonRemoteDependencyDataValidations(t, span, data)

	assert.Equal(t, defaultMessagingStatusCodeAsString, data.ResultCode)
	assert.True(t, data.Success)
	assert.Equal(t, defaultMessagingDestination, data.Name)
	assert.Equal(t, defaultMessagingSystem, data.Type)
}

func defaultInternalRemoteDependencyDataValidations(
	t *testing.T,
	span ptrace.Span,
	data *contracts.RemoteDependencyData,
) {
	assertAttributesCopiedToProperties(t, span.Attributes(), data.Properties)
	assert.Equal(t, "InProc", data.Type)
}

// Verifies that all attributes are copies to either the properties maps of the envelope's data element
func assertAttributesCopiedToProperties(
	t *testing.T,
	attributeMap pcommon.Map,
	properties map[string]string,
) {
	for k, v := range attributeMap.All() {
		p, exists := properties[k]
		assert.True(t, exists)

		switch v.Type() {
		case pcommon.ValueTypeStr:
			assert.Equal(t, v.Str(), p)
		case pcommon.ValueTypeBool:
			assert.Equal(t, strconv.FormatBool(v.Bool()), p)
		case pcommon.ValueTypeInt:
			assert.Equal(t, strconv.FormatInt(v.Int(), 10), p)
		case pcommon.ValueTypeDouble:
			assert.Equal(t, strconv.FormatFloat(v.Double(), 'f', -1, 64), p)
		}
	}
}

/*
The remainder of these methods are for building up test assets
*/
func getSpan(spanName string, spanKind ptrace.SpanKind, initialAttributes map[string]any) ptrace.Span {
	span := ptrace.NewSpan()
	span.SetTraceID(defaultTraceID)
	span.SetSpanID(defaultSpanID)
	span.SetParentSpanID(defaultParentSpanID)
	span.SetName(spanName)
	span.SetKind(spanKind)
	span.SetStartTimestamp(defaultSpanStartTime)
	span.SetEndTimestamp(defaultSpanEndTme)
	//nolint:errcheck
	span.Attributes().FromRaw(initialAttributes)
	return span
}

// Returns a default span event
func getSpanEvent(name string, initialAttributes map[string]any) ptrace.SpanEvent {
	spanEvent := ptrace.NewSpanEvent()
	spanEvent.SetName(name)
	spanEvent.SetTimestamp(defaultSpanEventTime)
	//nolint:errcheck
	spanEvent.Attributes().FromRaw(initialAttributes)
	return spanEvent
}

// Returns a default server span
func getServerSpan(spanName string, initialAttributes map[string]any) ptrace.Span {
	return getSpan(spanName, ptrace.SpanKindServer, initialAttributes)
}

// Returns a default client span
func getClientSpan(spanName string, initialAttributes map[string]any) ptrace.Span {
	return getSpan(spanName, ptrace.SpanKindClient, initialAttributes)
}

// Returns a default consumer span
func getConsumerSpan(spanName string, initialAttributes map[string]any) ptrace.Span {
	return getSpan(spanName, ptrace.SpanKindConsumer, initialAttributes)
}

// Returns a default producer span
func getProducerSpan(spanName string, initialAttributes map[string]any) ptrace.Span {
	return getSpan(spanName, ptrace.SpanKindProducer, initialAttributes)
}

// Returns a default internal span
func getInternalSpan(spanName string, initialAttributes map[string]any) ptrace.Span {
	return getSpan(spanName, ptrace.SpanKindInternal, initialAttributes)
}

func getDefaultHTTPServerSpan() ptrace.Span {
	return getServerSpan(
		defaultHTTPServerSpanName,
		requiredHTTPAttributes)
}

func getDefaultHTTPClientSpan() ptrace.Span {
	return getClientSpan(
		defaultHTTPClientSpanName,
		requiredHTTPAttributes)
}

func getDefaultRPCServerSpan() ptrace.Span {
	return getServerSpan(
		defaultRPCSpanName,
		requiredRPCAttributes)
}

func getDefaultRPCClientSpan() ptrace.Span {
	return getClientSpan(
		defaultRPCSpanName,
		requiredRPCAttributes)
}

func getDefaultDatabaseClientSpan() ptrace.Span {
	return getClientSpan(
		defaultDBSpanName,
		requiredDatabaseAttributes)
}

func getDefaultMessagingConsumerSpan() ptrace.Span {
	return getConsumerSpan(
		defaultMessagingSpanName,
		requiredMessagingAttributes)
}

func getDefaultMessagingProducerSpan() ptrace.Span {
	return getProducerSpan(
		defaultMessagingSpanName,
		requiredMessagingAttributes)
}

func getDefaultInternalSpan() ptrace.Span {
	return getInternalSpan(
		defaultInternalSpanName,
		map[string]any{})
}

// Returns a default Resource
func getResource() pcommon.Resource {
	r := pcommon.NewResource()
	r.Attributes().PutStr(conventions.AttributeServiceName, defaultServiceName)
	r.Attributes().PutStr(conventions.AttributeServiceNamespace, defaultServiceNamespace)
	r.Attributes().PutStr(conventions.AttributeServiceInstanceID, defaultServiceInstance)
	return r
}

// Returns a default instrumentation library
func getScope() pcommon.InstrumentationScope {
	il := pcommon.NewInstrumentationScope()
	il.SetName(defaultScopeName)
	il.SetVersion(defaultScopeVersion)
	return il
}
