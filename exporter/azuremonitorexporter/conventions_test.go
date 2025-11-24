// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.34.0"
)

func testHTTPAttributeMapping(t *testing.T, variant string) {
	httpAttributeValues := map[string]any{
		string(conventions.HTTPRequestMethodKey): string(conventions.HTTPRequestMethodKey),
		string(conventions.URLFullKey):           string(conventions.URLFullKey),
		string(conventions.URLPathKey):           string(conventions.URLPathKey),
		string(conventions.URLQueryKey):          string(conventions.URLQueryKey),
		string(conventions.URLSchemeKey):         string(conventions.URLSchemeKey),

		string(conventions.HTTPResponseStatusCodeKey):                "200",
		string(conventions.NetworkProtocolNameKey):                   string(conventions.NetworkProtocolNameKey),
		string(conventions.UserAgentOriginalKey):                     string(conventions.UserAgentOriginalKey),
		string(conventions.HTTPRequestHeader("content-length").Key):  "1",
		string(conventions.HTTPRequestBodySizeKey):                   2,
		string(conventions.HTTPResponseHeader("content-length").Key): "3",
		string(conventions.HTTPResponseBodySizeKey):                  4,

		string(conventions.HTTPRouteKey): string(conventions.HTTPRouteKey),
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

	assert.Equal(t, string(conventions.HTTPRequestMethodKey), httpAttributes.HTTPRequestMethod)
	assert.Equal(t, string(conventions.URLFullKey), httpAttributes.URLAttributes.URLFull)
	assert.Equal(t, string(conventions.URLPathKey), httpAttributes.URLAttributes.URLPath)
	assert.Equal(t, string(conventions.URLQueryKey), httpAttributes.URLAttributes.URLQuery)
	assert.Equal(t, string(conventions.URLSchemeKey), httpAttributes.URLAttributes.URLScheme)
	assert.Equal(t, int64(200), httpAttributes.HTTPResponseStatusCode)
	assert.Equal(t, string(conventions.NetworkProtocolNameKey), httpAttributes.NetworkAttributes.NetworkProtocolName)
	assert.Equal(t, string(conventions.UserAgentOriginalKey), httpAttributes.UserAgentAttributes.UserAgentOriginal)

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
	assert.Equal(t, string(conventions.HTTPRouteKey), httpAttributes.HTTPRoute)

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
		string(conventions.RPCSystemKey):  string(conventions.RPCSystemKey),
		string(conventions.RPCServiceKey): string(conventions.RPCServiceKey),
		string(conventions.RPCMethodKey):  string(conventions.RPCMethodKey),
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

	assert.Equal(t, string(conventions.RPCSystemKey), rpcAttributes.RPCSystem)
	assert.Equal(t, string(conventions.RPCServiceKey), rpcAttributes.RPCService)
	assert.Equal(t, string(conventions.RPCMethodKey), rpcAttributes.RPCMethod)

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
		string(conventions.DBCollectionNameKey):      string(conventions.DBCollectionNameKey),
		string(conventions.DBNamespaceKey):           string(conventions.DBNamespaceKey),
		string(conventions.DBOperationBatchSizeKey):  0,
		string(conventions.DBOperationNameKey):       string(conventions.DBOperationNameKey),
		string(conventions.DBQuerySummaryKey):        string(conventions.DBQuerySummaryKey),
		string(conventions.DBQueryTextKey):           string(conventions.DBQueryTextKey),
		string(conventions.DBResponseStatusCodeKey):  string(conventions.DBResponseStatusCodeKey),
		string(conventions.DBStoredProcedureNameKey): string(conventions.DBStoredProcedureNameKey),
		string(conventions.DBSystemNameKey):          string(conventions.DBSystemNameKey),
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

	assert.Equal(t, string(conventions.DBCollectionNameKey), databaseAttributes.DBCollectionName)
	assert.Equal(t, string(conventions.DBNamespaceKey), databaseAttributes.DBNamespace)
	assert.Equal(t, int64(0), databaseAttributes.DBOperationBatchSize)
	assert.Equal(t, string(conventions.DBOperationNameKey), databaseAttributes.DBOperationName)
	assert.Equal(t, string(conventions.DBQuerySummaryKey), databaseAttributes.DBQuerySummary)
	assert.Equal(t, string(conventions.DBQueryTextKey), databaseAttributes.DBQueryText)
	assert.Equal(t, string(conventions.DBResponseStatusCodeKey), databaseAttributes.DBResponseStatusCode)
	assert.Equal(t, string(conventions.DBStoredProcedureNameKey), databaseAttributes.DBStoredProcedureName)
	assert.Equal(t, string(conventions.DBSystemNameKey), databaseAttributes.DBSystemName)

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
		string(conventions.MessagingBatchMessageCountKey):           0,
		string(conventions.MessagingClientIDKey):                    string(conventions.MessagingClientIDKey),
		string(conventions.MessagingConsumerGroupNameKey):           string(conventions.MessagingConsumerGroupNameKey),
		string(conventions.MessagingDestinationAnonymousKey):        true,
		string(conventions.MessagingDestinationNameKey):             string(conventions.MessagingDestinationNameKey),
		string(conventions.MessagingDestinationPartitionIDKey):      string(conventions.MessagingDestinationPartitionIDKey),
		string(conventions.MessagingDestinationSubscriptionNameKey): string(conventions.MessagingDestinationSubscriptionNameKey),
		string(conventions.MessagingDestinationTemplateKey):         string(conventions.MessagingDestinationTemplateKey),
		string(conventions.MessagingDestinationTemporaryKey):        false,
		string(conventions.MessagingMessageBodySizeKey):             1,
		string(conventions.MessagingMessageConversationIDKey):       string(conventions.MessagingMessageConversationIDKey),
		string(conventions.MessagingMessageEnvelopeSizeKey):         2,
		string(conventions.MessagingMessageIDKey):                   string(conventions.MessagingMessageIDKey),
		string(conventions.MessagingOperationNameKey):               string(conventions.MessagingOperationNameKey),
		string(conventions.MessagingOperationTypeKey):               string(conventions.MessagingOperationTypeKey),
		string(conventions.MessagingSystemKey):                      string(conventions.MessagingSystemKey),
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
	assert.Equal(t, string(conventions.MessagingClientIDKey), messagingAttributes.MessagingClientID)
	assert.Equal(t, string(conventions.MessagingConsumerGroupNameKey), messagingAttributes.MessagingConsumerGroup)
	assert.True(t, messagingAttributes.MessagingDestinationAnonymous)
	assert.Equal(t, string(conventions.MessagingDestinationNameKey), messagingAttributes.MessagingDestination)
	assert.Equal(t, string(conventions.MessagingDestinationPartitionIDKey), messagingAttributes.MessagingDestinationPartitionID)
	assert.Equal(t, string(conventions.MessagingDestinationSubscriptionNameKey), messagingAttributes.MessagingDestinationSubName)
	assert.Equal(t, string(conventions.MessagingDestinationTemplateKey), messagingAttributes.MessagingDestinationTemplate)
	assert.False(t, messagingAttributes.MessagingDestinationTemporary)
	assert.Equal(t, int64(1), messagingAttributes.MessagingMessageBodySize)
	assert.Equal(t, string(conventions.MessagingMessageConversationIDKey), messagingAttributes.MessagingMessageConversationID)
	assert.Equal(t, int64(2), messagingAttributes.MessagingMessageEnvelopeSize)
	assert.Equal(t, string(conventions.MessagingMessageIDKey), messagingAttributes.MessagingMessageID)
	assert.Equal(t, string(conventions.MessagingOperationNameKey), messagingAttributes.MessagingOperation)
	assert.Equal(t, string(conventions.MessagingOperationTypeKey), messagingAttributes.MessagingOperationType)
	assert.Equal(t, string(conventions.MessagingSystemKey), messagingAttributes.MessagingSystem)
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
	attributeMap.PutStr(string(conventions.NetworkPeerPortKey), "xx")

	attrs := &networkAttributes{}
	attributeMap.Range(attrs.MapAttribute)

	// unset from default
	assert.Equal(t, int64(0), attrs.NetworkPeerPort)
}

func addClientAttributes(m pcommon.Map) {
	m.PutStr(string(conventions.ClientAddressKey), string(conventions.ClientAddressKey))
	m.PutInt(string(conventions.ClientPortKey), 3000)
}

func addServerAttributes(m pcommon.Map) {
	m.PutStr(string(conventions.ServerAddressKey), string(conventions.ServerAddressKey))
	m.PutInt(string(conventions.ServerPortKey), 61461)
}

func addNetworkAttributes(m pcommon.Map) {
	m.PutStr(string(conventions.NetworkTransportKey), string(conventions.NetworkTransportKey))
	m.PutStr(string(conventions.NetworkPeerAddressKey), string(conventions.NetworkPeerAddressKey))
	m.PutInt(string(conventions.NetworkPeerPortKey), 1)
	m.PutStr(string(conventions.NetworkLocalAddressKey), string(conventions.NetworkLocalAddressKey))
}

func networkAttributesValidations(t *testing.T, networkAttributes networkAttributes) {
	assert.Equal(t, string(conventions.NetworkTransportKey), networkAttributes.NetworkTransport)
	assert.Equal(t, string(conventions.NetworkPeerAddressKey), networkAttributes.NetworkPeerAddress)
	assert.Equal(t, int64(1), networkAttributes.NetworkPeerPort)
	assert.Equal(t, string(conventions.NetworkLocalAddressKey), networkAttributes.NetworkLocalAddress)
}

func serverAttributesValidations(t *testing.T, serverAttributes serverAttributes) {
	assert.Equal(t, string(conventions.ServerAddressKey), serverAttributes.ServerAddress)
	assert.Equal(t, int64(61461), serverAttributes.ServerPort)
}

func clientAttributesValidations(t *testing.T, clientAttributes clientAttributes) {
	assert.Equal(t, string(conventions.ClientAddressKey), clientAttributes.ClientAddress)
	assert.Equal(t, int64(3000), clientAttributes.ClientPort)
}
