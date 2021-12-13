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
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_loadMetadata(t *testing.T) {
	tests := []struct {
		name    string
		yml     string
		want    metadata
		wantErr string
	}{
		{
			name: "all options",
			yml:  "all_options.yaml",
			want: metadata{
				Name: "metricreceiver",
				Attributes: map[attributeName]attribute{
					"enumAttribute": {
						Description: "Attribute with a known set of values.",
						Value:       "",
						Enum:        []string{"red", "green", "blue"}},
					"freeFormAttribute": {
						Description: "Attribute that can take on any value.",
						Value:       ""},
					"freeFormAttributeWithValue": {
						Description: "Attribute that has alternate value set.",
						Value:       "state"}},
				Metrics: map[metricName]metric{
					"system.cpu.time": {
						Description:           "Total CPU seconds broken down by different states.",
						ExtendedDocumentation: "Additional information on CPU Time can be found [here](https://en.wikipedia.org/wiki/CPU_time).",
						Unit:                  "s",
						Sum: &sum{
							Aggregated: Aggregated{Aggregation: "cumulative"},
							Mono:       Mono{Monotonic: true},
						},
						// YmlData: nil,
						Attributes: []attributeName{"freeFormAttribute", "freeFormAttributeWithValue",
							"enumAttribute"}}},
			},
		},
		{
			name: "unknown metric attribute",
			yml:  "unknown_metric_attribute.yaml",
			want: metadata{},
			wantErr: "error validating struct:\n\tmetadata.Metrics[system.cpu.time]." +
				"Attributes[missing]: unknown attribute value\n",
		},
		{
			name: "no metric type",
			yml:  "no_metric_type.yaml",
			want: metadata{},
			wantErr: "metric system.cpu.time doesn't have a metric type key, " +
				"one of the following has to be specified: sum, gauge, histogram",
		},
		{
			name: "two metric types",
			yml:  "two_metric_types.yaml",
			want: metadata{},
			wantErr: "metric system.cpu.time has more than one metric type keys, " +
				"only one of the following has to be specified: sum, gauge, histogram",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadMetadata(path.Join("testdata", tt.yml))
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
