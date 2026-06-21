// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
)

func TestConfigValidate(t *testing.T) {
	validConnStr := "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=key;SharedAccessKey=dGVzdA==;EntityPath=hub"
	authID := component.MustNewID("azure_auth")

	tests := []struct {
		name    string
		cfg     *Config
		wantErr string
	}{
		{
			name:    "missing connection and auth",
			cfg:     &Config{},
			wantErr: errMissingConnection.Error(),
		},
		{
			name:    "invalid connection string",
			cfg:     &Config{Connection: "not-a-connection-string"},
			wantErr: "failed parsing connection string",
		},
		{
			name: "valid connection string",
			cfg:  &Config{Connection: validConnStr},
		},
		{
			name: "auth without event_hub.name",
			cfg: &Config{
				Auth:     &authID,
				EventHub: EventHubConfig{Namespace: "ns.servicebus.windows.net"},
			},
			wantErr: "event_hub.name is required when using auth",
		},
		{
			name: "auth without event_hub.namespace",
			cfg: &Config{
				Auth:     &authID,
				EventHub: EventHubConfig{Name: "myhub"},
			},
			wantErr: "event_hub.namespace is required when using auth",
		},
		{
			name: "valid auth config",
			cfg: &Config{
				Auth: &authID,
				EventHub: EventHubConfig{
					Name:      "myhub",
					Namespace: "ns.servicebus.windows.net",
				},
			},
		},
		{
			name: "partition_logs_by_resource_attributes and partition_logs_by_trace_id are mutually exclusive",
			cfg: &Config{
				Connection:                        validConnStr,
				PartitionLogsByResourceAttributes: true,
				PartitionLogsByTraceID:            true,
			},
			wantErr: errLogsPartitionExclusive.Error(),
		},
		{
			name: "partition_logs_by_resource_attributes alone is valid",
			cfg: &Config{
				Connection:                        validConnStr,
				PartitionLogsByResourceAttributes: true,
			},
		},
		{
			name: "partition_logs_by_trace_id alone is valid",
			cfg: &Config{
				Connection:             validConnStr,
				PartitionLogsByTraceID: true,
			},
		},
		{
			name: "partition_traces_by_id is valid",
			cfg: &Config{
				Connection:          validConnStr,
				PartitionTracesByID: true,
			},
		},
		{
			name: "partition_metrics_by_resource_attributes is valid",
			cfg: &Config{
				Connection:                           validConnStr,
				PartitionMetricsByResourceAttributes: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
