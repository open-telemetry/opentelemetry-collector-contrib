// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricstransformprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor/internal/metadata"
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
						Action:  "update",
						NewName: "new_name",
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
							Include:   "name1",
							MatchType: "strict",
						},
						Action:  "insert",
						NewName: "new_name",
						Operations: []Operation{
							{
								Action:   "add_label",
								NewLabel: "my_label",
								NewValue: "my_value",
							},
						},
					},
					{
						MetricIncludeFilter: FilterConfig{
							Include:   "new_name",
							MatchType: "strict",
							MatchLabels: map[string]string{
								"my_label": "my_value",
							},
						},
						Action:  "insert",
						NewName: "new_name_copy_1",
					},
					{
						MetricIncludeFilter: FilterConfig{
							Include:   "new_name",
							MatchType: "regexp",
							MatchLabels: map[string]string{
								"my_label": ".*label",
							},
						},
						Action:  "insert",
						NewName: "new_name_copy_2",
					},
					{
						MetricIncludeFilter: FilterConfig{
							Include:   "name2",
							MatchType: "",
						},
						Action: "update",
						Operations: []Operation{
							{
								Action:   "update_label",
								Label:    "label",
								NewLabel: "new_label_key",
								ValueActions: []ValueAction{
									{Value: "label1", NewValue: "new_label1"},
								},
							},
							{
								Action:          "aggregate_labels",
								LabelSet:        []string{"new_label1", "label2"},
								AggregationType: "sum",
							},
							{
								Action:           "aggregate_label_values",
								Label:            "new_label1",
								AggregationType:  "sum",
								AggregatedValues: []string{"value1", "value2"},
								NewValue:         "new_value",
							},
						},
					},
					{
						MetricIncludeFilter: FilterConfig{
							Include:   "name3",
							MatchType: "strict",
						},
						Action: "update",
						Operations: []Operation{
							{
								Action:     "delete_label_value",
								Label:      "my_label",
								LabelValue: "delete_me",
							},
						},
					},
					{
						MetricIncludeFilter: FilterConfig{
							Include:   "^regexp (?P<my_label>.*)$",
							MatchType: "regexp",
						},
						Action:       "combine",
						NewName:      "combined_metric_name",
						SubmatchCase: "lower",
					},
					{
						MetricIncludeFilter: FilterConfig{
							Include:   "name2",
							MatchType: "strict",
						},
						Action:              "group",
						GroupResourceLabels: map[string]string{"metric_group": "2"},
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
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
