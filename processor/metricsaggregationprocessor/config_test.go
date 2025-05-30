// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsaggregationprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricsaggregationprocessor/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		configFile string
		id         component.ID
		expected   component.Config
	}{
		{
			configFile: "config_full.yaml",
			id:         component.NewID(metadata.Type),
			expected: &Config{
				Transforms: []transform{
					{
						MetricIncludeFilter: FilterConfig{
							Include:   "name",
							MatchType: "",
						},
						Action: "combine",
						Operations: []Operation{
							{
								Action:          "aggregate_labels",
								LabelSet:        []string{"new_label1", "label2"},
								AggregationType: "sum",
							},
						},
					},
				},
			},
		},
		{
			configFile: "config_full.yaml",
			id:         component.NewIDWithName(metadata.Type, "multiple"),
			expected: &Config{
				Transforms: []transform{
					{
						MetricIncludeFilter: FilterConfig{
							Include:   "name2",
							MatchType: "strict",
						},
						Action: "insert",
						Operations: []Operation{
							{
								Action:          "aggregate_labels",
								LabelSet:        []string{"new_label1", "label2"},
								AggregationType: "sum",
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", tt.configFile))
			require.NoError(t, err)

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
