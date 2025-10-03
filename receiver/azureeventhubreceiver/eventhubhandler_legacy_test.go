// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"testing"
	"time"

	eventhub "github.com/Azure/azure-event-hubs-go/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLegacyHub struct {
	eventHubProperties eventhub.HubRuntimeInformation
	closed             bool
	receiveCalls       int
	receivePartitionID string
	receiveOptions     []eventhub.ReceiveOption
}

func (h *mockLegacyHub) GetRuntimeInformation(_ context.Context) (*eventhub.HubRuntimeInformation, error) {
	return &h.eventHubProperties, nil
}

func (h *mockLegacyHub) Receive(_ context.Context, partitionID string, _ eventhub.Handler, opts ...eventhub.ReceiveOption) (*eventhub.ListenerHandle, error) {
	h.receiveCalls++
	h.receiveOptions = opts
	h.receivePartitionID = partitionID
	return nil, nil
}

func (h *mockLegacyHub) Close(_ context.Context) error {
	h.closed = true
	return nil
}

func TestHubWrapperLegacyImpl_GetEventHubProperties(t *testing.T) {
	createdOn := time.Now()
	path := "test"
	partitionIDs := []string{"p1", "p2"}

	mockHubWrapper := &hubWrapperLegacyImpl{
		hub: &mockLegacyHub{
			eventHubProperties: eventhub.HubRuntimeInformation{
				CreatedAt:      createdOn,
				Path:           path,
				PartitionCount: len(partitionIDs),
				PartitionIDs:   partitionIDs,
			},
		},
		config: &Config{},
	}

	results, err := mockHubWrapper.GetRuntimeInformation(t.Context())
	require.NoError(t, err)

	assert.Equal(t, results.CreatedAt, createdOn)
	assert.Equal(t, results.Path, path)
	assert.Len(t, partitionIDs, results.PartitionCount)
	assert.Equal(t, results.PartitionIDs, partitionIDs)
}

func TestHubWrapperLegacyImpl_Receive(t *testing.T) {
	tests := []struct {
		name          string
		offset        string
		applyOffset   bool
		partitionID   string
		consumerGroup string

		shouldCallReceive   bool
		expectedPartitionID string
		expectedOptionCount int
	}{
		{
			name:                "simple",
			expectedOptionCount: 1, // Default to latest offset when no storage and no offset
			shouldCallReceive:   true,
		},
		{
			name:                "partition id",
			partitionID:         "partition1",
			expectedPartitionID: "partition1",
			shouldCallReceive:   true,
			expectedOptionCount: 1, // Default to latest offset when no storage and no offset
		},
		{
			name:                "offset with apply",
			offset:              "1",
			applyOffset:         true,
			shouldCallReceive:   true,
			expectedOptionCount: 1,
		},
		{
			name:                "offset without apply",
			offset:              "1",
			applyOffset:         false,
			shouldCallReceive:   true,
			expectedOptionCount: 1, // Default to latest offset when no storage and no offset
		},
		{
			name:                "no offset with apply",
			offset:              "",
			applyOffset:         true,
			shouldCallReceive:   true,
			expectedOptionCount: 1, // Default to latest offset when no storage and no offset
		},
		{
			name:                "offset with partition id",
			offset:              "1",
			partitionID:         "partition1",
			expectedPartitionID: "partition1",
			applyOffset:         true,
			shouldCallReceive:   true,
			expectedOptionCount: 1,
		},
		{
			name:                "consumer group",
			consumerGroup:       "cg1",
			shouldCallReceive:   true,
			expectedOptionCount: 2, // Consumer group + latest offset
		},
		{
			name:                "consumer group and offset",
			offset:              "1",
			applyOffset:         true,
			consumerGroup:       "cg1",
			shouldCallReceive:   true,
			expectedOptionCount: 2,
		},
	}

	for _, test := range tests {
		t.Run("Receive-"+test.name, func(t *testing.T) {
			hub := &mockLegacyHub{
				closed: false,
			}
			mockHubWrapper := &hubWrapperLegacyImpl{
				hub: hub,
				config: &Config{
					Offset:        test.offset,
					ConsumerGroup: test.consumerGroup,
				},
			}

			_, err := mockHubWrapper.Receive(t.Context(), test.partitionID, func(_ context.Context, _ *azureEvent) error {
				return nil
			}, test.applyOffset)
			require.NoError(t, err)

			if test.shouldCallReceive {
				assert.Equal(t, 1, hub.receiveCalls)
			} else {
				assert.Equal(t, 0, hub.receiveCalls)
			}

			assert.Len(t, hub.receiveOptions, test.expectedOptionCount)
			assert.Equal(t, test.expectedPartitionID, hub.receivePartitionID)
		})
	}
}

func TestHubWrapperLegacyImpl_Close(t *testing.T) {
	hub := &mockLegacyHub{
		closed: false,
	}
	mockHubWrapper := &hubWrapperLegacyImpl{
		hub:    hub,
		config: &Config{},
	}

	err := mockHubWrapper.Close(t.Context())
	require.NoError(t, err)
	assert.True(t, hub.closed)
}
