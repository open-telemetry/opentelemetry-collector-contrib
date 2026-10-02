// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package partitioningprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/partitioningprocessor/internal/metadata"
)

func TestConfig_Validate_NoKeys(t *testing.T) {
	cfg := &Config{}
	assert.Error(t, cfg.Validate())
}

func TestConfig_Validate_EmptyValue(t *testing.T) {
	cfg := &Config{Keys: map[string]string{"foo": ""}}
	assert.Error(t, cfg.Validate())
}

func TestConfig_Validate_NegativeMaxConcurrentPartitions(t *testing.T) {
	cfg := &Config{
		Keys:                    map[string]string{"tenant_id": `resource.attributes["tenant.id"]`},
		MaxConcurrentPartitions: -1,
	}
	assert.Error(t, cfg.Validate())
}

func TestConfig_Validate_OK(t *testing.T) {
	cfg := &Config{Keys: map[string]string{
		"tenant_id": `resource.attributes["tenant.id"]`,
	}}
	assert.NoError(t, cfg.Validate())
}

func TestConfiguredMaxConcurrentPartitions_Default(t *testing.T) {
	assert.Equal(t, defaultMaxConcurrentPartitions(), configuredMaxConcurrentPartitions(&Config{}))
}

func TestConfiguredMaxConcurrentPartitions_Explicit(t *testing.T) {
	assert.Equal(t, 7, configuredMaxConcurrentPartitions(&Config{MaxConcurrentPartitions: 7}))
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id          component.ID
		expected    *Config
		expectedErr string
	}{
		{
			id: component.NewID(metadata.Type),
			expected: &Config{
				Keys: map[string]string{
					"tenant_id": `resource.attributes["tenant.id"]`,
					"severity":  "log.severity_text",
				},
				MaxConcurrentPartitions: 4,
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "no_keys"),
			expectedErr: "at least one key must be configured",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "empty_expression"),
			expectedErr: `key "tenant_id" has an empty value expression`,
		},
		{
			id:          component.NewIDWithName(metadata.Type, "negative_concurrency"),
			expectedErr: "max_concurrent_partitions must be greater than or equal to 0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cfg := NewFactory().CreateDefaultConfig()
			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.expectedErr != "" {
				assert.ErrorContains(t, cfg.(*Config).Validate(), tt.expectedErr)
				return
			}
			assert.NoError(t, cfg.(*Config).Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
