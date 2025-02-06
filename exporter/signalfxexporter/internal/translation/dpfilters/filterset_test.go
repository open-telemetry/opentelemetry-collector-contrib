// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dpfilters

import (
	"testing"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"github.com/stretchr/testify/require"
)

func TestFilterSet(t *testing.T) {
	tests := []struct {
		name               string
		excludes           []MetricFilter
		includes           []MetricFilter
		expectedMatches    []*sfxpb.DataPoint
		expectedNonMatches []*sfxpb.DataPoint
		wantErr            bool
		wantErrMsg         string
	}{
		{
			name:     "Match based on simple metric name as string",
			excludes: []MetricFilter{{MetricName: "cpu.utilization"}},
			expectedMatches: []*sfxpb.DataPoint{
				{
					Metric: "cpu.utilization",
				},
			},
			expectedNonMatches: []*sfxpb.DataPoint{
				{
					Metric: "memory.utilization",
				},
			},
		},
		{
			name:     "Match based on simple metric name",
			excludes: []MetricFilter{{MetricNames: []string{"cpu.utilization"}}},
			expectedMatches: []*sfxpb.DataPoint{
				{
					Metric: "cpu.utilization",
				},
			},
			expectedNonMatches: []*sfxpb.DataPoint{
				{
					Metric: "memory.utilization",
				},
			},
		},
		{
			name:     "Match based on multiple metric names",
			excludes: []MetricFilter{{MetricNames: []string{"cpu.utilization", "memory.utilization"}}},
			expectedMatches: []*sfxpb.DataPoint{
				{
					Metric: "cpu.utilization",
				},
				{
					Metric: "memory.utilization",
				},
			},
			expectedNonMatches: []*sfxpb.DataPoint{
				{
					Metric: "disk.utilization",
				},
			},
		},
		{
			name:     "Match based on regex metric name",
			excludes: []MetricFilter{{MetricNames: []string{`/cpu\..*/`}}},
			expectedMatches: []*sfxpb.DataPoint{
				{
					Metric: "cpu.utilization",
				},
			},
			expectedNonMatches: []*sfxpb.DataPoint{
				{
					Metric: "disk.utilization",
				},
			},
		},
		{
			name:     "Match based on glob metric name",
			excludes: []MetricFilter{{MetricNames: []string{`cpu.util*`, "memor*"}}},
			expectedMatches: []*sfxpb.DataPoint{
				{
					Metric: "cpu.utilization",
				},
				{
					Metric: "memory.utilization",
				},
			},
			expectedNonMatches: []*sfxpb.DataPoint{
				{
					Metric: "disk.utilization",
				},
			},
		},
		{
			name: "Match based on dimension name as string",
			excludes: []MetricFilter{{
				Dimensions: map[string]any{
					"container_name": "PO",
				},
			}},
			expectedMatches: []*sfxpb.DataPoint{
				{
					Metric:     "cpu.utilization",
					Dimensions: []*sfxpb.Dimension{{Key: "container_name", Value: "PO"}},
				},
			},
			expectedNonMatches: []*sfxpb.DataPoint{
				{
					Metric:     "disk.utilization",
					Dimensions: []*sfxpb.Dimension{{Key: "container_name", Value: "test"}},
				},
			},
		},
		{
			name: "Match based on dimension name",
			excludes: []MetricFilter{{
				Dimensions: map[string]any{
					"container_name": []any{"PO"},
				},
			}},
			expectedMatches: []*sfxpb.DataPoint{
				{
					Metric:     "cpu.utilization",
					Dimensions: []*sfxpb.Dimension{{Key: "container_name", Value: "PO"}},
				},
			},
			expectedNonMatches: []*sfxpb.DataPoint{
				{
					Metric:     "disk.utilization",
					Dimensions: []*sfxpb.Dimension{{Key: "container_name", Value: "test"}},
				},
			},
		},
		{
			name: "Match based on dimension name regex",
			excludes: []MetricFilter{{
				Dimensions: map[string]any{
					"container_name": []any{`/^[A-Z][A-Z]$/`},
				},
			}},
			expectedMatches: []*sfxpb.DataPoint{
				{
					Metric:     "cpu.utilization",
					Dimensions: []*sfxpb.Dimension{{Key: "container_name", Value: "PO"}},
				},
			},
			expectedNonMatches: []*sfxpb.DataPoint{
				{
					Metric:     "disk.utilization",
					Dimensions: []*sfxpb.Dimension{{Key: "container_name", Value: "test"}},
				},
			},
		},
		{
			name: "Match based on dimension presence",
			excludes: []MetricFilter{{
				Dimensions: map[string]any{
					"container_name": []any{`/.+/`},
				},
			}},
			expectedMatches: []*sfxpb.DataPoint{
				{
					Metric:     "cpu.utilization",
					Dimensions: []*sfxpb.Dimension{{Key: "container_name", Value: "test"}},
				},
			},
			expectedNonMatches: []*sfxpb.DataPoint{
				{
					Metric:     "cpu.utilization",
					Dimensions: []*sfxpb.Dimension{{Key: "host", Value: "localhost"}},
				},
			},
		},
		{
			name: "Match based on dimension name glob",
			excludes: []MetricFilter{{
				Dimensions: map[string]any{
					"container_name": []any{`*O*`},
				},
			}},
			expectedMatches: []*sfxpb.DataPoint{
				{
					Metric:     "cpu.utilization",
					Dimensions: []*sfxpb.Dimension{{Key: "container_name", Value: "POD"}},
				},
				{
					Metric:     "cpu.utilization",
					Dimensions: []*sfxpb.Dimension{{Key: "container_name", Value: "POD123"}},
				},
			},
			expectedNonMatches: []*sfxpb.DataPoint{
				{
					Metric:     "disk.utilization",
					Dimensions: []*sfxpb.Dimension{{Key: "container_name", Value: "test"}},
				},
			},
		},
		{
			name: "Match based on conjunction of both dimensions and metric name",
			excludes: []MetricFilter{{
				MetricNames: []string{"*.utilization"},
				Dimensions: map[string]any{
					"container_name": []any{"test"},
				},
			}},
			expectedMatches: []*sfxpb.DataPoint{
				{
					Metric:     "disk.utilization",
					Dimensions: []*sfxpb.Dimension{{Key: "container_name", Value: "test"}},
				},
			},
			expectedNonMatches: []*sfxpb.DataPoint{
				{
					Metric:     "cpu.utilization",
					Dimensions: []*sfxpb.Dimension{{Key: "container_name", Value: "not matching"}},
				}, {
					Metric:     "disk.usage",
					Dimensions: []*sfxpb.Dimension{{Key: "container_name", Value: "test"}},
				},
			},
		},
		{
			name:     "Doesn't match if no dimension filter specified",
			excludes: []MetricFilter{{MetricNames: []string{"cpu.utilization"}}},
			expectedNonMatches: []*sfxpb.DataPoint{
				{
					Metric:     "disk.utilization",
					Dimensions: []*sfxpb.Dimension{{Key: "container_name", Value: "test"}},
				},
			},
		},
		{
			name: "Doesn't match if no metric name filter specified",
			excludes: []MetricFilter{{
				Dimensions: map[string]any{
					"container_name": []any{"mycontainer"},
				},
			}},
			expectedNonMatches: []*sfxpb.DataPoint{
				{
					Metric: "cpu.utilization",
				},
			},
		},
		{
			name: "Doesn't match metric when no (matching) dimensions exist",
			excludes: []MetricFilter{{
				Dimensions: map[string]any{
					"host":   []any{"localhost"},
					"system": []any{"r4"},
				},
			}},
			expectedNonMatches: []*sfxpb.DataPoint{
				{
					Metric: "cpu.utilization",
				},
				{
					Metric: "cpu.utilization",
					Dimensions: []*sfxpb.Dimension{
						{Key: "Host", Value: "localhost"},
					},
				},
			},
		},
		{
			name: "Matches on at least one dimension",
			excludes: []MetricFilter{{
				Dimensions: map[string]any{
					"host":   []any{"localhost"},
					"system": []any{"r4"},
				},
			}},
			expectedMatches: []*sfxpb.DataPoint{
				{
					Metric: "cpu.utilization",
					Dimensions: []*sfxpb.Dimension{
						{Key: "host", Value: "localhost"},
					},
				},
			},
		},
		{
			name: "Matches against all dimension pairs",
			excludes: []MetricFilter{{
				Dimensions: map[string]any{
					"host":   []any{"localhost"},
					"system": []any{"r4"},
				},
			}},
			expectedMatches: []*sfxpb.DataPoint{
				{
					Metric: "cpu.utilization",
					Dimensions: []*sfxpb.Dimension{
						{Key: "host", Value: "localhost"},
						{Key: "system", Value: "r4"},
					},
				},
			},
			expectedNonMatches: []*sfxpb.DataPoint{
				{
					Metric: "cpu.utilization",
					Dimensions: []*sfxpb.Dimension{
						{Key: "host", Value: "localhost"},
						{Key: "system", Value: "r3"},
					},
				},
			},
		},
		{
			name: "Negated dim values take precedent",
			excludes: []MetricFilter{{
				Dimensions: map[string]any{
					"container_name": []any{"*", "!pause", "!/.*idle/"},
				},
			}},
			expectedMatches: []*sfxpb.DataPoint{
				{
					Metric:     "cpu.utilization",
					Dimensions: []*sfxpb.Dimension{{Key: "container_name", Value: "mycontainer"}},
				},
			},
			expectedNonMatches: []*sfxpb.DataPoint{
				{
					Metric: "cpu.utilization",
				},
				{
					Metric:     "cpu.utilization",
					Dimensions: []*sfxpb.Dimension{{Key: "container_name", Value: "pause"}},
				},
				{
					Metric:     "cpu.utilization",
					Dimensions: []*sfxpb.Dimension{{Key: "container_name", Value: "is_idle"}},
				},
			},
		},
		{
			name:       "Error creating exclude empty filter",
			excludes:   []MetricFilter{{}},
			wantErr:    true,
			wantErrMsg: "metric filter must have at least one metric or dimension defined on it",
		},
		{
			name:       "Error creating include empty filter",
			includes:   []MetricFilter{{}},
			wantErr:    true,
			wantErrMsg: "metric filter must have at least one metric or dimension defined on it",
		},
		{
			name: "Error creating filter with empty dimension list",
			excludes: []MetricFilter{{
				Dimensions: map[string]any{
					"dim": []any{},
				},
			}},
			wantErr:    true,
			wantErrMsg: "string map value in filter cannot be empty",
		},
		{
			name:       "Error creating filter with invalid glob",
			excludes:   []MetricFilter{{MetricNames: []string{"cpu.*["}}},
			wantErr:    true,
			wantErrMsg: "unexpected end of input",
		},
		{
			name: "Error creating filter with invalid glob in dimensions",
			excludes: []MetricFilter{{
				Dimensions: map[string]any{
					"container_name": []any{"cpu.*["},
				},
			}},
			wantErr:    true,
			wantErrMsg: "unexpected end of input",
		},
		{
			name: "Error on invalid dimensions input",
			excludes: []MetricFilter{{
				Dimensions: map[string]any{
					"host": 1,
				},
			}},
			wantErr:    true,
			wantErrMsg: "1 should be either a string or string list",
		},
		{
			name:     "Match in include filters correctly overrides exclude",
			excludes: []MetricFilter{{MetricNames: []string{"cpu.utilization"}}},
			includes: []MetricFilter{{MetricNames: []string{"cpu.utilization"}}},
			expectedNonMatches: []*sfxpb.DataPoint{
				{
					Metric: "cpu.utilization",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f, err := NewFilterSet(test.excludes, test.includes)
			if test.wantErr {
				require.EqualError(t, err, test.wantErrMsg)
				require.Nil(t, f)
				return
			}
			require.NoError(t, err)

			for _, metric := range test.expectedMatches {
				require.True(t, f.Matches(metric))
			}

			for _, metric := range test.expectedNonMatches {
				require.False(t, f.Matches(metric))
			}
		})
	}
}
