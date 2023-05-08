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

package datadogexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"
)

func TestSendAggregations(t *testing.T) {
	tests := []struct {
		name              string
		cfgMap            *confmap.Conf
		expectedAggrValue bool
		warnings          []string
		err               string
	}{
		{
			name: "both metrics::histograms::send_count_sum_metrics and metrics::histograms::send_aggregation_metrics",
			cfgMap: confmap.NewFromStringMap(map[string]any{
				"metrics": map[string]any{
					"histograms": map[string]any{
						"send_count_sum_metrics":   true,
						"send_aggregation_metrics": true,
					},
				},
			}),
			err: "\"metrics::histograms::send_count_sum_metrics\" and \"metrics::histograms::send_aggregation_metrics\" can't be both set at the same time: use \"metrics::histograms::send_aggregation_metrics\" only instead",
		},
		{
			name: "metrics::histograms::send_count_sum_metrics set to true",
			cfgMap: confmap.NewFromStringMap(map[string]any{
				"metrics": map[string]any{
					"histograms": map[string]any{
						"send_count_sum_metrics": true,
					},
				},
			}),
			expectedAggrValue: true,
			warnings: []string{
				"\"metrics::histograms::send_count_sum_metrics\" has been deprecated in favor of \"metrics::histograms::send_aggregation_metrics\"",
			},
		},
		{
			name: "metrics::histograms::send_count_sum_metrics set to false",
			cfgMap: confmap.NewFromStringMap(map[string]any{
				"metrics": map[string]any{
					"histograms": map[string]any{
						"send_count_sum_metrics": false,
					},
				},
			}),
			warnings: []string{
				"\"metrics::histograms::send_count_sum_metrics\" has been deprecated in favor of \"metrics::histograms::send_aggregation_metrics\"",
			},
		},
		{
			name:              "metrics::histograms::send_count_sum_metrics and metrics::histograms::send_aggregation_metrics unset",
			cfgMap:            confmap.New(),
			expectedAggrValue: false,
		},
		{
			name: "metrics::histograms::send_aggregation_metrics set",
			cfgMap: confmap.NewFromStringMap(map[string]any{
				"metrics": map[string]any{
					"histograms": map[string]any{
						"send_aggregation_metrics": true,
					},
				},
			}),
			expectedAggrValue: true,
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			f := NewFactory()
			cfg := f.CreateDefaultConfig().(*Config)
			err := cfg.Unmarshal(testInstance.cfgMap)
			if err != nil || testInstance.err != "" {
				assert.EqualError(t, err, testInstance.err)
			} else {
				assert.Equal(t, testInstance.expectedAggrValue, cfg.Metrics.HistConfig.SendAggregations)
				var warningStr []string
				for _, warning := range cfg.warnings {
					warningStr = append(warningStr, warning.Error())
				}
				assert.ElementsMatch(t, testInstance.warnings, warningStr)
			}
		})
	}

}
