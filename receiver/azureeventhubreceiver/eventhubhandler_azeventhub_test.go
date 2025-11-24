// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockPartitionClient struct {
	eventData []*azeventhubs.ReceivedEventData
	closed    bool
}

func (p *mockPartitionClient) ReceiveEvents(_ context.Context, maxBatchSize int, _ *azeventhubs.ReceiveEventsOptions) ([]*azeventhubs.ReceivedEventData, error) {
	events := make([]*azeventhubs.ReceivedEventData, 0, maxBatchSize)

	for i := range maxBatchSize {
		event := &azeventhubs.ReceivedEventData{
			EventData: azeventhubs.EventData{
				Body: fmt.Appendf(nil, `{"id": %d}`, i+1),
			},
		}
		events = append(events, event)
	}

	p.eventData = events
	return events, nil
}

func (p *mockPartitionClient) Close(_ context.Context) error {
	p.closed = true
	return nil
}

type mockAzeventHub struct {
	eventHubProperties  azeventhubs.EventHubProperties
	partitionProperties azeventhubs.PartitionProperties
	partitionID         string
	offset              string
	closed              bool
}

func (a *mockAzeventHub) GetEventHubProperties(_ context.Context, _ *azeventhubs.GetEventHubPropertiesOptions) (azeventhubs.EventHubProperties, error) {
	return a.eventHubProperties, nil
}

func (a *mockAzeventHub) GetPartitionProperties(_ context.Context, _ string, _ *azeventhubs.GetPartitionPropertiesOptions) (azeventhubs.PartitionProperties, error) {
	return a.partitionProperties, nil
}

func (a *mockAzeventHub) NewPartitionClient(partitionID string, options *azeventhubs.PartitionClientOptions) (azPartitionClient, error) {
	a.partitionID = partitionID
	if options != nil && options.StartPosition.Offset != nil {
		a.offset = *options.StartPosition.Offset
	}

	return &mockPartitionClient{}, nil
}

func (a *mockAzeventHub) Close(_ context.Context) error {
	a.closed = true
	return nil
}

func TestHubWrapperAzeventhubImpl_GetEventHubProperties(t *testing.T) {
	createdOn := time.Now()
	name := "test"
	partitionIDs := []string{"p1", "p2"}

	mockHubWrapper := &hubWrapperAzeventhubImpl{
		hub: &mockAzeventHub{
			eventHubProperties: azeventhubs.EventHubProperties{
				CreatedOn:             createdOn,
				Name:                  name,
				PartitionIDs:          partitionIDs,
				GeoReplicationEnabled: false,
			},
		},
		config:  &Config{},
		storage: nil,
	}

	results, err := mockHubWrapper.GetRuntimeInformation(t.Context())
	require.NoError(t, err)

	assert.Equal(t, createdOn, results.CreatedAt)
	assert.Equal(t, name, results.Path)
	assert.Equal(t, len(partitionIDs), results.PartitionCount)
	assert.Equal(t, partitionIDs, results.PartitionIDs)
}

func TestHubWrapperAzeventhubImpl_Receive(t *testing.T) {
	testCases := []struct {
		name           string
		hub            *mockAzeventHub
		config         *Config
		applyOffset    bool
		expectErr      bool
		expectedOffset string
	}{
		{
			name:      "nil hub",
			hub:       nil,
			config:    &Config{},
			expectErr: true,
		},
		{
			name: "normal event handling",
			hub:  &mockAzeventHub{},
			config: &Config{
				Connection:    "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=abcd;EntityPath=main",
				MaxPollEvents: 2,
				PollRate:      1,
			},
			expectErr: false,
		},
		{
			name: "apply offset",
			hub:  &mockAzeventHub{},
			config: &Config{
				Connection: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=abcd;EntityPath=main",
				Offset:     "100",
				PollRate:   1,
			},
			expectedOffset: "100",
			applyOffset:    true,
			expectErr:      false,
		},
		{
			name: "apply offset false",
			hub:  &mockAzeventHub{},
			config: &Config{
				Connection: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=abcd;EntityPath=main",
				Offset:     "100",
			},
			expectedOffset: "",
			applyOffset:    false,
			expectErr:      false,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
			defer cancel()

			h := &hubWrapperAzeventhubImpl{
				hub:    test.hub,
				config: test.config,
			}

			listener, err := h.Receive(
				ctx,
				"p1",
				func(_ context.Context, _ *azureEvent) error {
					return nil
				},
				test.applyOffset,
			)
			if test.expectErr {
				require.Error(t, err)
				cancel()
				return
			}

			require.Equal(t, test.expectedOffset, test.hub.offset)
			require.NoError(t, err)
			<-listener.Done()
		})
	}
}

func TestHubWrapperAzeventhubImpl_Close(t *testing.T) {
	hub := &mockAzeventHub{
		closed: false,
	}
	mockHubWrapper := &hubWrapperAzeventhubImpl{
		hub:     hub,
		config:  &Config{},
		storage: nil,
	}

	err := mockHubWrapper.Close(t.Context())
	require.NoError(t, err)
	assert.True(t, hub.closed)
}

func TestHubWrapperAzeventhubImpl_Namespace(t *testing.T) {
	mockHubWrapper := &hubWrapperAzeventhubImpl{
		hub: &mockAzeventHub{},
		config: &Config{
			Connection: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=abc+AEhE+b8yI=;EntityPath=main",
		},
		storage: nil,
	}

	namespace, err := mockHubWrapper.namespace()
	require.NoError(t, err)
	assert.Equal(t, "test", namespace)

	mockHubWrapper = &hubWrapperAzeventhubImpl{
		hub: &mockAzeventHub{},
		config: &Config{
			Connection: "bad connection",
		},
		storage: nil,
	}

	_, err = mockHubWrapper.namespace()
	require.Error(t, err)
}

func TestPartitionListener_SetErr(t *testing.T) {
	p := partitionListener{}
	require.NoError(t, p.err)
	p.setErr(errors.New("test"))
	assert.Equal(t, "test", p.err.Error())
}

func TestGetConsumerGroup(t *testing.T) {
	testCases := []struct {
		name          string
		consumerGroup string
		expectedGroup string
	}{
		{
			name:          "empty consumer group defaults to $Default",
			consumerGroup: "",
			expectedGroup: "$Default",
		},
		{
			name:          "custom consumer group is preserved",
			consumerGroup: "custom-group",
			expectedGroup: "custom-group",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			config := &Config{
				ConsumerGroup: test.consumerGroup,
			}

			result := getConsumerGroup(config)
			assert.Equal(t, test.expectedGroup, result)
		})
	}
}

func TestStartPos(t *testing.T) {
	testCases := []struct {
		enableStorage bool
		applyOffset   bool
		offset        string
		namespace     string
		eventHubName  string
		consumerGroup string
		partitionID   string

		storageClient *mockStorageClient

		expectedSeqNumber *int64
		expectedLatest    *bool
		expectedOffset    *string
	}{
		{
			enableStorage: false,
			applyOffset:   true,
			offset:        "10",
			namespace:     "test",
			eventHubName:  "name",
			consumerGroup: "cg",
			partitionID:   "0",

			expectedOffset: to.Ptr("10"),
		},
		{
			enableStorage: false,
			applyOffset:   false,
			offset:        "10",
			namespace:     "test",
			eventHubName:  "name",
			consumerGroup: "cg",
			partitionID:   "0",

			expectedLatest: to.Ptr(true),
		},
		{
			enableStorage: false,
			applyOffset:   false,
			offset:        "",
			namespace:     "test",
			eventHubName:  "name",
			consumerGroup: "cg",
			partitionID:   "0",

			expectedLatest: to.Ptr(true),
		},
		{
			enableStorage: true,
			applyOffset:   false,
			offset:        "",
			namespace:     "test",
			eventHubName:  "name",
			consumerGroup: "cg",
			partitionID:   "0",

			expectedLatest: to.Ptr(true),
		},
		{
			enableStorage: true,
			applyOffset:   true,
			offset:        "10",
			namespace:     "test",
			eventHubName:  "name",
			consumerGroup: "cg",
			partitionID:   "0",

			expectedOffset: to.Ptr("10"),
		},
		{
			enableStorage: true,
			applyOffset:   false,
			offset:        "",
			namespace:     "test",
			eventHubName:  "name",
			consumerGroup: "cg",
			partitionID:   "0",
			storageClient: &mockStorageClient{
				storage: map[string][]byte{
					"test/name/cg/0": []byte(`{"sequenceNumber": 100}`),
				},
			},

			expectedSeqNumber: to.Ptr(int64(100)),
		},
		{
			enableStorage: true,
			applyOffset:   false,
			offset:        "",
			namespace:     "test",
			eventHubName:  "name",
			consumerGroup: "cg",
			partitionID:   "0",
			storageClient: &mockStorageClient{
				storage: map[string][]byte{
					"test/name/cg/0": []byte(`{"seqNumber": 200}`),
				},
			},

			expectedSeqNumber: to.Ptr(int64(200)),
		},
	}

	for _, test := range testCases {
		h := hubWrapperAzeventhubImpl{}
		h.config = &Config{
			Offset: test.offset,
		}
		if test.enableStorage {
			s := &mockStorageClient{}
			if test.storageClient != nil {
				s = test.storageClient
			}
			h.storage = getStorageCheckpointPersister(s)
		}
		startPos := h.getStartPos(
			test.applyOffset,
			test.namespace,
			test.eventHubName,
			test.consumerGroup,
			test.partitionID,
		)
		if test.expectedSeqNumber != nil {
			require.Equal(t, *test.expectedSeqNumber, *startPos.SequenceNumber)
		}
		if test.expectedLatest != nil {
			require.Equal(t, *test.expectedLatest, *startPos.Latest)
		}
		if test.expectedOffset != nil {
			require.Equal(t, *test.expectedOffset, *startPos.Offset)
		}
	}
}
