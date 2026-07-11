// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cumulativetodeltaprocessor

import (
	"maps"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor/internal/tracking"
)

func TestValidMetricTypeListMatchesMap(t *testing.T) {
	fromMap := slices.Collect(maps.Keys(validMetricTypes))
	slices.Sort(fromMap)
	assert.Equal(t, fromMap, validMetricTypeList)
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id           component.ID
		expected     component.Config
		errorMessage string
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				Include: MatchMetrics{
					Metrics: []string{
						"metric1",
						"metric2",
					},
					Config: filterset.Config{
						MatchType:    "strict",
						RegexpConfig: nil,
					},
				},
				Exclude: MatchMetrics{
					Metrics: []string{
						"metric3",
						"metric4",
					},
					Config: filterset.Config{
						MatchType:    "strict",
						RegexpConfig: nil,
					},
				},
				MaxStaleness: 10 * time.Second,
				InitialValue: tracking.InitialValueAuto,
			},
		},
		{
			id:       component.NewIDWithName(metadata.Type, "empty"),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "regexp"),
			expected: &Config{
				Include: MatchMetrics{
					Metrics: []string{
						"a*",
					},
					Config: filterset.Config{
						MatchType:    "regexp",
						RegexpConfig: nil,
					},
				},
				Exclude: MatchMetrics{
					Metrics: []string{
						"b*",
					},
					Config: filterset.Config{
						MatchType:    "regexp",
						RegexpConfig: nil,
					},
				},
				MaxStaleness: 10 * time.Second,
				InitialValue: tracking.InitialValueAuto,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "metric_type_filter"),
			expected: &Config{
				Include: MatchMetrics{
					Metrics: []string{
						"a*",
					},
					Config: filterset.Config{
						MatchType:    "regexp",
						RegexpConfig: nil,
					},
					MetricTypes: []string{
						"sum",
					},
				},
				Exclude: MatchMetrics{
					Metrics: []string{
						"b*",
					},
					Config: filterset.Config{
						MatchType:    "regexp",
						RegexpConfig: nil,
					},
					MetricTypes: []string{
						"histogram",
					},
				},
				MaxStaleness: 10 * time.Second,
				InitialValue: tracking.InitialValueAuto,
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "invalid_include_metric_type_filter"),
			errorMessage: "found invalid metric type in include.metric_types: gauge. Valid values are [exponentialhistogram histogram sum]",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "invalid_exclude_metric_type_filter"),
			errorMessage: "found invalid metric type in exclude.metric_types: Invalid. Valid values are [exponentialhistogram histogram sum]",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "missing_match_type"),
			errorMessage: "match_type must be set if metrics are supplied",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "missing_name"),
			errorMessage: "metrics must be supplied if match_type is set",
		},
		{
			id: component.NewIDWithName(metadata.Type, "auto"),
			expected: &Config{
				MaxStaleness: 1 * time.Hour,
				InitialValue: tracking.InitialValueAuto,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "keep"),
			expected: &Config{
				MaxStaleness: 1 * time.Hour,
				InitialValue: tracking.InitialValueKeep,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "drop"),
			expected: &Config{
				MaxStaleness: 1 * time.Hour,
				InitialValue: tracking.InitialValueDrop,
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

			if tt.expected == nil {
				assert.EqualError(t, xconfmap.Validate(cfg), tt.errorMessage)
				return
			}
			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestLoadDeprecatedConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id           component.ID
		expected     component.Config
		errorMessage string
	}{
		{
			id: component.NewIDWithName(metadata.DeprecatedType, ""),
			expected: &Config{
				Include: MatchMetrics{
					Metrics: []string{
						"metric1",
						"metric2",
					},
					Config: filterset.Config{
						MatchType:    "strict",
						RegexpConfig: nil,
					},
				},
				Exclude: MatchMetrics{
					Metrics: []string{
						"metric3",
						"metric4",
					},
					Config: filterset.Config{
						MatchType:    "strict",
						RegexpConfig: nil,
					},
				},
				MaxStaleness: 10 * time.Second,
				InitialValue: tracking.InitialValueAuto,
			},
		},
		{
			id:       component.NewIDWithName(metadata.DeprecatedType, "empty"),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.DeprecatedType, "regexp"),
			expected: &Config{
				Include: MatchMetrics{
					Metrics: []string{
						"a*",
					},
					Config: filterset.Config{
						MatchType:    "regexp",
						RegexpConfig: nil,
					},
				},
				Exclude: MatchMetrics{
					Metrics: []string{
						"b*",
					},
					Config: filterset.Config{
						MatchType:    "regexp",
						RegexpConfig: nil,
					},
				},
				MaxStaleness: 10 * time.Second,
				InitialValue: tracking.InitialValueAuto,
			},
		},
		{
			id: component.NewIDWithName(metadata.DeprecatedType, "metric_type_filter"),
			expected: &Config{
				Include: MatchMetrics{
					Metrics: []string{
						"a*",
					},
					Config: filterset.Config{
						MatchType:    "regexp",
						RegexpConfig: nil,
					},
					MetricTypes: []string{
						"sum",
					},
				},
				Exclude: MatchMetrics{
					Metrics: []string{
						"b*",
					},
					Config: filterset.Config{
						MatchType:    "regexp",
						RegexpConfig: nil,
					},
					MetricTypes: []string{
						"histogram",
					},
				},
				MaxStaleness: 10 * time.Second,
				InitialValue: tracking.InitialValueAuto,
			},
		},
		{
			id:           component.NewIDWithName(metadata.DeprecatedType, "invalid_include_metric_type_filter"),
			errorMessage: "found invalid metric type in include.metric_types: gauge. Valid values are [exponentialhistogram histogram sum]",
		},
		{
			id:           component.NewIDWithName(metadata.DeprecatedType, "invalid_exclude_metric_type_filter"),
			errorMessage: "found invalid metric type in exclude.metric_types: Invalid. Valid values are [exponentialhistogram histogram sum]",
		},
		{
			id:           component.NewIDWithName(metadata.DeprecatedType, "missing_match_type"),
			errorMessage: "match_type must be set if metrics are supplied",
		},
		{
			id:           component.NewIDWithName(metadata.DeprecatedType, "missing_name"),
			errorMessage: "metrics must be supplied if match_type is set",
		},
		{
			id: component.NewIDWithName(metadata.DeprecatedType, "auto"),
			expected: &Config{
				MaxStaleness: 1 * time.Hour,
				InitialValue: tracking.InitialValueAuto,
			},
		},
		{
			id: component.NewIDWithName(metadata.DeprecatedType, "keep"),
			expected: &Config{
				MaxStaleness: 1 * time.Hour,
				InitialValue: tracking.InitialValueKeep,
			},
		},
		{
			id: component.NewIDWithName(metadata.DeprecatedType, "drop"),
			expected: &Config{
				MaxStaleness: 1 * time.Hour,
				InitialValue: tracking.InitialValueDrop,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config-deprecated.yaml"))
			require.NoError(t, err)

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
