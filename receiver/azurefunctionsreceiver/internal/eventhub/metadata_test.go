// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eventhub

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testValidEventHubMetadataJSON = `{"TriggerPartitionContext":{"FullyQualifiedNamespace":"ns.servicebus.windows.net","EventHubName":"logs","ConsumerGroup":"$Default","PartitionId":"0"}}`

func TestExtractMetadata(t *testing.T) {
	t.Run("valid_event_hub", func(t *testing.T) {
		attrs := ExtractMetadata([]byte(testValidEventHubMetadataJSON))
		require.NotEmpty(t, attrs)
		assert.Equal(t, "ns.servicebus.windows.net", attrs[AttrEventHubNamespace])
		assert.Equal(t, "logs", attrs[AttrEventHubName])
		assert.Equal(t, "$Default", attrs[AttrEventHubConsumerGroup])
		assert.Equal(t, "0", attrs[AttrEventHubPartitionID])
	})

	t.Run("empty_or_no_context_returns_empty_map", func(t *testing.T) {
		assert.Empty(t, ExtractMetadata(nil))
		assert.Empty(t, ExtractMetadata([]byte("{}")))
	})

	t.Run("invalid_json_returns_nil", func(t *testing.T) {
		assert.Nil(t, ExtractMetadata([]byte("not json")))
	})
}

func TestExtractMetadata_roundtrip_from_protocol(t *testing.T) {
	// Simulate what the handler gets: protocol.InvokeRequest with Metadata as RawMessage.
	type invokeRequest struct {
		Data     map[string]string `json:"Data"`
		Metadata json.RawMessage   `json:"Metadata"`
	}
	body := []byte(`{"Data":{"logs":"[\"dGVzdA==\"]"},"Metadata":{"TriggerPartitionContext":{"EventHubName":"logs","PartitionId":"1"}}}`)
	var req invokeRequest
	require.NoError(t, json.Unmarshal(body, &req))
	attrs := ExtractMetadata(req.Metadata)
	require.NotEmpty(t, attrs)
	assert.Equal(t, "logs", attrs[AttrEventHubName])
	assert.Equal(t, "1", attrs[AttrEventHubPartitionID])
}
