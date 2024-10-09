// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter

import (
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter/internal/metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
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
				Url: "https://<account>.blob.core.windows.net/",
				Auth: &Authentication{
					Type:         "service_principal",
					TenantId:     "<tenand id>",
					ClientId:     "<client id>",
					ClientSecret: "<client secret>",
				},
				Container: &Container{
					Metrics: "test",
					Logs:    "test",
					Traces:  "test",
				},
				BlobNameFormat: &BlobNameFormat{
					FormatType: "{{.Year}}/{{.Month}}/{{.Day}}/{{.BlobName}}_{{.Hour}}_{{.Minute}}_{{.Second}}_{{.SerialNum}}.{{.FileExtension}}",
					BlobName: &BlobName{
						Metrics: "metrics",
						Logs:    "logs",
						Traces:  "traces",
					},
					Year:   "2006",
					Month:  "01",
					Day:    "02",
					Hour:   "15",
					Minute: "04",
					Second: "05",
				},
				FormatType: "json",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "smi"),
			expected: &Config{
				Url: "https://<account>.blob.core.windows.net/",
				Auth: &Authentication{
					Type: "system_managed_identity",
				},
				Container: &Container{
					Metrics: "test",
					Logs:    "test",
					Traces:  "test",
				},
				BlobNameFormat: &BlobNameFormat{
					FormatType: "{{.Year}}/{{.Month}}/{{.Day}}/{{.BlobName}}_{{.Hour}}_{{.Minute}}_{{.Second}}_{{.SerialNum}}.{{.FileExtension}}",
					BlobName: &BlobName{
						Metrics: "metrics",
						Logs:    "logs",
						Traces:  "traces",
					},
					Year:   "2006",
					Month:  "01",
					Day:    "02",
					Hour:   "15",
					Minute: "04",
					Second: "05",
				},
				FormatType: "proto",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "umi"),
			expected: &Config{
				Url: "https://<account>.blob.core.windows.net/",
				Auth: &Authentication{
					Type:     "user_managed_identity",
					ClientId: "<user managed identity id>",
				},
				Container: &Container{
					Metrics: "test",
					Logs:    "test",
					Traces:  "test",
				},
				BlobNameFormat: &BlobNameFormat{
					FormatType: "{{.Year}}/{{.Month}}/{{.Day}}/{{.BlobName}}_{{.Hour}}_{{.Minute}}_{{.Second}}_{{.SerialNum}}.{{.FileExtension}}",
					BlobName: &BlobName{
						Metrics: "metrics",
						Logs:    "logs",
						Traces:  "traces",
					},
					Year:   "2006",
					Month:  "01",
					Day:    "02",
					Hour:   "15",
					Minute: "04",
					Second: "05",
				},
				FormatType: "json",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "conn-string"),
			expected: &Config{
				Auth: &Authentication{
					Type:             "connection_string",
					ConnectionString: "DefaultEndpointsProtocol=https;AccountName=<account>;AccountKey=<account key>;EndpointSuffix=core.windows.net",
				},
				Container: &Container{
					Metrics: "test",
					Logs:    "test",
					Traces:  "test",
				},
				BlobNameFormat: &BlobNameFormat{
					FormatType: "{{.Year}}/{{.Month}}/{{.Day}}/{{.BlobName}}_{{.Hour}}_{{.Minute}}_{{.Second}}_{{.SerialNum}}.{{.FileExtension}}",
					BlobName: &BlobName{
						Metrics: "metrics",
						Logs:    "logs",
						Traces:  "traces",
					},
					Year:   "2006",
					Month:  "01",
					Day:    "02",
					Hour:   "15",
					Minute: "04",
					Second: "05",
				},
				FormatType: "json",
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
				assert.EqualError(t, component.ValidateConfig(cfg), tt.errorMessage)
				return
			}
			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
