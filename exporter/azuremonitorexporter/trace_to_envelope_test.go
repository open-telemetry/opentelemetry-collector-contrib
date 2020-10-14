// Copyright OpenTelemetry Authors
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
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"
)

const (
	defaultRequestDataEnvelopeName          = "Microsoft.ApplicationInsights.Request"
	defaultRemoteDependencyDataEnvelopeName = "Microsoft.ApplicationInsights.RemoteDependency"
	defaultServiceName                      = "foo"
	defaultServiceNamespace                 = "ns1"
	defaultServiceInstance                  = "112345"
	defaultInstrumentationLibraryName       = "myinstrumentationlib"
	defaultInstrumentationLibraryVersion    = "1.0"
	defaultHTTPMethod                       = "GET"
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
	defaultTraceID                = []byte{35, 191, 77, 229, 162, 242, 217, 75, 148, 170, 81, 99, 227, 163, 145, 25}
	defaultTraceIDAsHex           = fmt.Sprintf("%02x", defaultTraceID)
	defaultSpanID                 = []byte{35, 191, 77, 229, 162, 242, 217, 75, 148, 170, 81, 99, 227, 163, 145, 26}
	defaultSpanIDAsHex            = fmt.Sprintf("%02x", defaultSpanID)
	defaultParentSpanID           = []byte{35, 191, 77, 229, 162, 242, 217, 75, 148, 170, 81, 99, 227, 163, 145, 27}
	defaultParentSpanIDAsHex      = fmt.Sprintf("%02x", defaultParentSpanID)
	defaultSpanStartTime          = pdata.TimestampUnixNano(0)
	defaultSpanEndTme             = pdata.TimestampUnixNano(60000000000)
	defaultSpanDuration           = formatDuration(toTime(defaultSpanEndTme).Sub(toTime(defaultSpanStartTime)))
	defaultHTTPStatusCodeAsString = strconv.FormatInt(defaultHTTPStatusCode, 10)
	defaultRPCStatusCodeAsString  = strconv.FormatInt(defaultRPCStatusCode, 10)

	// Same as RPC codes?
	defaultDatabaseStatusCodeAsString  = strconv.FormatInt(defaultRPCStatusCode, 10)
	defaultMessagingStatusCodeAsString = strconv.FormatInt(defaultRPCStatusCode, 10)

	// Required attribute for any HTTP Span
	requiredHTTPAttributes = map[string]pdata.AttributeValue{
		conventions.AttributeHTTPMethod: pdata.NewAttributeValueString(defaultHTTPMethod),
	}

	// Required attribute for any RPC Span
	requiredRPCAttributes = map[string]pdata.AttributeValue{
		conventions.AttributeRPCSystem: pdata.NewAttributeValueString(defaultRPCSystem),
	}

	requiredDatabaseAttributes = map[string]pdata.AttributeValue{
		attributeDBSystem: pdata.NewAttributeValueString(defaultDBSystem),
		attributeDBName:   pdata.NewAttributeValueString(defaultDBName),
	}

	requiredMessagingAttributes = map[string]pdata.AttributeValue{
		conventions.AttributeMessagingSystem:      pdata.NewAttributeValueString(defaultMessagingSystem),
		conventions.AttributeMessagingDestination: pdata.NewAttributeValueString(defaultMessagingDestination),
	}

	defaultResource               = getResource()
	defaultInstrumentationLibrary = getInstrumentationLibrary()
)

/*
	https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/trace/semantic_conventions/http.md#http-server-semantic-conventions
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
// - an error  http.status_code
// - http.route is specified which should replace Span name as part of the RequestData name
// - no  http.client_ip or net.peer.ip specified which causes data.Source to be empty
// - adds a few different types of attributes
func TestHTTPServerSpanToRequestDataAttributeSet1(t *testing.T) {
	span := getDefaultHTTPServerSpan()
	span.Status().InitEmpty()
	span.Status().SetCode(0)
	spanAttributes := span.Attributes()

	set := map[string]pdata.AttributeValue{
		// http.scheme, http.host, http.target => data.Url
		conventions.AttributeHTTPScheme: pdata.NewAttributeValueString("https"),
		conventions.AttributeHTTPHost:   pdata.NewAttributeValueString("foo"),
		conventions.AttributeHTTPTarget: pdata.NewAttributeValueString("/bar?biz=baz"),

		// A non 2xx status code
		conventions.AttributeHTTPStatusCode: pdata.NewAttributeValueInt(400),

		// A specific http route
		conventions.AttributeHTTPRoute: pdata.NewAttributeValueString("bizzle"),

		// Unused but should get copied to the RequestData .Properties and .Measurements
		"somebool":   pdata.NewAttributeValueBool(false),
		"somedouble": pdata.NewAttributeValueDouble(0.1),
	}

	appendToAttributeMap(spanAttributes, set)

	envelope, _ := spanToEnvelope(defaultResource, defaultInstrumentationLibrary, span, zap.NewNop())
	commonEnvelopeValidations(t, span, envelope, defaultRequestDataEnvelopeName)
	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.RequestData)

	// set verification
	commonRequestDataValidations(t, span, data)

	assert.Equal(t, "400", data.ResponseCode)
	assert.False(t, data.Success)
	assert.Equal(t, "", data.Source)
	assert.Equal(t, "GET /bizzle", data.Name)
	assert.Equal(t, "https://foo/bar?biz=baz", data.Url)
}

// Tests proper assignment for a HTTP server span
// http.scheme, http.server_name, net.host.port, http.target
// Also tests:
// - net.peer.ip => data.Source
func TestHTTPServerSpanToRequestDataAttributeSet2(t *testing.T) {
	span := getDefaultHTTPServerSpan()
	spanAttributes := span.Attributes()

	appendToAttributeMap(
		spanAttributes,
		map[string]pdata.AttributeValue{
			conventions.AttributeHTTPStatusCode: pdata.NewAttributeValueInt(defaultHTTPStatusCode),
			conventions.AttributeHTTPScheme:     pdata.NewAttributeValueString("https"),
			conventions.AttributeHTTPServerName: pdata.NewAttributeValueString("foo"),
			conventions.AttributeNetHostPort:    pdata.NewAttributeValueInt(81),
			conventions.AttributeHTTPTarget:     pdata.NewAttributeValueString("/bar?biz=baz"),

			conventions.AttributeNetPeerIP: pdata.NewAttributeValueString("127.0.0.1"),
		})

	envelope, _ := spanToEnvelope(defaultResource, defaultInstrumentationLibrary, span, zap.NewNop())
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

	appendToAttributeMap(
		spanAttributes,
		map[string]pdata.AttributeValue{
			conventions.AttributeHTTPStatusCode: pdata.NewAttributeValueInt(defaultHTTPStatusCode),
			conventions.AttributeHTTPScheme:     pdata.NewAttributeValueString("https"),
			conventions.AttributeNetHostName:    pdata.NewAttributeValueString("foo"),
			conventions.AttributeNetHostPort:    pdata.NewAttributeValueInt(81),
			conventions.AttributeHTTPTarget:     pdata.NewAttributeValueString("/bar?biz=baz"),

			conventions.AttributeHTTPClientIP: pdata.NewAttributeValueString("127.0.0.2"),
			conventions.AttributeNetPeerIP:    pdata.NewAttributeValueString("127.0.0.1"),
		})

	envelope, _ := spanToEnvelope(defaultResource, defaultInstrumentationLibrary, span, zap.NewNop())
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

	appendToAttributeMap(
		spanAttributes,
		map[string]pdata.AttributeValue{
			conventions.AttributeHTTPStatusCode: pdata.NewAttributeValueInt(defaultHTTPStatusCode),
			conventions.AttributeHTTPURL:        pdata.NewAttributeValueString("https://foo:81/bar?biz=baz"),
		})

	envelope, _ := spanToEnvelope(defaultResource, defaultInstrumentationLibrary, span, zap.NewNop())
	commonEnvelopeValidations(t, span, envelope, defaultRequestDataEnvelopeName)
	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.RequestData)
	defaultHTTPRequestDataValidations(t, span, data)
	assert.Equal(t, "https://foo:81/bar?biz=baz", data.Url)
}

/*
	https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/trace/semantic_conventions/http.md#http-server-semantic-conventions
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

	appendToAttributeMap(
		spanAttributes,
		map[string]pdata.AttributeValue{
			conventions.AttributeHTTPURL: pdata.NewAttributeValueString("https://foo:81/bar?biz=baz"),

			conventions.AttributeHTTPStatusCode: pdata.NewAttributeValueInt(400),
		})

	envelope, _ := spanToEnvelope(defaultResource, defaultInstrumentationLibrary, span, zap.NewNop())
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

	appendToAttributeMap(
		spanAttributes,
		map[string]pdata.AttributeValue{
			// http.scheme, http.host, http.target => data.Url
			conventions.AttributeHTTPStatusCode: pdata.NewAttributeValueInt(defaultHTTPStatusCode),
			conventions.AttributeHTTPScheme:     pdata.NewAttributeValueString("https"),
			conventions.AttributeHTTPHost:       pdata.NewAttributeValueString("foo"),
			conventions.AttributeHTTPTarget:     pdata.NewAttributeValueString("bar/12345?biz=baz"),

			// A specific http.route
			conventions.AttributeHTTPRoute: pdata.NewAttributeValueString("/bar/:baz_id"),
		})

	envelope, _ := spanToEnvelope(defaultResource, defaultInstrumentationLibrary, span, zap.NewNop())
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

	appendToAttributeMap(
		spanAttributes,
		map[string]pdata.AttributeValue{
			conventions.AttributeHTTPStatusCode: pdata.NewAttributeValueInt(defaultHTTPStatusCode),
			conventions.AttributeHTTPScheme:     pdata.NewAttributeValueString("https"),
			conventions.AttributeNetPeerName:    pdata.NewAttributeValueString("foo"),
			conventions.AttributeNetPeerPort:    pdata.NewAttributeValueInt(81),
			conventions.AttributeHTTPTarget:     pdata.NewAttributeValueString("/bar?biz=baz"),
		})

	envelope, _ := spanToEnvelope(defaultResource, defaultInstrumentationLibrary, span, zap.NewNop())
	commonEnvelopeValidations(t, span, envelope, defaultRemoteDependencyDataEnvelopeName)
	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)
	defaultHTTPRemoteDependencyDataValidations(t, span, data)
	assert.Equal(t, "https://foo:81/bar?biz=baz", data.Data)
}

// Tests proper assignment for a HTTP client span
// http.scheme, net.peer.ip, net.peer.port, http.target
func TestHTTPClientSpanToRemoteDependencyAttributeSet4(t *testing.T) {
	span := getDefaultHTTPClientSpan()
	spanAttributes := span.Attributes()

	appendToAttributeMap(
		spanAttributes,
		map[string]pdata.AttributeValue{
			conventions.AttributeHTTPStatusCode: pdata.NewAttributeValueInt(defaultHTTPStatusCode),
			conventions.AttributeHTTPScheme:     pdata.NewAttributeValueString("https"),
			conventions.AttributeNetPeerIP:      pdata.NewAttributeValueString("127.0.0.1"),
			conventions.AttributeNetPeerPort:    pdata.NewAttributeValueInt(81),
			conventions.AttributeHTTPTarget:     pdata.NewAttributeValueString("/bar?biz=baz"),
		})

	envelope, _ := spanToEnvelope(defaultResource, defaultInstrumentationLibrary, span, zap.NewNop())
	commonEnvelopeValidations(t, span, envelope, defaultRemoteDependencyDataEnvelopeName)
	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)
	defaultHTTPRemoteDependencyDataValidations(t, span, data)
	assert.Equal(t, "https://127.0.0.1:81/bar?biz=baz", data.Data)
}

// Tests proper assignment for an RPC server span
func TestRPCServerSpanToRequestData(t *testing.T) {
	span := getDefaultRPCServerSpan()
	spanAttributes := span.Attributes()

	appendToAttributeMap(
		spanAttributes,
		map[string]pdata.AttributeValue{
			conventions.AttributeNetPeerName: pdata.NewAttributeValueString("foo"),
			conventions.AttributeNetPeerIP:   pdata.NewAttributeValueString("127.0.0.1"),
			conventions.AttributeNetPeerPort: pdata.NewAttributeValueInt(81),
		})

	envelope, _ := spanToEnvelope(defaultResource, defaultInstrumentationLibrary, span, zap.NewNop())
	commonEnvelopeValidations(t, span, envelope, defaultRequestDataEnvelopeName)
	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.RequestData)
	defaultRPCRequestDataValidations(t, span, data, "foo:81")

	// test fallback to peerip
	appendToAttributeMap(
		spanAttributes,
		map[string]pdata.AttributeValue{
			conventions.AttributeNetPeerName: pdata.NewAttributeValueString(""),
			conventions.AttributeNetPeerIP:   pdata.NewAttributeValueString("127.0.0.1"),
		})

	envelope, _ = spanToEnvelope(defaultResource, defaultInstrumentationLibrary, span, zap.NewNop())
	data = envelope.Data.(*contracts.Data).BaseData.(*contracts.RequestData)
	defaultRPCRequestDataValidations(t, span, data, "127.0.0.1:81")
}

// Tests proper assignment for an RPC client span
func TestRPCClientSpanToRemoteDependencyData(t *testing.T) {
	span := getDefaultRPCClientSpan()
	spanAttributes := span.Attributes()

	appendToAttributeMap(
		spanAttributes,
		map[string]pdata.AttributeValue{
			conventions.AttributeNetPeerName: pdata.NewAttributeValueString("foo"),
			conventions.AttributeNetPeerPort: pdata.NewAttributeValueInt(81),
			conventions.AttributeNetPeerIP:   pdata.NewAttributeValueString("127.0.0.1"),
		})

	envelope, _ := spanToEnvelope(defaultResource, defaultInstrumentationLibrary, span, zap.NewNop())
	commonEnvelopeValidations(t, span, envelope, defaultRemoteDependencyDataEnvelopeName)
	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)
	defaultRPCRemoteDependencyDataValidations(t, span, data, "foo:81")

	// test fallback to peerip
	appendToAttributeMap(
		spanAttributes,
		map[string]pdata.AttributeValue{
			conventions.AttributeNetPeerName: pdata.NewAttributeValueString(""),
			conventions.AttributeNetPeerIP:   pdata.NewAttributeValueString("127.0.0.1"),
		})

	envelope, _ = spanToEnvelope(defaultResource, defaultInstrumentationLibrary, span, zap.NewNop())
	data = envelope.Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)
	defaultRPCRemoteDependencyDataValidations(t, span, data, "127.0.0.1:81")
}

// Tests proper assignment for a Database client span
func TestDatabaseClientSpanToRemoteDependencyData(t *testing.T) {
	span := getDefaultDatabaseClientSpan()
	spanAttributes := span.Attributes()

	appendToAttributeMap(
		spanAttributes,
		map[string]pdata.AttributeValue{
			conventions.AttributeDBStatement: pdata.NewAttributeValueString(defaultDBStatement),
			conventions.AttributeNetPeerName: pdata.NewAttributeValueString("foo"),
			conventions.AttributeNetPeerPort: pdata.NewAttributeValueInt(81),
		})

	envelope, _ := spanToEnvelope(defaultResource, defaultInstrumentationLibrary, span, zap.NewNop())
	commonEnvelopeValidations(t, span, envelope, defaultRemoteDependencyDataEnvelopeName)
	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)
	defaultDatabaseRemoteDependencyDataValidations(t, span, data)

	assert.Equal(t, "foo:81", data.Target)
	assert.Equal(t, defaultDBStatement, data.Data)

	// Test the fallback to data.Data fallback to DBOperation
	appendToAttributeMap(
		spanAttributes,
		map[string]pdata.AttributeValue{
			conventions.AttributeDBStatement: pdata.NewAttributeValueString(""),
			attributeDBOperation:             pdata.NewAttributeValueString(defaultDBOperation),
		})

	envelope, _ = spanToEnvelope(defaultResource, defaultInstrumentationLibrary, span, zap.NewNop())
	data = envelope.Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)
	assert.Equal(t, defaultDBOperation, data.Data)
}

// Tests proper assignment for a Messaging consumer span
func TestMessagingConsumerSpanToRequestData(t *testing.T) {
	span := getDefaultMessagingConsumerSpan()
	spanAttributes := span.Attributes()

	appendToAttributeMap(
		spanAttributes,
		map[string]pdata.AttributeValue{
			conventions.AttributeMessagingURL: pdata.NewAttributeValueString(defaultMessagingURL),
			conventions.AttributeNetPeerName:  pdata.NewAttributeValueString("foo"),
			conventions.AttributeNetPeerPort:  pdata.NewAttributeValueInt(81),
		})

	envelope, _ := spanToEnvelope(defaultResource, defaultInstrumentationLibrary, span, zap.NewNop())
	commonEnvelopeValidations(t, span, envelope, defaultRequestDataEnvelopeName)
	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.RequestData)
	defaultMessagingRequestDataValidations(t, span, data)

	assert.Equal(t, defaultMessagingURL, data.Source)

	// test fallback from MessagingURL to net.* properties
	appendToAttributeMap(
		spanAttributes,
		map[string]pdata.AttributeValue{
			conventions.AttributeMessagingURL: pdata.NewAttributeValueString(""),
		})

	envelope, _ = spanToEnvelope(defaultResource, defaultInstrumentationLibrary, span, zap.NewNop())
	data = envelope.Data.(*contracts.Data).BaseData.(*contracts.RequestData)

	assert.Equal(t, "foo:81", data.Source)
}

// Tests proper assignment for a Messaging producer span
func TestMessagingProducerSpanToRequestData(t *testing.T) {
	span := getDefaultMessagingProducerSpan()
	spanAttributes := span.Attributes()

	appendToAttributeMap(
		spanAttributes,
		map[string]pdata.AttributeValue{
			conventions.AttributeMessagingURL: pdata.NewAttributeValueString(defaultMessagingURL),
			conventions.AttributeNetPeerName:  pdata.NewAttributeValueString("foo"),
			conventions.AttributeNetPeerPort:  pdata.NewAttributeValueInt(81),
		})

	envelope, _ := spanToEnvelope(defaultResource, defaultInstrumentationLibrary, span, zap.NewNop())
	commonEnvelopeValidations(t, span, envelope, defaultRemoteDependencyDataEnvelopeName)
	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)
	defaultMessagingRemoteDependencyDataValidations(t, span, data)

	assert.Equal(t, defaultMessagingURL, data.Target)

	// test fallback from MessagingURL to net.* properties
	appendToAttributeMap(
		spanAttributes,
		map[string]pdata.AttributeValue{
			conventions.AttributeMessagingURL: pdata.NewAttributeValueString(""),
		})

	envelope, _ = spanToEnvelope(defaultResource, defaultInstrumentationLibrary, span, zap.NewNop())
	data = envelope.Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)

	assert.Equal(t, "foo:81", data.Target)
}

// Tests proper assignment for an internal span
func TestUnknownInternalSpanToRemoteDependencyData(t *testing.T) {
	span := getDefaultInternalSpan()
	spanAttributes := span.Attributes()

	appendToAttributeMap(
		spanAttributes,
		map[string]pdata.AttributeValue{
			"foo": pdata.NewAttributeValueString("bar"),
		})

	envelope, _ := spanToEnvelope(defaultResource, defaultInstrumentationLibrary, span, zap.NewNop())
	commonEnvelopeValidations(t, span, envelope, defaultRemoteDependencyDataEnvelopeName)
	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)
	defaultInternalRemoteDependencyDataValidations(t, span, data)
}

// Tests that spans with unspecified kind are treated similar to internal spans
func TestUnspecifiedSpanToInProcRemoteDependencyData(t *testing.T) {
	span := getDefaultInternalSpan()
	span.SetKind(pdata.SpanKindUNSPECIFIED)

	envelope, _ := spanToEnvelope(defaultResource, defaultInstrumentationLibrary, span, zap.NewNop())
	commonEnvelopeValidations(t, span, envelope, defaultRemoteDependencyDataEnvelopeName)
	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)
	defaultInternalRemoteDependencyDataValidations(t, span, data)
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

/*
	These methods are for handling some common validations
*/
func commonEnvelopeValidations(
	t *testing.T,
	span pdata.Span,
	envelope *contracts.Envelope,
	expectedEnvelopeName string) {

	assert.NotNil(t, envelope)
	assert.Equal(t, expectedEnvelopeName, envelope.Name)
	assert.Equal(t, toTime(span.StartTime()).Format(time.RFC3339Nano), envelope.Time)
	assert.Equal(t, defaultTraceIDAsHex, envelope.Tags[contracts.OperationId])
	assert.Equal(t, defaultParentSpanIDAsHex, envelope.Tags[contracts.OperationParentId])
	assert.Equal(t, defaultServiceNamespace+"."+defaultServiceName, envelope.Tags[contracts.CloudRole])
	assert.Equal(t, defaultServiceInstance, envelope.Tags[contracts.CloudRoleInstance])
	assert.NotNil(t, envelope.Data)

	if expectedEnvelopeName == defaultRequestDataEnvelopeName {
		requestData := envelope.Data.(*contracts.Data).BaseData.(*contracts.RequestData)
		assert.Equal(t, requestData.Name, envelope.Tags[contracts.OperationName])
	}
}

// Validate common stuff across any Span -> RequestData translation
func commonRequestDataValidations(
	t *testing.T,
	span pdata.Span,
	data *contracts.RequestData) {

	assertAttributesCopiedToPropertiesOrMeasurements(t, span.Attributes(), data.Properties, data.Measurements)
	assert.Equal(t, defaultSpanIDAsHex, data.Id)
	assert.Equal(t, defaultSpanDuration, data.Duration)

	assert.Equal(t, defaultInstrumentationLibraryName, data.Properties[instrumentationLibraryName])
	assert.Equal(t, defaultInstrumentationLibraryVersion, data.Properties[instrumentationLibraryVersion])
}

// Validate common RequestData values for HTTP Spans created using the default test values
func defaultHTTPRequestDataValidations(
	t *testing.T,
	span pdata.Span,
	data *contracts.RequestData) {

	commonRequestDataValidations(t, span, data)

	assert.Equal(t, defaultHTTPStatusCodeAsString, data.ResponseCode)
	assert.Equal(t, defaultHTTPStatusCode <= 399, data.Success)
	assert.Equal(t, defaultHTTPMethod+" "+defaultHTTPServerSpanName, data.Name)
}

// Validate common stuff across any Span -> RemoteDependencyData translation
func commonRemoteDependencyDataValidations(
	t *testing.T,
	span pdata.Span,
	data *contracts.RemoteDependencyData) {

	assertAttributesCopiedToPropertiesOrMeasurements(t, span.Attributes(), data.Properties, data.Measurements)
	assert.Equal(t, defaultSpanIDAsHex, data.Id)
	assert.Equal(t, defaultSpanDuration, data.Duration)
}

// Validate common RemoteDependencyData values for HTTP Spans created using the default test values
func defaultHTTPRemoteDependencyDataValidations(
	t *testing.T,
	span pdata.Span,
	data *contracts.RemoteDependencyData) {

	commonRemoteDependencyDataValidations(t, span, data)

	assert.Equal(t, defaultHTTPStatusCodeAsString, data.ResultCode)
	assert.Equal(t, defaultHTTPStatusCode >= 200 && defaultHTTPStatusCode <= 299, data.Success)
	assert.Equal(t, defaultHTTPClientSpanName, data.Name)
	assert.Equal(t, "HTTP", data.Type)
}

func defaultRPCRequestDataValidations(
	t *testing.T,
	span pdata.Span,
	data *contracts.RequestData,
	expectedDataSource string) {

	commonRequestDataValidations(t, span, data)

	assert.Equal(t, defaultRPCStatusCodeAsString, data.ResponseCode)
	assert.True(t, data.Success)
	assert.Equal(t, defaultRPCSystem+" "+defaultRPCSpanName, data.Name)
	assert.Equal(t, data.Name, data.Url)
	assert.Equal(t, expectedDataSource, data.Source)
}

func defaultRPCRemoteDependencyDataValidations(
	t *testing.T,
	span pdata.Span,
	data *contracts.RemoteDependencyData,
	expectedDataTarget string) {

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
	span pdata.Span,
	data *contracts.RemoteDependencyData) {

	commonRemoteDependencyDataValidations(t, span, data)

	assert.Equal(t, defaultDatabaseStatusCodeAsString, data.ResultCode)
	assert.True(t, data.Success)
	assert.Equal(t, defaultDBSpanName, data.Name)
	assert.Equal(t, defaultDBSystem, data.Type)
}

func defaultMessagingRequestDataValidations(
	t *testing.T,
	span pdata.Span,
	data *contracts.RequestData) {

	commonRequestDataValidations(t, span, data)

	assert.Equal(t, defaultMessagingStatusCodeAsString, data.ResponseCode)
	assert.True(t, data.Success)
	assert.Equal(t, defaultMessagingDestination, data.Name)
}

func defaultMessagingRemoteDependencyDataValidations(
	t *testing.T,
	span pdata.Span,
	data *contracts.RemoteDependencyData) {

	commonRemoteDependencyDataValidations(t, span, data)

	assert.Equal(t, defaultMessagingStatusCodeAsString, data.ResultCode)
	assert.True(t, data.Success)
	assert.Equal(t, defaultMessagingDestination, data.Name)
	assert.Equal(t, defaultMessagingSystem, data.Type)
}

func defaultInternalRemoteDependencyDataValidations(
	t *testing.T,
	span pdata.Span,
	data *contracts.RemoteDependencyData) {

	assertAttributesCopiedToPropertiesOrMeasurements(t, span.Attributes(), data.Properties, data.Measurements)
	assert.Equal(t, "InProc", data.Type)
}

// Verifies that all attributes are copies to either the properties or measurements maps of the envelope's data element
func assertAttributesCopiedToPropertiesOrMeasurements(
	t *testing.T,
	attributeMap pdata.AttributeMap,
	properties map[string]string,
	measurements map[string]float64) {

	attributeMap.ForEach(
		func(k string, v pdata.AttributeValue) {
			switch v.Type() {
			case pdata.AttributeValueSTRING:
				p, exists := properties[k]
				assert.True(t, exists)
				assert.Equal(t, v.StringVal(), p)
			case pdata.AttributeValueBOOL:
				p, exists := properties[k]
				assert.True(t, exists)
				assert.Equal(t, strconv.FormatBool(v.BoolVal()), p)
			case pdata.AttributeValueINT:
				m, exists := measurements[k]
				assert.True(t, exists)
				assert.Equal(t, float64(v.IntVal()), m)
			case pdata.AttributeValueDOUBLE:
				m, exists := measurements[k]
				assert.True(t, exists)
				assert.Equal(t, float64(v.DoubleVal()), m)
			}
		})
}

/*
	The remainder of these methods are for building up test assets
*/
func getSpan(spanName string, spanKind pdata.SpanKind, initialAttributes map[string]pdata.AttributeValue) pdata.Span {
	span := pdata.NewSpan()
	span.InitEmpty()
	span.SetTraceID(pdata.NewTraceID(defaultTraceID))
	span.SetSpanID(pdata.NewSpanID(defaultSpanID))
	span.SetParentSpanID(pdata.NewSpanID(defaultParentSpanID))
	span.SetName(spanName)
	span.SetKind(spanKind)
	span.SetStartTime(defaultSpanStartTime)
	span.SetEndTime(defaultSpanEndTme)
	span.Attributes().InitFromMap(initialAttributes)
	return span
}

// Returns a default server span
func getServerSpan(spanName string, initialAttributes map[string]pdata.AttributeValue) pdata.Span {
	return getSpan(spanName, pdata.SpanKindSERVER, initialAttributes)
}

// Returns a default client span
func getClientSpan(spanName string, initialAttributes map[string]pdata.AttributeValue) pdata.Span {
	return getSpan(spanName, pdata.SpanKindCLIENT, initialAttributes)
}

// Returns a default consumer span
func getConsumerSpan(spanName string, initialAttributes map[string]pdata.AttributeValue) pdata.Span {
	return getSpan(spanName, pdata.SpanKindCONSUMER, initialAttributes)
}

// Returns a default producer span
func getProducerSpan(spanName string, initialAttributes map[string]pdata.AttributeValue) pdata.Span {
	return getSpan(spanName, pdata.SpanKindPRODUCER, initialAttributes)
}

// Returns a default internal span
func getInternalSpan(spanName string, initialAttributes map[string]pdata.AttributeValue) pdata.Span {
	return getSpan(spanName, pdata.SpanKindINTERNAL, initialAttributes)
}

func getDefaultHTTPServerSpan() pdata.Span {
	return getServerSpan(
		defaultHTTPServerSpanName,
		requiredHTTPAttributes)
}

func getDefaultHTTPClientSpan() pdata.Span {
	return getClientSpan(
		defaultHTTPClientSpanName,
		requiredHTTPAttributes)
}

func getDefaultRPCServerSpan() pdata.Span {
	return getServerSpan(
		defaultRPCSpanName,
		requiredRPCAttributes)
}

func getDefaultRPCClientSpan() pdata.Span {
	return getClientSpan(
		defaultRPCSpanName,
		requiredRPCAttributes)
}

func getDefaultDatabaseClientSpan() pdata.Span {
	return getClientSpan(
		defaultDBSpanName,
		requiredDatabaseAttributes)
}

func getDefaultMessagingConsumerSpan() pdata.Span {
	return getConsumerSpan(
		defaultMessagingSpanName,
		requiredMessagingAttributes)
}

func getDefaultMessagingProducerSpan() pdata.Span {
	return getProducerSpan(
		defaultMessagingSpanName,
		requiredMessagingAttributes)
}

func getDefaultInternalSpan() pdata.Span {
	return getInternalSpan(
		defaultInternalSpanName,
		map[string]pdata.AttributeValue{})
}

// Returns a default Resource
func getResource() pdata.Resource {
	r := pdata.NewResource()
	r.InitEmpty()
	r.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		conventions.AttributeServiceName:      pdata.NewAttributeValueString(defaultServiceName),
		conventions.AttributeServiceNamespace: pdata.NewAttributeValueString(defaultServiceNamespace),
		conventions.AttributeServiceInstance:  pdata.NewAttributeValueString(defaultServiceInstance),
	})

	return r
}

// Returns a default instrumentation library
func getInstrumentationLibrary() pdata.InstrumentationLibrary {
	il := pdata.NewInstrumentationLibrary()
	il.InitEmpty()
	il.SetName(defaultInstrumentationLibraryName)
	il.SetVersion(defaultInstrumentationLibraryVersion)
	return il
}

// Adds a map of AttributeValues to an existing AttributeMap
func appendToAttributeMap(attributeMap pdata.AttributeMap, maps ...map[string]pdata.AttributeValue) {
	for _, m := range maps {
		for k, v := range m {
			attributeMap.Upsert(k, v)
		}
	}
}
