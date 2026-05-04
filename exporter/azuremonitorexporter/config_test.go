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
				ConnectionString:   "InstrumentationKey=00000000-0000-0000-0000-000000000000;IngestionEndpoint=https://ingestion.azuremonitor.com/",
				InstrumentationKey: "00000000-0000-0000-0000-000000000000",
				MaxBatchSize:       100,
				MaxBatchInterval:   10 * time.Second,
				SpanEventsEnabled:  false,
				ClientConfig: confighttp.ClientConfig{
					Endpoint: "https://dc.services.visualstudio.com/v2/track",
				},
				QueueSettings: configoptional.Some(func() exporterhelper.QueueBatchConfig {
					queue := exporterhelper.NewDefaultQueueConfig()
					queue.QueueSize = 1000
					queue.NumConsumers = 10
					queue.StorageID = &disk
					return queue
				}()),
				ShutdownTimeout: 2 * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "3"),
			expected: &Config{
				ConnectionString:                       "InstrumentationKey=00000000-0000-0000-0000-000000000000;IngestionEndpoint=https://ingestion.azuremonitor.com/",
				MaxBatchSize:                           1024,
				MaxBatchInterval:                       10 * time.Second,
				ShutdownTimeout:                        1 * time.Second,
				NonErrorHTTPStatusCodes:                []int{404, 409},
				AlignHTTPServerSpanSuccessWithOTelSpec: true,
				QueueSettings:                          configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
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

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfigValidate_NonErrorHTTPStatusCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		codes   []int
		wantErr string
	}{
		{name: "empty list ok", codes: nil},
		{name: "valid range ok", codes: []int{100, 200, 404, 599}},
		{name: "below 100 rejected", codes: []int{99}, wantErr: "99 is not a valid HTTP status code"},
		{name: "above 599 rejected", codes: []int{600}, wantErr: "600 is not a valid HTTP status code"},
		{name: "negative rejected", codes: []int{-1}, wantErr: "-1 is not a valid HTTP status code"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{NonErrorHTTPStatusCodes: tt.codes}
			err := cfg.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}
