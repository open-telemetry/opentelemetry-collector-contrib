// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/stretchr/testify/require"
)

func newDimension(value string) *armmonitor.LocalizableString {
	return to.Ptr(armmonitor.LocalizableString{Value: to.Ptr(value)})
}

func TestFilterDimensions(t *testing.T) {
	type args struct {
		dimensions   []*armmonitor.LocalizableString
		cfg          DimensionsConfig
		resourceType string
		metricName   string
	}

	tests := []struct {
		name     string
		args     args
		expected []string
	}{
		{
			name: "always empty if dimensions disabled",
			args: args{
				dimensions: []*armmonitor.LocalizableString{
					newDimension("foo"),
					newDimension("bar"),
				},
				cfg: DimensionsConfig{
					Enabled: to.Ptr(false),
				},
				resourceType: "rt1",
				metricName:   "m1",
			},
			expected: nil,
		},
		{
			name: "split by dimensions should be enabled by default",
			args: args{
				dimensions: []*armmonitor.LocalizableString{
					newDimension("foo"),
					newDimension("bar"),
				},
				cfg:          DimensionsConfig{}, // enabled by default
				resourceType: "rt1",
				metricName:   "m1",
			},
			expected: []string{"foo", "bar"},
		},
		{
			name: "overrides takes precedence over input",
			args: args{
				dimensions: []*armmonitor.LocalizableString{
					newDimension("foo"),
					newDimension("bar"),
				},
				cfg: DimensionsConfig{
					Enabled: to.Ptr(true),
					Overrides: NestedListAlias{
						"rt1": {
							"m1": {
								"foo",
							},
						},
					},
				},
				resourceType: "rt1",
				metricName:   "m1",
			},
			expected: []string{"foo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := filterDimensions(tt.args.dimensions, tt.args.cfg, tt.args.resourceType, tt.args.metricName)
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestBuildDimensionsFilter(t *testing.T) {
	type args struct {
		dimensionsStr string
	}

	tests := []struct {
		name     string
		args     args
		expected *string
	}{
		{
			name: "empty given dimensions string",
			args: args{
				dimensionsStr: "",
			},
			expected: nil,
		},
		{
			name: "build dimensions filter",
			args: args{
				dimensionsStr: "bar,foo",
			},
			expected: to.Ptr("bar eq '*' and foo eq '*'"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := buildDimensionsFilter(tt.args.dimensionsStr)
			require.EqualValues(t, tt.expected, actual)
		})
	}
}

func TestSerializeDimensions(t *testing.T) {
	type args struct {
		dimensions []string
	}

	tests := []struct {
		name     string
		args     args
		expected string
	}{
		{
			name: "empty given dimensions",
			args: args{
				dimensions: []string{},
			},
			expected: "",
		},
		{
			name: "nil given dimensions",
			args: args{
				dimensions: []string{},
			},
			expected: "",
		},
		{
			name: "reorder dimensions",
			args: args{
				dimensions: []string{"foo", "bar"},
			},
			expected: "bar,foo",
		},
		{
			name: "trim spaces dimensions",
			args: args{
				dimensions: []string{"  bar", "foo "},
			},
			expected: "bar,foo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := serializeDimensions(tt.args.dimensions)
			require.EqualValues(t, tt.expected, actual)
		})
	}
}
