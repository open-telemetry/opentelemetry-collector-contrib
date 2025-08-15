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
	receivePartitionId string
	receiveOptions     []eventhub.ReceiveOption
}

func (h *mockLegacyHub) GetRuntimeInformation(ctx context.Context) (*eventhub.HubRuntimeInformation, error) {
	return &h.eventHubProperties, nil
}

func (h *mockLegacyHub) Receive(ctx context.Context, partitionID string, handler eventhub.Handler, opts ...eventhub.ReceiveOption) (*eventhub.ListenerHandle, error) {
	h.receiveCalls += 1
	h.receiveOptions = opts
	h.receivePartitionId = partitionID
	return nil, nil
}

func (h *mockLegacyHub) Close(ctx context.Context) error {
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

	results, err := mockHubWrapper.GetRuntimeInformation(context.Background())
	require.NoError(t, err)

	assert.Equal(t, results.CreatedAt, createdOn)
	assert.Equal(t, results.Path, path)
	assert.Equal(t, results.PartitionCount, len(partitionIDs))
	assert.Equal(t, results.PartitionIDs, partitionIDs)
}

func TestHubWrapperLegacyImpl_Receive(t *testing.T) {
	tests := []struct {
		name          string
		offset        string
		applyOffset   bool
		partitionId   string
		consumerGroup string

		shouldCallReceive   bool
		expectedPartitionId string
		expectedOptionCount int
	}{
		{
			name:                "simple",
			expectedOptionCount: 0,
			shouldCallReceive:   true,
		},
		{
			name:                "partition id",
			partitionId:         "partition1",
			expectedPartitionId: "partition1",
			shouldCallReceive:   true,
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
			expectedOptionCount: 0,
		},
		{
			name:                "no offset with apply",
			offset:              "",
			applyOffset:         true,
			shouldCallReceive:   true,
			expectedOptionCount: 0,
		},
		{
			name:                "offset with partition id",
			offset:              "1",
			partitionId:         "partition1",
			expectedPartitionId: "partition1",
			applyOffset:         true,
			shouldCallReceive:   true,
			expectedOptionCount: 1,
		},
		{
			name:                "consumer group",
			consumerGroup:       "cg1",
			shouldCallReceive:   true,
			expectedOptionCount: 1,
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

			mockHubWrapper.Receive(context.Background(), test.partitionId, func(ctx context.Context, event *AzureEvent) error {
				return nil
			}, test.applyOffset)

			if test.shouldCallReceive {
				assert.Equal(t, hub.receiveCalls, 1)
			} else {
				assert.Equal(t, hub.receiveCalls, 0)
			}

			assert.Equal(t, len(hub.receiveOptions), test.expectedOptionCount)
			assert.Equal(t, hub.receivePartitionId, test.expectedPartitionId)
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

	err := mockHubWrapper.Close(context.Background())
	require.NoError(t, err)
	assert.Equal(t, hub.closed, true)
}
