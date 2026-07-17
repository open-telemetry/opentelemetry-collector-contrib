// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2"
)

type azureEvent struct {
	AzEventData *azeventhubs.ReceivedEventData
}

func (a *azureEvent) EnqueueTime() *time.Time {
	if a.AzEventData != nil {
		return a.AzEventData.EnqueuedTime
	}
	return nil
}

func (a *azureEvent) Properties() map[string]any {
	if a.AzEventData != nil {
		return a.AzEventData.Properties
	}
	return nil
}

func (a *azureEvent) Data() []byte {
	if a.AzEventData != nil {
		return a.AzEventData.Body
	}
	return nil
}
