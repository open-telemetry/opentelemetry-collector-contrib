// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
				Type:           "file",
				SemConvVersion: "1.9.0",
				Status: &Status{
					Class: "receiver",
					Stability: map[string][]string{
						"development": {"logs"},
						"beta":        {"traces"},
						"stable":      {"metrics"},
					},
					Distributions: []string{"contrib"},
					Warnings:      []string{"Any additional information that should be brought to the consumer's attention"},
				},
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
				ScopeName:       "otelcol",
				ShortFolderName: ".",
			},
		},
		{
			name: "testdata/parent.yaml",
			want: metadata{
				Type:            "subcomponent",
				Parent:          "parentComponent",
				ScopeName:       "otelcol",
				ShortFolderName: "testdata",
			},
		},
		{
			name:    "testdata/invalid_type_rattr.yaml",
			want:    metadata{},
			wantErr: "1 error(s) decoding:\n\n* error decoding 'resource_attributes[string.resource.attr].type': invalid type: \"invalidtype\"",
		},
		{
			name:    "testdata/no_enabled.yaml",
			want:    metadata{},
			wantErr: "1 error(s) decoding:\n\n* error decoding 'metrics[system.cpu.time]': missing required field: `enabled`",
		},
		{
			name: "testdata/no_value_type.yaml",
			want: metadata{},
			wantErr: "1 error(s) decoding:\n\n* error decoding 'metrics[system.cpu.time]': 1 error(s) decoding:\n\n" +
				"* error decoding 'sum': missing required field: `value_type`",
		},
		{
			name:    "testdata/unknown_value_type.yaml",
			wantErr: "1 error(s) decoding:\n\n* error decoding 'metrics[system.cpu.time]': 1 error(s) decoding:\n\n* error decoding 'sum': 1 error(s) decoding:\n\n* error decoding 'value_type': invalid value_type: \"unknown\"",
		},
		{
			name:    "testdata/no_aggregation.yaml",
			want:    metadata{},
			wantErr: "1 error(s) decoding:\n\n* error decoding 'metrics[default.metric]': 1 error(s) decoding:\n\n* error decoding 'sum': missing required field: `aggregation`",
		},
		{
			name:    "testdata/invalid_aggregation.yaml",
			want:    metadata{},
			wantErr: "1 error(s) decoding:\n\n* error decoding 'metrics[default.metric]': 1 error(s) decoding:\n\n* error decoding 'sum': 1 error(s) decoding:\n\n* error decoding 'aggregation': invalid aggregation: \"invalidaggregation\"",
		},
		{
			name:    "testdata/invalid_type_attr.yaml",
			want:    metadata{},
			wantErr: "1 error(s) decoding:\n\n* error decoding 'attributes[used_attr].type': invalid type: \"invalidtype\"",
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
