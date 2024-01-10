// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuredataexplorerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter/internal/metadata"
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
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				ClusterURI:     "https://CLUSTER.kusto.windows.net",
				ApplicationID:  "f80da32c-108c-415c-a19e-643f461a677a",
				ApplicationKey: "xx-xx-xx-xx",
				TenantID:       "21ff9e36-fbaa-43c8-98ba-00431ea10bc3",
				Database:       "oteldb",
				MetricTable:    "OTELMetrics",
				LogTable:       "OTELLogs",
				TraceTable:     "OTELTraces",
				IngestionType:  managedIngestType,
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "2"),
			errorMessage: `either ["application_id" , "application_key" , "tenant_id"] or ["managed_identity_id"] are needed for auth`,
		},
		{
			id:           component.NewIDWithName(metadata.Type, "3"),
			errorMessage: `unsupported configuration for ingestion_type. Accepted types [managed, queued] Provided [streaming]`,
		},
		{
			id: component.NewIDWithName(metadata.Type, "4"),
			expected: &Config{
				ClusterURI:        "https://CLUSTER.kusto.windows.net",
				ManagedIdentityID: "bf61f0ec-1f01-11ee-be56-0242ac120002",
				Database:          "oteldb",
				MetricTable:       "OTELMetrics",
				LogTable:          "OTELLogs",
				TraceTable:        "OTELTraces",
				IngestionType:     managedIngestType,
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "5"),
			errorMessage: `managed_identity_id should be a UUID string (for User Managed Identity) or system (for System Managed Identity)`,
		},
		{
			id: component.NewIDWithName(metadata.Type, "6"),
			expected: &Config{
				ClusterURI:        "https://CLUSTER.kusto.windows.net",
				ManagedIdentityID: "system",
				Database:          "oteldb",
				MetricTable:       "OTELMetrics",
				LogTable:          "OTELLogs",
				TraceTable:        "OTELTraces",
				IngestionType:     managedIngestType,
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "7"),
			errorMessage: `clusterURI config is mandatory`,
		},
		{
			id: component.NewIDWithName(metadata.Type, "8"),
			expected: &Config{
				ClusterURI:     "https://CLUSTER.kusto.windows.net",
				ApplicationID:  "f80da32c-108c-415c-a19e-643f461a677a",
				ApplicationKey: "xx-xx-xx-xx",
				TenantID:       "21ff9e36-fbaa-43c8-98ba-00431ea10bc3",
				Database:       "oteldb",
				MetricTable:    "OTELMetrics",
				LogTable:       "OTELLogs",
				TraceTable:     "OTELTraces",
				IngestionType:  managedIngestType,
				TimeoutSettings: exporterhelper.TimeoutSettings{
					Timeout: 10 * time.Second,
				},
				BackOffConfig: configretry.BackOffConfig{
					Enabled:         true,
					InitialInterval: 10 * time.Second,
					MaxInterval:     60 * time.Second,
					MaxElapsedTime:  10 * time.Minute,
				},
				QueueSettings: exporterhelper.QueueSettings{
					Enabled:      true,
					NumConsumers: 2,
					QueueSize:    10,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			if tt.expected == nil {
				assert.EqualError(t, component.ValidateConfig(cfg), tt.errorMessage)
				return
			}
			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
