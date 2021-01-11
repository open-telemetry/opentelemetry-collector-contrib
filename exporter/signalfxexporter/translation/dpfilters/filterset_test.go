// Copyright 2021, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dpfilters

import (
	"testing"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"github.com/stretchr/testify/require"
)

func TestFilterSet(t *testing.T) {
	tests := []struct {
		name                 string
		metricNameInFilter   string
		metricsNamesInFilter []string
		dimensionsInFilter   map[string]interface{}
		expectedMatches      []*sfxpb.DataPoint
		expectedNonMatches   []*sfxpb.DataPoint
		wantErr              bool
		wantErrMsg           string
	}{
		{
			name:               "Match based on simple metric name as string",
			metricNameInFilter: "cpu.utilization",
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
			name:                 "Match based on simple metric name",
			metricsNamesInFilter: []string{"cpu.utilization"},
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
			name:                 "Match based on multiple metric names",
			metricsNamesInFilter: []string{"cpu.utilization", "memory.utilization"},
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
			name:                 "Match based on regex metric name",
			metricsNamesInFilter: []string{`/cpu\..*/`},
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
			name:                 "Match based on glob metric name",
			metricsNamesInFilter: []string{`cpu.util*`, "memor*"},
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
			dimensionsInFilter: map[string]interface{}{
				"container_name": "PO",
			},
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
			dimensionsInFilter: map[string]interface{}{
				"container_name": []interface{}{"PO"},
			},
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
			dimensionsInFilter: map[string]interface{}{
				"container_name": []interface{}{`/^[A-Z][A-Z]$/`},
			},
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
			dimensionsInFilter: map[string]interface{}{
				"container_name": []interface{}{`/.+/`},
			},
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
			dimensionsInFilter: map[string]interface{}{
				"container_name": []interface{}{`*O*`},
			},
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
			name:                 "Match based on conjunction of both dimensions and metric name",
			metricsNamesInFilter: []string{"*.utilization"},
			dimensionsInFilter: map[string]interface{}{
				"container_name": []interface{}{"test"},
			},
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
			name:                 "Doesn't match if no dimension filter specified",
			metricsNamesInFilter: []string{"cpu.utilization"},
			expectedNonMatches: []*sfxpb.DataPoint{
				{
					Metric:     "disk.utilization",
					Dimensions: []*sfxpb.Dimension{{Key: "container_name", Value: "test"}},
				},
			},
		},
		{
			name: "Doesn't match if no metric name filter specified",
			dimensionsInFilter: map[string]interface{}{
				"container_name": []interface{}{"mycontainer"},
			},
			expectedNonMatches: []*sfxpb.DataPoint{
				{
					Metric: "cpu.utilization",
				},
			},
		},
		{
			name: "Doesn't match metric when no (matching) dimensions exist",
			dimensionsInFilter: map[string]interface{}{
				"host":   []interface{}{"localhost"},
				"system": []interface{}{"r4"},
			},
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
			dimensionsInFilter: map[string]interface{}{
				"host":   []interface{}{"localhost"},
				"system": []interface{}{"r4"},
			},
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
			dimensionsInFilter: map[string]interface{}{
				"host":   []interface{}{"localhost"},
				"system": []interface{}{"r4"},
			},
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
			dimensionsInFilter: map[string]interface{}{
				"container_name": []interface{}{"*", "!pause", "!/.*idle/"},
			},
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
			name:       "Error creating empty filter",
			wantErr:    true,
			wantErrMsg: "metric filter must have at least one metric or dimension defined on it",
		},
		{
			name: "Error creating filter with empty dimension list",
			dimensionsInFilter: map[string]interface{}{
				"dim": []interface{}{},
			},
			wantErr:    true,
			wantErrMsg: "string map value in filter cannot be empty",
		},
		{
			name:                 "Error creating filter with invalid glob",
			metricsNamesInFilter: []string{"cpu.*["},
			wantErr:              true,
			wantErrMsg:           "unexpected end of input",
		},
		{
			name: "Error creating filter with invalid glob in dimensions",
			dimensionsInFilter: map[string]interface{}{
				"container_name": []interface{}{"cpu.*["},
			},
			wantErr:    true,
			wantErrMsg: "unexpected end of input",
		},
		{
			name: "Error on invalid dimensions input",
			dimensionsInFilter: map[string]interface{}{
				"host": 1,
			},
			wantErr:    true,
			wantErrMsg: "1 should be either a string or string list",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f, err := NewFilterSet([]MetricFilter{
				{
					MetricName:  test.metricNameInFilter,
					MetricNames: test.metricsNamesInFilter,
					Dimensions:  test.dimensionsInFilter,
				},
			})
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
