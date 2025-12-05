// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"

import (
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.34.0"
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

// clientAttributes is the set of known client attributes
type clientAttributes struct {
	// see https://github.com/open-telemetry/semantic-conventions/blob/v1.34.0/docs/registry/attributes/client.md
	ClientAddress string
	ClientPort    int64
}

// serverAttributes is the set of known server attributes
type serverAttributes struct {
	// see https://github.com/open-telemetry/semantic-conventions/blob/v1.34.0/docs/registry/attributes/server.md
	ServerAddress string
	ServerPort    int64
}

// networkAttributes is the set of known network attributes
type networkAttributes struct {
	// see https://github.com/open-telemetry/semantic-conventions/blob/v1.34.0/docs/registry/attributes/network.md
	NetworkLocalAddress    string
	NetworkLocalPort       int64
	NetworkPeerAddress     string
	NetworkPeerPort        int64
	NetworkProtocolName    string
	NetworkProtocolVersion string
	NetworkTransport       string
	NetworkType            string
}

// MapAttribute attempts to map a Span attribute to one of the known types
func (attrs *networkAttributes) MapAttribute(k string, v pcommon.Value) bool {
	switch k {
	case string(conventions.NetworkLocalAddressKey):
		attrs.NetworkLocalAddress = v.Str()
	case string(conventions.NetworkLocalPortKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.NetworkLocalPort = val
		}
	case string(conventions.NetworkPeerAddressKey):
		attrs.NetworkPeerAddress = v.Str()
	case string(conventions.NetworkPeerPortKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.NetworkPeerPort = val
		}
	case string(conventions.NetworkProtocolNameKey):
		attrs.NetworkProtocolName = v.Str()
	case string(conventions.NetworkProtocolVersionKey):
		attrs.NetworkProtocolVersion = v.Str()
	case string(conventions.NetworkTransportKey):
		attrs.NetworkTransport = v.Str()
	case string(conventions.NetworkTypeKey):
	}
	return true
}

// urlAttributes is the set of known attributes for URL
type urlAttributes struct {
	// common attributes
	// https://github.com/open-telemetry/semantic-conventions/blob/v1.34.0/docs/registry/attributes/url.md
	URLFragment string
	URLFull     string
	URLPath     string
	URLQuery    string
	URLScheme   string
}

type userAgentAttributes struct {
	UserAgentOriginal string
	UserAgentName     string
	UserAgentVersion  string
}

// httpAttributes is the set of known attributes for HTTP Spans
type httpAttributes struct {
	// common attributes
	// https://github.com/open-telemetry/semantic-conventions/blob/v1.34.0/docs/registry/attributes/http.md
	HTTPRequestHeaders        map[string][]string
	HTTPRequestMethod         string
	HTTPRequestMethodOriginal string
	HTTPRequestResendCount    int64
	HTTPResponseHeaders       map[string][]string
	HTTPResponseStatusCode    int64
	HTTPRoute                 string

	HTTPRequestBodySize  int64
	HTTPResponseBodySize int64

	URLAttributes       urlAttributes
	ClientAttributes    clientAttributes
	ServerAttributes    serverAttributes
	UserAgentAttributes userAgentAttributes

	// any net.*
	NetworkAttributes networkAttributes
}

func setHeader(headers *map[string][]string, key string, v pcommon.Value) {
	if *headers == nil {
		*headers = make(map[string][]string)
	}
	switch v.Type() {
	case pcommon.ValueTypeSlice:
		slice := v.Slice()
		var vals []string
		for i := 0; i < slice.Len(); i++ {
			vals = append(vals, slice.At(i).Str())
		}
		(*headers)[key] = vals
	case pcommon.ValueTypeStr:
		(*headers)[key] = []string{v.Str()}
	}
}

// MapAttribute attempts to map a Span attribute to one of the known types
func (attrs *httpAttributes) MapAttribute(k string, v pcommon.Value) bool {
	const reqHeaderPrefix = "http.request.header."
	const respHeaderPrefix = "http.response.header."

	switch {
	case strings.HasPrefix(k, reqHeaderPrefix):
		headerName := k[len(reqHeaderPrefix):]
		setHeader(&attrs.HTTPRequestHeaders, headerName, v)
		return true

	case strings.HasPrefix(k, respHeaderPrefix):
		headerName := k[len(respHeaderPrefix):]
		setHeader(&attrs.HTTPResponseHeaders, headerName, v)
		return true
	}

	switch k {
	case string(conventions.HTTPRequestMethodKey):
		attrs.HTTPRequestMethod = v.Str()
	case string(conventions.HTTPRequestMethodOriginalKey):
		attrs.HTTPRequestMethodOriginal = v.Str()
	case string(conventions.HTTPRequestResendCountKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.HTTPRequestResendCount = val
		}
	case string(conventions.HTTPResponseStatusCodeKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.HTTPResponseStatusCode = val
		}
	case string(conventions.HTTPRouteKey):
		attrs.HTTPRoute = v.Str()

	case string(conventions.HTTPResponseBodySizeKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.HTTPResponseBodySize = val
		}
	case string(conventions.HTTPRequestBodySizeKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.HTTPRequestBodySize = val
		}
	// URL attributes
	case string(conventions.URLFragmentKey):
		attrs.URLAttributes.URLFragment = v.Str()
	case string(conventions.URLFullKey):
		attrs.URLAttributes.URLFull = v.Str()
	case string(conventions.URLPathKey):
		attrs.URLAttributes.URLPath = v.Str()
	case string(conventions.URLQueryKey):
		attrs.URLAttributes.URLQuery = v.Str()
	case string(conventions.URLSchemeKey):
		attrs.URLAttributes.URLScheme = v.Str()

	// Network/server/client address attributes (new, replacing http.host)
	case string(conventions.ServerAddressKey):
		attrs.ServerAttributes.ServerAddress = v.Str()
	case string(conventions.ServerPortKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.ServerAttributes.ServerPort = val
		}
	case string(conventions.ClientAddressKey):
		attrs.ClientAttributes.ClientAddress = v.Str()
	case string(conventions.ClientPortKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.ClientAttributes.ClientPort = val
		}
	// User agent attributes
	case string(conventions.UserAgentOriginalKey):
		attrs.UserAgentAttributes.UserAgentOriginal = v.Str()
	case string(conventions.UserAgentNameKey):
		attrs.UserAgentAttributes.UserAgentName = v.Str()
	case string(conventions.UserAgentVersionKey):
		attrs.UserAgentAttributes.UserAgentVersion = v.Str()

	default:
		attrs.NetworkAttributes.MapAttribute(k, v)
	}

	return true
}

// rpcAttributes is the set of known attributes for RPC Spans
type rpcAttributes struct {
	RPCSystem         string
	RPCService        string
	RPCMethod         string
	RPCGRPCStatusCode int64

	ClientAttributes  clientAttributes
	ServerAttributes  serverAttributes
	NetworkAttributes networkAttributes
}

// MapAttribute attempts to map a Span attribute to one of the known types
func (attrs *rpcAttributes) MapAttribute(k string, v pcommon.Value) bool {
	switch k {
	case string(conventions.RPCSystemKey):
		attrs.RPCSystem = v.Str()
	case string(conventions.RPCServiceKey):
		attrs.RPCService = v.Str()
	case string(conventions.RPCMethodKey):
		attrs.RPCMethod = v.Str()
	case string(conventions.RPCGRPCStatusCodeKey):
		attrs.RPCGRPCStatusCode = v.Int()

	case string(conventions.ServerAddressKey):
		attrs.ServerAttributes.ServerAddress = v.Str()
	case string(conventions.ServerPortKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.ServerAttributes.ServerPort = val
		}
	case string(conventions.ClientAddressKey):
		attrs.ClientAttributes.ClientAddress = v.Str()
	case string(conventions.ClientPortKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.ClientAttributes.ClientPort = val
		}

	default:
		attrs.NetworkAttributes.MapAttribute(k, v)
	}
	return true
}

// databaseAttributes is the set of known attributes for Database Spans
type databaseAttributes struct {
	DBCollectionName      string
	DBNamespace           string
	DBOperationBatchSize  int64
	DBOperationName       string
	DBQuerySummary        string
	DBQueryText           string
	DBResponseStatusCode  string
	DBStoredProcedureName string
	DBSystemName          string

	ClientAttributes clientAttributes
	ServerAttributes serverAttributes

	NetworkAttributes networkAttributes
}

// MapAttribute attempts to map a Span attribute to one of the known types
func (attrs *databaseAttributes) MapAttribute(k string, v pcommon.Value) bool {
	switch k {
	case string(conventions.DBCollectionNameKey):
		attrs.DBCollectionName = v.Str()
	case string(conventions.DBNamespaceKey):
		attrs.DBNamespace = v.Str()
	case string(conventions.DBOperationBatchSizeKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.DBOperationBatchSize = val
		}
	case string(conventions.DBOperationNameKey):
		attrs.DBOperationName = v.Str()
	case string(conventions.DBQuerySummaryKey):
		attrs.DBQuerySummary = v.Str()
	case string(conventions.DBQueryTextKey):
		attrs.DBQueryText = v.Str()
	case string(conventions.DBResponseStatusCodeKey):
		attrs.DBResponseStatusCode = v.Str()
	case string(conventions.DBStoredProcedureNameKey):
		attrs.DBStoredProcedureName = v.Str()
	case string(conventions.DBSystemNameKey):
		attrs.DBSystemName = v.Str()

	case string(conventions.ServerAddressKey):
		attrs.ServerAttributes.ServerAddress = v.Str()
	case string(conventions.ServerPortKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.ServerAttributes.ServerPort = val
		}
	case string(conventions.ClientAddressKey):
		attrs.ClientAttributes.ClientAddress = v.Str()
	case string(conventions.ClientPortKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.ClientAttributes.ClientPort = val
		}

	default:
		attrs.NetworkAttributes.MapAttribute(k, v)
	}
	return true
}

// messagingAttributes is the set of known attributes for Messaging Spans
type messagingAttributes struct {
	MessagingBatchMessageCount      int64
	MessagingClientID               string
	MessagingConsumerGroup          string
	MessagingDestinationAnonymous   bool
	MessagingDestination            string
	MessagingDestinationPartitionID string
	MessagingDestinationSubName     string
	MessagingDestinationTemplate    string
	MessagingDestinationTemporary   bool
	MessagingMessageBodySize        int64
	MessagingMessageConversationID  string
	MessagingMessageEnvelopeSize    int64
	MessagingMessageID              string
	MessagingOperation              string
	MessagingOperationType          string
	MessagingSystem                 string

	ClientAttributes clientAttributes
	ServerAttributes serverAttributes

	NetworkAttributes networkAttributes
}

// MapAttribute attempts to map a Span attribute to one of the known types
func (attrs *messagingAttributes) MapAttribute(k string, v pcommon.Value) bool {
	switch k {
	case string(conventions.MessagingBatchMessageCountKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.MessagingBatchMessageCount = val
		}
	case string(conventions.MessagingClientIDKey):
		attrs.MessagingClientID = v.Str()
	case string(conventions.MessagingConsumerGroupNameKey):
		attrs.MessagingConsumerGroup = v.Str()
	case string(conventions.MessagingDestinationAnonymousKey):
		attrs.MessagingDestinationAnonymous = v.Bool()
	case string(conventions.MessagingDestinationNameKey):
		attrs.MessagingDestination = v.Str()
	case string(conventions.MessagingDestinationPartitionIDKey):
		attrs.MessagingDestinationPartitionID = v.Str()
	case string(conventions.MessagingDestinationSubscriptionNameKey):
		attrs.MessagingDestinationSubName = v.Str()
	case string(conventions.MessagingDestinationTemplateKey):
		attrs.MessagingDestinationTemplate = v.Str()
	case string(conventions.MessagingDestinationTemporaryKey):
		attrs.MessagingDestinationTemporary = v.Bool()
	case string(conventions.MessagingMessageBodySizeKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.MessagingMessageBodySize = val
		}
	case string(conventions.MessagingMessageConversationIDKey):
		attrs.MessagingMessageConversationID = v.Str()
	case string(conventions.MessagingMessageEnvelopeSizeKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.MessagingMessageEnvelopeSize = val
		}
	case string(conventions.MessagingMessageIDKey):
		attrs.MessagingMessageID = v.Str()
	case string(conventions.MessagingOperationNameKey):
		attrs.MessagingOperation = v.Str()
	case string(conventions.MessagingOperationTypeKey):
		attrs.MessagingOperationType = v.Str()
	case string(conventions.MessagingSystemKey):
		attrs.MessagingSystem = v.Str()

	case string(conventions.ServerAddressKey):
		attrs.ServerAttributes.ServerAddress = v.Str()
	case string(conventions.ServerPortKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.ServerAttributes.ServerPort = val
		}
	case string(conventions.ClientAddressKey):
		attrs.ClientAttributes.ClientAddress = v.Str()
	case string(conventions.ClientPortKey):
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.ClientAttributes.ClientPort = val
		}

	default:
		attrs.NetworkAttributes.MapAttribute(k, v)
	}
	return true
}

// exceptionAttributes is the set of known attributes for Exception events
type exceptionAttributes struct {
	ExceptionMessage    string
	ExceptionStackTrace string
	ExceptionType       string
}

// MapAttribute attempts to map a SpanEvent attribute to one of the known types
func (attrs *exceptionAttributes) MapAttribute(k string, v pcommon.Value) bool {
	switch k {
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
