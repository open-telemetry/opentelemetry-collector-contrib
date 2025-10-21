// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"

// Contains code common to both trace and metrics exporters

import (
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.34.0"
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
	msLinks                string = "_MS.links"
)

var (
	errUnexpectedAttributeValueType = errors.New("attribute value type is unexpected")
	errUnsupportedSpanType          = errors.New("unsupported Span type")
)

// Used to identify the type of a received Span
type spanType int8

type msLink struct {
	OperationID string `json:"operation_Id"`
	ID          string `json:"id"`
}

// Transforms a tuple of pcommon.Resource, pcommon.InstrumentationScope, ptrace.Span into one or more of AppInsights contracts.Envelope
// This is the only method that should be targeted in the unit tests
func spanToEnvelopes(
	resource pcommon.Resource,
	instrumentationScope pcommon.InstrumentationScope,
	span ptrace.Span,
	spanEventsEnabled bool,
	logger *zap.Logger,
) ([]*contracts.Envelope, error) {
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

	if userID, exists := attributeMap.Get(string(conventions.EnduserIDKey)); exists {
		envelope.Tags[contracts.UserId] = userID.Str()
	}

	switch spanKind {
	case ptrace.SpanKindServer, ptrace.SpanKindConsumer:
		requestData := spanToRequestData(span, incomingSpanType)
		dataProperties = requestData.Properties
		dataSanitizeFunc = requestData.Sanitize
		envelope.Name = requestData.EnvelopeName("")
		envelope.Tags[contracts.OperationName] = requestData.Name
		data.BaseData = requestData
		data.BaseType = requestData.BaseType()
	case ptrace.SpanKindClient, ptrace.SpanKindProducer, ptrace.SpanKindInternal:
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
	if statusMessage != "" {
		dataProperties[attributeOtelStatusDescription] = statusMessage
	}

	envelope.Data = data

	resourceAttributes := resource.Attributes()
	applyResourcesToDataProperties(dataProperties, resourceAttributes)
	applyInstrumentationScopeValueToDataProperties(dataProperties, instrumentationScope)
	applyCloudTagsToEnvelope(envelope, resourceAttributes)
	applyInternalSdkVersionTagToEnvelope(envelope)
	applyLinksToDataProperties(dataProperties, span.Links(), logger)

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
		spanEventEnvelope.Tags[contracts.OperationParentId] = traceutil.SpanIDToHexOrEmptyString(span.SpanID())

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
		applyInternalSdkVersionTagToEnvelope(envelope)

		// Sanitize the base data, the envelope and envelope tags
		sanitize(dataSanitizeFunc, logger)
		sanitize(func() []string { return spanEventEnvelope.Sanitize() }, logger)
		sanitize(func() []string { return contracts.SanitizeTags(spanEventEnvelope.Tags) }, logger)

		envelopes = append(envelopes, spanEventEnvelope)
	}

	return envelopes, nil
}

func applyLinksToDataProperties(dataProperties map[string]string, spanLinkSlice ptrace.SpanLinkSlice, logger *zap.Logger) {
	if spanLinkSlice.Len() == 0 {
		return
	}

	links := make([]msLink, 0, spanLinkSlice.Len())

	for i := 0; i < spanLinkSlice.Len(); i++ {
		link := spanLinkSlice.At(i)
		links = append(links, msLink{
			OperationID: traceutil.TraceIDToHexOrEmptyString(link.TraceID()),
			ID:          traceutil.SpanIDToHexOrEmptyString(link.SpanID()),
		})
	}

	if len(links) > 0 {
		if jsonBytes, err := json.Marshal(links); err == nil {
			dataProperties[msLinks] = string(jsonBytes)
		} else {
			logger.Warn("Failed to marshal span links to JSON", zap.Error(err))
		}
	}
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
	data.ResponseCode, data.Success = getDefaultFormattedSpanStatus(span.Status())

	switch incomingSpanType {
	case httpSpanType:
		fillRequestDataHTTP(span, data)
	case rpcSpanType:
		fillRequestDataRPC(span, data)
	case messagingSpanType:
		fillRequestDataMessaging(span, data)
	case unknownSpanType:
		copyAttributesWithoutMapping(span.Attributes(), data.Properties)
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
		copyAttributesWithoutMapping(span.Attributes(), data.Properties)
	}

	return data
}

// Maps SpanEvent to AppInsights ExceptionData
func spanEventToExceptionData(spanEvent ptrace.SpanEvent) *contracts.ExceptionData {
	data := contracts.NewExceptionData()
	data.Properties = make(map[string]string)

	attrs := copyAndExtractExceptionAttributes(spanEvent.Attributes(), data.Properties)

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

	copyAttributesWithoutMapping(spanEvent.Attributes(), data.Properties)
	return data
}

func getFormattedHTTPStatusValues(statusCode int64) (statusAsString string, success bool) {
	// see https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/http.md#status
	return strconv.FormatInt(statusCode, 10), statusCode >= 100 && statusCode <= 399
}

// Maps HTTP Server Span to AppInsights RequestData
// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/http.md#semantic-conventions-for-http-spans
func fillRequestDataHTTP(span ptrace.Span, data *contracts.RequestData) {
	attrs := copyAndExtractHTTPAttributes(span.Attributes(), data.Properties)

	if attrs.HTTPResponseStatusCode != 0 {
		data.ResponseCode, data.Success = getFormattedHTTPStatusValues(attrs.HTTPResponseStatusCode)
	}

	var sb strings.Builder

	// Construct data.Name
	// The data.Name should be {HTTP METHOD} {HTTP SERVER ROUTE TEMPLATE}
	// https://github.com/microsoft/ApplicationInsights-Home/blob/f1f9f619d74557c8db3dbde4b49c4193e10d8a81/EndpointSpecs/Schemas/Bond/RequestData.bond#L32
	sb.WriteString(attrs.HTTPRequestMethod)
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

	if attrs.URLAttributes.URLPath != "" {
		attrs.URLAttributes.URLPath = prefixIfNecessary(attrs.URLAttributes.URLPath, "/")
	}

	serverPort := ""
	if attrs.ServerAttributes.ServerPort != 0 {
		serverPort = strconv.FormatInt(attrs.ServerAttributes.ServerPort, 10)
	}

	switch {
	case attrs.URLAttributes.URLScheme != "" && attrs.ServerAttributes.ServerAddress != "" && serverPort == "" && attrs.URLAttributes.URLPath != "":
		sb.WriteString(attrs.URLAttributes.URLScheme)
		sb.WriteString("://")
		sb.WriteString(attrs.ServerAttributes.ServerAddress)
		sb.WriteString(attrs.URLAttributes.URLPath)
		if attrs.URLAttributes.URLQuery != "" {
			sb.WriteString(prefixIfNecessary(attrs.URLAttributes.URLQuery, "?"))
		}
		data.Url = sb.String()
	case attrs.URLAttributes.URLScheme != "" && attrs.ServerAttributes.ServerAddress != "" && serverPort != "" && attrs.URLAttributes.URLPath != "":
		sb.WriteString(attrs.URLAttributes.URLScheme)
		sb.WriteString("://")
		sb.WriteString(attrs.ServerAttributes.ServerAddress)
		sb.WriteString(":")
		sb.WriteString(serverPort)
		sb.WriteString(attrs.URLAttributes.URLPath)
		if attrs.URLAttributes.URLQuery != "" {
			sb.WriteString(prefixIfNecessary(attrs.URLAttributes.URLQuery, "?"))
		}
		data.Url = sb.String()
	case attrs.URLAttributes.URLFull != "":
		if _, err := url.Parse(attrs.URLAttributes.URLFull); err == nil {
			data.Url = attrs.URLAttributes.URLFull
		}
	}

	sb.Reset()

	if attrs.ClientAttributes.ClientAddress != "" {
		data.Source = attrs.ClientAttributes.ClientAddress
	} else if attrs.NetworkAttributes.NetworkPeerAddress != "" {
		data.Source = attrs.NetworkAttributes.NetworkPeerAddress
	}
}

// Maps HTTP Client Span to AppInsights RemoteDependencyData
// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/http.md
func fillRemoteDependencyDataHTTP(span ptrace.Span, data *contracts.RemoteDependencyData) {
	attrs := copyAndExtractHTTPAttributes(span.Attributes(), data.Properties)

	data.Type = "HTTP"
	if attrs.HTTPResponseStatusCode != 0 {
		data.ResultCode, data.Success = getFormattedHTTPStatusValues(attrs.HTTPResponseStatusCode)
	}

	var sb strings.Builder

	sb.WriteString(attrs.HTTPRequestMethod)

	if attrs.HTTPRoute != "" {
		sb.WriteString(" ")
		sb.WriteString(attrs.HTTPRoute)
	}

	data.Name = sb.String()
	sb.Reset()

	if attrs.URLAttributes.URLPath != "" {
		attrs.URLAttributes.URLPath = prefixIfNecessary(attrs.URLAttributes.URLPath, "/")
	}

	clientPortStr := ""
	if attrs.ClientAttributes.ClientPort != 0 {
		clientPortStr = strconv.FormatInt(attrs.ClientAttributes.ClientPort, 10)
	}

	switch {
	case attrs.URLAttributes.URLFull != "":
		if u, err := url.Parse(attrs.URLAttributes.URLFull); err == nil {
			data.Data = attrs.URLAttributes.URLFull
			data.Target = u.Host
		}
	case attrs.URLAttributes.URLScheme != "" && attrs.ClientAttributes.ClientAddress != "" && clientPortStr == "" && attrs.URLAttributes.URLPath != "":
		sb.WriteString(attrs.URLAttributes.URLScheme)
		sb.WriteString("://")
		sb.WriteString(attrs.ClientAttributes.ClientAddress)
		sb.WriteString(attrs.URLAttributes.URLPath)
		if attrs.URLAttributes.URLQuery != "" {
			sb.WriteString(prefixIfNecessary(attrs.URLAttributes.URLQuery, "?"))
		}
		data.Data = sb.String()
		data.Target = attrs.ClientAttributes.ClientAddress

	case attrs.URLAttributes.URLScheme != "" && attrs.ClientAttributes.ClientAddress != "" && clientPortStr != "" && attrs.URLAttributes.URLPath != "":
		sb.WriteString(attrs.URLAttributes.URLScheme)
		sb.WriteString("://")
		sb.WriteString(attrs.ClientAttributes.ClientAddress)
		sb.WriteString(":")
		sb.WriteString(clientPortStr)
		sb.WriteString(attrs.URLAttributes.URLPath)
		if attrs.URLAttributes.URLQuery != "" {
			sb.WriteString(prefixIfNecessary(attrs.URLAttributes.URLQuery, "?"))
		}
		data.Data = sb.String()

		sb.Reset()
		sb.WriteString(attrs.ClientAttributes.ClientAddress)
		sb.WriteString(":")
		sb.WriteString(clientPortStr)
		data.Target = sb.String()

	case attrs.URLAttributes.URLScheme != "" && attrs.NetworkAttributes.NetworkPeerAddress != "" && clientPortStr != "" && attrs.URLAttributes.URLPath != "":
		sb.WriteString(attrs.URLAttributes.URLScheme)
		sb.WriteString("://")
		sb.WriteString(attrs.NetworkAttributes.NetworkPeerAddress)
		sb.WriteString(":")
		sb.WriteString(clientPortStr)
		sb.WriteString(attrs.URLAttributes.URLPath)
		if attrs.URLAttributes.URLQuery != "" {
			sb.WriteString(prefixIfNecessary(attrs.URLAttributes.URLQuery, "?"))
		}
		data.Data = sb.String()

		sb.Reset()
		sb.WriteString(attrs.NetworkAttributes.NetworkPeerAddress)
		sb.WriteString(":")
		sb.WriteString(clientPortStr)
		data.Target = sb.String()
	}
}

// Maps RPC Server Span to AppInsights RequestData
// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/rpc.md
func fillRequestDataRPC(span ptrace.Span, data *contracts.RequestData) {
	attrs := copyAndExtractRPCAttributes(span.Attributes(), data.Properties)

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

	writeFormatedFromNetworkServerOrClient(&attrs.NetworkAttributes, attrs.ServerAttributes.ServerAddress, attrs.ServerAttributes.ServerPort, &sb)

	data.Source = sb.String()
}

// Maps RPC Client Span to AppInsights RemoteDependencyData
// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/rpc.md
func fillRemoteDependencyDataRPC(span ptrace.Span, data *contracts.RemoteDependencyData) {
	attrs := copyAndExtractRPCAttributes(span.Attributes(), data.Properties)

	data.ResultCode = getRPCStatusCodeAsString(attrs)

	// Set the .Data property to .Name which contain the full RPC method
	data.Data = data.Name

	data.Type = attrs.RPCSystem

	var sb strings.Builder

	writeFormatedFromNetworkServerOrClient(&attrs.NetworkAttributes, attrs.ClientAttributes.ClientAddress, attrs.ClientAttributes.ClientPort, &sb)
	data.Target = sb.String()
}

// Returns the RPC status code as a string
func getRPCStatusCodeAsString(rpcAttributes *rpcAttributes) (statusCodeAsString string) {
	// Honor the attribute rpc.grpc.status_code if there
	if rpcAttributes.RPCGRPCStatusCode != 0 {
		return strconv.FormatInt(rpcAttributes.RPCGRPCStatusCode, 10)
	}
	return "0"
}

// Maps Database Client Span to AppInsights RemoteDependencyData
// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/database.md
func fillRemoteDependencyDataDatabase(span ptrace.Span, data *contracts.RemoteDependencyData) {
	attrs := copyAndExtractDatabaseAttributes(span.Attributes(), data.Properties)

	data.Type = attrs.DBSystemName

	if attrs.DBQueryText != "" {
		data.Data = attrs.DBQueryText
	} else if attrs.DBOperationName != "" {
		data.Data = attrs.DBOperationName
	}

	var sb strings.Builder
	writeFormatedFromNetworkServerOrClient(&attrs.NetworkAttributes, attrs.ClientAttributes.ClientAddress, attrs.ClientAttributes.ClientPort, &sb)
	data.Target = sb.String()
}

// Maps Messaging Consumer/Server Span to AppInsights RequestData
// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/messaging.md
func fillRequestDataMessaging(span ptrace.Span, data *contracts.RequestData) {
	attrs := copyAndExtractMessagingAttributes(span.Attributes(), data.Properties)

	// TODO Understand how to map attributes to RequestData fields
	var sb strings.Builder
	writeFormatedFromNetworkServerOrClient(&attrs.NetworkAttributes, attrs.ServerAttributes.ServerAddress, attrs.ServerAttributes.ServerPort, &sb)
	data.Source = sb.String()
}

// Maps Messaging Producer/Client Span to AppInsights RemoteDependencyData
// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/messaging.md
func fillRemoteDependencyDataMessaging(span ptrace.Span, data *contracts.RemoteDependencyData) {
	attrs := copyAndExtractMessagingAttributes(span.Attributes(), data.Properties)

	// TODO Understand how to map attributes to RemoteDependencyData fields
	data.Type = attrs.MessagingSystem

	var sb strings.Builder
	writeFormatedFromNetworkServerOrClient(&attrs.NetworkAttributes, attrs.ClientAttributes.ClientAddress, attrs.ClientAttributes.ClientPort, &sb)
	data.Target = sb.String()
}

// Copies all attributes to either properties or measurements and passes the key/value to another mapping function
func copyAndMapAttributes(
	attributeMap pcommon.Map,
	properties map[string]string,
	mappingFunc func(k string, v pcommon.Value),
) {
	for k, v := range attributeMap.All() {
		setAttributeValueAsProperty(k, v, properties)
		if mappingFunc != nil {
			mappingFunc(k, v)
		}
	}
}

// Copies all attributes to either properties or measurements without any kind of mapping to a known set of attributes
func copyAttributesWithoutMapping(
	attributeMap pcommon.Map,
	properties map[string]string,
) {
	copyAndMapAttributes(attributeMap, properties, nil)
}

// Attribute extraction logic for HTTP Span attributes
func copyAndExtractHTTPAttributes(
	attributeMap pcommon.Map,
	properties map[string]string,
) *httpAttributes {
	attrs := &httpAttributes{}
	copyAndMapAttributes(
		attributeMap,
		properties,
		func(k string, v pcommon.Value) { attrs.MapAttribute(k, v) })

	return attrs
}

// Attribute extraction logic for RPC Span attributes
func copyAndExtractRPCAttributes(
	attributeMap pcommon.Map,
	properties map[string]string,
) *rpcAttributes {
	attrs := &rpcAttributes{}
	copyAndMapAttributes(
		attributeMap,
		properties,
		func(k string, v pcommon.Value) { attrs.MapAttribute(k, v) })

	return attrs
}

// Attribute extraction logic for Database Span attributes
func copyAndExtractDatabaseAttributes(
	attributeMap pcommon.Map,
	properties map[string]string,
) *databaseAttributes {
	attrs := &databaseAttributes{}
	copyAndMapAttributes(
		attributeMap,
		properties,
		func(k string, v pcommon.Value) { attrs.MapAttribute(k, v) })

	return attrs
}

// Attribute extraction logic for Messaging Span attributes
func copyAndExtractMessagingAttributes(
	attributeMap pcommon.Map,
	properties map[string]string,
) *messagingAttributes {
	attrs := &messagingAttributes{}
	copyAndMapAttributes(
		attributeMap,
		properties,
		func(k string, v pcommon.Value) { attrs.MapAttribute(k, v) })

	return attrs
}

// Attribute extraction logic for Span event exception attributes
func copyAndExtractExceptionAttributes(
	attributeMap pcommon.Map,
	properties map[string]string,
) *exceptionAttributes {
	attrs := &exceptionAttributes{}
	copyAndMapAttributes(
		attributeMap,
		properties,
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
	if _, exists := attributeMap.Get(string(conventions.RPCSystemKey)); exists {
		return rpcSpanType
	}

	// HTTP
	if _, exists := attributeMap.Get(string(conventions.HTTPRequestMethodKey)); exists {
		return httpSpanType
	}

	// Database
	if _, exists := attributeMap.Get(string(conventions.DBSystemNameKey)); exists {
		return databaseSpanType
	}

	// Messaging
	if _, exists := attributeMap.Get(string(conventions.MessagingSystemKey)); exists {
		return messagingSpanType
	}

	if _, exists := attributeMap.Get(string(conventions.FaaSTriggerKey)); exists {
		return faasSpanType
	}

	return unknownSpanType
}

// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/api.md#set-status
func getDefaultFormattedSpanStatus(spanStatus ptrace.Status) (statusCodeAsString string, success bool) {
	code := spanStatus.Code()

	return strconv.FormatInt(int64(code), 10), code != ptrace.StatusCodeError
}

func writeFormatedFromNetworkServerOrClient(networkAttributes *networkAttributes, addressName string, addressPort int64, sb *strings.Builder) {
	// server.address or client.address
	if addressName != "" {
		sb.WriteString(addressName)
	} else {
		sb.WriteString(networkAttributes.NetworkPeerAddress)
	}

	if addressPort != 0 {
		sb.WriteString(":")
		sb.WriteString(strconv.FormatInt(addressPort, 10))
	}
}

func setAttributeValueAsProperty(
	key string,
	attributeValue pcommon.Value,
	properties map[string]string,
) {
	switch attributeValue.Type() {
	case pcommon.ValueTypeBool:
		properties[key] = strconv.FormatBool(attributeValue.Bool())

	case pcommon.ValueTypeStr:
		properties[key] = attributeValue.Str()

	case pcommon.ValueTypeInt:
		properties[key] = strconv.FormatInt(attributeValue.Int(), 10)

	case pcommon.ValueTypeDouble:
		properties[key] = strconv.FormatFloat(attributeValue.Double(), 'f', -1, 64)
	}
}

func prefixIfNecessary(s, prefix string) string {
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
