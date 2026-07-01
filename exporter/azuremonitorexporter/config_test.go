// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	disk := component.MustNewIDWithName("disk", "")

	clientConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	clientConfig.MaxIdleConns = 0
	clientConfig.IdleConnTimeout = 0
	clientConfig.ForceAttemptHTTP2 = false
	clientConfig.Endpoint = "https://dc.services.visualstudio.com/v2/track"

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "2"),
			expected: &Config{
				ConnectionString:                          "InstrumentationKey=00000000-0000-0000-0000-000000000000;IngestionEndpoint=https://ingestion.azuremonitor.com/",
				InstrumentationKey:                        "00000000-0000-0000-0000-000000000000",
				MaxBatchSize:                              100,
				MaxBatchInterval:                          10 * time.Second,
				SpanEventsEnabled:                         false,
				NonErrorHTTPStatusCodes:                   []int{404, 409},
				AlignHTTPServerRequestSuccessWithOTelSpec: true,
				ClientConfig:                              clientConfig,
				QueueSettings: configoptional.Some(func() exporterhelper.QueueBatchConfig {
					queue := exporterhelper.NewDefaultQueueConfig()
					queue.QueueSize = 1000
					queue.NumConsumers = 10
					queue.StorageID = &disk
					return queue
				}()),
				ShutdownTimeout: 2 * time.Second,
				TagMappings: TagMappingsConfig{
					CloudRoleInstance:  []string{"service.instance.id"},
					ApplicationVersion: []string{"service.version"},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "tag_mappings"),
			expected: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.ConnectionString = "InstrumentationKey=00000000-0000-0000-0000-000000000000;IngestionEndpoint=https://ingestion.azuremonitor.com/"
				cfg.TagMappings = TagMappingsConfig{
					CloudRoleInstance:  []string{"host.name", "service.instance.id", "unknown-instance"},
					ApplicationVersion: []string{"service.version"},
				}
				return cfg
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     *Config
		wantErr string
	}{
		{
			name: "default config is valid",
			cfg:  createDefaultConfig().(*Config),
		},
		{
			name: "tag_mappings unset is valid",
			cfg:  &Config{},
		},
		{
			name: "configured tag_mappings are valid",
			cfg: &Config{TagMappings: TagMappingsConfig{
				CloudRoleInstance:  []string{"host.name", "service.instance.id"},
				ApplicationVersion: []string{"service.version", "v0.0.0"},
			}},
		},
		{
			name: "configured non_error_http_status_codes are valid",
			cfg:  &Config{NonErrorHTTPStatusCodes: []int{404, 409}},
		},
		{
			name:    "non_error_http_status_codes rejects status code below valid range",
			cfg:     &Config{NonErrorHTTPStatusCodes: []int{99}},
			wantErr: "non_error_http_status_codes contains invalid HTTP status code 99",
		},
		{
			name:    "non_error_http_status_codes rejects status code above valid range",
			cfg:     &Config{NonErrorHTTPStatusCodes: []int{600}},
			wantErr: "non_error_http_status_codes contains invalid HTTP status code 600",
		},
		{
			name: "explicit empty cloud_role_instance is rejected",
			cfg: &Config{TagMappings: TagMappingsConfig{
				CloudRoleInstance: []string{},
			}},
			wantErr: "tag_mappings.cloud_role_instance must contain at least one source when set",
		},
		{
			name: "explicit empty application_version is rejected",
			cfg: &Config{TagMappings: TagMappingsConfig{
				ApplicationVersion: []string{},
			}},
			wantErr: "tag_mappings.application_version must contain at least one source when set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}
