// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubsexporter

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
	"github.com/stretchr/testify/mock"
)

// mockEventHubProducerClient is a mock implementation using testify/mock
type mockEventHubProducerClient struct {
	mock.Mock
}

func (m *mockEventHubProducerClient) NewEventDataBatch(ctx context.Context, options *azeventhubs.EventDataBatchOptions) (eventDataBatch, error) {
	args := m.Called(ctx, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(eventDataBatch), args.Error(1)
}

func (m *mockEventHubProducerClient) SendEventDataBatch(ctx context.Context, batch eventDataBatch, options *azeventhubs.SendEventDataBatchOptions) error {
	args := m.Called(ctx, batch, options)
	return args.Error(0)
}

func (m *mockEventHubProducerClient) Close(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// mockEventDataBatch is a mock for EventDataBatch
type mockEventDataBatch struct {
	mock.Mock
	events   []*azeventhubs.EventData
	maxBytes int
}

func (m *mockEventDataBatch) AddEventData(eventData *azeventhubs.EventData, options *azeventhubs.AddEventDataOptions) error {
	args := m.Called(eventData, options)
	if args.Error(0) == nil {
		m.events = append(m.events, eventData)
	}
	return args.Error(0)
}

func (m *mockEventDataBatch) NumEvents() int32 {
	args := m.Called()
	if len(args) > 0 && args.Get(0) != nil {
		return args.Get(0).(int32)
	}
	return int32(len(m.events))
}
