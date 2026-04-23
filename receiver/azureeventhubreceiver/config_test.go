// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	authID := component.MustNewID("azureauth")

	tests := []struct {
		id                  component.ID
		expected            component.Config
		expectedErrContains string
	}{
		{
			id: component.NewID(metadata.Type),
			expected: &Config{
				Connection: "Endpoint=sb://namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=superSecret1234=;EntityPath=hubName",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "auth"),
			expected: &Config{
				EventHub: EventHubConfig{
					Name:      "hubName",
					Namespace: "namespace.servicebus.windows.net",
				},
				Auth: &authID,
			},
		},
		{
			id:                  component.NewIDWithName(metadata.Type, "missing_connection"),
			expectedErrContains: "missing connection",
		},
		{
			id:                  component.NewIDWithName(metadata.Type, "invalid_connection_string"),
			expectedErrContains: "failed parsing connection string",
		},
		{
			id:                  component.NewIDWithName(metadata.Type, "invalid_format"),
			expectedErrContains: "invalid format",
		},
		{
			id:                  component.NewIDWithName(metadata.Type, "offset_without_partition"),
			expectedErrContains: "cannot use 'offset' without 'partition'",
		},
		{
			id: component.NewIDWithName(metadata.Type, "offset_with_partition"),
			expected: &Config{
				Connection: "Endpoint=sb://namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=superSecret1234=;EntityPath=hubName",
				Partition:  "foo",
				Offset:     "1234-5566",
			},
		},
		{
			id:                  component.NewIDWithName(metadata.Type, "auth_missing_event_hub_name"),
			expectedErrContains: "event_hub.name is required when using auth",
		},
		{
			id:                  component.NewIDWithName(metadata.Type, "auth_missing_namespace"),
			expectedErrContains: "event_hub.namespace is required when using auth",
		},
		{
			id: component.NewIDWithName(metadata.Type, "blob_checkpoint_store"),
			expected: &Config{
				Connection: "Endpoint=sb://namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=superSecret1234=;EntityPath=hubName",
				BlobCheckpointStore: &BlobCheckpointStoreConfig{
					Connection:    "DefaultEndpointsProtocol=https;AccountName=myaccount;AccountKey=mykey;EndpointSuffix=core.windows.net",
					ContainerName: "eventhub-checkpoints",
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "blob_checkpoint_store_auth"),
			expected: &Config{
				EventHub: EventHubConfig{
					Name:      "hubName",
					Namespace: "namespace.servicebus.windows.net",
				},
				Auth: &authID,
				BlobCheckpointStore: &BlobCheckpointStoreConfig{
					StorageAccountURL: "https://myaccount.blob.core.windows.net",
					ContainerName:     "eventhub-checkpoints",
				},
			},
		},
		{
			id:                  component.NewIDWithName(metadata.Type, "blob_checkpoint_store_missing_container"),
			expectedErrContains: "blob_checkpoint_store.container_name is required",
		},
		{
			id:                  component.NewIDWithName(metadata.Type, "blob_checkpoint_store_missing_connection"),
			expectedErrContains: "blob_checkpoint_store.connection is required when not using auth",
		},
		{
			id:                  component.NewIDWithName(metadata.Type, "blob_checkpoint_store_auth_missing_url"),
			expectedErrContains: "blob_checkpoint_store.storage_account_url is required when using auth",
		},
		{
			id:                  component.NewIDWithName(metadata.Type, "blob_checkpoint_store_with_partition"),
			expectedErrContains: "blob_checkpoint_store is mutually exclusive with partition and offset",
		},
		{
			id:                  component.NewIDWithName(metadata.Type, "blob_checkpoint_store_with_storage"),
			expectedErrContains: "blob_checkpoint_store is mutually exclusive with storage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.expectedErrContains != "" {
				assert.ErrorContains(t, xconfmap.Validate(cfg), tt.expectedErrContains)
				return
			}
			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
