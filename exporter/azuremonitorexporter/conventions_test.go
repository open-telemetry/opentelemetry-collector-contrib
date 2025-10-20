// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.34.0"
)

func TestHTTPAttributeMapping(t *testing.T) {
	httpAttributeValues := map[string]any{
		string(conventions.HTTPRequestMethodKey): string(conventions.HTTPRequestMethodKey),
		string(conventions.URLFullKey):           string(conventions.URLFullKey),
		string(conventions.URLPathKey):           string(conventions.URLPathKey),
		string(conventions.URLQueryKey):          string(conventions.URLQueryKey),
		string(conventions.URLSchemeKey):         string(conventions.URLSchemeKey),

		// Exercise the INT or STRING logic
		string(conventions.HTTPResponseStatusCodeKey):                "200",
		string(conventions.NetworkProtocolNameKey):                   string(conventions.NetworkProtocolNameKey),
		string(conventions.UserAgentOriginalKey):                     string(conventions.UserAgentOriginalKey),
		string(conventions.HTTPRequestHeader("content-length").Key):  "1",
		string(conventions.HTTPRequestBodySizeKey):                   2,
		string(conventions.HTTPResponseHeader("content-length").Key): "3",
		string(conventions.HTTPResponseBodySizeKey):                  4,

		string(conventions.HTTPRouteKey):     string(conventions.HTTPRouteKey),
		string(conventions.ServerAddressKey): string(conventions.ServerAddressKey),
		string(conventions.ClientAddressKey): string(conventions.ClientAddressKey),
	}

	attributeMap := pcommon.NewMap()
	assert.NoError(t, attributeMap.FromRaw(httpAttributeValues))

	addNetworkAttributes(attributeMap)

	httpAttributes := &httpAttributes{}
	attributeMap.Range(httpAttributes.MapAttribute)

	assert.Equal(t, string(conventions.HTTPRequestMethodKey), httpAttributes.HttpRequestMethod)
	assert.Equal(t, string(conventions.URLFullKey), httpAttributes.UrlAttributes.UrlFull)
	assert.Equal(t, string(conventions.URLPathKey), httpAttributes.UrlAttributes.UrlPath)
	assert.Equal(t, string(conventions.URLQueryKey), httpAttributes.UrlAttributes.UrlQuery)

	assert.Equal(t, string(conventions.URLSchemeKey), httpAttributes.UrlAttributes.UrlScheme)
	assert.Equal(t, int64(200), httpAttributes.HttpResponseStatusCode)
	assert.Equal(t, string(conventions.NetworkProtocolNameKey), httpAttributes.NetworkAttributes.NetworkProtocolName)
	assert.Equal(t, string(conventions.UserAgentOriginalKey), httpAttributes.UserAgentAttributes.UserAgentOriginal)

	reqCL := httpAttributes.HttpRequestHeaders["content-length"][0]
	reqCLInt, err := strconv.ParseInt(reqCL, 10, 64)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), reqCLInt)

	assert.Equal(t, int64(2), httpAttributes.HttpRequestBodySize)

	resCL := httpAttributes.HttpResponseHeaders["content-length"][0]
	resCLInt, err := strconv.ParseInt(resCL, 10, 64)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), resCLInt)

	assert.Equal(t, int64(4), httpAttributes.HttpResponseBodySize)
	assert.Equal(t, string(conventions.HTTPRouteKey), httpAttributes.HttpRoute)
	assert.Equal(t, string(conventions.ServerAddressKey), httpAttributes.ServerAttributes.ServerAddress)
	assert.Equal(t, string(conventions.ClientAddressKey), httpAttributes.ClientAttributes.ClientAddress)

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

	rpcAttributes := &rpcAttributes{}
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

	databaseAttributes := &databaseAttributes{}
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

	messagingAttributes := &messagingAttributes{}
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

	attrs := &networkAttributes{}
	attributeMap.Range(attrs.MapAttribute)

	// unset from default
	assert.Equal(t, int64(0), attrs.NetPeerPort)
}

func addNetworkAttributes(m pcommon.Map) {
	m.PutStr(string(conventions.NetworkTransportKey), string(conventions.NetworkTransportKey))
	m.PutStr(string(conventions.NetworkPeerAddressKey), string(conventions.NetworkPeerAddressKey))
	m.PutInt(string(conventions.NetworkPeerPortKey), 1)
	//TODO: This is either client address or server address, not both
	m.PutStr(string(conventions.NetworkPeerNameKey), string(conventions.NetworkPeerNameKey))

	m.PutStr(string(conventions.NetworkLocalAddressKey), string(conventions.NetworkLocalAddressKey))
	m.PutInt(string(conventions.ServerPortKey), 2)
	m.PutStr(string(conventions.ServerAddressKey), string(conventions.ServerAddressKey))
}

func networkAttributesValidations(t *testing.T, networkAttributes networkAttributes) {
	assert.Equal(t, string(conventions.NetworkTransportKey), networkAttributes.NetworkTransport)
	assert.Equal(t, string(conventions.NetworkPeerAddressKey), networkAttributes.NetworkPeerAddress)
	assert.Equal(t, int64(1), networkAttributes.NetworkPeerPort)
	//TODO: This is either client address or server address, not both
	assert.Equal(t, string(conventions.NetworkPeerNameKey), networkAttributes.NetworkPeerName)

	assert.Equal(t, string(conventions.NetworkLocalAddressKey), networkAttributes.NetworkLocalAddress)
	//TODO: Replace with SeverPort
	assert.Equal(t, int64(2), networkAttributes.NetworkHostPort)
	//TODO: Replace with ServerAddress
	assert.Equal(t, string(conventions.NetworkHostNameKey), networkAttributes.NetworkHostName)
}
