// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"

// Contains code common to both trace and metrics exporters

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

const (
	unknownSpanType   spanType = 0
	httpSpanType      spanType = 1
	rpcSpanType       spanType = 2
	databaseSpanType  spanType = 3
	messagingSpanType spanType = 4
	faasSpanType      spanType = 5

	exceptionSpanEventName string = "exception"
)

var (
	errUnexpectedAttributeValueType = errors.New("attribute value type is unexpected")
	errUnsupportedSpanType          = errors.New("unsupported Span type")
)

// Used to identify the type of a received Span
type spanType int8

// Transforms a tuple of pcommon.Resource, pcommon.InstrumentationScope, ptrace.Span into one or more of AppInsights contracts.Envelope
// This is the only method that should be targeted in the unit tests
func spanToEnvelopes(
	resource pcommon.Resource,
	instrumentationScope pcommon.InstrumentationScope,
	span ptrace.Span,
	spanEventsEnabled bool,
	logger *zap.Logger) ([]*contracts.Envelope, error) {

	spanKind := span.Kind()

	// According to the SpanKind documentation, we can assume it to be INTERNAL
	// when we get UNSPECIFIED.
	if spanKind == ptrace.SpanKindUnspecified {
		spanKind = ptrace.SpanKindInternal
	}

	attributeMap := span.Attributes()
	incomingSpanType := mapIncomingSpanToType(attributeMap)

	// For now, FaaS spans are unsupported
	if incomingSpanType == faasSpanType {
		return nil, errUnsupportedSpanType
	}

	var envelopes []*contracts.Envelope
	var dataSanitizeFunc func() []string
	var dataProperties map[string]string

	// First map the span itself
	envelope := newEnvelope(span, toTime(span.StartTimestamp()).Format(time.RFC3339Nano))

	data := contracts.NewData()

	if userID, exists := attributeMap.Get(conventions.AttributeEnduserID); exists {
		envelope.Tags[contracts.UserId] = userID.Str()
	}

	if spanKind == ptrace.SpanKindServer || spanKind == ptrace.SpanKindConsumer {
		requestData := spanToRequestData(span, incomingSpanType)
		dataProperties = requestData.Properties
		dataSanitizeFunc = requestData.Sanitize
		envelope.Name = requestData.EnvelopeName("")
		envelope.Tags[contracts.OperationName] = requestData.Name
		data.BaseData = requestData
		data.BaseType = requestData.BaseType()
	} else if spanKind == ptrace.SpanKindClient || spanKind == ptrace.SpanKindProducer || spanKind == ptrace.SpanKindInternal {
		remoteDependencyData := spanToRemoteDependencyData(span, incomingSpanType)

		// Regardless of the detected Span type, if the SpanKind is Internal we need to set data.Type to InProc
		if spanKind == ptrace.SpanKindInternal {
			remoteDependencyData.Type = "InProc"
		}

		dataProperties = remoteDependencyData.Properties
		dataSanitizeFunc = remoteDependencyData.Sanitize
		envelope.Name = remoteDependencyData.EnvelopeName("")
		data.BaseData = remoteDependencyData
		data.BaseType = remoteDependencyData.BaseType()
	}

	// Record the raw Span status values as properties
	dataProperties[attributeOtelStatusCode] = traceutil.StatusCodeStr(span.Status().Code())
	statusMessage := span.Status().Message()
	if len(statusMessage) > 0 {
		dataProperties[attributeOtelStatusDescription] = statusMessage
	}

	envelope.Data = data

	resourceAttributes := resource.Attributes()
	applyResourcesToDataProperties(dataProperties, resourceAttributes)
	applyInstrumentationScopeValueToDataProperties(dataProperties, instrumentationScope)
	applyCloudTagsToEnvelope(envelope, resourceAttributes)

	// Sanitize the base data, the envelope and envelope tags
	sanitize(dataSanitizeFunc, logger)
	sanitize(func() []string { return envelope.Sanitize() }, logger)
	sanitize(func() []string { return contracts.SanitizeTags(envelope.Tags) }, logger)

	envelopes = append(envelopes, envelope)

	// Now add the span events. We always export exception events.
	for i := 0; i < span.Events().Len(); i++ {
		spanEvent := span.Events().At(i)

		// skip non-exception events if configured
		if spanEvent.Name() != exceptionSpanEventName && !spanEventsEnabled {
			continue
		}

		spanEventEnvelope := newEnvelope(span, toTime(spanEvent.Timestamp()).Format(time.RFC3339Nano))

		data := contracts.NewData()

		// Exceptions are a special case of span event.
		// See https://opentelemetry.io/docs/reference/specification/trace/semantic_conventions/exceptions/#recording-an-exception
		if spanEvent.Name() == exceptionSpanEventName {
			exceptionData := spanEventToExceptionData(spanEvent)
			dataSanitizeFunc = exceptionData.Sanitize
			dataProperties = exceptionData.Properties
			data.BaseData = exceptionData
			data.BaseType = exceptionData.BaseType()
			spanEventEnvelope.Name = exceptionData.EnvelopeName("")
		} else {
			messageData := spanEventToMessageData(spanEvent)
			dataSanitizeFunc = messageData.Sanitize
			dataProperties = messageData.Properties
			data.BaseData = messageData
			data.BaseType = messageData.BaseType()
			spanEventEnvelope.Name = messageData.EnvelopeName("")
		}

		spanEventEnvelope.Data = data

		applyResourcesToDataProperties(dataProperties, resourceAttributes)
		applyInstrumentationScopeValueToDataProperties(dataProperties, instrumentationScope)
		applyCloudTagsToEnvelope(spanEventEnvelope, resourceAttributes)

		// Sanitize the base data, the envelope and envelope tags
		sanitize(dataSanitizeFunc, logger)
		sanitize(func() []string { return spanEventEnvelope.Sanitize() }, logger)
		sanitize(func() []string { return contracts.SanitizeTags(spanEventEnvelope.Tags) }, logger)

		envelopes = append(envelopes, spanEventEnvelope)
	}

	return envelopes, nil
}

// Creates a new envelope with some basic tags populated
func newEnvelope(span ptrace.Span, time string) *contracts.Envelope {
	envelope := contracts.NewEnvelope()
	envelope.Tags = make(map[string]string)
	envelope.Time = time
	envelope.Tags[contracts.OperationId] = traceutil.TraceIDToHexOrEmptyString(span.TraceID())
	envelope.Tags[contracts.OperationParentId] = traceutil.SpanIDToHexOrEmptyString(span.ParentSpanID())
	return envelope
}

// Maps Server/Consumer Span to AppInsights RequestData
func spanToRequestData(span ptrace.Span, incomingSpanType spanType) *contracts.RequestData {
	// See https://github.com/microsoft/ApplicationInsights-Go/blob/master/appinsights/contracts/requestdata.go
	// Start with some reasonable default for server spans.
	data := contracts.NewRequestData()
	data.Id = traceutil.SpanIDToHexOrEmptyString(span.SpanID())
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
func spanToRemoteDependencyData(span ptrace.Span, incomingSpanType spanType) *contracts.RemoteDependencyData {
	// https://github.com/microsoft/ApplicationInsights-Go/blob/master/appinsights/contracts/remotedependencydata.go
	// Start with some reasonable default for dependent spans.
	data := contracts.NewRemoteDependencyData()
	data.Id = traceutil.SpanIDToHexOrEmptyString(span.SpanID())
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

// Maps SpanEvent to AppInsights ExceptionData
func spanEventToExceptionData(spanEvent ptrace.SpanEvent) *contracts.ExceptionData {
	data := contracts.NewExceptionData()
	data.Properties = make(map[string]string)
	data.Measurements = make(map[string]float64)

	attrs := copyAndExtractExceptionAttributes(spanEvent.Attributes(), data.Properties, data.Measurements)

	details := contracts.NewExceptionDetails()
	details.TypeName = attrs.ExceptionType
	details.Message = attrs.ExceptionMessage
	details.Stack = attrs.ExceptionStackTrace
	details.HasFullStack = details.Stack != ""
	details.ParsedStack = []*contracts.StackFrame{}

	data.Exceptions = []*contracts.ExceptionDetails{details}
	data.SeverityLevel = contracts.Error
	return data
}

// Maps SpanEvent to AppInsights MessageData
func spanEventToMessageData(spanEvent ptrace.SpanEvent) *contracts.MessageData {
	data := contracts.NewMessageData()
	data.Message = spanEvent.Name()
	data.Properties = make(map[string]string)

	copyAttributesWithoutMapping(spanEvent.Attributes(), data.Properties, nil)
	return data
}

func getFormattedHTTPStatusValues(statusCode int64) (statusAsString string, success bool) {
	// see https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/http.md#status
	return strconv.FormatInt(statusCode, 10), statusCode >= 100 && statusCode <= 399
}

// Maps HTTP Server Span to AppInsights RequestData
// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/http.md#semantic-conventions-for-http-spans
func fillRequestDataHTTP(span ptrace.Span, data *contracts.RequestData) {
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
	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/http.md#name
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

	switch {
	case attrs.HTTPScheme != "" && attrs.HTTPHost != "" && attrs.HTTPTarget != "":
		sb.WriteString(attrs.HTTPScheme)
		sb.WriteString("://")
		sb.WriteString(attrs.HTTPHost)
		sb.WriteString(attrs.HTTPTarget)
		data.Url = sb.String()
	case attrs.HTTPScheme != "" && attrs.HTTPServerName != "" && netHostPortAsString != "" && attrs.HTTPTarget != "":
		sb.WriteString(attrs.HTTPScheme)
		sb.WriteString("://")
		sb.WriteString(attrs.HTTPServerName)
		sb.WriteString(":")
		sb.WriteString(netHostPortAsString)
		sb.WriteString(attrs.HTTPTarget)
		data.Url = sb.String()
	case attrs.HTTPScheme != "" && attrs.NetworkAttributes.NetHostName != "" && netHostPortAsString != "" && attrs.HTTPTarget != "":
		sb.WriteString(attrs.HTTPScheme)
		sb.WriteString("://")
		sb.WriteString(attrs.NetworkAttributes.NetHostName)
		sb.WriteString(":")
		sb.WriteString(netHostPortAsString)
		sb.WriteString(attrs.HTTPTarget)
		data.Url = sb.String()
	case attrs.HTTPURL != "":
		if _, err := url.Parse(attrs.HTTPURL); err == nil {
			data.Url = attrs.HTTPURL
		}
	}

	sb.Reset()

	// data.Source should be the client ip if available or fallback to net.peer.ip
	// https://github.com/microsoft/ApplicationInsights-Home/blob/f1f9f619d74557c8db3dbde4b49c4193e10d8a81/EndpointSpecs/Schemas/Bond/RequestData.bond#L28
	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/http.md#http-server-semantic-conventions
	if attrs.HTTPClientIP != "" {
		data.Source = attrs.HTTPClientIP
	} else if attrs.NetworkAttributes.NetPeerIP != "" {
		data.Source = attrs.NetworkAttributes.NetPeerIP
	}
}

// Maps HTTP Client Span to AppInsights RemoteDependencyData
// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/http.md
func fillRemoteDependencyDataHTTP(span ptrace.Span, data *contracts.RemoteDependencyData) {
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
	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/http.md#name
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

	switch {
	case attrs.HTTPURL != "":
		if u, err := url.Parse(attrs.HTTPURL); err == nil {
			data.Data = attrs.HTTPURL
			data.Target = u.Host
		}
	case attrs.HTTPScheme != "" && attrs.HTTPHost != "" && attrs.HTTPTarget != "":
		sb.WriteString(attrs.HTTPScheme)
		sb.WriteString("://")
		sb.WriteString(attrs.HTTPHost)
		sb.WriteString(attrs.HTTPTarget)
		data.Data = sb.String()
		data.Target = attrs.HTTPHost
	case attrs.HTTPScheme != "" && attrs.NetworkAttributes.NetPeerName != "" && netPeerPortAsString != "" && attrs.HTTPTarget != "":
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
	case attrs.HTTPScheme != "" && attrs.NetworkAttributes.NetPeerIP != "" && netPeerPortAsString != "" && attrs.HTTPTarget != "":
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
// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/rpc.md
func fillRequestDataRPC(span ptrace.Span, data *contracts.RequestData) {
	attrs := copyAndExtractRPCAttributes(span.Attributes(), data.Properties, data.Measurements)

	data.ResponseCode = getRPCStatusCodeAsString(attrs)

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
// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/rpc.md
func fillRemoteDependencyDataRPC(span ptrace.Span, data *contracts.RemoteDependencyData) {
	attrs := copyAndExtractRPCAttributes(span.Attributes(), data.Properties, data.Measurements)

	data.ResultCode = getRPCStatusCodeAsString(attrs)

	// Set the .Data property to .Name which contain the full RPC method
	data.Data = data.Name

	data.Type = attrs.RPCSystem

	var sb strings.Builder
	writeFormattedPeerAddressFromNetworkAttributes(&attrs.NetworkAttributes, &sb)
	data.Target = sb.String()
}

// Returns the RPC status code as a string
func getRPCStatusCodeAsString(rpcAttributes *RPCAttributes) (statusCodeAsString string) {
	// Honor the attribute rpc.grpc.status_code if there
	if rpcAttributes.RPCGRPCStatusCode != 0 {
		return strconv.FormatInt(rpcAttributes.RPCGRPCStatusCode, 10)
	}
	return "0"
}

// Maps Database Client Span to AppInsights RemoteDependencyData
// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/database.md
func fillRemoteDependencyDataDatabase(span ptrace.Span, data *contracts.RemoteDependencyData) {
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
// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/messaging.md
func fillRequestDataMessaging(span ptrace.Span, data *contracts.RequestData) {
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
// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/messaging.md
func fillRemoteDependencyDataMessaging(span ptrace.Span, data *contracts.RemoteDependencyData) {
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
	attributeMap pcommon.Map,
	properties map[string]string,
	measurements map[string]float64,
	mappingFunc func(k string, v pcommon.Value)) {

	attributeMap.Range(func(k string, v pcommon.Value) bool {
		setAttributeValueAsPropertyOrMeasurement(k, v, properties, measurements)
		if mappingFunc != nil {
			mappingFunc(k, v)
		}
		return true
	})
}

// Copies all attributes to either properties or measurements without any kind of mapping to a known set of attributes
func copyAttributesWithoutMapping(
	attributeMap pcommon.Map,
	properties map[string]string,
	measurements map[string]float64) {

	copyAndMapAttributes(attributeMap, properties, measurements, nil)
}

// Attribute extraction logic for HTTP Span attributes
func copyAndExtractHTTPAttributes(
	attributeMap pcommon.Map,
	properties map[string]string,
	measurements map[string]float64) *HTTPAttributes {

	attrs := &HTTPAttributes{}
	copyAndMapAttributes(
		attributeMap,
		properties,
		measurements,
		func(k string, v pcommon.Value) { attrs.MapAttribute(k, v) })

	return attrs
}

// Attribute extraction logic for RPC Span attributes
func copyAndExtractRPCAttributes(
	attributeMap pcommon.Map,
	properties map[string]string,
	measurements map[string]float64) *RPCAttributes {

	attrs := &RPCAttributes{}
	copyAndMapAttributes(
		attributeMap,
		properties,
		measurements,
		func(k string, v pcommon.Value) { attrs.MapAttribute(k, v) })

	return attrs
}

// Attribute extraction logic for Database Span attributes
func copyAndExtractDatabaseAttributes(
	attributeMap pcommon.Map,
	properties map[string]string,
	measurements map[string]float64) *DatabaseAttributes {

	attrs := &DatabaseAttributes{}
	copyAndMapAttributes(
		attributeMap,
		properties,
		measurements,
		func(k string, v pcommon.Value) { attrs.MapAttribute(k, v) })

	return attrs
}

// Attribute extraction logic for Messaging Span attributes
func copyAndExtractMessagingAttributes(
	attributeMap pcommon.Map,
	properties map[string]string,
	measurements map[string]float64) *MessagingAttributes {

	attrs := &MessagingAttributes{}
	copyAndMapAttributes(
		attributeMap,
		properties,
		measurements,
		func(k string, v pcommon.Value) { attrs.MapAttribute(k, v) })

	return attrs
}

// Attribute extraction logic for Span event exception attributes
func copyAndExtractExceptionAttributes(
	attributeMap pcommon.Map,
	properties map[string]string,
	measurements map[string]float64) *ExceptionAttributes {

	attrs := &ExceptionAttributes{}
	copyAndMapAttributes(
		attributeMap,
		properties,
		measurements,
		func(k string, v pcommon.Value) { attrs.MapAttribute(k, v) })

	return attrs
}

func formatSpanDuration(span ptrace.Span) string {
	startTime := toTime(span.StartTimestamp())
	endTime := toTime(span.EndTimestamp())
	return formatDuration(endTime.Sub(startTime))
}

// Maps incoming Span to a type defined in the specification
func mapIncomingSpanToType(attributeMap pcommon.Map) spanType {
	// No attributes
	if attributeMap.Len() == 0 {
		return unknownSpanType
	}

	// RPC
	if _, exists := attributeMap.Get(conventions.AttributeRPCSystem); exists {
		return rpcSpanType
	}

	// HTTP
	if _, exists := attributeMap.Get(conventions.AttributeHTTPMethod); exists {
		return httpSpanType
	}

	// Database
	if _, exists := attributeMap.Get(conventions.AttributeDBSystem); exists {
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

// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/api.md#set-status
func getDefaultFormattedSpanStatus(spanStatus ptrace.Status) (statusCodeAsString string, success bool) {
	code := spanStatus.Code()

	return strconv.FormatInt(int64(code), 10), code != ptrace.StatusCodeError
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

// Sets the attribute value as a property or measurement.
// Int and floats go to measurements if measurements is not nil, otherwise everything goes to properties.
func setAttributeValueAsPropertyOrMeasurement(
	key string,
	attributeValue pcommon.Value,
	properties map[string]string,
	measurements map[string]float64) {

	switch attributeValue.Type() {
	case pcommon.ValueTypeBool:
		properties[key] = strconv.FormatBool(attributeValue.Bool())

	case pcommon.ValueTypeStr:
		properties[key] = attributeValue.Str()

	case pcommon.ValueTypeInt:
		if measurements == nil {
			properties[key] = strconv.FormatInt(attributeValue.Int(), 10)
		} else {
			measurements[key] = float64(attributeValue.Int())
		}

	case pcommon.ValueTypeDouble:
		if measurements == nil {
			properties[key] = strconv.FormatFloat(attributeValue.Double(), 'f', -1, 64)
		} else {
			measurements[key] = attributeValue.Double()
		}
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
