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
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Microsoft/ApplicationInsights-Go/appinsights/contracts"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
)

const (
	spanAttributeKeyComponent = "component"

	// common attributes keys for HTTP spans
	// https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/data-http.md#common-attributes
	spanAttributeKeyHTTPMethod     = "http.method"
	spanAttributeKeyHTTPUrl        = "http.url"
	spanAttributeKeyHTTPTarget     = "http.target"
	spanAttributeKeyHTTPHost       = "http.host"
	spanAttributeKeyHTTPScheme     = "http.scheme"
	spanAttributeKeyHTTPStatusCode = "http.status_code"
	spanAttributeKeyHTTPFlavor     = "http.flavor"

	// attributes key for HTTP server spans
	// https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/data-http.md#http-server
	spanAttributeKeyHTTPServerName = "http.server_name"
	spanAttributeKeyHostName       = "host.name"
	spanAttributeKeyHostPort       = "host.port"
	spanAttributeKeyHTTPRoute      = "http.route"
	spanAttributeKeyHTTPClientIP   = "http.client_ip"

	// general purpose peer attribute keys used across various types
	spanAttributeKeyPeerAddress  = "peer.address"
	spanAttributeKeyPeerService  = "peer.service"
	spanAttributeKeyPeerHostname = "peer.hostname"
	spanAttributeKeyPeerPort     = "peer.port"
	spanAttributeKeyPeerIP       = "peer.ip"
	spanAttributeKeyPeerIPv4     = "peer.ipv4"
	spanAttributeKeyPeerIPv6     = "peer.ipv6"

	// RPC attribute keys
	spanAttributeKeyRPCStatusCode    = "status_code"
	spanAttributeKeyRPCStatusMessage = "status_message"

	// Database keys
	// https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/data-database.md
	spanAttributeKeyDbType      = "db.type"
	spanAttributeKeyDbInstance  = "db.instance"
	spanAttributeKeyDbStatement = "db.statement"
	spanAttributeKeyDbUser      = "db.user"
)

type traceExporter struct {
	config           *Config
	transportChannel transportChannel
	logger           *zap.Logger
}

func idToHex(source []byte) string {
	if source == nil {
		return ""
	}

	return fmt.Sprintf("%02x", source)
}

func formatSpanDuration(span *tracepb.Span) string {
	startTime := toTime(span.StartTime)
	endTime := toTime(span.EndTime)
	return formatDuration(endTime.Sub(startTime))
}

func onAttributeStringValueExists(attributes map[string]*tracepb.AttributeValue, key string, action func(val string)) {
	if val, exists := attributes[key]; exists {
		action(attributeValueAsString(val))
	}
}

func attributeValueAsString(val *tracepb.AttributeValue) string {
	if wrapper := val.GetStringValue(); wrapper != nil {
		return wrapper.GetValue()
	}

	return ""
}

func setAttributeValueAsPropertyOrMeasurement(
	key string,
	attributeValue *tracepb.AttributeValue,
	properties map[string]string,
	measurements map[string]float64) {

	switch value := attributeValue.Value.(type) {
	case *tracepb.AttributeValue_BoolValue:
		properties[key] = strconv.FormatBool(value.BoolValue)

	case *tracepb.AttributeValue_StringValue:
		properties[key] = attributeValueAsString(attributeValue)

	case *tracepb.AttributeValue_IntValue:
		measurements[key] = float64(value.IntValue)

	case *tracepb.AttributeValue_DoubleValue:
		measurements[key] = float64(value.DoubleValue)
	}
}

// Transforms a wire-format Span to an AppInsights Envelope
func (exporter *traceExporter) spanToEnvelope(
	instrumentationKey string,
	span *tracepb.Span) (*contracts.Envelope, error) {

	envelope := contracts.NewEnvelope()
	envelope.Tags = make(map[string]string)
	envelope.IKey = instrumentationKey
	envelope.Time = toTime(span.StartTime).Format(time.RFC3339Nano)

	traceIDHexString := idToHex(span.TraceId)
	envelope.Tags[contracts.OperationId] = traceIDHexString
	envelope.Tags[contracts.OperationParentId] = idToHex(span.ParentSpanId)

	data := contracts.NewData()

	if span.Kind == tracepb.Span_SERVER {
		requestData := spanToRequestData(span)
		exporter.sanitize(func() []string { return requestData.Sanitize() })
		envelope.Name = requestData.EnvelopeName("")
		envelope.Tags[contracts.OperationName] = requestData.Name
		data.BaseData = requestData
		data.BaseType = requestData.BaseType()
	} else if span.Kind == tracepb.Span_CLIENT || span.Kind == tracepb.Span_SPAN_KIND_UNSPECIFIED {
		remoteDependencyData := spanToRemoteDependencyData(span)
		exporter.sanitize(func() []string { return remoteDependencyData.Sanitize() })
		envelope.Name = remoteDependencyData.EnvelopeName("")
		data.BaseData = remoteDependencyData
		data.BaseType = remoteDependencyData.BaseType()
	}

	envelope.Data = data
	exporter.sanitize(func() []string { return envelope.Sanitize() })
	exporter.sanitize(func() []string { return contracts.SanitizeTags(envelope.Tags) })

	return envelope, nil
}

// Transforms a wire format Span to AppInsights RequestData
func spanToRequestData(span *tracepb.Span) *contracts.RequestData {
	/*
		Request type comes from a few attributes.

		HTTP
		https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/data-http.md

		RPC (gRPC)
		https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/data-rpc.md

		Database
		https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/data-database.md
	*/

	// https://github.com/microsoft/ApplicationInsights-Go/blob/master/appinsights/contracts/requestdata.go
	// Start with some reasonable default for server spans.
	data := contracts.NewRequestData()
	data.Id = idToHex(span.SpanId)
	data.Name = span.Name.Value
	data.Duration = formatSpanDuration(span)
	data.Properties = make(map[string]string)
	data.Measurements = make(map[string]float64)
	data.ResponseCode = "0"
	data.Success = true

	if span.Attributes != nil && span.Attributes.AttributeMap != nil {
		attributes := span.Attributes.AttributeMap
		component := ""
		onAttributeStringValueExists(attributes, spanAttributeKeyComponent, func(val string) { component = val })

		// TODO remove this once the OpenTelemetry wire format protocol is adopted.
		// The specs indicate that component is a required tag
		onAttributeStringValueExists(attributes, spanAttributeKeyHTTPMethod, func(val string) { component = "http" })

		switch component {
		case "":
			fillRequestDataInternal(span, data)
		case "http":
			fillRequestDataHTTP(span, data)
		case "grpc":
			fillRequestDataGrpc(span, data)
		default:
		}
	}

	return data
}

// Sets properties on an AppInsights RequestData from the wire format Span when the inbound Span type is HTTP
func fillRequestDataHTTP(span *tracepb.Span, data *contracts.RequestData) {
	/*
		The set of expected OpenTelemetry attribute sets for HTTP server span:
		https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/data-http.md#http-server

		Order of preference is:
			http.scheme, http.host, http.target
			http.scheme, http.server_name, host.port, http.target
			http.scheme, host.name, host.port, http.target
			http.url
	*/
	attributes := span.Attributes.AttributeMap
	httpMethod := ""
	httpScheme := ""
	httpHost := ""
	httpTarget := ""
	httpServerName := ""
	hostPort := ""
	hostName := ""
	httpURL := ""

	for k, v := range attributes {
		switch k {
		case spanAttributeKeyComponent:
			// do nothing
		case spanAttributeKeyHTTPMethod:
			httpMethod = attributeValueAsString(v)
		case spanAttributeKeyHTTPScheme:
			httpScheme = attributeValueAsString(v)
		case spanAttributeKeyHTTPHost:
			httpHost = attributeValueAsString(v)
		case spanAttributeKeyHTTPTarget:
			httpTarget = prefixIfNecessary(attributeValueAsString(v), "/")
		case spanAttributeKeyHTTPServerName:
			httpServerName = attributeValueAsString(v)
		case spanAttributeKeyHostPort:
			switch value := v.Value.(type) {
			case *tracepb.AttributeValue_StringValue:
				hostPort = attributeValueAsString(v)
			case *tracepb.AttributeValue_IntValue:
				hostPort = strconv.FormatInt(value.IntValue, 10)
			}
		case spanAttributeKeyHostName:
			hostName = attributeValueAsString(v)
		case spanAttributeKeyHTTPUrl:
			httpURL = attributeValueAsString(v)
		case spanAttributeKeyHTTPStatusCode:
			var statusCode int

			switch value := v.Value.(type) {
			case *tracepb.AttributeValue_IntValue:
				data.ResponseCode = strconv.FormatInt(value.IntValue, 10)
				statusCode = int(value.IntValue)
			case *tracepb.AttributeValue_StringValue:
				data.ResponseCode = attributeValueAsString(v)
				statusCode, _ = strconv.Atoi(data.ResponseCode)
			}

			data.Success = statusCode <= 399
		case spanAttributeKeyHTTPClientIP:
			data.Source = attributeValueAsString(v)
		default:
			// Anything not on this list above ends up as a custom property
			setAttributeValueAsPropertyOrMeasurement(k, v, data.Properties, data.Measurements)
		}
	}

	var sb strings.Builder

	if httpScheme != "" && httpHost != "" && httpTarget != "" {
		sb.WriteString(httpMethod)
		sb.WriteString(" ")
		sb.WriteString(httpTarget)
		data.Name = sb.String()

		sb.Reset()
		sb.WriteString(httpScheme)
		sb.WriteString("://")
		sb.WriteString(httpHost)
		sb.WriteString(httpTarget)
		data.Url = sb.String()
	} else if httpScheme != "" && httpServerName != "" && hostPort != "" && httpTarget != "" {
		sb.WriteString(httpMethod)
		sb.WriteString(" ")
		sb.WriteString(httpTarget)
		data.Name = sb.String()

		sb.Reset()
		sb.WriteString(httpScheme)
		sb.WriteString("://")
		sb.WriteString(httpServerName)
		sb.WriteString(":")
		sb.WriteString(hostPort)
		sb.WriteString(httpTarget)
		data.Url = sb.String()
	} else if httpScheme != "" && hostName != "" && hostPort != "" && httpTarget != "" {
		sb.WriteString(httpMethod)
		sb.WriteString(" ")
		sb.WriteString(httpTarget)
		data.Name = sb.String()

		sb.Reset()
		sb.WriteString(httpScheme)
		sb.WriteString("://")
		sb.WriteString(hostName)
		sb.WriteString(":")
		sb.WriteString(hostPort)
		sb.WriteString(httpTarget)
		data.Url = sb.String()
	} else if httpURL != "" {
		if u, err := url.Parse(httpURL); err == nil {
			sb.WriteString(httpMethod)
			sb.WriteString(" ")
			sb.WriteString(u.Path)
			data.Name = sb.String()
			data.Url = httpURL
		}
	}
}

// Sets properties on an AppInsights RequestData from the wire format Span when the inbound Span type is unknown
func fillRequestDataInternal(span *tracepb.Span, data *contracts.RequestData) {
	// Everything is a custom attribute here
	for k, v := range span.Attributes.AttributeMap {
		setAttributeValueAsPropertyOrMeasurement(k, v, data.Properties, data.Measurements)
	}
}

// Sets properties on an AppInsights RequestData from the wire format Span when the inbound Span type is gRPC
func fillRequestDataGrpc(span *tracepb.Span, data *contracts.RequestData) {
	// https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/data-rpc.md
	for k, v := range span.Attributes.AttributeMap {
		switch k {
		case spanAttributeKeyComponent:
			// do nothing
		case spanAttributeKeyRPCStatusCode:
			data.ResponseCode = attributeValueAsString(v)
			data.Success = data.ResponseCode == string(codes.OK)
		default:
			// Anything not on this list above ends up as a custom property
			setAttributeValueAsPropertyOrMeasurement(k, v, data.Properties, data.Measurements)
		}
	}
}

// Transforms a wire format Span to AppInsights RemoteDependencyData
func spanToRemoteDependencyData(span *tracepb.Span) *contracts.RemoteDependencyData {
	/*
		What type of dependency? Determination comes from a few attributes.

		HTTP
		https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/data-http.md

		RPC (gRPC)
		https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/data-rpc.md

		Database
		https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/data-database.md
	*/

	// https://github.com/microsoft/ApplicationInsights-Go/blob/master/appinsights/contracts/remotedependencydata.go
	// Start with some reasonable default for dependent spans.
	data := contracts.NewRemoteDependencyData()
	data.Id = idToHex(span.SpanId)
	data.Name = span.Name.Value
	data.ResultCode = "0"
	data.Duration = formatSpanDuration(span)
	data.Success = true
	data.Properties = make(map[string]string)
	data.Measurements = make(map[string]float64)
	data.Type = "InProc"

	if span.Attributes != nil && span.Attributes.AttributeMap != nil {
		attributes := span.Attributes.AttributeMap
		component := ""
		onAttributeStringValueExists(attributes, spanAttributeKeyComponent, func(val string) { component = val })

		// TODO remove this once the OpenTelemetry wire format protocol is adopted.
		// The specs indicate that component is a required tag
		onAttributeStringValueExists(attributes, spanAttributeKeyHTTPMethod, func(val string) { component = "http" })

		switch component {
		case "":
			fillRemoteDependencyDataInternal(span, data)
		case "http":
			fillRemoteDependencyDataHTTP(span, data)
		case "grpc":
			fillRemoteDependencyDataGrpc(span, data)
		default:
			dbType := ""
			onAttributeStringValueExists(attributes, spanAttributeKeyDbType, func(val string) { dbType = val })
			if dbType != "" {
				fillRemoteDependencyDataDatabase(span, data)
			}
		}
	}

	return data
}

// Sets properties on an AppInsights RemoteDependencyData from the wire format Span when the outbound Span type is a database call
func fillRemoteDependencyDataDatabase(span *tracepb.Span, data *contracts.RemoteDependencyData) {
	for k, v := range span.Attributes.AttributeMap {

		switch k {
		case spanAttributeKeyDbType:
			data.Type = attributeValueAsString(v)
		case spanAttributeKeyDbStatement:
			data.Data = attributeValueAsString(v)
		case spanAttributeKeyPeerAddress:
			data.Target = attributeValueAsString(v)
		default:
			setAttributeValueAsPropertyOrMeasurement(k, v, data.Properties, data.Measurements)
		}
	}
}

// Sets properties on an AppInsights RemoteDependencyData from the wire format Span when the Span type is unknoown
func fillRemoteDependencyDataInternal(span *tracepb.Span, data *contracts.RemoteDependencyData) {
	data.Type = "InProc"

	// Everything is a custom attribute here
	for k, v := range span.Attributes.AttributeMap {
		setAttributeValueAsPropertyOrMeasurement(k, v, data.Properties, data.Measurements)
	}
}

// Sets properties on an AppInsights RemoteDependencyData from the wire format Span when the outbound Span type is HTTP
func fillRemoteDependencyDataHTTP(span *tracepb.Span, data *contracts.RemoteDependencyData) {
	data.Type = "Http"
	data.ResultCode = "200"

	/*
		The set of expected OpenTelemetry attribute sets for HTTP client span:
		https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/data-http.md#http-client

		Order of preference is:
			http.url
			http.scheme, http.host, http.target
			http.scheme, peer.hostname, peer.port, http.target
			http.scheme, peer.ip, peer.port, http.target
	*/
	httpMethod := ""
	httpURL := ""
	httpScheme := ""
	httpHost := ""
	httpTarget := ""
	peerHostName := ""
	peerPort := ""
	peerIP := ""

	for k, v := range span.Attributes.AttributeMap {
		switch k {
		case spanAttributeKeyComponent:
			// do nothing
		case spanAttributeKeyHTTPMethod:
			httpMethod = attributeValueAsString(v)
		case spanAttributeKeyHTTPUrl:
			httpURL = attributeValueAsString(v)
		case spanAttributeKeyHTTPScheme:
			httpScheme = attributeValueAsString(v)
		case spanAttributeKeyHTTPHost:
			httpHost = attributeValueAsString(v)
		case spanAttributeKeyHTTPTarget:
			httpTarget = prefixIfNecessary(attributeValueAsString(v), "/")
		case spanAttributeKeyPeerHostname:
			peerHostName = attributeValueAsString(v)
		case spanAttributeKeyPeerIP:
			peerIP = attributeValueAsString(v)
		case spanAttributeKeyPeerPort:
			peerPort = attributeValueAsString(v)
		case spanAttributeKeyHTTPStatusCode:
			var statusCode int

			switch value := v.Value.(type) {
			case *tracepb.AttributeValue_IntValue:
				data.ResultCode = strconv.FormatInt(value.IntValue, 10)
				statusCode = int(value.IntValue)
			case *tracepb.AttributeValue_StringValue:
				data.ResultCode = attributeValueAsString(v)
				statusCode, _ = strconv.Atoi(data.ResultCode)
			}

			data.Success = statusCode <= 399
		default:
			// Anything not on this list above ends up as a custom property
			setAttributeValueAsPropertyOrMeasurement(k, v, data.Properties, data.Measurements)
		}
	}

	var sb strings.Builder

	if httpURL != "" {
		if u, err := url.Parse(httpURL); err == nil {
			sb.WriteString(httpMethod)
			sb.WriteString(" ")
			sb.WriteString(u.Path)
			data.Name = sb.String()
			data.Data = httpURL
			data.Target = u.Host
		}
	} else if httpScheme != "" && httpHost != "" && httpTarget != "" {
		sb.WriteString(httpMethod)
		sb.WriteString(" ")
		sb.WriteString(httpTarget)
		data.Name = sb.String()

		sb.Reset()
		sb.WriteString(httpScheme)
		sb.WriteString("://")
		sb.WriteString(httpHost)
		sb.WriteString(httpTarget)
		data.Data = sb.String()
		data.Target = httpHost
	} else if httpScheme != "" && peerHostName != "" && peerPort != "" && httpTarget != "" {
		sb.WriteString(httpMethod)
		sb.WriteString(" ")
		sb.WriteString(httpTarget)
		data.Name = sb.String()

		sb.Reset()
		sb.WriteString(httpScheme)
		sb.WriteString("://")
		sb.WriteString(peerHostName)
		sb.WriteString(":")
		sb.WriteString(peerPort)
		sb.WriteString(httpTarget)
		data.Data = sb.String()

		sb.Reset()
		sb.WriteString(peerHostName)
		sb.WriteString(":")
		sb.WriteString(peerPort)
		data.Target = sb.String()
	} else if httpScheme != "" && peerIP != "" && peerPort != "" && httpTarget != "" {
		sb.WriteString(httpMethod)
		sb.WriteString(" ")
		sb.WriteString(httpTarget)
		data.Name = sb.String()

		sb.Reset()
		sb.WriteString(httpScheme)
		sb.WriteString("://")
		sb.WriteString(peerIP)
		sb.WriteString(":")
		sb.WriteString(peerPort)
		sb.WriteString(httpTarget)
		data.Data = sb.String()

		sb.Reset()
		sb.WriteString(peerIP)
		sb.WriteString(":")
		sb.WriteString(peerPort)
		data.Target = sb.String()
	}
}

// Sets properties on an AppInsights RemoteDependencyData from the wire format Span when the outbound Span type is gRPC
func fillRemoteDependencyDataGrpc(span *tracepb.Span, data *contracts.RemoteDependencyData) {
	data.Type = "Grpc"

	// https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/data-rpc.md
	peerService := ""
	peerHostName := ""
	peerPort := ""

	for k, v := range span.Attributes.AttributeMap {
		switch k {
		case spanAttributeKeyComponent:
			// do nothing
		case spanAttributeKeyRPCStatusCode:
			data.ResultCode = attributeValueAsString(v)
			code, err := strconv.Atoi(data.ResultCode)
			if err == nil {
				data.Success = code == int(codes.OK)
			}
		case spanAttributeKeyPeerService:
			peerService = attributeValueAsString(v)
		case spanAttributeKeyPeerHostname:
			peerHostName = attributeValueAsString(v)
		case spanAttributeKeyPeerPort:
			peerPort = attributeValueAsString(v)
		default:
			// Anything not on this list above ends up as a custom property
			setAttributeValueAsPropertyOrMeasurement(k, v, data.Properties, data.Measurements)
		}
	}

	var sb strings.Builder
	if peerService != "" && peerHostName != "" && peerPort != "" {
		sb.WriteString(peerHostName)
		sb.WriteString(":")
		sb.WriteString(peerPort)
		data.Target = sb.String()

		// append the service name to the host:port
		sb.WriteString(peerService)
		data.Data = sb.String()
	}
}

func prefixIfNecessary(s string, prefix string) string {
	if strings.HasPrefix(s, prefix) {
		return s
	}

	return prefix + s
}

func (exporter *traceExporter) sanitize(sanitizeFunc func() []string) {
	sanitizeWithCallback(sanitizeFunc, nil, exporter.logger)
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

// populateResourceAttributes populates resource attributes to telemetry envelope.
func (exporter *traceExporter) populateResourceAttributes(
	traceData consumerdata.TraceData,
	envelope *contracts.Envelope,
) {
	// Old trace exporter populates trace resource attributes to Node and Resource Labels.
	// https://github.com/open-telemetry/opentelemetry-collector/blob/master/translator/internaldata/resource_to_oc.go#L54
	var properties map[string]string
	if data, ok := envelope.Data.(*contracts.Data); ok {
		switch d := data.BaseData.(type) {
		case *contracts.RemoteDependencyData:
			properties = d.Properties
		case *contracts.RequestData:
			properties = d.Properties
		}
	}

	// Extract service.namespace and populate the other resource attributes to properties
	cloudRolePrefix := ""
	if traceData.Resource != nil && traceData.Resource.Labels != nil && len(traceData.Resource.Labels) > 0 {
		for k, v := range traceData.Resource.Labels {
			switch k {
			case conventions.AttributeServiceNamespace:
				cloudRolePrefix = v + "."
			default:
				if properties != nil {
					properties[k] = v
				}
			}
		}
	}

	if traceData.Node != nil {
		// ai.cloud.role is the name of role which represents current service name
		if traceData.Node.GetServiceInfo() != nil {
			envelope.Tags[contracts.CloudRole] = cloudRolePrefix + traceData.Node.ServiceInfo.GetName()
		}
		// ai.cloud.roleinstance is the name of the instance where service is running
		if traceData.Node.GetIdentifier() != nil {
			envelope.Tags[contracts.CloudRoleInstance] = traceData.Node.Identifier.GetHostName()
		}
	}
}

func (exporter *traceExporter) pushTraceData(
	context context.Context,
	traceData consumerdata.TraceData,
) (droppedSpans int, err error) {

	spanCount := len(traceData.Spans)
	if spanCount == 0 {
		return 0, nil
	}

	for _, wireFormatSpan := range traceData.Spans {
		if envelope, err := exporter.spanToEnvelope(exporter.config.InstrumentationKey, wireFormatSpan); err == nil && exporter.transportChannel != nil {
			// The resource attributes needs to be given to populate ai.cloud.role and ai.cloud.roleinstance
			// when you create new exporter.
			//
			// - service.name
			// - service.namespace
			// - host.hostname
			//
			// OTLP Exporter example:
			//   ...
			//   exp, _ := otlp.NewExporter(otlp.WithInsecure(), otlp.WithAddress("localhost:9090"))
			//   tp, _ := sdktrace.NewProvider(
			//   	sdktrace.WithSyncer(exp),
			//   	sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
			//   	sdktrace.WithResourceAttributes(
			//   		key.String(resourcekeys.ServiceKeyName, "your_service"),
			//   		key.String(resourcekeys.ServiceKeyNamespace, "namespace"),
			//   		key.String(resourcekeys.HostKeyHostName, "hostname"),
			//   	),
			//   )
			//   global.SetTraceProvider(tp)
			//
			exporter.populateResourceAttributes(traceData, envelope)

			// This is a fire and forget operation
			exporter.transportChannel.Send(envelope)
		} else {
			// Only tracks the inability to transform a wire format Span to an AppInsights envelope
			droppedSpans++
		}
	}

	return droppedSpans, nil
}

// Returns a new instance of the trace exporter
func newTraceExporter(config *Config, transportChannel transportChannel, logger *zap.Logger) (component.TraceExporterOld, error) {

	exporter := &traceExporter{
		config:           config,
		transportChannel: transportChannel,
		logger:           logger,
	}

	exp, err := exporterhelper.NewTraceExporterOld(
		config,
		exporter.pushTraceData)

	return exp, err
}
