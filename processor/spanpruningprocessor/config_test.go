// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanpruningprocessor/internal/metadata"
)

var defaultHistogramBuckets = []time.Duration{
	5 * time.Millisecond,
	10 * time.Millisecond,
	25 * time.Millisecond,
	50 * time.Millisecond,
	100 * time.Millisecond,
	250 * time.Millisecond,
	500 * time.Millisecond,
	time.Second,
	2500 * time.Millisecond,
	5 * time.Second,
	10 * time.Second,
}

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
				GroupByAttributes:           []string{"db.operation"},
				MinSpansToAggregate:         5,
				MaxParentDepth:              1,
				AggregationAttributePrefix:  "aggregation.",
				AggregationHistogramBuckets: defaultHistogramBuckets,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "custom"),
			expected: &Config{
				GroupByAttributes:           []string{"db.operation", "db.name"},
				MinSpansToAggregate:         3,
				MaxParentDepth:              1,
				AggregationAttributePrefix:  "batch.",
				AggregationHistogramBuckets: defaultHistogramBuckets,
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
				AggregationAttributePrefix: "aggregation.",
				GroupByAttributes:          []string{"db.operation"},
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
		{
			name: "empty aggregation_attribute_prefix",
			config: &Config{
				MinSpansToAggregate:        2,
				AggregationAttributePrefix: "",
			},
			expectError: true,
		},
		{
			name: "whitespace-only aggregation_attribute_prefix",
			config: &Config{
				MinSpansToAggregate:        2,
				AggregationAttributePrefix: "   ",
			},
			expectError: true,
		},
		{
			name: "empty group_by_attributes pattern",
			config: &Config{
				MinSpansToAggregate:        2,
				AggregationAttributePrefix: "aggregation.",
				GroupByAttributes:          []string{"db.operation", ""},
			},
			expectError: true,
		},
		{
			name: "whitespace-only group_by_attributes pattern",
			config: &Config{
				MinSpansToAggregate:        2,
				AggregationAttributePrefix: "aggregation.",
				GroupByAttributes:          []string{"db.operation", "   "},
			},
			expectError: true,
		},
		{
			name: "invalid glob pattern in group_by_attributes",
			config: &Config{
				MinSpansToAggregate:        2,
				AggregationAttributePrefix: "aggregation.",
				GroupByAttributes:          []string{"db.operation", "[invalid*"},
			},
			expectError: true,
		},

		{
			name: "max_parent_depth unlimited",
			config: &Config{
				MinSpansToAggregate:        2,
				AggregationAttributePrefix: "aggregation.",
				MaxParentDepth:             -1,
			},
			expectError: false,
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

func TestEnableAttributeLossAnalysis(t *testing.T) {
	factory := NewFactory()

	t.Run("disabled by default", func(t *testing.T) {
		cfg := factory.CreateDefaultConfig().(*Config)
		assert.False(t, cfg.EnableAttributeLossAnalysis)
	})

	t.Run("can be enabled", func(t *testing.T) {
		c := &Config{
			MinSpansToAggregate:         2,
			AggregationAttributePrefix:  "aggregation.",
			EnableAttributeLossAnalysis: true,
		}
		assert.True(t, c.EnableAttributeLossAnalysis)
	})

	t.Run("can be disabled explicitly", func(t *testing.T) {
		c := &Config{
			MinSpansToAggregate:         2,
			AggregationAttributePrefix:  "aggregation.",
			EnableAttributeLossAnalysis: false,
		}
		assert.False(t, c.EnableAttributeLossAnalysis)
	})
}
