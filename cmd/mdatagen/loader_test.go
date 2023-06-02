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
			name: "metadata.yaml",
			want: metadata{
				Type:           "testreceiver",
				SemConvVersion: "1.9.0",
				ResourceAttributes: map[attributeName]attribute{
					"string.resource.attr": {
						Description: "Resource attribute with any string value.",
						Enabled:     true,
						Type: ValueType{
							ValueType: pcommon.ValueTypeStr,
						},
					},
					"string.enum.resource.attr": {
						Description: "Resource attribute with a known set of string values.",
						Enabled:     true,
						Enum:        []string{"one", "two"},
						Type: ValueType{
							ValueType: pcommon.ValueTypeStr,
						},
					},
					"optional.resource.attr": {
						Description: "Explicitly disabled ResourceAttribute.",
						Enabled:     false,
						Type: ValueType{
							ValueType: pcommon.ValueTypeStr,
						},
					},
					"slice.resource.attr": {
						Description: "Resource attribute with a slice value.",
						Enabled:     true,
						Type: ValueType{
							ValueType: pcommon.ValueTypeSlice,
						},
					},
					"map.resource.attr": {
						Description: "Resource attribute with a map value.",
						Enabled:     true,
						Type: ValueType{
							ValueType: pcommon.ValueTypeMap,
						},
					},
				},
				Attributes: map[attributeName]attribute{
					"enum_attr": {
						Description:  "Attribute with a known set of string values.",
						NameOverride: "",
						Enum:         []string{"red", "green", "blue"},
						Type: ValueType{
							ValueType: pcommon.ValueTypeStr,
						},
					},
					"string_attr": {
						Description:  "Attribute with any string value.",
						NameOverride: "",
						Type: ValueType{
							ValueType: pcommon.ValueTypeStr,
						},
					},
					"overridden_int_attr": {
						Description:  "Integer attribute with overridden name.",
						NameOverride: "state",
						Type: ValueType{
							ValueType: pcommon.ValueTypeInt,
						},
					},
					"boolean_attr": {
						Description: "Attribute with a boolean value.",
						Type: ValueType{
							ValueType: pcommon.ValueTypeBool,
						},
					},
					"slice_attr": {
						Description: "Attribute with a slice value.",
						Type: ValueType{
							ValueType: pcommon.ValueTypeSlice,
						},
					},
					"map_attr": {
						Description: "Attribute with a map value.",
						Type: ValueType{
							ValueType: pcommon.ValueTypeMap,
						},
					},
				},
				Metrics: map[metricName]metric{
					"default.metric": {
						Enabled:               true,
						Description:           "Monotonic cumulative sum int metric enabled by default.",
						ExtendedDocumentation: "The metric will be become optional soon.",
						Warnings: warnings{
							IfEnabledNotSet: "This metric will be disabled by default soon.",
						},
						Unit: "s",
						Sum: &sum{
							MetricValueType: MetricValueType{pmetric.NumberDataPointValueTypeInt},
							Aggregated:      Aggregated{Aggregation: pmetric.AggregationTemporalityCumulative},
							Mono:            Mono{Monotonic: true},
						},
						Attributes: []attributeName{"string_attr", "overridden_int_attr", "enum_attr", "slice_attr", "map_attr"},
					},
					"optional.metric": {
						Enabled:     false,
						Description: "[DEPRECATED] Gauge double metric disabled by default.",
						Warnings: warnings{
							IfConfigured: "This metric is deprecated and will be removed soon.",
						},
						Unit: "1",
						Gauge: &gauge{
							MetricValueType: MetricValueType{pmetric.NumberDataPointValueTypeDouble},
						},
						Attributes: []attributeName{"string_attr", "boolean_attr"},
					},
					"default.metric.to_be_removed": {
						Enabled:               true,
						Description:           "[DEPRECATED] Non-monotonic delta sum double metric enabled by default.",
						ExtendedDocumentation: "The metric will be will be removed soon.",
						Warnings: warnings{
							IfEnabled: "This metric is deprecated and will be removed soon.",
						},
						Unit: "s",
						Sum: &sum{
							MetricValueType: MetricValueType{pmetric.NumberDataPointValueTypeDouble},
							Aggregated:      Aggregated{Aggregation: pmetric.AggregationTemporalityDelta},
							Mono:            Mono{Monotonic: false},
						},
					},
				},
			},
		},
		{
			name:    "testdata/unknown_metric_attribute.yaml",
			want:    metadata{},
			wantErr: "metric \"system.cpu.time\" refers to undefined attributes: [missing]",
		},
		{
			name: "testdata/no_metric_type.yaml",
			want: metadata{},
			wantErr: "metric system.cpu.time doesn't have a metric type key, " +
				"one of the following has to be specified: sum, gauge",
		},
		{
			name:    "testdata/no_enabled.yaml",
			want:    metadata{},
			wantErr: "1 error(s) decoding:\n\n* error decoding 'metrics[system.cpu.time]': missing required field: `enabled`",
		},
		{
			name: "testdata/two_metric_types.yaml",
			want: metadata{},
			wantErr: "metric system.cpu.time has more than one metric type keys, " +
				"only one of the following has to be specified: sum, gauge",
		},
		{
			name: "testdata/no_value_type.yaml",
			want: metadata{},
			wantErr: "1 error(s) decoding:\n\n* error decoding 'metrics[system.cpu.time]': 1 error(s) decoding:\n\n" +
				"* error decoding 'sum': missing required field: `value_type`",
		},
		{
			name: "testdata/unknown_value_type.yaml",
			want: metadata{},
			wantErr: "1 error(s) decoding:\n\n* error decoding 'metrics[system.cpu.time]': 1 error(s) decoding:\n\n" +
				"* error decoding 'sum': 1 error(s) decoding:\n\n* error decoding 'value_type': invalid value_type: \"unknown\"",
		},
		{
			name:    "testdata/unused_attribute.yaml",
			want:    metadata{},
			wantErr: "unused attributes: [unused_attr]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadMetadata(tt.name)
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
