// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"

import (
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.7.0"
)

/*
	This file encapsulates the extraction logic for the various kinds attributes
*/

const (
	// TODO replace with convention.* values once/if available
	attributeOtelStatusCode                          string = "otel.status_code"
	attributeOtelStatusDescription                   string = "otel.status_description"
	attributeMicrosoftCustomEventName                string = "microsoft.custom_event.name"
	attributeApplicationInsightsEventMarkerAttribute string = "APPLICATION_INSIGHTS_EVENT_MARKER_ATTRIBUTE"
)

// NetworkAttributes is the set of known network attributes
type NetworkAttributes struct {
	// see https://github.com/open-telemetry/semantic-conventions/blob/main/docs/attributes-registry/network.md#network-attributes
	NetTransport string
	NetPeerIP    string
	NetPeerPort  int64
	NetPeerName  string
	NetHostIP    string
	NetHostPort  int64
	NetHostName  string
}

// MapAttribute attempts to map a Span attribute to one of the known types
func (attrs *NetworkAttributes) MapAttribute(k string, v pcommon.Value) bool {
	switch k {
	case conventions.AttributeNetTransport:
		attrs.NetTransport = v.Str()
	case conventions.AttributeNetPeerIP:
		attrs.NetPeerIP = v.Str()
	case conventions.AttributeNetPeerPort:
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.NetPeerPort = val
		}
	case conventions.AttributeNetPeerName:
		attrs.NetPeerName = v.Str()
	case conventions.AttributeNetHostIP:
		attrs.NetHostIP = v.Str()
	case conventions.AttributeNetHostPort:
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.NetHostPort = val
		}
	case conventions.AttributeNetHostName:
		attrs.NetHostName = v.Str()
	}
	return true
}

// HTTPAttributes is the set of known attributes for HTTP Spans
type HTTPAttributes struct {
	// common attributes
	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/http.md#common-attributes
	HTTPMethod                            string
	HTTPURL                               string
	HTTPTarget                            string
	HTTPHost                              string
	HTTPScheme                            string
	HTTPStatusCode                        int64
	HTTPStatusText                        string
	HTTPFlavor                            string
	HTTPUserAgent                         string
	HTTPRequestContentLength              int64
	HTTPRequestContentLengthUncompressed  int64
	HTTPResponseContentLength             int64
	HTTPResponseContentLengthUncompressed int64

	// Server Span specific
	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/http.md#http-server-semantic-conventions
	HTTPRoute      string
	HTTPServerName string
	HTTPClientIP   string

	// any net.*
	NetworkAttributes NetworkAttributes
}

// MapAttribute attempts to map a Span attribute to one of the known types
func (attrs *HTTPAttributes) MapAttribute(k string, v pcommon.Value) bool {
	switch k {
	case conventions.AttributeHTTPMethod:
		attrs.HTTPMethod = v.Str()
	case conventions.AttributeHTTPURL:
		attrs.HTTPURL = v.Str()
	case conventions.AttributeHTTPTarget:
		attrs.HTTPTarget = v.Str()
	case conventions.AttributeHTTPHost:
		attrs.HTTPHost = v.Str()
	case conventions.AttributeHTTPScheme:
		attrs.HTTPScheme = v.Str()
	case conventions.AttributeHTTPStatusCode:
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.HTTPStatusCode = val
		}
	case "http.status_text":
		attrs.HTTPStatusText = v.Str()
	case conventions.AttributeHTTPFlavor:
		attrs.HTTPFlavor = v.Str()
	case conventions.AttributeHTTPUserAgent:
		attrs.HTTPUserAgent = v.Str()
	case conventions.AttributeHTTPRequestContentLength:
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.HTTPRequestContentLength = val
		}
	case conventions.AttributeHTTPRequestContentLengthUncompressed:
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.HTTPRequestContentLengthUncompressed = val
		}
	case conventions.AttributeHTTPResponseContentLength:
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.HTTPResponseContentLength = val
		}
	case conventions.AttributeHTTPResponseContentLengthUncompressed:
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.HTTPResponseContentLengthUncompressed = val
		}

	case conventions.AttributeHTTPRoute:
		attrs.HTTPRoute = v.Str()
	case conventions.AttributeHTTPServerName:
		attrs.HTTPServerName = v.Str()
	case conventions.AttributeHTTPClientIP:
		attrs.HTTPClientIP = v.Str()

	default:
		attrs.NetworkAttributes.MapAttribute(k, v)
	}

	return true
}

// RPCAttributes is the set of known attributes for RPC Spans
type RPCAttributes struct {
	RPCSystem         string
	RPCService        string
	RPCMethod         string
	RPCGRPCStatusCode int64
	NetworkAttributes NetworkAttributes
}

// MapAttribute attempts to map a Span attribute to one of the known types
func (attrs *RPCAttributes) MapAttribute(k string, v pcommon.Value) bool {
	switch k {
	case conventions.AttributeRPCSystem:
		attrs.RPCSystem = v.Str()
	case conventions.AttributeRPCService:
		attrs.RPCService = v.Str()
	case conventions.AttributeRPCMethod:
		attrs.RPCMethod = v.Str()
	case conventions.AttributeRPCGRPCStatusCode:
		attrs.RPCGRPCStatusCode = v.Int()

	default:
		attrs.NetworkAttributes.MapAttribute(k, v)
	}
	return true
}

// DatabaseAttributes is the set of known attributes for Database Spans
type DatabaseAttributes struct {
	DBSystem              string
	DBConnectionString    string
	DBUser                string
	DBName                string
	DBStatement           string
	DBOperation           string
	DBMSSQLInstanceName   string
	DBJDBCDriverClassName string
	DBCassandraKeyspace   string
	DBHBaseNamespace      string
	DBRedisDatabaseIndex  string
	DBMongoDBCollection   string
	NetworkAttributes     NetworkAttributes
}

// MapAttribute attempts to map a Span attribute to one of the known types
func (attrs *DatabaseAttributes) MapAttribute(k string, v pcommon.Value) bool {
	switch k {
	case conventions.AttributeDBSystem:
		attrs.DBSystem = v.Str()
	case conventions.AttributeDBConnectionString:
		attrs.DBConnectionString = v.Str()
	case conventions.AttributeDBUser:
		attrs.DBUser = v.Str()
	case conventions.AttributeDBStatement:
		attrs.DBStatement = v.Str()
	case conventions.AttributeDBOperation:
		attrs.DBOperation = v.Str()
	case conventions.AttributeDBMSSQLInstanceName:
		attrs.DBMSSQLInstanceName = v.Str()
	case conventions.AttributeDBJDBCDriverClassname:
		attrs.DBJDBCDriverClassName = v.Str()
	case conventions.AttributeDBCassandraKeyspace:
		attrs.DBCassandraKeyspace = v.Str()
	case conventions.AttributeDBHBaseNamespace:
		attrs.DBHBaseNamespace = v.Str()
	case conventions.AttributeDBRedisDBIndex:
		attrs.DBRedisDatabaseIndex = v.Str()
	case conventions.AttributeDBMongoDBCollection:
		attrs.DBMongoDBCollection = v.Str()

	default:
		attrs.NetworkAttributes.MapAttribute(k, v)
	}
	return true
}

// MessagingAttributes is the set of known attributes for Messaging Spans
type MessagingAttributes struct {
	MessagingSystem                       string
	MessagingDestination                  string
	MessagingDestinationKind              string
	MessagingTempDestination              string
	MessagingProtocol                     string
	MessagingProtocolVersion              string
	MessagingURL                          string
	MessagingMessageID                    string
	MessagingConversationID               string
	MessagingMessagePayloadSize           int64
	MessagingMessagePayloadCompressedSize int64
	MessagingOperation                    string
	NetworkAttributes                     NetworkAttributes
}

// MapAttribute attempts to map a Span attribute to one of the known types
func (attrs *MessagingAttributes) MapAttribute(k string, v pcommon.Value) bool {
	switch k {
	case conventions.AttributeMessagingSystem:
		attrs.MessagingSystem = v.Str()
	case conventions.AttributeMessagingDestination:
		attrs.MessagingDestination = v.Str()
	case conventions.AttributeMessagingDestinationKind:
		attrs.MessagingDestinationKind = v.Str()
	case conventions.AttributeMessagingTempDestination:
		attrs.MessagingTempDestination = v.Str()
	case conventions.AttributeMessagingProtocol:
		attrs.MessagingProtocol = v.Str()
	case conventions.AttributeMessagingProtocolVersion:
		attrs.MessagingProtocolVersion = v.Str()
	case conventions.AttributeMessagingURL:
		attrs.MessagingURL = v.Str()
	case conventions.AttributeMessagingMessageID:
		attrs.MessagingMessageID = v.Str()
	case conventions.AttributeMessagingConversationID:
		attrs.MessagingConversationID = v.Str()
	case conventions.AttributeMessagingMessagePayloadSizeBytes:
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.MessagingMessagePayloadSize = val
		}
	case conventions.AttributeMessagingMessagePayloadCompressedSizeBytes:
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.MessagingMessagePayloadCompressedSize = val
		}
	case conventions.AttributeMessagingOperation:
		attrs.MessagingOperation = v.Str()

	default:
		attrs.NetworkAttributes.MapAttribute(k, v)
	}
	return true
}

// ExceptionAttributes is the set of known attributes for Exception events
type ExceptionAttributes struct {
	ExceptionEscaped    string
	ExceptionMessage    string
	ExceptionStackTrace string
	ExceptionType       string
}

// MapAttribute attempts to map a SpanEvent attribute to one of the known types
func (attrs *ExceptionAttributes) MapAttribute(k string, v pcommon.Value) bool {
	switch k {
	case conventions.AttributeExceptionEscaped:
		attrs.ExceptionEscaped = v.Str()
	case conventions.AttributeExceptionMessage:
		attrs.ExceptionMessage = v.Str()
	case conventions.AttributeExceptionStacktrace:
		attrs.ExceptionStackTrace = v.Str()
	case conventions.AttributeExceptionType:
		attrs.ExceptionType = v.Str()
	}
	return true
}

// Tries to return the value of the attribute as an int64
func getAttributeValueAsInt(attributeValue pcommon.Value) (int64, error) {
	switch attributeValue.Type() {
	case pcommon.ValueTypeStr:
		// try to cast the string values to int64
		if val, err := strconv.Atoi(attributeValue.Str()); err == nil {
			return int64(val), nil
		}
	case pcommon.ValueTypeInt:
		return attributeValue.Int(), nil
	}

	return int64(0), errUnexpectedAttributeValueType
}
