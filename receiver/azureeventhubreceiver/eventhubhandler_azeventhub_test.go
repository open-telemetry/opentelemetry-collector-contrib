// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockPartitionClient struct {
	closed bool
}

func (p *mockPartitionClient) ReceiveEvents(ctx context.Context, maxBatchSize int, options *azeventhubs.ReceiveEventsOptions) ([]*azeventhubs.ReceivedEventData, error) {
	events := make([]*azeventhubs.ReceivedEventData, 0, maxBatchSize)

	for i := range maxBatchSize {
		event := &azeventhubs.ReceivedEventData{
			EventData: azeventhubs.EventData{
				Body: fmt.Appendf(nil, `{"id": %d}`, i+1),
			},
		}
		events = append(events, event)
	}

	return events, nil
}

func (p *mockPartitionClient) Close(ctx context.Context) error {
	p.closed = true
	return nil
}

type mockAzeventHub struct {
	eventHubProperties  azeventhubs.EventHubProperties
	partitionProperties azeventhubs.PartitionProperties
	partitionId         string
	offset              string
	closed              bool
}

func (a *mockAzeventHub) GetEventHubProperties(ctx context.Context, options *azeventhubs.GetEventHubPropertiesOptions) (azeventhubs.EventHubProperties, error) {
	return a.eventHubProperties, nil
}

func (a *mockAzeventHub) GetPartitionProperties(ctx context.Context, partitionID string, options *azeventhubs.GetPartitionPropertiesOptions) (azeventhubs.PartitionProperties, error) {
	return a.partitionProperties, nil
}

func (a *mockAzeventHub) NewPartitionClient(partitionID string, options *azeventhubs.PartitionClientOptions) (azPartitionClient, error) {
	a.partitionId = partitionID
	if options != nil && options.StartPosition.Offset != nil {
		a.offset = *options.StartPosition.Offset
	}

	return &mockPartitionClient{}, nil
}

func (a *mockAzeventHub) Close(ctx context.Context) error {
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

	results, err := mockHubWrapper.GetRuntimeInformation(context.Background())
	require.NoError(t, err)

	assert.Equal(t, results.CreatedAt, createdOn)
	assert.Equal(t, results.Path, name)
	assert.Equal(t, results.PartitionCount, len(partitionIDs))
	assert.Equal(t, results.PartitionIDs, partitionIDs)
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
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			h := &hubWrapperAzeventhubImpl{
				hub:    test.hub,
				config: test.config,
			}

			listener, err := h.Receive(
				ctx,
				"p1",
				func(ctx context.Context, event *AzureEvent) error {
					return nil
				},
				test.applyOffset,
			)
			if test.expectErr {
				require.Error(t, err)
				cancel()
				return
			}

			require.Equal(t, test.hub.offset, test.expectedOffset)
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

	err := mockHubWrapper.Close(context.Background())
	require.NoError(t, err)
	assert.Equal(t, hub.closed, true)
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
	assert.Equal(t, namespace, "test.servicebus.windows.net")

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
	assert.Equal(t, p.err, nil)
	p.setErr(errors.New("test"))
	assert.Equal(t, p.err.Error(), "test")
}
