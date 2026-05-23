// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2"
	"github.com/stretchr/testify/assert"
)

func TestAzureEvent_EnqueueTime(t *testing.T) {
	now := time.Now()
	t.Run("AzEventData non-nil", func(t *testing.T) {
		ev := &azeventhubs.ReceivedEventData{
			EnqueuedTime: &now,
		}
		a := azureEvent{AzEventData: ev}
		assert.Equal(t, &now, a.EnqueueTime())
	})
	t.Run("Both nil", func(t *testing.T) {
		a := azureEvent{}
		assert.Nil(t, a.EnqueueTime())
	})
}

func TestAzureEvent_Properties(t *testing.T) {
	props := map[string]any{
		"key1": "value1",
		"key2": 2,
	}
	t.Run("AzEventData non-nil", func(t *testing.T) {
		ev := &azeventhubs.ReceivedEventData{
			EventData: azeventhubs.EventData{
				Properties: props,
			},
		}
		a := azureEvent{AzEventData: ev}
		assert.Equal(t, props, a.Properties())
	})
	t.Run("nil", func(t *testing.T) {
		a := azureEvent{}
		assert.Nil(t, a.Properties())
	})
}

func TestAzureEvent_Data(t *testing.T) {
	data := []byte("Testing azure events")
	t.Run("AzEventData non-nil", func(t *testing.T) {
		ev := &azeventhubs.ReceivedEventData{
			EventData: azeventhubs.EventData{
				Body: data,
			},
		}
		a := azureEvent{AzEventData: ev}
		assert.Equal(t, data, a.Data())
	})
	t.Run("nil", func(t *testing.T) {
		a := azureEvent{}
		assert.Nil(t, a.Data())
	})
}
