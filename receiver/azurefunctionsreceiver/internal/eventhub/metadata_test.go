// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eventhub

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMetadata(t *testing.T) {
	t.Run("valid_event_hub", func(t *testing.T) {
		raw := []byte(`{"TriggerPartitionContext":{"FullyQualifiedNamespace":"ns.servicebus.windows.net","EventHubName":"logs","ConsumerGroup":"$Default","PartitionId":"0"}}`)
		m, err := ParseMetadata(raw)
		require.NoError(t, err)
		assert.Equal(t, "ns.servicebus.windows.net", m.TriggerPartitionContext.FullyQualifiedNamespace)
		assert.Equal(t, "logs", m.TriggerPartitionContext.EventHubName)
		assert.Equal(t, "$Default", m.TriggerPartitionContext.ConsumerGroup)
		assert.Equal(t, "0", m.TriggerPartitionContext.PartitionId)
	})

	t.Run("empty", func(t *testing.T) {
		m, err := ParseMetadata(nil)
		require.NoError(t, err)
		assert.Empty(t, m.TriggerPartitionContext.EventHubName)
	})

	t.Run("empty_object", func(t *testing.T) {
		m, err := ParseMetadata([]byte("{}"))
		require.NoError(t, err)
		assert.Empty(t, m.TriggerPartitionContext.EventHubName)
	})

	t.Run("invalid_json", func(t *testing.T) {
		_, err := ParseMetadata([]byte("not json"))
		assert.Error(t, err)
	})
}

func TestMetadata_ResourceAttributes(t *testing.T) {
	t.Run("populated", func(t *testing.T) {
		m := Metadata{
			TriggerPartitionContext: TriggerPartitionContext{
				FullyQualifiedNamespace: "ns.servicebus.windows.net",
				EventHubName:            "logs",
				ConsumerGroup:           "ecf",
				PartitionId:             "3",
			},
		}
		attrs := m.ResourceAttributes()
		assert.Equal(t, "ns.servicebus.windows.net", attrs[AttrEventHubNamespace])
		assert.Equal(t, "logs", attrs[AttrEventHubName])
		assert.Equal(t, "ecf", attrs[AttrEventHubConsumerGroup])
		assert.Equal(t, "3", attrs[AttrEventHubPartitionID])
	})

	t.Run("empty_event_hub_name_returns_nil_like", func(t *testing.T) {
		m := Metadata{}
		attrs := m.ResourceAttributes()
		assert.Empty(t, attrs)
	})

	t.Run("only_event_hub_name", func(t *testing.T) {
		m := Metadata{
			TriggerPartitionContext: TriggerPartitionContext{EventHubName: "myhub"},
		}
		attrs := m.ResourceAttributes()
		assert.Len(t, attrs, 1)
		assert.Equal(t, "myhub", attrs[AttrEventHubName])
	})
}

func TestExtractMetadata(t *testing.T) {
	t.Run("valid_event_hub", func(t *testing.T) {
		raw := []byte(`{"TriggerPartitionContext":{"FullyQualifiedNamespace":"ns.servicebus.windows.net","EventHubName":"logs","ConsumerGroup":"$Default","PartitionId":"0"}}`)
		attrs := ExtractMetadata(raw)
		require.NotNil(t, attrs)
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

func TestParseMetadata_roundtrip_from_protocol(t *testing.T) {
	// Simulate what the handler gets: protocol.InvokeRequest with Metadata as RawMessage.
	type invokeRequest struct {
		Data     map[string]string `json:"Data"`
		Metadata json.RawMessage   `json:"Metadata"`
	}
	body := []byte(`{"Data":{"logs":"[\"dGVzdA==\"]"},"Metadata":{"TriggerPartitionContext":{"EventHubName":"logs","PartitionId":"1"}}}`)
	var req invokeRequest
	require.NoError(t, json.Unmarshal(body, &req))
	m, err := ParseMetadata(req.Metadata)
	require.NoError(t, err)
	attrs := m.ResourceAttributes()
	assert.Equal(t, "logs", attrs[AttrEventHubName])
	assert.Equal(t, "1", attrs[AttrEventHubPartitionID])
}
