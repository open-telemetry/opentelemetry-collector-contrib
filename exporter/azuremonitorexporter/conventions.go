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
	"strconv"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
)

/*
	This file encapsulates the extraction logic for the various kinds attributes
*/

const (
	// TODO replace with convention.* values once/if available
	attributeRPCGRPCStatusCode     string = "rpc.grpc.status_code"
	attributeOtelStatusCode        string = "otel.status_code"
	attributeOtelStatusDescription string = "otel.status_description"
)

// NetworkAttributes is the set of known network attributes
type NetworkAttributes struct {
	// see https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/span-general.md#general-network-connection-attributes
	NetTransport string
	NetPeerIP    string
	NetPeerPort  int64
	NetPeerName  string
	NetHostIP    string
	NetHostPort  int64
	NetHostName  string
}

// MapAttribute attempts to map a Span attribute to one of the known types
func (attrs *NetworkAttributes) MapAttribute(k string, v pdata.AttributeValue) {
	switch k {
	case conventions.AttributeNetTransport:
		attrs.NetTransport = v.StringVal()
	case conventions.AttributeNetPeerIP:
		attrs.NetPeerIP = v.StringVal()
	case conventions.AttributeNetPeerPort:
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.NetPeerPort = val
		}
	case conventions.AttributeNetPeerName:
		attrs.NetPeerName = v.StringVal()
	case conventions.AttributeNetHostIP:
		attrs.NetHostIP = v.StringVal()
	case conventions.AttributeNetHostPort:
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.NetHostPort = val
		}
	case conventions.AttributeNetHostName:
		attrs.NetHostName = v.StringVal()
	}
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
func (attrs *HTTPAttributes) MapAttribute(k string, v pdata.AttributeValue) {
	switch k {
	case conventions.AttributeHTTPMethod:
		attrs.HTTPMethod = v.StringVal()
	case conventions.AttributeHTTPURL:
		attrs.HTTPURL = v.StringVal()
	case conventions.AttributeHTTPTarget:
		attrs.HTTPTarget = v.StringVal()
	case conventions.AttributeHTTPHost:
		attrs.HTTPHost = v.StringVal()
	case conventions.AttributeHTTPScheme:
		attrs.HTTPScheme = v.StringVal()
	case conventions.AttributeHTTPStatusCode:
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.HTTPStatusCode = val
		}
	case conventions.AttributeHTTPStatusText:
		attrs.HTTPStatusText = v.StringVal()
	case conventions.AttributeHTTPFlavor:
		attrs.HTTPFlavor = v.StringVal()
	case conventions.AttributeHTTPUserAgent:
		attrs.HTTPUserAgent = v.StringVal()
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
		attrs.HTTPRoute = v.StringVal()
	case conventions.AttributeHTTPServerName:
		attrs.HTTPServerName = v.StringVal()
	case conventions.AttributeHTTPClientIP:
		attrs.HTTPClientIP = v.StringVal()

	default:
		attrs.NetworkAttributes.MapAttribute(k, v)
	}
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
func (attrs *RPCAttributes) MapAttribute(k string, v pdata.AttributeValue) {
	switch k {
	case conventions.AttributeRPCSystem:
		attrs.RPCSystem = v.StringVal()
	case conventions.AttributeRPCService:
		attrs.RPCService = v.StringVal()
	case conventions.AttributeRPCMethod:
		attrs.RPCMethod = v.StringVal()
	case attributeRPCGRPCStatusCode:
		attrs.RPCGRPCStatusCode = v.IntVal()

	default:
		attrs.NetworkAttributes.MapAttribute(k, v)
	}
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
func (attrs *DatabaseAttributes) MapAttribute(k string, v pdata.AttributeValue) {
	switch k {
	case conventions.AttributeDBSystem:
		attrs.DBSystem = v.StringVal()
	case conventions.AttributeDBConnectionString:
		attrs.DBConnectionString = v.StringVal()
	case conventions.AttributeDBUser:
		attrs.DBUser = v.StringVal()
	case conventions.AttributeDBStatement:
		attrs.DBStatement = v.StringVal()
	case conventions.AttributeDBOperation:
		attrs.DBOperation = v.StringVal()
	case conventions.AttributeDBMsSQLInstanceName:
		attrs.DBMSSQLInstanceName = v.StringVal()
	case conventions.AttributeDBJDBCDriverClassname:
		attrs.DBJDBCDriverClassName = v.StringVal()
	case conventions.AttributeDBCassandraKeyspace:
		attrs.DBCassandraKeyspace = v.StringVal()
	case conventions.AttributeDBHBaseNamespace:
		attrs.DBHBaseNamespace = v.StringVal()
	case conventions.AttributeDBRedisDatabaseIndex:
		attrs.DBRedisDatabaseIndex = v.StringVal()
	case conventions.AttributeDBMongoDBCollection:
		attrs.DBMongoDBCollection = v.StringVal()

	default:
		attrs.NetworkAttributes.MapAttribute(k, v)
	}
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
func (attrs *MessagingAttributes) MapAttribute(k string, v pdata.AttributeValue) {
	switch k {
	case conventions.AttributeMessagingSystem:
		attrs.MessagingSystem = v.StringVal()
	case conventions.AttributeMessagingDestination:
		attrs.MessagingDestination = v.StringVal()
	case conventions.AttributeMessagingDestinationKind:
		attrs.MessagingDestinationKind = v.StringVal()
	case conventions.AttributeMessagingTempDestination:
		attrs.MessagingTempDestination = v.StringVal()
	case conventions.AttributeMessagingProtocol:
		attrs.MessagingProtocol = v.StringVal()
	case conventions.AttributeMessagingProtocolVersion:
		attrs.MessagingProtocolVersion = v.StringVal()
	case conventions.AttributeMessagingURL:
		attrs.MessagingURL = v.StringVal()
	case conventions.AttributeMessagingMessageID:
		attrs.MessagingMessageID = v.StringVal()
	case conventions.AttributeMessagingConversationID:
		attrs.MessagingConversationID = v.StringVal()
	case conventions.AttributeMessagingPayloadSize:
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.MessagingMessagePayloadSize = val
		}
	case conventions.AttributeMessagingPayloadCompressedSize:
		if val, err := getAttributeValueAsInt(v); err == nil {
			attrs.MessagingMessagePayloadCompressedSize = val
		}
	case conventions.AttributeMessagingOperation:
		attrs.MessagingOperation = v.StringVal()

	default:
		attrs.NetworkAttributes.MapAttribute(k, v)
	}
}

// Tries to return the value of the attribute as an int64
func getAttributeValueAsInt(attributeValue pdata.AttributeValue) (int64, error) {
	switch attributeValue.Type() {
	case pdata.AttributeValueSTRING:
		// try to cast the string values to int64
		if val, err := strconv.Atoi(attributeValue.StringVal()); err == nil {
			return int64(val), nil
		}
	case pdata.AttributeValueINT:
		return attributeValue.IntVal(), nil
	}

	return int64(0), errUnexpectedAttributeValueType
}
