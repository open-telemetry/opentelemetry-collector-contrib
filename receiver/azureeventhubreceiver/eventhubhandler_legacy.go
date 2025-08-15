// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"

	eventhub "github.com/Azure/azure-event-hubs-go/v3"
)

type legacyHubWrapper interface {
	GetRuntimeInformation(ctx context.Context) (*eventhub.HubRuntimeInformation, error)
	Receive(ctx context.Context, partitionID string, handler eventhub.Handler, opts ...eventhub.ReceiveOption) (*eventhub.ListenerHandle, error)
	Close(ctx context.Context) error
}

type hubWrapperLegacyImpl struct {
	hub    legacyHubWrapper
	config *Config
}

func (h *hubWrapperLegacyImpl) GetRuntimeInformation(ctx context.Context) (*hubRuntimeInfo, error) {
	if h.hub != nil {
		info, err := h.hub.GetRuntimeInformation(ctx)
		if err != nil {
			return nil, err
		}
		return &hubRuntimeInfo{
			CreatedAt:      info.CreatedAt,
			PartitionCount: info.PartitionCount,
			PartitionIDs:   info.PartitionIDs,
			Path:           info.Path,
		}, nil
	}
	return nil, implError
}

func (h *hubWrapperLegacyImpl) Receive(ctx context.Context, partitionID string, handler hubHandler, applyOffset bool) (listenerHandleWrapper, error) {
	receiverOptions := []eventhub.ReceiveOption{}
	if applyOffset && h.config.Offset != "" {
		receiverOptions = append(receiverOptions, eventhub.ReceiveWithStartingOffset(h.config.Offset))
	}

	if h.config.ConsumerGroup != "" {
		receiverOptions = append(receiverOptions, eventhub.ReceiveWithConsumerGroup(h.config.ConsumerGroup))
	}

	if h.hub != nil {
		l, err := h.hub.Receive(ctx, partitionID, func(ctx context.Context, event *eventhub.Event) error {
			return handler(ctx, &azureEvent{
				EventHubEvent: event,
			})
		}, receiverOptions...)

		return l, err
	}
	return nil, implError
}

func (h *hubWrapperLegacyImpl) Close(ctx context.Context) error {
	if h.hub != nil {
		return h.hub.Close(ctx)
	}
	return implError
}
