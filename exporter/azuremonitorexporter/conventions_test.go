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
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
)

func TestHTTPAttributeMapping(t *testing.T) {
	httpAttributeValues := map[string]pdata.AttributeValue{
		conventions.AttributeHTTPMethod: pdata.NewAttributeValueString(conventions.AttributeHTTPMethod),
		conventions.AttributeHTTPURL:    pdata.NewAttributeValueString(conventions.AttributeHTTPURL),
		conventions.AttributeHTTPTarget: pdata.NewAttributeValueString(conventions.AttributeHTTPTarget),
		conventions.AttributeHTTPHost:   pdata.NewAttributeValueString(conventions.AttributeHTTPHost),
		conventions.AttributeHTTPScheme: pdata.NewAttributeValueString(conventions.AttributeHTTPScheme),

		// Exercise the INT or STRING logic
		conventions.AttributeHTTPStatusCode:                        pdata.NewAttributeValueString("200"),
		conventions.AttributeHTTPStatusText:                        pdata.NewAttributeValueString(conventions.AttributeHTTPStatusText),
		conventions.AttributeHTTPFlavor:                            pdata.NewAttributeValueString(conventions.AttributeHTTPFlavor),
		conventions.AttributeHTTPUserAgent:                         pdata.NewAttributeValueString(conventions.AttributeHTTPUserAgent),
		conventions.AttributeHTTPRequestContentLength:              pdata.NewAttributeValueInt(1),
		conventions.AttributeHTTPRequestContentLengthUncompressed:  pdata.NewAttributeValueInt(2),
		conventions.AttributeHTTPResponseContentLength:             pdata.NewAttributeValueInt(3),
		conventions.AttributeHTTPResponseContentLengthUncompressed: pdata.NewAttributeValueInt(4),

		conventions.AttributeHTTPRoute:      pdata.NewAttributeValueString(conventions.AttributeHTTPRoute),
		conventions.AttributeHTTPServerName: pdata.NewAttributeValueString(conventions.AttributeHTTPServerName),
		conventions.AttributeHTTPClientIP:   pdata.NewAttributeValueString(conventions.AttributeHTTPClientIP),
	}

	attributeMap := pdata.NewAttributeMap()
	attributeMap.InitFromMap(httpAttributeValues)

	// Add all the network attributes
	appendToAttributeMap(attributeMap, getNetworkAttributes())

	httpAttributes := &HTTPAttributes{}
	attributeMap.Range(httpAttributes.MapAttribute)

	assert.Equal(t, conventions.AttributeHTTPMethod, httpAttributes.HTTPMethod)
	assert.Equal(t, conventions.AttributeHTTPURL, httpAttributes.HTTPURL)
	assert.Equal(t, conventions.AttributeHTTPTarget, httpAttributes.HTTPTarget)
	assert.Equal(t, conventions.AttributeHTTPHost, httpAttributes.HTTPHost)
	assert.Equal(t, conventions.AttributeHTTPScheme, httpAttributes.HTTPScheme)
	assert.Equal(t, int64(200), httpAttributes.HTTPStatusCode)
	assert.Equal(t, conventions.AttributeHTTPStatusText, httpAttributes.HTTPStatusText)
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
	rpcAttributeValues := map[string]pdata.AttributeValue{
		conventions.AttributeRPCSystem:  pdata.NewAttributeValueString(conventions.AttributeRPCSystem),
		conventions.AttributeRPCService: pdata.NewAttributeValueString(conventions.AttributeRPCService),
		conventions.AttributeRPCMethod:  pdata.NewAttributeValueString(conventions.AttributeRPCMethod),
	}

	attributeMap := pdata.NewAttributeMap()
	attributeMap.InitFromMap(rpcAttributeValues)

	// Add all the network attributes
	appendToAttributeMap(attributeMap, getNetworkAttributes())

	rpcAttributes := &RPCAttributes{}
	attributeMap.Range(rpcAttributes.MapAttribute)

	assert.Equal(t, conventions.AttributeRPCSystem, rpcAttributes.RPCSystem)
	assert.Equal(t, conventions.AttributeRPCService, rpcAttributes.RPCService)
	assert.Equal(t, conventions.AttributeRPCMethod, rpcAttributes.RPCMethod)

	networkAttributesValidations(t, rpcAttributes.NetworkAttributes)
}

func TestDatabaseAttributeMapping(t *testing.T) {
	databaseAttributeValues := map[string]pdata.AttributeValue{
		conventions.AttributeDBSystem:              pdata.NewAttributeValueString(conventions.AttributeDBSystem),
		conventions.AttributeDBConnectionString:    pdata.NewAttributeValueString(conventions.AttributeDBConnectionString),
		conventions.AttributeDBUser:                pdata.NewAttributeValueString(conventions.AttributeDBUser),
		conventions.AttributeDBStatement:           pdata.NewAttributeValueString(conventions.AttributeDBStatement),
		conventions.AttributeDBOperation:           pdata.NewAttributeValueString(conventions.AttributeDBOperation),
		conventions.AttributeDBMSSQLInstanceName:   pdata.NewAttributeValueString(conventions.AttributeDBMSSQLInstanceName),
		conventions.AttributeDBJDBCDriverClassname: pdata.NewAttributeValueString(conventions.AttributeDBJDBCDriverClassname),
		conventions.AttributeDBCassandraKeyspace:   pdata.NewAttributeValueString(conventions.AttributeDBCassandraKeyspace),
		conventions.AttributeDBHBaseNamespace:      pdata.NewAttributeValueString(conventions.AttributeDBHBaseNamespace),
		conventions.AttributeDBRedisDBIndex:        pdata.NewAttributeValueString(conventions.AttributeDBRedisDBIndex),
		conventions.AttributeDBMongoDBCollection:   pdata.NewAttributeValueString(conventions.AttributeDBMongoDBCollection),
	}

	attributeMap := pdata.NewAttributeMap()
	attributeMap.InitFromMap(databaseAttributeValues)

	// Add all the network attributes
	appendToAttributeMap(attributeMap, getNetworkAttributes())

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
	messagingAttributeValues := map[string]pdata.AttributeValue{
		conventions.AttributeMessagingSystem:                            pdata.NewAttributeValueString(conventions.AttributeMessagingSystem),
		conventions.AttributeMessagingDestination:                       pdata.NewAttributeValueString(conventions.AttributeMessagingDestination),
		conventions.AttributeMessagingDestinationKind:                   pdata.NewAttributeValueString(conventions.AttributeMessagingDestinationKind),
		conventions.AttributeMessagingTempDestination:                   pdata.NewAttributeValueString(conventions.AttributeMessagingTempDestination),
		conventions.AttributeMessagingProtocol:                          pdata.NewAttributeValueString(conventions.AttributeMessagingProtocol),
		conventions.AttributeMessagingProtocolVersion:                   pdata.NewAttributeValueString(conventions.AttributeMessagingProtocolVersion),
		conventions.AttributeMessagingURL:                               pdata.NewAttributeValueString(conventions.AttributeMessagingURL),
		conventions.AttributeMessagingMessageID:                         pdata.NewAttributeValueString(conventions.AttributeMessagingMessageID),
		conventions.AttributeMessagingConversationID:                    pdata.NewAttributeValueString(conventions.AttributeMessagingConversationID),
		conventions.AttributeMessagingMessagePayloadSizeBytes:           pdata.NewAttributeValueInt(1),
		conventions.AttributeMessagingMessagePayloadCompressedSizeBytes: pdata.NewAttributeValueInt(2),
		conventions.AttributeMessagingOperation:                         pdata.NewAttributeValueString(conventions.AttributeMessagingOperation),
	}

	attributeMap := pdata.NewAttributeMap()
	attributeMap.InitFromMap(messagingAttributeValues)

	// Add all the network attributes
	appendToAttributeMap(attributeMap, getNetworkAttributes())

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
	// Try this out with any attribute struct with an int value
	values := map[string]pdata.AttributeValue{
		conventions.AttributeNetPeerPort: pdata.NewAttributeValueString("xx"),
	}

	attributeMap := pdata.NewAttributeMap()
	attributeMap.InitFromMap(values)

	attrs := &NetworkAttributes{}
	attributeMap.Range(attrs.MapAttribute)

	// unset from default
	assert.Equal(t, int64(0), attrs.NetPeerPort)
}

func getNetworkAttributes() map[string]pdata.AttributeValue {
	return map[string]pdata.AttributeValue{
		conventions.AttributeNetTransport: pdata.NewAttributeValueString(conventions.AttributeNetTransport),
		conventions.AttributeNetPeerIP:    pdata.NewAttributeValueString(conventions.AttributeNetPeerIP),
		conventions.AttributeNetPeerPort:  pdata.NewAttributeValueInt(1),
		conventions.AttributeNetPeerName:  pdata.NewAttributeValueString(conventions.AttributeNetPeerName),
		conventions.AttributeNetHostIP:    pdata.NewAttributeValueString(conventions.AttributeNetHostIP),
		conventions.AttributeNetHostPort:  pdata.NewAttributeValueInt(2),
		conventions.AttributeNetHostName:  pdata.NewAttributeValueString(conventions.AttributeNetHostName),
	}
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
