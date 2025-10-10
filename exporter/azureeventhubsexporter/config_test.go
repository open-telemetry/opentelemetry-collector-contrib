// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubsexporter

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureeventhubsexporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, "conn-string"),
			expected: &Config{
				Auth: Authentication{
					Type:             ConnectionString,
					ConnectionString: "Endpoint=sb://my-namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=mykey",
				},
				EventHub: TelemetryConfig{
					Traces:  "otel-traces",
					Metrics: "otel-metrics",
					Logs:    "otel-logs",
				},
				FormatType: "json",
				PartitionKey: PartitionKeyConfig{
					Source: "random",
				},
				MaxEventSize:  1048576,
				BatchSize:     100,
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "sp"),
			expected: &Config{
				Namespace: "my-namespace.servicebus.windows.net",
				Auth: Authentication{
					Type:         ServicePrincipal,
					TenantID:     "tenant-id",
					ClientID:     "client-id",
					ClientSecret: "client-secret",
				},
				EventHub: TelemetryConfig{
					Traces:  "traces",
					Metrics: "metrics",
					Logs:    "logs",
				},
				FormatType: "proto",
				PartitionKey: PartitionKeyConfig{
					Source: "resource_attribute",
					Value:  "service.name",
				},
				MaxEventSize:  1048576,
				BatchSize:     100,
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "smi"),
			expected: &Config{
				Namespace: "my-namespace.servicebus.windows.net",
				Auth: Authentication{
					Type: SystemManagedIdentity,
				},
				EventHub: TelemetryConfig{
					Traces:  "traces",
					Metrics: "metrics",
					Logs:    "logs",
				},
				FormatType: "json",
				PartitionKey: PartitionKeyConfig{
					Source: "trace_id",
				},
				MaxEventSize:  1048576,
				BatchSize:     100,
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "umi"),
			expected: &Config{
				Namespace: "my-namespace.servicebus.windows.net",
				Auth: Authentication{
					Type:     UserManagedIdentity,
					ClientID: "client-id",
				},
				EventHub: TelemetryConfig{
					Traces:  "traces",
					Metrics: "metrics",
					Logs:    "logs",
				},
				FormatType: "json",
				PartitionKey: PartitionKeyConfig{
					Source: "static",
					Value:  "my-partition",
				},
				MaxEventSize:  1048576,
				BatchSize:     100,
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectedErr string
	}{
		{
			name: "valid connection string config",
			config: &Config{
				Auth: Authentication{
					Type:             ConnectionString,
					ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=Test;SharedAccessKey=key",
				},
				EventHub: TelemetryConfig{
					Traces:  "traces",
					Metrics: "metrics",
					Logs:    "logs",
				},
				FormatType:   "json",
				MaxEventSize: 1048576,
				BatchSize:    100,
				PartitionKey: PartitionKeyConfig{
					Source: "random",
				},
			},
			expectedErr: "",
		},
		{
			name: "missing namespace for non-connection-string auth",
			config: &Config{
				Auth: Authentication{
					Type: ServicePrincipal,
				},
				FormatType:   "json",
				MaxEventSize: 1048576,
				BatchSize:    100,
			},
			expectedErr: "namespace cannot be empty when auth type is not connection_string",
		},
		{
			name: "missing connection string",
			config: &Config{
				Auth: Authentication{
					Type: ConnectionString,
				},
				FormatType:   "json",
				MaxEventSize: 1048576,
				BatchSize:    100,
			},
			expectedErr: "connection_string cannot be empty when auth type is connection_string",
		},
		{
			name: "missing service principal fields",
			config: &Config{
				Namespace: "test.servicebus.windows.net",
				Auth: Authentication{
					Type: ServicePrincipal,
				},
				FormatType:   "json",
				MaxEventSize: 1048576,
				BatchSize:    100,
			},
			expectedErr: "tenant_id, client_id and client_secret cannot be empty when auth type is service_principal",
		},
		{
			name: "missing user managed identity client id",
			config: &Config{
				Namespace: "test.servicebus.windows.net",
				Auth: Authentication{
					Type: UserManagedIdentity,
				},
				FormatType:   "json",
				MaxEventSize: 1048576,
				BatchSize:    100,
			},
			expectedErr: "client_id cannot be empty when auth type is user_managed_identity",
		},
		{
			name: "missing workload identity fields",
			config: &Config{
				Namespace: "test.servicebus.windows.net",
				Auth: Authentication{
					Type: WorkloadIdentity,
				},
				FormatType:   "json",
				MaxEventSize: 1048576,
				BatchSize:    100,
			},
			expectedErr: "tenant_id, client_id and federated_token_file cannot be empty when auth type is workload_identity",
		},
		{
			name: "invalid format type",
			config: &Config{
				Namespace: "test.servicebus.windows.net",
				Auth: Authentication{
					Type: DefaultCredentials,
				},
				FormatType:   "xml",
				MaxEventSize: 1048576,
				BatchSize:    100,
			},
			expectedErr: "unknown format type: xml",
		},
		{
			name: "invalid max event size - too large",
			config: &Config{
				Namespace: "test.servicebus.windows.net",
				Auth: Authentication{
					Type: DefaultCredentials,
				},
				FormatType:   "json",
				MaxEventSize: 2000000,
				BatchSize:    100,
			},
			expectedErr: "max_event_size must be between 1 and 1048576 bytes (1MB)",
		},
		{
			name: "invalid max event size - zero",
			config: &Config{
				Namespace: "test.servicebus.windows.net",
				Auth: Authentication{
					Type: DefaultCredentials,
				},
				FormatType:   "json",
				MaxEventSize: 0,
				BatchSize:    100,
			},
			expectedErr: "max_event_size must be between 1 and 1048576 bytes (1MB)",
		},
		{
			name: "invalid batch size",
			config: &Config{
				Namespace: "test.servicebus.windows.net",
				Auth: Authentication{
					Type: DefaultCredentials,
				},
				FormatType:   "json",
				MaxEventSize: 1048576,
				BatchSize:    0,
			},
			expectedErr: "batch_size must be greater than 0",
		},
		{
			name: "static partition key without value",
			config: &Config{
				Namespace: "test.servicebus.windows.net",
				Auth: Authentication{
					Type: DefaultCredentials,
				},
				FormatType:   "json",
				MaxEventSize: 1048576,
				BatchSize:    100,
				PartitionKey: PartitionKeyConfig{
					Source: "static",
				},
			},
			expectedErr: "partition_key.value cannot be empty when source is static",
		},
		{
			name: "resource_attribute partition key without value",
			config: &Config{
				Namespace: "test.servicebus.windows.net",
				Auth: Authentication{
					Type: DefaultCredentials,
				},
				FormatType:   "json",
				MaxEventSize: 1048576,
				BatchSize:    100,
				PartitionKey: PartitionKeyConfig{
					Source: "resource_attribute",
				},
			},
			expectedErr: "partition_key.value must specify the attribute name when source is resource_attribute",
		},
		{
			name: "invalid partition key source",
			config: &Config{
				Namespace: "test.servicebus.windows.net",
				Auth: Authentication{
					Type: DefaultCredentials,
				},
				FormatType:   "json",
				MaxEventSize: 1048576,
				BatchSize:    100,
				PartitionKey: PartitionKeyConfig{
					Source: "invalid_source",
				},
			},
			expectedErr: "unknown partition_key.source: invalid_source",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.expectedErr)
			}
		})
	}
}
