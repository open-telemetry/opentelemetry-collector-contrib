// Copyright The OpenTelemetry Authors
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

package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func Test_loadMetadata(t *testing.T) {
	tests := []struct {
		name    string
		want    metadata
		wantErr string
	}{
		{
			name: "all_options.yaml",
			want: metadata{
				Name:           "metricreceiver",
				SemConvVersion: "1.9.0",
				Attributes: map[attributeName]attribute{
					"enumAttribute": {
						Description:  "Attribute with a known set of values.",
						NameOverride: "",
						Enum:         []string{"red", "green", "blue"},
						Type: ValueType{
							ValueType: pcommon.ValueTypeStr,
						},
					},
					"freeFormAttribute": {
						Description:  "Attribute that can take on any value.",
						NameOverride: "",
						Type: ValueType{
							ValueType: pcommon.ValueTypeStr,
						},
					},
					"freeFormAttributeWithValue": {
						Description:  "Attribute that has alternate value set.",
						NameOverride: "state",
						Type: ValueType{
							ValueType: pcommon.ValueTypeStr,
						},
					},
					"booleanValueType": {
						Description:  "Attribute with a boolean value.",
						NameOverride: "0",
						Type: ValueType{
							ValueType: pcommon.ValueTypeBool,
						},
					}},
				Metrics: map[metricName]metric{
					"system.cpu.time": {
						Enabled:               true,
						Description:           "Total CPU seconds broken down by different states.",
						ExtendedDocumentation: "Additional information on CPU Time can be found [here](https://en.wikipedia.org/wiki/CPU_time).",
						Unit:                  "s",
						Sum: &sum{
							MetricValueType: MetricValueType{pmetric.NumberDataPointValueTypeDouble},
							Aggregated:      Aggregated{Aggregation: pmetric.AggregationTemporalityCumulative},
							Mono:            Mono{Monotonic: true},
						},
						Attributes: []attributeName{"freeFormAttribute", "freeFormAttributeWithValue", "enumAttribute", "booleanValueType"},
					},
					"system.cpu.utilization": {
						Enabled:     false,
						Description: "Percentage of CPU time broken down by different states.",
						Unit:        "1",
						Gauge: &gauge{
							MetricValueType: MetricValueType{pmetric.NumberDataPointValueTypeDouble},
						},
						Attributes: []attributeName{"enumAttribute", "booleanValueType"},
					},
				},
			},
		},
		{
			name:    "unknown_metric_attribute.yaml",
			want:    metadata{},
			wantErr: "metric \"system.cpu.time\" refers to undefined attributes: [missing]",
		},
		{
			name: "no_metric_type.yaml",
			want: metadata{},
			wantErr: "metric system.cpu.time doesn't have a metric type key, " +
				"one of the following has to be specified: sum, gauge",
		},
		{
			name:    "no_enabled.yaml",
			want:    metadata{},
			wantErr: "1 error(s) decoding:\n\n* error decoding 'metrics[system.cpu.time]': missing required field: `enabled`",
		},
		{
			name: "two_metric_types.yaml",
			want: metadata{},
			wantErr: "metric system.cpu.time has more than one metric type keys, " +
				"only one of the following has to be specified: sum, gauge",
		},
		{
			name: "no_value_type.yaml",
			want: metadata{},
			wantErr: "1 error(s) decoding:\n\n* error decoding 'metrics[system.cpu.time]': 1 error(s) decoding:\n\n" +
				"* error decoding 'sum': missing required field: `value_type`",
		},
		{
			name: "unknown_value_type.yaml",
			want: metadata{},
			wantErr: "1 error(s) decoding:\n\n* error decoding 'metrics[system.cpu.time]': 1 error(s) decoding:\n\n" +
				"* error decoding 'sum': 1 error(s) decoding:\n\n* error decoding 'value_type': invalid value_type: \"unknown\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadMetadata(filepath.Join("testdata", tt.name))
			if tt.wantErr != "" {
				require.Error(t, err)
				require.EqualError(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
			}
		})
	}
}
