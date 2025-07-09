// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id           component.ID
		expected     component.Config
		errorMessage string
	}{
		{
			id: component.NewIDWithName(metadata.Type, "sp"),
			expected: &Config{
				URL: "https://fakeaccount.blob.core.windows.net/",
				Auth: &Authentication{
					Type:         "service_principal",
					TenantID:     "e4b5a5f0-3d6a-4b1c-9e2f-7c8a1b8f2c3d",
					ClientID:     "e4b5a5f0-3d6a-4b1c-9e2f-7c8a1b8f2c3d",
					ClientSecret: "e4b5a5f0-3d6a-4b1c-9e2f-7c8a1b8f2c3d",
				},
				Container: &TelemetryConfig{
					Metrics: "test",
					Logs:    "test",
					Traces:  "test",
				},
				BlobNameFormat: &BlobNameFormat{
					MetricsFormat:  "2006/01/02/metrics_15_04_05.json",
					LogsFormat:     "2006/01/02/logs_15_04_05.json",
					TracesFormat:   "2006/01/02/traces_15_04_05.json",
					SerialNumRange: 10000,
					Params:         map[string]string{},
				},
				FormatType:    "json",
				Encodings:     &Encodings{},
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				AppendBlob: &AppendBlob{
					Enabled:   false,
					Separator: "\n",
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "smi"),
			expected: &Config{
				URL: "https://fakeaccount.blob.core.windows.net/",
				Auth: &Authentication{
					Type: "system_managed_identity",
				},
				Container: &TelemetryConfig{
					Metrics: "test",
					Logs:    "test",
					Traces:  "test",
				},
				BlobNameFormat: &BlobNameFormat{
					MetricsFormat:  "2006/01/02/metrics_15_04_05.json",
					LogsFormat:     "2006/01/02/logs_15_04_05.json",
					TracesFormat:   "2006/01/02/traces_15_04_05.json",
					SerialNumRange: 10000,
					Params:         map[string]string{},
				},
				FormatType:    "proto",
				Encodings:     &Encodings{},
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				AppendBlob: &AppendBlob{
					Enabled:   false,
					Separator: "\n",
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "umi"),
			expected: &Config{
				URL: "https://fakeaccount.blob.core.windows.net/",
				Auth: &Authentication{
					Type:     "user_managed_identity",
					ClientID: "e4b5a5f0-3d6a-4b1c-9e2f-7c8a1b8f2c3d",
				},
				Container: &TelemetryConfig{
					Metrics: "test",
					Logs:    "test",
					Traces:  "test",
				},
				BlobNameFormat: &BlobNameFormat{
					MetricsFormat:  "2006/01/02/metrics_15_04_05.json",
					LogsFormat:     "2006/01/02/logs_15_04_05.json",
					TracesFormat:   "2006/01/02/traces_15_04_05.json",
					SerialNumRange: 10000,
					Params:         map[string]string{},
				},
				FormatType:    "json",
				Encodings:     &Encodings{},
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				AppendBlob: &AppendBlob{
					Enabled:   false,
					Separator: "\n",
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "conn-string"),
			expected: &Config{
				Auth: &Authentication{
					Type:             "connection_string",
					ConnectionString: "DefaultEndpointsProtocol=https;AccountName=fakeaccount;AccountKey=ZmFrZWtleQ==;EndpointSuffix=core.windows.net",
				},
				Container: &TelemetryConfig{
					Metrics: "test",
					Logs:    "test",
					Traces:  "test",
				},
				BlobNameFormat: &BlobNameFormat{
					MetricsFormat:  "2006/01/02/metrics_15_04_05.json",
					LogsFormat:     "2006/01/02/logs_15_04_05.json",
					TracesFormat:   "2006/01/02/traces_15_04_05.json",
					SerialNumRange: 10000,
					Params:         map[string]string{},
				},
				FormatType:    "json",
				Encodings:     &Encodings{},
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				AppendBlob: &AppendBlob{
					Enabled:   false,
					Separator: "\n",
				},
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "err1"),
			errorMessage: "url cannot be empty when auth type is not connection_string",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "err2"),
			errorMessage: "connection_string cannot be empty when auth type is connection_string",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "err3"),
			errorMessage: "tenant_id, client_id and client_secret cannot be empty when auth type is service-principal",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "err4"),
			errorMessage: "client_id cannot be empty when auth type is user_managed_identity",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "err5"),
			errorMessage: "unknown format type: custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.expected == nil {
				assert.EqualError(t, xconfmap.Validate(cfg), tt.errorMessage)
				return
			}
			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
