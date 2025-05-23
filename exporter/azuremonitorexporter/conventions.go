// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"

import (
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.7.0"
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
	case string(conventions.NetTransportKey):
		attrs.NetTransport = v.Str()
	case string(conventions.NetPeerIPKey):
		attrs.NetPeerIP = v.Str()
	case string(conventions.NetPeerPortKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.NetPeerPort = val
		}
	case string(conventions.NetPeerNameKey):
		attrs.NetPeerName = v.Str()
	case string(conventions.NetHostIPKey):
		attrs.NetHostIP = v.Str()
	case string(conventions.NetHostPortKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.NetHostPort = val
		}
	case string(conventions.NetHostNameKey):
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
	case string(conventions.HTTPMethodKey):
		attrs.HTTPMethod = v.Str()
	case string(conventions.HTTPURLKey):
		attrs.HTTPURL = v.Str()
	case string(conventions.HTTPTargetKey):
		attrs.HTTPTarget = v.Str()
	case string(conventions.HTTPHostKey):
		attrs.HTTPHost = v.Str()
	case string(conventions.HTTPSchemeKey):
		attrs.HTTPScheme = v.Str()
	case string(conventions.HTTPStatusCodeKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.HTTPStatusCode = val
		}
	case "http.status_text":
		attrs.HTTPStatusText = v.Str()
	case string(conventions.HTTPFlavorKey):
		attrs.HTTPFlavor = v.Str()
	case string(conventions.HTTPUserAgentKey):
		attrs.HTTPUserAgent = v.Str()
	case string(conventions.HTTPRequestContentLengthKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.HTTPRequestContentLength = val
		}
	case string(conventions.HTTPRequestContentLengthUncompressedKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.HTTPRequestContentLengthUncompressed = val
		}
	case string(conventions.HTTPResponseContentLengthKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.HTTPResponseContentLength = val
		}
	case string(conventions.HTTPResponseContentLengthUncompressedKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.HTTPResponseContentLengthUncompressed = val
		}

	case string(conventions.HTTPRouteKey):
		attrs.HTTPRoute = v.Str()
	case string(conventions.HTTPServerNameKey):
		attrs.HTTPServerName = v.Str()
	case string(conventions.HTTPClientIPKey):
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
	case string(conventions.RPCSystemKey):
		attrs.RPCSystem = v.Str()
	case string(conventions.RPCServiceKey):
		attrs.RPCService = v.Str()
	case string(conventions.RPCMethodKey):
		attrs.RPCMethod = v.Str()
	case string(conventions.RPCGRPCStatusCodeKey):
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
	case string(conventions.DBSystemKey):
		attrs.DBSystem = v.Str()
	case string(conventions.DBConnectionStringKey):
		attrs.DBConnectionString = v.Str()
	case string(conventions.DBUserKey):
		attrs.DBUser = v.Str()
	case string(conventions.DBStatementKey):
		attrs.DBStatement = v.Str()
	case string(conventions.DBOperationKey):
		attrs.DBOperation = v.Str()
	case string(conventions.DBMSSQLInstanceNameKey):
		attrs.DBMSSQLInstanceName = v.Str()
	case string(conventions.DBJDBCDriverClassnameKey):
		attrs.DBJDBCDriverClassName = v.Str()
	case string(conventions.DBCassandraKeyspaceKey):
		attrs.DBCassandraKeyspace = v.Str()
	case string(conventions.DBHBaseNamespaceKey):
		attrs.DBHBaseNamespace = v.Str()
	case string(conventions.DBRedisDBIndexKey):
		attrs.DBRedisDatabaseIndex = v.Str()
	case string(conventions.DBMongoDBCollectionKey):
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
	case string(conventions.MessagingSystemKey):
		attrs.MessagingSystem = v.Str()
	case string(conventions.MessagingDestinationKey):
		attrs.MessagingDestination = v.Str()
	case string(conventions.MessagingDestinationKindKey):
		attrs.MessagingDestinationKind = v.Str()
	case string(conventions.MessagingTempDestinationKey):
		attrs.MessagingTempDestination = v.Str()
	case string(conventions.MessagingProtocolKey):
		attrs.MessagingProtocol = v.Str()
	case string(conventions.MessagingProtocolVersionKey):
		attrs.MessagingProtocolVersion = v.Str()
	case string(conventions.MessagingURLKey):
		attrs.MessagingURL = v.Str()
	case string(conventions.MessagingMessageIDKey):
		attrs.MessagingMessageID = v.Str()
	case string(conventions.MessagingConversationIDKey):
		attrs.MessagingConversationID = v.Str()
	case string(conventions.MessagingMessagePayloadSizeBytesKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.MessagingMessagePayloadSize = val
		}
	case string(conventions.MessagingMessagePayloadCompressedSizeBytesKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.MessagingMessagePayloadCompressedSize = val
		}
	case string(conventions.MessagingOperationKey):
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
	case string(conventions.ExceptionEscapedKey):
		attrs.ExceptionEscaped = v.Str()
	case string(conventions.ExceptionMessageKey):
		attrs.ExceptionMessage = v.Str()
	case string(conventions.ExceptionStacktraceKey):
		attrs.ExceptionStackTrace = v.Str()
	case string(conventions.ExceptionTypeKey):
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
