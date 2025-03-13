// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faroexporter

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
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
			id: component.NewIDWithName("faro", ""),
			expected: &Config{
				QueueSettings: exporterhelper.NewDefaultQueueConfig(),
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				ClientConfig:  confighttp.ClientConfig{Endpoint: "https://faro.example.com/collect"},
			},
		},
		{
			id: component.NewIDWithName("faro", "2"),
			expected: &Config{
				QueueSettings: exporterhelper.NewDefaultQueueConfig(),
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				ClientConfig:  confighttp.ClientConfig{Endpoint: "https://faro.example.com/collect"},
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

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
