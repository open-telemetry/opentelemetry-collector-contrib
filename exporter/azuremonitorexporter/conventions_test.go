// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.7.0"
)

func TestHTTPAttributeMapping(t *testing.T) {
	httpAttributeValues := map[string]any{
		string(conventions.HTTPMethodKey): string(conventions.HTTPMethodKey),
		string(conventions.HTTPURLKey):    string(conventions.HTTPURLKey),
		string(conventions.HTTPTargetKey): string(conventions.HTTPTargetKey),
		string(conventions.HTTPHostKey):   string(conventions.HTTPHostKey),
		string(conventions.HTTPSchemeKey): string(conventions.HTTPSchemeKey),

		// Exercise the INT or STRING logic
		string(conventions.HTTPStatusCodeKey):                        "200",
		"http.status_text":                                           "http.status_text",
		string(conventions.HTTPFlavorKey):                            string(conventions.HTTPFlavorKey),
		string(conventions.HTTPUserAgentKey):                         string(conventions.HTTPUserAgentKey),
		string(conventions.HTTPRequestContentLengthKey):              1,
		string(conventions.HTTPRequestContentLengthUncompressedKey):  2,
		string(conventions.HTTPResponseContentLengthKey):             3,
		string(conventions.HTTPResponseContentLengthUncompressedKey): 4,

		string(conventions.HTTPRouteKey):      string(conventions.HTTPRouteKey),
		string(conventions.HTTPServerNameKey): string(conventions.HTTPServerNameKey),
		string(conventions.HTTPClientIPKey):   string(conventions.HTTPClientIPKey),
	}

	attributeMap := pcommon.NewMap()
	assert.NoError(t, attributeMap.FromRaw(httpAttributeValues))

	addNetworkAttributes(attributeMap)

	httpAttributes := &HTTPAttributes{}
	attributeMap.Range(httpAttributes.MapAttribute)

	assert.Equal(t, string(conventions.HTTPMethodKey), httpAttributes.HTTPMethod)
	assert.Equal(t, string(conventions.HTTPURLKey), httpAttributes.HTTPURL)
	assert.Equal(t, string(conventions.HTTPTargetKey), httpAttributes.HTTPTarget)
	assert.Equal(t, string(conventions.HTTPHostKey), httpAttributes.HTTPHost)
	assert.Equal(t, string(conventions.HTTPSchemeKey), httpAttributes.HTTPScheme)
	assert.Equal(t, int64(200), httpAttributes.HTTPStatusCode)
	assert.Equal(t, "http.status_text", httpAttributes.HTTPStatusText)
	assert.Equal(t, string(conventions.HTTPFlavorKey), httpAttributes.HTTPFlavor)
	assert.Equal(t, string(conventions.HTTPUserAgentKey), httpAttributes.HTTPUserAgent)
	assert.Equal(t, int64(1), httpAttributes.HTTPRequestContentLength)
	assert.Equal(t, int64(2), httpAttributes.HTTPRequestContentLengthUncompressed)
	assert.Equal(t, int64(3), httpAttributes.HTTPResponseContentLength)
	assert.Equal(t, int64(4), httpAttributes.HTTPResponseContentLengthUncompressed)
	assert.Equal(t, string(conventions.HTTPRouteKey), httpAttributes.HTTPRoute)
	assert.Equal(t, string(conventions.HTTPServerNameKey), httpAttributes.HTTPServerName)
	assert.Equal(t, string(conventions.HTTPClientIPKey), httpAttributes.HTTPClientIP)

	networkAttributesValidations(t, httpAttributes.NetworkAttributes)
}

func TestRPCPAttributeMapping(t *testing.T) {
	rpcAttributeValues := map[string]any{
		string(conventions.RPCSystemKey):  string(conventions.RPCSystemKey),
		string(conventions.RPCServiceKey): string(conventions.RPCServiceKey),
		string(conventions.RPCMethodKey):  string(conventions.RPCMethodKey),
	}

	attributeMap := pcommon.NewMap()
	assert.NoError(t, attributeMap.FromRaw(rpcAttributeValues))

	addNetworkAttributes(attributeMap)

	rpcAttributes := &RPCAttributes{}
	attributeMap.Range(rpcAttributes.MapAttribute)

	assert.Equal(t, string(conventions.RPCSystemKey), rpcAttributes.RPCSystem)
	assert.Equal(t, string(conventions.RPCServiceKey), rpcAttributes.RPCService)
	assert.Equal(t, string(conventions.RPCMethodKey), rpcAttributes.RPCMethod)

	networkAttributesValidations(t, rpcAttributes.NetworkAttributes)
}

func TestDatabaseAttributeMapping(t *testing.T) {
	databaseAttributeValues := map[string]any{
		string(conventions.DBSystemKey):              string(conventions.DBSystemKey),
		string(conventions.DBConnectionStringKey):    string(conventions.DBConnectionStringKey),
		string(conventions.DBUserKey):                string(conventions.DBUserKey),
		string(conventions.DBStatementKey):           string(conventions.DBStatementKey),
		string(conventions.DBOperationKey):           string(conventions.DBOperationKey),
		string(conventions.DBMSSQLInstanceNameKey):   string(conventions.DBMSSQLInstanceNameKey),
		string(conventions.DBJDBCDriverClassnameKey): string(conventions.DBJDBCDriverClassnameKey),
		string(conventions.DBCassandraKeyspaceKey):   string(conventions.DBCassandraKeyspaceKey),
		string(conventions.DBHBaseNamespaceKey):      string(conventions.DBHBaseNamespaceKey),
		string(conventions.DBRedisDBIndexKey):        string(conventions.DBRedisDBIndexKey),
		string(conventions.DBMongoDBCollectionKey):   string(conventions.DBMongoDBCollectionKey),
	}

	attributeMap := pcommon.NewMap()
	assert.NoError(t, attributeMap.FromRaw(databaseAttributeValues))

	addNetworkAttributes(attributeMap)

	databaseAttributes := &DatabaseAttributes{}
	attributeMap.Range(databaseAttributes.MapAttribute)

	assert.Equal(t, string(conventions.DBSystemKey), databaseAttributes.DBSystem)
	assert.Equal(t, string(conventions.DBConnectionStringKey), databaseAttributes.DBConnectionString)
	assert.Equal(t, string(conventions.DBUserKey), databaseAttributes.DBUser)
	assert.Equal(t, string(conventions.DBStatementKey), databaseAttributes.DBStatement)
	assert.Equal(t, string(conventions.DBOperationKey), databaseAttributes.DBOperation)
	assert.Equal(t, string(conventions.DBMSSQLInstanceNameKey), databaseAttributes.DBMSSQLInstanceName)
	assert.Equal(t, string(conventions.DBJDBCDriverClassnameKey), databaseAttributes.DBJDBCDriverClassName)
	assert.Equal(t, string(conventions.DBCassandraKeyspaceKey), databaseAttributes.DBCassandraKeyspace)
	assert.Equal(t, string(conventions.DBHBaseNamespaceKey), databaseAttributes.DBHBaseNamespace)
	assert.Equal(t, string(conventions.DBMongoDBCollectionKey), databaseAttributes.DBMongoDBCollection)
	networkAttributesValidations(t, databaseAttributes.NetworkAttributes)
}

func TestMessagingAttributeMapping(t *testing.T) {
	messagingAttributeValues := map[string]any{
		string(conventions.MessagingSystemKey):                            string(conventions.MessagingSystemKey),
		string(conventions.MessagingDestinationKey):                       string(conventions.MessagingDestinationKey),
		string(conventions.MessagingDestinationKindKey):                   string(conventions.MessagingDestinationKindKey),
		string(conventions.MessagingTempDestinationKey):                   string(conventions.MessagingTempDestinationKey),
		string(conventions.MessagingProtocolKey):                          string(conventions.MessagingProtocolKey),
		string(conventions.MessagingProtocolVersionKey):                   string(conventions.MessagingProtocolVersionKey),
		string(conventions.MessagingURLKey):                               string(conventions.MessagingURLKey),
		string(conventions.MessagingMessageIDKey):                         string(conventions.MessagingMessageIDKey),
		string(conventions.MessagingConversationIDKey):                    string(conventions.MessagingConversationIDKey),
		string(conventions.MessagingMessagePayloadSizeBytesKey):           1,
		string(conventions.MessagingMessagePayloadCompressedSizeBytesKey): 2,
		string(conventions.MessagingOperationKey):                         string(conventions.MessagingOperationKey),
	}

	attributeMap := pcommon.NewMap()
	assert.NoError(t, attributeMap.FromRaw(messagingAttributeValues))

	addNetworkAttributes(attributeMap)

	messagingAttributes := &MessagingAttributes{}
	attributeMap.Range(messagingAttributes.MapAttribute)

	assert.Equal(t, string(conventions.MessagingSystemKey), messagingAttributes.MessagingSystem)
	assert.Equal(t, string(conventions.MessagingDestinationKey), messagingAttributes.MessagingDestination)
	assert.Equal(t, string(conventions.MessagingDestinationKindKey), messagingAttributes.MessagingDestinationKind)
	assert.Equal(t, string(conventions.MessagingTempDestinationKey), messagingAttributes.MessagingTempDestination)
	assert.Equal(t, string(conventions.MessagingProtocolKey), messagingAttributes.MessagingProtocol)
	assert.Equal(t, string(conventions.MessagingProtocolVersionKey), messagingAttributes.MessagingProtocolVersion)
	assert.Equal(t, string(conventions.MessagingURLKey), messagingAttributes.MessagingURL)
	assert.Equal(t, string(conventions.MessagingMessageIDKey), messagingAttributes.MessagingMessageID)
	assert.Equal(t, string(conventions.MessagingConversationIDKey), messagingAttributes.MessagingConversationID)
	assert.Equal(t, string(conventions.MessagingOperationKey), messagingAttributes.MessagingOperation)
	assert.Equal(t, int64(1), messagingAttributes.MessagingMessagePayloadSize)
	assert.Equal(t, int64(2), messagingAttributes.MessagingMessagePayloadCompressedSize)
	networkAttributesValidations(t, messagingAttributes.NetworkAttributes)
}

// Tests what happens when an attribute that should be an int is not
func TestAttributeMappingWithSomeBadValues(t *testing.T) {
	attributeMap := pcommon.NewMap()
	attributeMap.PutStr(string(conventions.NetPeerPortKey), "xx")

	attrs := &NetworkAttributes{}
	attributeMap.Range(attrs.MapAttribute)

	// unset from default
	assert.Equal(t, int64(0), attrs.NetPeerPort)
}

func addNetworkAttributes(m pcommon.Map) {
	m.PutStr(string(conventions.NetTransportKey), string(conventions.NetTransportKey))
	m.PutStr(string(conventions.NetPeerIPKey), string(conventions.NetPeerIPKey))
	m.PutInt(string(conventions.NetPeerPortKey), 1)
	m.PutStr(string(conventions.NetPeerNameKey), string(conventions.NetPeerNameKey))
	m.PutStr(string(conventions.NetHostIPKey), string(conventions.NetHostIPKey))
	m.PutInt(string(conventions.NetHostPortKey), 2)
	m.PutStr(string(conventions.NetHostNameKey), string(conventions.NetHostNameKey))
}

func networkAttributesValidations(t *testing.T, networkAttributes NetworkAttributes) {
	assert.Equal(t, string(conventions.NetTransportKey), networkAttributes.NetTransport)
	assert.Equal(t, string(conventions.NetPeerIPKey), networkAttributes.NetPeerIP)
	assert.Equal(t, int64(1), networkAttributes.NetPeerPort)
	assert.Equal(t, string(conventions.NetPeerNameKey), networkAttributes.NetPeerName)
	assert.Equal(t, string(conventions.NetHostIPKey), networkAttributes.NetHostIP)
	assert.Equal(t, int64(2), networkAttributes.NetHostPort)
	assert.Equal(t, string(conventions.NetHostNameKey), networkAttributes.NetHostName)
}
