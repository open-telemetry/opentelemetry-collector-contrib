// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func testHTTPAttributeMapping(t *testing.T, variant string) {
	httpAttributeValues := map[string]any{
		"http.request.method": "http.request.method",
		"url.full":            "url.full",
		"url.path":            "url.path",
		"url.query":           "url.query",
		"url.scheme":          "url.scheme",

		"http.response.status_code":           "200",
		"network.protocol.name":               "network.protocol.name",
		"user_agent.original":                 "user_agent.original",
		"http.request.header.content-length":  "1",
		"http.request.body.size":              2,
		"http.response.header.content-length": "3",
		"http.response.body.size":             4,

		"http.route": "http.route",
	}

	attributeMap := pcommon.NewMap()
	assert.NoError(t, attributeMap.FromRaw(httpAttributeValues))
	addNetworkAttributes(attributeMap)

	httpAttributes := &httpAttributes{}

	switch variant {
	case "client":
		addClientAttributes(attributeMap)
	case "server":
		addServerAttributes(attributeMap)
	default:
		t.Fatalf("Unknown variant: %s", variant)
	}

	attributeMap.Range(httpAttributes.MapAttribute)

	assert.Equal(t, "http.request.method", httpAttributes.HTTPRequestMethod)
	assert.Equal(t, "url.full", httpAttributes.URLAttributes.URLFull)
	assert.Equal(t, "url.path", httpAttributes.URLAttributes.URLPath)
	assert.Equal(t, "url.query", httpAttributes.URLAttributes.URLQuery)
	assert.Equal(t, "url.scheme", httpAttributes.URLAttributes.URLScheme)
	assert.Equal(t, int64(200), httpAttributes.HTTPResponseStatusCode)
	assert.Equal(t, "network.protocol.name", httpAttributes.NetworkAttributes.NetworkProtocolName)
	assert.Equal(t, "user_agent.original", httpAttributes.UserAgentAttributes.UserAgentOriginal)

	vals, ok := httpAttributes.HTTPRequestHeaders["content-length"]
	require.True(t, ok)
	require.NotEmpty(t, vals)
	reqCL := vals[0]
	reqCLInt, err := strconv.ParseInt(reqCL, 10, 64)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), reqCLInt)

	assert.Equal(t, int64(2), httpAttributes.HTTPRequestBodySize)

	valsRes, okRes := httpAttributes.HTTPResponseHeaders["content-length"]
	require.True(t, okRes)
	require.NotEmpty(t, valsRes)
	reqRes := valsRes[0]
	reqResInt, err := strconv.ParseInt(reqRes, 10, 64)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), reqResInt)

	assert.Equal(t, int64(4), httpAttributes.HTTPResponseBodySize)
	assert.Equal(t, "http.route", httpAttributes.HTTPRoute)

	networkAttributesValidations(t, httpAttributes.NetworkAttributes)

	if variant == "client" {
		clientAttributesValidations(t, httpAttributes.ClientAttributes)
	} else {
		serverAttributesValidations(t, httpAttributes.ServerAttributes)
	}
}

func TestHTTPAttributeMapping(t *testing.T) {
	for _, variant := range []string{"client", "server"} {
		t.Run(variant, func(t *testing.T) {
			testHTTPAttributeMapping(t, variant)
		})
	}
}

func testRPCPAttributeMapping(t *testing.T, variant string) {
	rpcAttributeValues := map[string]any{
		"rpc.system":  "rpc.system",
		"rpc.service": "rpc.service",
		"rpc.method":  "rpc.method",
	}

	attributeMap := pcommon.NewMap()
	assert.NoError(t, attributeMap.FromRaw(rpcAttributeValues))

	addNetworkAttributes(attributeMap)

	switch variant {
	case "client":
		addClientAttributes(attributeMap)
	case "server":
		addServerAttributes(attributeMap)
	default:
		t.Fatalf("Unknown variant: %s", variant)
	}

	rpcAttributes := &rpcAttributes{}
	attributeMap.Range(rpcAttributes.MapAttribute)

	assert.Equal(t, "rpc.system", rpcAttributes.RPCSystem)
	assert.Equal(t, "rpc.service", rpcAttributes.RPCService)
	assert.Equal(t, "rpc.method", rpcAttributes.RPCMethod)

	networkAttributesValidations(t, rpcAttributes.NetworkAttributes)

	if variant == "client" {
		clientAttributesValidations(t, rpcAttributes.ClientAttributes)
	} else {
		serverAttributesValidations(t, rpcAttributes.ServerAttributes)
	}
}

func TestRPCPAttributeMapping(t *testing.T) {
	for _, variant := range []string{"client", "server"} {
		t.Run(variant, func(t *testing.T) {
			testRPCPAttributeMapping(t, variant)
		})
	}
}

func testDatabaseAttributeMapping(t *testing.T, variant string) {
	databaseAttributeValues := map[string]any{
		"db.collection.name":       "db.collection.name",
		"db.namespace":             "db.namespace",
		"db.operation.batch.size":  0,
		"db.operation.name":        "db.operation.name",
		"db.query.summary":         "db.query.summary",
		"db.query.text":            "db.query.text",
		"db.response.status_code":  "db.response.status_code",
		"db.stored_procedure.name": "db.stored_procedure.name",
		"db.system.name":           "db.system.name",
	}

	attributeMap := pcommon.NewMap()
	assert.NoError(t, attributeMap.FromRaw(databaseAttributeValues))

	addNetworkAttributes(attributeMap)

	switch variant {
	case "client":
		addClientAttributes(attributeMap)
	case "server":
		addServerAttributes(attributeMap)
	default:
		t.Fatalf("Unknown variant: %s", variant)
	}

	databaseAttributes := &databaseAttributes{}
	attributeMap.Range(databaseAttributes.MapAttribute)

	assert.Equal(t, "db.collection.name", databaseAttributes.DBCollectionName)
	assert.Equal(t, "db.namespace", databaseAttributes.DBNamespace)
	assert.Equal(t, int64(0), databaseAttributes.DBOperationBatchSize)
	assert.Equal(t, "db.operation.name", databaseAttributes.DBOperationName)
	assert.Equal(t, "db.query.summary", databaseAttributes.DBQuerySummary)
	assert.Equal(t, "db.query.text", databaseAttributes.DBQueryText)
	assert.Equal(t, "db.response.status_code", databaseAttributes.DBResponseStatusCode)
	assert.Equal(t, "db.stored_procedure.name", databaseAttributes.DBStoredProcedureName)
	assert.Equal(t, "db.system.name", databaseAttributes.DBSystemName)

	networkAttributesValidations(t, databaseAttributes.NetworkAttributes)

	if variant == "client" {
		clientAttributesValidations(t, databaseAttributes.ClientAttributes)
	} else {
		serverAttributesValidations(t, databaseAttributes.ServerAttributes)
	}
}

func TestDatabaseAttributeMapping(t *testing.T) {
	for _, variant := range []string{"client", "server"} {
		t.Run(variant, func(t *testing.T) {
			testDatabaseAttributeMapping(t, variant)
		})
	}
}

func testMessagingAttributeMapping(t *testing.T, variant string) {
	messagingAttributeValues := map[string]any{
		"messaging.batch.message_count":           0,
		"messaging.client.id":                     "messaging.client.id",
		"messaging.consumer.group.name":           "messaging.consumer.group.name",
		"messaging.destination.anonymous":         true,
		"messaging.destination.name":              "messaging.destination.name",
		"messaging.destination.partition.id":      "messaging.destination.partition.id",
		"messaging.destination.subscription.name": "messaging.destination.subscription.name",
		"messaging.destination.template":          "messaging.destination.template",
		"messaging.destination.temporary":         false,
		"messaging.message.body.size":             1,
		"messaging.message.conversation_id":       "messaging.message.conversation_id",
		"messaging.message.envelope.size":         2,
		"messaging.message.id":                    "messaging.message.id",
		"messaging.operation.name":                "messaging.operation.name",
		"messaging.operation.type":                "messaging.operation.type",
		"messaging.system":                        "messaging.system",
	}

	attributeMap := pcommon.NewMap()
	assert.NoError(t, attributeMap.FromRaw(messagingAttributeValues))

	addNetworkAttributes(attributeMap)

	switch variant {
	case "client":
		addClientAttributes(attributeMap)
	case "server":
		addServerAttributes(attributeMap)
	default:
		t.Fatalf("Unknown variant: %s", variant)
	}

	messagingAttributes := &messagingAttributes{}
	attributeMap.Range(messagingAttributes.MapAttribute)

	assert.Equal(t, int64(0), messagingAttributes.MessagingBatchMessageCount)
	assert.Equal(t, "messaging.client.id", messagingAttributes.MessagingClientID)
	assert.Equal(t, "messaging.consumer.group.name", messagingAttributes.MessagingConsumerGroup)
	assert.True(t, messagingAttributes.MessagingDestinationAnonymous)
	assert.Equal(t, "messaging.destination.name", messagingAttributes.MessagingDestination)
	assert.Equal(t, "messaging.destination.partition.id", messagingAttributes.MessagingDestinationPartitionID)
	assert.Equal(t, "messaging.destination.subscription.name", messagingAttributes.MessagingDestinationSubName)
	assert.Equal(t, "messaging.destination.template", messagingAttributes.MessagingDestinationTemplate)
	assert.False(t, messagingAttributes.MessagingDestinationTemporary)
	assert.Equal(t, int64(1), messagingAttributes.MessagingMessageBodySize)
	assert.Equal(t, "messaging.message.conversation_id", messagingAttributes.MessagingMessageConversationID)
	assert.Equal(t, int64(2), messagingAttributes.MessagingMessageEnvelopeSize)
	assert.Equal(t, "messaging.message.id", messagingAttributes.MessagingMessageID)
	assert.Equal(t, "messaging.operation.name", messagingAttributes.MessagingOperation)
	assert.Equal(t, "messaging.operation.type", messagingAttributes.MessagingOperationType)
	assert.Equal(t, "messaging.system", messagingAttributes.MessagingSystem)
	networkAttributesValidations(t, messagingAttributes.NetworkAttributes)

	if variant == "client" {
		clientAttributesValidations(t, messagingAttributes.ClientAttributes)
	} else {
		serverAttributesValidations(t, messagingAttributes.ServerAttributes)
	}
}

func TestMessagingAttributeMapping(t *testing.T) {
	for _, variant := range []string{"client", "server"} {
		t.Run(variant, func(t *testing.T) {
			testMessagingAttributeMapping(t, variant)
		})
	}
}

// Tests what happens when an attribute that should be an int is not
func TestAttributeMappingWithSomeBadValues(t *testing.T) {
	attributeMap := pcommon.NewMap()
	attributeMap.PutStr("network.peer.port", "xx")

	attrs := &networkAttributes{}
	attributeMap.Range(attrs.MapAttribute)

	// unset from default
	assert.Equal(t, int64(0), attrs.NetworkPeerPort)
}

func addClientAttributes(m pcommon.Map) {
	m.PutStr("client.address", "client.address")
	m.PutInt("client.port", 3000)
}

func addServerAttributes(m pcommon.Map) {
	m.PutStr("server.address", "server.address")
	m.PutInt("server.port", 61461)
}

func addNetworkAttributes(m pcommon.Map) {
	m.PutStr("network.transport", "network.transport")
	m.PutStr("network.peer.address", "network.peer.address")
	m.PutInt("network.peer.port", 1)
	m.PutStr("network.local.address", "network.local.address")
}

func networkAttributesValidations(t *testing.T, networkAttributes networkAttributes) {
	assert.Equal(t, "network.transport", networkAttributes.NetworkTransport)
	assert.Equal(t, "network.peer.address", networkAttributes.NetworkPeerAddress)
	assert.Equal(t, int64(1), networkAttributes.NetworkPeerPort)
	assert.Equal(t, "network.local.address", networkAttributes.NetworkLocalAddress)
}

func serverAttributesValidations(t *testing.T, serverAttributes serverAttributes) {
	assert.Equal(t, "server.address", serverAttributes.ServerAddress)
	assert.Equal(t, int64(61461), serverAttributes.ServerPort)
}

func clientAttributesValidations(t *testing.T, clientAttributes clientAttributes) {
	assert.Equal(t, "client.address", clientAttributes.ClientAddress)
	assert.Equal(t, int64(3000), clientAttributes.ClientPort)
}
