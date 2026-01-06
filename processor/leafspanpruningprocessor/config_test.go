// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package leafspanpruningprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/leafspanpruningprocessor/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id           component.ID
		expected     *Config
		errorMessage string
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				GroupByAttributes:          []string{"db.operation"},
				MinSpansToAggregate:        2,
				SummarySpanNameSuffix:      "_aggregated",
				AggregationAttributePrefix: "aggregation.",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "custom"),
			expected: &Config{
				GroupByAttributes:          []string{"db.operation", "db.name"},
				MinSpansToAggregate:        3,
				SummarySpanNameSuffix:      "_batch",
				AggregationAttributePrefix: "batch.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			oCfg := cfg.(*Config)
			if tt.errorMessage != "" {
				assert.EqualError(t, oCfg.Validate(), tt.errorMessage)
				return
			}

			assert.NoError(t, oCfg.Validate())
			assert.Equal(t, tt.expected, oCfg)
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid config",
			config: &Config{
				MinSpansToAggregate:        2,
				SummarySpanNameSuffix:      "_aggregated",
				AggregationAttributePrefix: "aggregation.",
			},
			expectError: false,
		},
		{
			name: "min_spans_to_aggregate below minimum",
			config: &Config{
				MinSpansToAggregate: 1,
			},
			expectError: true,
		},
		{
			name: "min_spans_to_aggregate zero",
			config: &Config{
				MinSpansToAggregate: 0,
			},
			expectError: true,
		},
		{
			name: "min_spans_to_aggregate negative",
			config: &Config{
				MinSpansToAggregate: -1,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
