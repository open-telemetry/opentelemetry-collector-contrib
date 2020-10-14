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

// Contains code common to both trace and metrics exporters
import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
)

const (
	unknownSpanType   spanType = 0
	httpSpanType      spanType = 1
	rpcSpanType       spanType = 2
	databaseSpanType  spanType = 3
	messagingSpanType spanType = 4
	faasSpanType      spanType = 5

	instrumentationLibraryName    string = "instrumentationlibrary.name"
	instrumentationLibraryVersion string = "instrumentationlibrary.version"
)

var (
	errUnexpectedAttributeValueType = errors.New("attribute value type is unexpected")
	errUnsupportedSpanType          = errors.New("unsupported Span type")
)

// Used to identify the type of a received Span
type spanType int8

// Transforms a tuple of pdata.Resource, pdata.InstrumentationLibrary, pdata.Span into an AppInsights contracts.Envelope
// This is the only method that should be targeted in the unit tests
func spanToEnvelope(
	resource pdata.Resource,
	instrumentationLibrary pdata.InstrumentationLibrary,
	span pdata.Span,
	logger *zap.Logger) (*contracts.Envelope, error) {

	spanKind := span.Kind()

	// According to the SpanKind documentation, we can assume it to be INTERNAL
	// when we get UNSPECIFIED.
	if spanKind == pdata.SpanKindUNSPECIFIED {
		spanKind = pdata.SpanKindINTERNAL
	}

	attributeMap := span.Attributes()
	incomingSpanType := mapIncomingSpanToType(attributeMap)

	// For now, FaaS spans are unsupported
	if incomingSpanType == faasSpanType {
		return nil, errUnsupportedSpanType
	}

	envelope := contracts.NewEnvelope()
	envelope.Tags = make(map[string]string)
	envelope.Time = toTime(span.StartTime()).Format(time.RFC3339Nano)
	traceIDHexString := idToHex(span.TraceID().Bytes())
	envelope.Tags[contracts.OperationId] = traceIDHexString
	envelope.Tags[contracts.OperationParentId] = idToHex(span.ParentSpanID().Bytes())

	data := contracts.NewData()
	var dataSanitizeFunc func() []string
	var dataProperties map[string]string

	if spanKind == pdata.SpanKindSERVER || spanKind == pdata.SpanKindCONSUMER {
		requestData := spanToRequestData(span, incomingSpanType)
		dataProperties = requestData.Properties
		dataSanitizeFunc = requestData.Sanitize
		envelope.Name = requestData.EnvelopeName("")
		envelope.Tags[contracts.OperationName] = requestData.Name
		data.BaseData = requestData
		data.BaseType = requestData.BaseType()
	} else if spanKind == pdata.SpanKindCLIENT || spanKind == pdata.SpanKindPRODUCER || spanKind == pdata.SpanKindINTERNAL {
		remoteDependencyData := spanToRemoteDependencyData(span, incomingSpanType)

		// Regardless of the detected Span type, if the SpanKind is Internal we need to set data.Type to InProc
		if spanKind == pdata.SpanKindINTERNAL {
			remoteDependencyData.Type = "InProc"
		}

		dataProperties = remoteDependencyData.Properties
		dataSanitizeFunc = remoteDependencyData.Sanitize
		envelope.Name = remoteDependencyData.EnvelopeName("")
		data.BaseData = remoteDependencyData
		data.BaseType = remoteDependencyData.BaseType()
	}

	envelope.Data = data
	resourceAttributes := resource.Attributes()

	// Copy all the resource labels into the base data properties. Resource values are always strings
	resourceAttributes.ForEach(func(k string, v pdata.AttributeValue) { dataProperties[k] = v.StringVal() })

	// Copy the instrumentation properties
	if !instrumentationLibrary.IsNil() {
		if instrumentationLibrary.Name() != "" {
			dataProperties[instrumentationLibraryName] = instrumentationLibrary.Name()
		}

		if instrumentationLibrary.Version() != "" {
			dataProperties[instrumentationLibraryVersion] = instrumentationLibrary.Version()
		}
	}

	// Extract key service.* labels from the Resource labels and construct CloudRole and CloudRoleInstance envelope tags
	// https://github.com/open-telemetry/opentelemetry-specification/tree/master/specification/resource/semantic_conventions
	if serviceName, serviceNameExists := resourceAttributes.Get(conventions.AttributeServiceName); serviceNameExists {
		cloudRole := serviceName.StringVal()

		if serviceNamespace, serviceNamespaceExists := resourceAttributes.Get(conventions.AttributeServiceNamespace); serviceNamespaceExists {
			cloudRole = serviceNamespace.StringVal() + "." + cloudRole
		}

		envelope.Tags[contracts.CloudRole] = cloudRole
	}

	if serviceInstance, exists := resourceAttributes.Get(conventions.AttributeServiceInstance); exists {
		envelope.Tags[contracts.CloudRoleInstance] = serviceInstance.StringVal()
	}

	// Sanitize the base data, the envelope and envelope tags
	sanitize(dataSanitizeFunc, logger)
	sanitize(func() []string { return envelope.Sanitize() }, logger)
	sanitize(func() []string { return contracts.SanitizeTags(envelope.Tags) }, logger)

	return envelope, nil
}

// Maps Server/Consumer Span to AppInsights RequestData
func spanToRequestData(span pdata.Span, incomingSpanType spanType) *contracts.RequestData {
	// See https://github.com/microsoft/ApplicationInsights-Go/blob/master/appinsights/contracts/requestdata.go
	// Start with some reasonable default for server spans.
	data := contracts.NewRequestData()
	data.Id = idToHex(span.SpanID().Bytes())
	data.Name = span.Name()
	data.Duration = formatSpanDuration(span)
	data.Properties = make(map[string]string)
	data.Measurements = make(map[string]float64)
	data.ResponseCode, data.Success = getDefaultFormattedSpanStatus(span.Status())

	switch incomingSpanType {
	case httpSpanType:
		fillRequestDataHTTP(span, data)
	case rpcSpanType:
		fillRequestDataRPC(span, data)
	case messagingSpanType:
		fillRequestDataMessaging(span, data)
	case unknownSpanType:
		copyAttributesWithoutMapping(span.Attributes(), data.Properties, data.Measurements)
	}

	return data
}

// Maps Span to AppInsights RemoteDependencyData
func spanToRemoteDependencyData(span pdata.Span, incomingSpanType spanType) *contracts.RemoteDependencyData {
	// https://github.com/microsoft/ApplicationInsights-Go/blob/master/appinsights/contracts/remotedependencydata.go
	// Start with some reasonable default for dependent spans.
	data := contracts.NewRemoteDependencyData()
	data.Id = idToHex(span.SpanID().Bytes())
	data.Name = span.Name()
	data.ResultCode, data.Success = getDefaultFormattedSpanStatus(span.Status())
	data.Duration = formatSpanDuration(span)
	data.Properties = make(map[string]string)
	data.Measurements = make(map[string]float64)

	switch incomingSpanType {
	case httpSpanType:
		fillRemoteDependencyDataHTTP(span, data)
	case rpcSpanType:
		fillRemoteDependencyDataRPC(span, data)
	case databaseSpanType:
		fillRemoteDependencyDataDatabase(span, data)
	case messagingSpanType:
		fillRemoteDependencyDataMessaging(span, data)
	case unknownSpanType:
		copyAttributesWithoutMapping(span.Attributes(), data.Properties, data.Measurements)
	}

	return data
}

func getFormattedHTTPStatusValues(statusCode int64) (statusAsString string, success bool) {
	// see https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/trace/semantic_conventions/http.md#status
	return strconv.FormatInt(statusCode, 10), statusCode >= 100 && statusCode <= 399
}

// Maps HTTP Server Span to AppInsights RequestData
// https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/trace/semantic_conventions/http.md#semantic-conventions-for-http-spans
func fillRequestDataHTTP(span pdata.Span, data *contracts.RequestData) {
	attrs := copyAndExtractHTTPAttributes(span.Attributes(), data.Properties, data.Measurements)

	if attrs.HTTPStatusCode != 0 {
		data.ResponseCode, data.Success = getFormattedHTTPStatusValues(attrs.HTTPStatusCode)
	}

	var sb strings.Builder

	// Construct data.Name
	// The data.Name should be {HTTP METHOD} {HTTP SERVER ROUTE TEMPLATE}
	// https://github.com/microsoft/ApplicationInsights-Home/blob/f1f9f619d74557c8db3dbde4b49c4193e10d8a81/EndpointSpecs/Schemas/Bond/RequestData.bond#L32
	sb.WriteString(attrs.HTTPMethod)
	sb.WriteString(" ")

	// Use httpRoute if available otherwise fallback to the span name
	// https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/trace/semantic_conventions/http.md#name
	if attrs.HTTPRoute != "" {
		sb.WriteString(prefixIfNecessary(attrs.HTTPRoute, "/"))
	} else {
		sb.WriteString(span.Name())
	}

	data.Name = sb.String()
	sb.Reset()

	/*
		To construct the value for data.Url we will use the following sets of attributes as defined by the otel spec
		Order of preference is:
		http.scheme, http.host, http.target
		http.scheme, http.server_name, net.host.port, http.target
		http.scheme, net.host.name, net.host.port, http.target
		http.url
	*/

	if attrs.HTTPTarget != "" {
		attrs.HTTPTarget = prefixIfNecessary(attrs.HTTPTarget, "/")
	}

	netHostPortAsString := ""
	if attrs.NetworkAttributes.NetHostPort != 0 {
		netHostPortAsString = strconv.FormatInt(attrs.NetworkAttributes.NetHostPort, 10)
	}

	if attrs.HTTPScheme != "" && attrs.HTTPHost != "" && attrs.HTTPTarget != "" {
		sb.WriteString(attrs.HTTPScheme)
		sb.WriteString("://")
		sb.WriteString(attrs.HTTPHost)
		sb.WriteString(attrs.HTTPTarget)
		data.Url = sb.String()
	} else if attrs.HTTPScheme != "" && attrs.HTTPServerName != "" && netHostPortAsString != "" && attrs.HTTPTarget != "" {
		sb.WriteString(attrs.HTTPScheme)
		sb.WriteString("://")
		sb.WriteString(attrs.HTTPServerName)
		sb.WriteString(":")
		sb.WriteString(netHostPortAsString)
		sb.WriteString(attrs.HTTPTarget)
		data.Url = sb.String()
	} else if attrs.HTTPScheme != "" && attrs.NetworkAttributes.NetHostName != "" && netHostPortAsString != "" && attrs.HTTPTarget != "" {
		sb.WriteString(attrs.HTTPScheme)
		sb.WriteString("://")
		sb.WriteString(attrs.NetworkAttributes.NetHostName)
		sb.WriteString(":")
		sb.WriteString(netHostPortAsString)
		sb.WriteString(attrs.HTTPTarget)
		data.Url = sb.String()
	} else if attrs.HTTPURL != "" {
		if _, err := url.Parse(attrs.HTTPURL); err == nil {
			data.Url = attrs.HTTPURL
		}
	}

	sb.Reset()

	// data.Source should be the client ip if available or fallback to net.peer.ip
	// https://github.com/microsoft/ApplicationInsights-Home/blob/f1f9f619d74557c8db3dbde4b49c4193e10d8a81/EndpointSpecs/Schemas/Bond/RequestData.bond#L28
	// https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/trace/semantic_conventions/http.md#http-server-semantic-conventions
	if attrs.HTTPClientIP != "" {
		data.Source = attrs.HTTPClientIP
	} else if attrs.NetworkAttributes.NetPeerIP != "" {
		data.Source = attrs.NetworkAttributes.NetPeerIP
	}
}

// Maps HTTP Client Span to AppInsights RemoteDependencyData
// https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/trace/semantic_conventions/http.md
func fillRemoteDependencyDataHTTP(span pdata.Span, data *contracts.RemoteDependencyData) {
	attrs := copyAndExtractHTTPAttributes(span.Attributes(), data.Properties, data.Measurements)

	data.Type = "HTTP"
	if attrs.HTTPStatusCode != 0 {
		data.ResultCode, data.Success = getFormattedHTTPStatusValues(attrs.HTTPStatusCode)
	}

	var sb strings.Builder

	// Construct data.Name
	// The data.Name should default to {HTTP METHOD} and include {HTTP ROUTE TEMPLATE} (if available)
	sb.WriteString(attrs.HTTPMethod)

	// Use httpRoute if available otherwise fallback to the HTTP method
	// https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/trace/semantic_conventions/http.md#name
	if attrs.HTTPRoute != "" {
		sb.WriteString(" ")
		sb.WriteString(attrs.HTTPRoute)
	}

	data.Name = sb.String()
	sb.Reset()

	/*
		Order of preference is:
		http.url
		http.scheme, http.host, http.target
		http.scheme, net.peer.name, net.peer.port, http.target
		http.scheme, net.peer.ip, net.peer.port, http.target
	*/

	// prefix httpTarget, if specified
	if attrs.HTTPTarget != "" {
		attrs.HTTPTarget = prefixIfNecessary(attrs.HTTPTarget, "/")
	}

	netPeerPortAsString := ""
	if attrs.NetworkAttributes.NetPeerPort != 0 {
		netPeerPortAsString = strconv.FormatInt(attrs.NetworkAttributes.NetPeerPort, 10)
	}

	if attrs.HTTPURL != "" {
		if u, err := url.Parse(attrs.HTTPURL); err == nil {
			data.Data = attrs.HTTPURL
			data.Target = u.Host
		}
	} else if attrs.HTTPScheme != "" && attrs.HTTPHost != "" && attrs.HTTPTarget != "" {
		sb.WriteString(attrs.HTTPScheme)
		sb.WriteString("://")
		sb.WriteString(attrs.HTTPHost)
		sb.WriteString(attrs.HTTPTarget)
		data.Data = sb.String()
		data.Target = attrs.HTTPHost
	} else if attrs.HTTPScheme != "" && attrs.NetworkAttributes.NetPeerName != "" && netPeerPortAsString != "" && attrs.HTTPTarget != "" {
		sb.WriteString(attrs.HTTPScheme)
		sb.WriteString("://")
		sb.WriteString(attrs.NetworkAttributes.NetPeerName)
		sb.WriteString(":")
		sb.WriteString(netPeerPortAsString)
		sb.WriteString(attrs.HTTPTarget)
		data.Data = sb.String()

		sb.Reset()
		sb.WriteString(attrs.NetworkAttributes.NetPeerName)
		sb.WriteString(":")
		sb.WriteString(netPeerPortAsString)
		data.Target = sb.String()
	} else if attrs.HTTPScheme != "" && attrs.NetworkAttributes.NetPeerIP != "" && netPeerPortAsString != "" && attrs.HTTPTarget != "" {
		sb.WriteString(attrs.HTTPScheme)
		sb.WriteString("://")
		sb.WriteString(attrs.NetworkAttributes.NetPeerIP)
		sb.WriteString(":")
		sb.WriteString(netPeerPortAsString)
		sb.WriteString(attrs.HTTPTarget)
		data.Data = sb.String()

		sb.Reset()
		sb.WriteString(attrs.NetworkAttributes.NetPeerIP)
		sb.WriteString(":")
		sb.WriteString(netPeerPortAsString)
		data.Target = sb.String()
	}
}

// Maps RPC Server Span to AppInsights RequestData
// https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/trace/semantic_conventions/rpc.md
func fillRequestDataRPC(span pdata.Span, data *contracts.RequestData) {
	attrs := copyAndExtractRPCAttributes(span.Attributes(), data.Properties, data.Measurements)

	var sb strings.Builder

	sb.WriteString(attrs.RPCSystem)
	sb.WriteString(" ")
	sb.WriteString(data.Name)

	// Prefix the name with the type of RPC
	data.Name = sb.String()

	// Set the .Data property to .Name which contain the full RPC method
	data.Url = data.Name

	sb.Reset()

	writeFormattedPeerAddressFromNetworkAttributes(&attrs.NetworkAttributes, &sb)

	data.Source = sb.String()
}

// Maps RPC Client Span to AppInsights RemoteDependencyData
// https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/trace/semantic_conventions/rpc.md
func fillRemoteDependencyDataRPC(span pdata.Span, data *contracts.RemoteDependencyData) {
	attrs := copyAndExtractRPCAttributes(span.Attributes(), data.Properties, data.Measurements)

	// Set the .Data property to .Name which contain the full RPC method
	data.Data = data.Name

	data.Type = attrs.RPCSystem

	var sb strings.Builder
	writeFormattedPeerAddressFromNetworkAttributes(&attrs.NetworkAttributes, &sb)
	data.Target = sb.String()
}

// Maps Database Client Span to AppInsights RemoteDependencyData
// https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/trace/semantic_conventions/database.md
func fillRemoteDependencyDataDatabase(span pdata.Span, data *contracts.RemoteDependencyData) {
	attrs := copyAndExtractDatabaseAttributes(span.Attributes(), data.Properties, data.Measurements)

	data.Type = attrs.DBSystem

	if attrs.DBStatement != "" {
		data.Data = attrs.DBStatement
	} else if attrs.DBOperation != "" {
		data.Data = attrs.DBOperation
	}

	var sb strings.Builder
	writeFormattedPeerAddressFromNetworkAttributes(&attrs.NetworkAttributes, &sb)
	data.Target = sb.String()
}

// Maps Messaging Consumer/Server Span to AppInsights RequestData
// https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/trace/semantic_conventions/messaging.md
func fillRequestDataMessaging(span pdata.Span, data *contracts.RequestData) {
	attrs := copyAndExtractMessagingAttributes(span.Attributes(), data.Properties, data.Measurements)

	// TODO Understand how to map attributes to RequestData fields
	if attrs.MessagingURL != "" {
		data.Source = attrs.MessagingURL
	} else {
		var sb strings.Builder
		writeFormattedPeerAddressFromNetworkAttributes(&attrs.NetworkAttributes, &sb)
		data.Source = sb.String()
	}
}

// Maps Messaging Producer/Client Span to AppInsights RemoteDependencyData
// https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/trace/semantic_conventions/messaging.md
func fillRemoteDependencyDataMessaging(span pdata.Span, data *contracts.RemoteDependencyData) {
	attrs := copyAndExtractMessagingAttributes(span.Attributes(), data.Properties, data.Measurements)

	// TODO Understand how to map attributes to RemoteDependencyData fields
	data.Data = attrs.MessagingURL
	data.Type = attrs.MessagingSystem

	if attrs.MessagingURL != "" {
		data.Target = attrs.MessagingURL
	} else {
		var sb strings.Builder
		writeFormattedPeerAddressFromNetworkAttributes(&attrs.NetworkAttributes, &sb)
		data.Target = sb.String()
	}
}

// Copies all attributes to either properties or measurements and passes the key/value to another mapping function
func copyAndMapAttributes(
	attributeMap pdata.AttributeMap,
	properties map[string]string,
	measurements map[string]float64,
	mappingFunc func(k string, v pdata.AttributeValue)) {

	attributeMap.ForEach(
		func(k string, v pdata.AttributeValue) {
			setAttributeValueAsPropertyOrMeasurement(k, v, properties, measurements)

			if mappingFunc != nil {
				mappingFunc(k, v)
			}
		})
}

// Copies all attributes to either properties or measurements without any kind of mapping to a known set of attributes
func copyAttributesWithoutMapping(
	attributeMap pdata.AttributeMap,
	properties map[string]string,
	measurements map[string]float64) {

	copyAndMapAttributes(attributeMap, properties, measurements, nil)
}

// Attribute extraction logic for HTTP Span attributes
func copyAndExtractHTTPAttributes(
	attributeMap pdata.AttributeMap,
	properties map[string]string,
	measurements map[string]float64) *HTTPAttributes {

	attrs := &HTTPAttributes{}
	copyAndMapAttributes(
		attributeMap,
		properties,
		measurements,
		func(k string, v pdata.AttributeValue) { attrs.MapAttribute(k, v) })

	return attrs
}

// Attribute extraction logic for RPC Span attributes
func copyAndExtractRPCAttributes(
	attributeMap pdata.AttributeMap,
	properties map[string]string,
	measurements map[string]float64) *RPCAttributes {

	attrs := &RPCAttributes{}
	copyAndMapAttributes(
		attributeMap,
		properties,
		measurements,
		func(k string, v pdata.AttributeValue) { attrs.MapAttribute(k, v) })

	return attrs
}

// Attribute extraction logic for Database Span attributes
func copyAndExtractDatabaseAttributes(
	attributeMap pdata.AttributeMap,
	properties map[string]string,
	measurements map[string]float64) *DatabaseAttributes {

	attrs := &DatabaseAttributes{}
	copyAndMapAttributes(
		attributeMap,
		properties,
		measurements,
		func(k string, v pdata.AttributeValue) { attrs.MapAttribute(k, v) })

	return attrs
}

// Attribute extraction logic for Messaging Span attributes
func copyAndExtractMessagingAttributes(
	attributeMap pdata.AttributeMap,
	properties map[string]string,
	measurements map[string]float64) *MessagingAttributes {

	attrs := &MessagingAttributes{}
	copyAndMapAttributes(
		attributeMap,
		properties,
		measurements,
		func(k string, v pdata.AttributeValue) { attrs.MapAttribute(k, v) })

	return attrs
}

func idToHex(source []byte) string {
	if source == nil {
		return ""
	}

	return fmt.Sprintf("%02x", source)
}

func formatSpanDuration(span pdata.Span) string {
	startTime := toTime(span.StartTime())
	endTime := toTime(span.EndTime())
	return formatDuration(endTime.Sub(startTime))
}

// Maps incoming Span to a type defined in the specification
func mapIncomingSpanToType(attributeMap pdata.AttributeMap) spanType {
	// No attributes
	if attributeMap.Len() == 0 {
		return unknownSpanType
	}

	// HTTP
	if _, exists := attributeMap.Get(conventions.AttributeHTTPMethod); exists {
		return httpSpanType
	}

	// RPC
	if _, exists := attributeMap.Get(conventions.AttributeRPCSystem); exists {
		return rpcSpanType
	}

	// Database
	if _, exists := attributeMap.Get(attributeDBSystem); exists {
		return databaseSpanType
	}

	// Messaging
	if _, exists := attributeMap.Get(conventions.AttributeMessagingSystem); exists {
		return messagingSpanType
	}

	if _, exists := attributeMap.Get(conventions.AttributeFaaSTrigger); exists {
		return faasSpanType
	}

	return unknownSpanType
}

// map to the standard gRPC status codes if specified, otherwise default to 0 - OK
// https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/trace/api.md#status
func getDefaultFormattedSpanStatus(spanStatus pdata.SpanStatus) (statusCodeAsString string, success bool) {
	if spanStatus.IsNil() {
		return "0", true
	}

	statusCode := int32(spanStatus.Code())
	return strconv.FormatInt(int64(statusCode), 10), statusCode == int32(codes.OK)
}

func writeFormattedPeerAddressFromNetworkAttributes(networkAttributes *NetworkAttributes, sb *strings.Builder) {
	// Favor name over IP for
	if networkAttributes.NetPeerName != "" {
		sb.WriteString(networkAttributes.NetPeerName)
	} else if networkAttributes.NetPeerIP != "" {
		sb.WriteString(networkAttributes.NetPeerIP)
	}

	if networkAttributes.NetPeerPort != 0 {
		sb.WriteString(":")
		sb.WriteString(strconv.FormatInt(networkAttributes.NetPeerPort, 10))
	}
}

func setAttributeValueAsPropertyOrMeasurement(
	key string,
	attributeValue pdata.AttributeValue,
	properties map[string]string,
	measurements map[string]float64) {

	switch attributeValue.Type() {
	case pdata.AttributeValueBOOL:
		properties[key] = strconv.FormatBool(attributeValue.BoolVal())

	case pdata.AttributeValueSTRING:
		properties[key] = attributeValue.StringVal()

	case pdata.AttributeValueINT:
		measurements[key] = float64(attributeValue.IntVal())

	case pdata.AttributeValueDOUBLE:
		measurements[key] = float64(attributeValue.DoubleVal())
	}
}

func prefixIfNecessary(s string, prefix string) string {
	if strings.HasPrefix(s, prefix) {
		return s
	}

	return prefix + s
}

func sanitize(sanitizeFunc func() []string, logger *zap.Logger) {
	sanitizeWithCallback(sanitizeFunc, nil, logger)
}

func sanitizeWithCallback(sanitizeFunc func() []string, warningCallback func(string), logger *zap.Logger) {
	sanitizeWarnings := sanitizeFunc()
	for _, warning := range sanitizeWarnings {
		if warningCallback == nil {
			// TODO error handling
			logger.Warn(warning)
		} else {
			warningCallback(warning)
		}
	}
}
