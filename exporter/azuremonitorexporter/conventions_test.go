// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

func TestHTTPAttributeMapping(t *testing.T) {
	httpAttributeValues := map[string]interface{}{
		conventions.AttributeHTTPMethod: conventions.AttributeHTTPMethod,
		conventions.AttributeHTTPURL:    conventions.AttributeHTTPURL,
		conventions.AttributeHTTPTarget: conventions.AttributeHTTPTarget,
		conventions.AttributeHTTPHost:   conventions.AttributeHTTPHost,
		conventions.AttributeHTTPScheme: conventions.AttributeHTTPScheme,

		// Exercise the INT or STRING logic
		conventions.AttributeHTTPStatusCode:                        "200",
		"http.status_text":                                         "http.status_text",
		conventions.AttributeHTTPFlavor:                            conventions.AttributeHTTPFlavor,
		conventions.AttributeHTTPUserAgent:                         conventions.AttributeHTTPUserAgent,
		conventions.AttributeHTTPRequestContentLength:              1,
		conventions.AttributeHTTPRequestContentLengthUncompressed:  2,
		conventions.AttributeHTTPResponseContentLength:             3,
		conventions.AttributeHTTPResponseContentLengthUncompressed: 4,

		conventions.AttributeHTTPRoute:      conventions.AttributeHTTPRoute,
		conventions.AttributeHTTPServerName: conventions.AttributeHTTPServerName,
		conventions.AttributeHTTPClientIP:   conventions.AttributeHTTPClientIP,
	}

	attributeMap := pcommon.NewMap()
	assert.NoError(t, attributeMap.FromRaw(httpAttributeValues))

	addNetworkAttributes(attributeMap)

	httpAttributes := &HTTPAttributes{}
	attributeMap.Range(httpAttributes.MapAttribute)

	assert.Equal(t, conventions.AttributeHTTPMethod, httpAttributes.HTTPMethod)
	assert.Equal(t, conventions.AttributeHTTPURL, httpAttributes.HTTPURL)
	assert.Equal(t, conventions.AttributeHTTPTarget, httpAttributes.HTTPTarget)
	assert.Equal(t, conventions.AttributeHTTPHost, httpAttributes.HTTPHost)
	assert.Equal(t, conventions.AttributeHTTPScheme, httpAttributes.HTTPScheme)
	assert.Equal(t, int64(200), httpAttributes.HTTPStatusCode)
	assert.Equal(t, "http.status_text", httpAttributes.HTTPStatusText)
	assert.Equal(t, conventions.AttributeHTTPFlavor, httpAttributes.HTTPFlavor)
	assert.Equal(t, conventions.AttributeHTTPUserAgent, httpAttributes.HTTPUserAgent)
	assert.Equal(t, int64(1), httpAttributes.HTTPRequestContentLength)
	assert.Equal(t, int64(2), httpAttributes.HTTPRequestContentLengthUncompressed)
	assert.Equal(t, int64(3), httpAttributes.HTTPResponseContentLength)
	assert.Equal(t, int64(4), httpAttributes.HTTPResponseContentLengthUncompressed)
	assert.Equal(t, conventions.AttributeHTTPRoute, httpAttributes.HTTPRoute)
	assert.Equal(t, conventions.AttributeHTTPServerName, httpAttributes.HTTPServerName)
	assert.Equal(t, conventions.AttributeHTTPClientIP, httpAttributes.HTTPClientIP)

	networkAttributesValidations(t, httpAttributes.NetworkAttributes)
}

func TestRPCPAttributeMapping(t *testing.T) {
	rpcAttributeValues := map[string]interface{}{
		conventions.AttributeRPCSystem:  conventions.AttributeRPCSystem,
		conventions.AttributeRPCService: conventions.AttributeRPCService,
		conventions.AttributeRPCMethod:  conventions.AttributeRPCMethod,
	}

	attributeMap := pcommon.NewMap()
	assert.NoError(t, attributeMap.FromRaw(rpcAttributeValues))

	addNetworkAttributes(attributeMap)

	rpcAttributes := &RPCAttributes{}
	attributeMap.Range(rpcAttributes.MapAttribute)

	assert.Equal(t, conventions.AttributeRPCSystem, rpcAttributes.RPCSystem)
	assert.Equal(t, conventions.AttributeRPCService, rpcAttributes.RPCService)
	assert.Equal(t, conventions.AttributeRPCMethod, rpcAttributes.RPCMethod)

	networkAttributesValidations(t, rpcAttributes.NetworkAttributes)
}

func TestDatabaseAttributeMapping(t *testing.T) {
	databaseAttributeValues := map[string]interface{}{
		conventions.AttributeDBSystem:              conventions.AttributeDBSystem,
		conventions.AttributeDBConnectionString:    conventions.AttributeDBConnectionString,
		conventions.AttributeDBUser:                conventions.AttributeDBUser,
		conventions.AttributeDBStatement:           conventions.AttributeDBStatement,
		conventions.AttributeDBOperation:           conventions.AttributeDBOperation,
		conventions.AttributeDBMSSQLInstanceName:   conventions.AttributeDBMSSQLInstanceName,
		conventions.AttributeDBJDBCDriverClassname: conventions.AttributeDBJDBCDriverClassname,
		conventions.AttributeDBCassandraKeyspace:   conventions.AttributeDBCassandraKeyspace,
		conventions.AttributeDBHBaseNamespace:      conventions.AttributeDBHBaseNamespace,
		conventions.AttributeDBRedisDBIndex:        conventions.AttributeDBRedisDBIndex,
		conventions.AttributeDBMongoDBCollection:   conventions.AttributeDBMongoDBCollection,
	}

	attributeMap := pcommon.NewMap()
	assert.NoError(t, attributeMap.FromRaw(databaseAttributeValues))

	addNetworkAttributes(attributeMap)

	databaseAttributes := &DatabaseAttributes{}
	attributeMap.Range(databaseAttributes.MapAttribute)

	assert.Equal(t, conventions.AttributeDBSystem, databaseAttributes.DBSystem)
	assert.Equal(t, conventions.AttributeDBConnectionString, databaseAttributes.DBConnectionString)
	assert.Equal(t, conventions.AttributeDBUser, databaseAttributes.DBUser)
	assert.Equal(t, conventions.AttributeDBStatement, databaseAttributes.DBStatement)
	assert.Equal(t, conventions.AttributeDBOperation, databaseAttributes.DBOperation)
	assert.Equal(t, conventions.AttributeDBMSSQLInstanceName, databaseAttributes.DBMSSQLInstanceName)
	assert.Equal(t, conventions.AttributeDBJDBCDriverClassname, databaseAttributes.DBJDBCDriverClassName)
	assert.Equal(t, conventions.AttributeDBCassandraKeyspace, databaseAttributes.DBCassandraKeyspace)
	assert.Equal(t, conventions.AttributeDBHBaseNamespace, databaseAttributes.DBHBaseNamespace)
	assert.Equal(t, conventions.AttributeDBMongoDBCollection, databaseAttributes.DBMongoDBCollection)
	networkAttributesValidations(t, databaseAttributes.NetworkAttributes)
}

func TestMessagingAttributeMapping(t *testing.T) {
	messagingAttributeValues := map[string]interface{}{
		conventions.AttributeMessagingSystem:                            conventions.AttributeMessagingSystem,
		conventions.AttributeMessagingDestination:                       conventions.AttributeMessagingDestination,
		conventions.AttributeMessagingDestinationKind:                   conventions.AttributeMessagingDestinationKind,
		conventions.AttributeMessagingTempDestination:                   conventions.AttributeMessagingTempDestination,
		conventions.AttributeMessagingProtocol:                          conventions.AttributeMessagingProtocol,
		conventions.AttributeMessagingProtocolVersion:                   conventions.AttributeMessagingProtocolVersion,
		conventions.AttributeMessagingURL:                               conventions.AttributeMessagingURL,
		conventions.AttributeMessagingMessageID:                         conventions.AttributeMessagingMessageID,
		conventions.AttributeMessagingConversationID:                    conventions.AttributeMessagingConversationID,
		conventions.AttributeMessagingMessagePayloadSizeBytes:           1,
		conventions.AttributeMessagingMessagePayloadCompressedSizeBytes: 2,
		conventions.AttributeMessagingOperation:                         conventions.AttributeMessagingOperation,
	}

	attributeMap := pcommon.NewMap()
	assert.NoError(t, attributeMap.FromRaw(messagingAttributeValues))

	addNetworkAttributes(attributeMap)

	messagingAttributes := &MessagingAttributes{}
	attributeMap.Range(messagingAttributes.MapAttribute)

	assert.Equal(t, conventions.AttributeMessagingSystem, messagingAttributes.MessagingSystem)
	assert.Equal(t, conventions.AttributeMessagingDestination, messagingAttributes.MessagingDestination)
	assert.Equal(t, conventions.AttributeMessagingDestinationKind, messagingAttributes.MessagingDestinationKind)
	assert.Equal(t, conventions.AttributeMessagingTempDestination, messagingAttributes.MessagingTempDestination)
	assert.Equal(t, conventions.AttributeMessagingProtocol, messagingAttributes.MessagingProtocol)
	assert.Equal(t, conventions.AttributeMessagingProtocolVersion, messagingAttributes.MessagingProtocolVersion)
	assert.Equal(t, conventions.AttributeMessagingURL, messagingAttributes.MessagingURL)
	assert.Equal(t, conventions.AttributeMessagingMessageID, messagingAttributes.MessagingMessageID)
	assert.Equal(t, conventions.AttributeMessagingConversationID, messagingAttributes.MessagingConversationID)
	assert.Equal(t, conventions.AttributeMessagingOperation, messagingAttributes.MessagingOperation)
	assert.Equal(t, int64(1), messagingAttributes.MessagingMessagePayloadSize)
	assert.Equal(t, int64(2), messagingAttributes.MessagingMessagePayloadCompressedSize)
	networkAttributesValidations(t, messagingAttributes.NetworkAttributes)
}

// Tests what happens when an attribute that should be an int is not
func TestAttributeMappingWithSomeBadValues(t *testing.T) {
	attributeMap := pcommon.NewMap()
	attributeMap.PutStr(conventions.AttributeNetPeerPort, "xx")

	attrs := &NetworkAttributes{}
	attributeMap.Range(attrs.MapAttribute)

	// unset from default
	assert.Equal(t, int64(0), attrs.NetPeerPort)
}

func addNetworkAttributes(m pcommon.Map) {
	m.PutStr(conventions.AttributeNetTransport, conventions.AttributeNetTransport)
	m.PutStr(conventions.AttributeNetPeerIP, conventions.AttributeNetPeerIP)
	m.PutInt(conventions.AttributeNetPeerPort, 1)
	m.PutStr(conventions.AttributeNetPeerName, conventions.AttributeNetPeerName)
	m.PutStr(conventions.AttributeNetHostIP, conventions.AttributeNetHostIP)
	m.PutInt(conventions.AttributeNetHostPort, 2)
	m.PutStr(conventions.AttributeNetHostName, conventions.AttributeNetHostName)
}

func networkAttributesValidations(t *testing.T, networkAttributes NetworkAttributes) {
	assert.Equal(t, conventions.AttributeNetTransport, networkAttributes.NetTransport)
	assert.Equal(t, conventions.AttributeNetPeerIP, networkAttributes.NetPeerIP)
	assert.Equal(t, int64(1), networkAttributes.NetPeerPort)
	assert.Equal(t, conventions.AttributeNetPeerName, networkAttributes.NetPeerName)
	assert.Equal(t, conventions.AttributeNetHostIP, networkAttributes.NetHostIP)
	assert.Equal(t, int64(2), networkAttributes.NetHostPort)
	assert.Equal(t, conventions.AttributeNetHostName, networkAttributes.NetHostName)
}
