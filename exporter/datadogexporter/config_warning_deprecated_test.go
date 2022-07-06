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

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/service/featuregate"
)

func TestDeprecationSendMonotonic(t *testing.T) {
	// Override 'exporter.datadog.hostname.preview' feature flag value to remove warning
	featuregate.GetRegistry().Apply(map[string]bool{metadata.HostnamePreviewFeatureGate: true})
	defer featuregate.GetRegistry().Apply(map[string]bool{metadata.HostnamePreviewFeatureGate: false})

	tests := []struct {
		name         string
		cfgMap       *confmap.Conf
		expectedMode CumulativeMonotonicSumMode
		warnings     []string
		err          string
	}{
		{
			name: "both metrics::send_monotonic and new metrics::sums::cumulative_monotonic_mode",
			cfgMap: confmap.NewFromStringMap(map[string]interface{}{
				"metrics": map[string]interface{}{
					"send_monotonic_counter": true,
					"sums": map[string]interface{}{
						"cumulative_monotonic_mode": "to_delta",
					},
				},
			}),
			err: "\"metrics::send_monotonic_counter\" and \"metrics::sums::cumulative_monotonic_mode\" can't be both set at the same time: use \"metrics::sums::cumulative_monotonic_mode\" only instead",
		},
		{
			name: "metrics::send_monotonic set to true",
			cfgMap: confmap.NewFromStringMap(map[string]interface{}{
				"metrics": map[string]interface{}{
					"send_monotonic_counter": true,
				},
			}),
			expectedMode: CumulativeMonotonicSumModeToDelta,
			warnings: []string{
				"\"metrics::send_monotonic_counter\" has been deprecated in favor of \"metrics::sums::cumulative_monotonic_mode\" and will be removed in v0.50.0 or later. See github.com/open-telemetry/opentelemetry-collector-contrib/issues/8489",
			},
		},
		{
			name: "metrics::send_monotonic set to false",
			cfgMap: confmap.NewFromStringMap(map[string]interface{}{
				"metrics": map[string]interface{}{
					"send_monotonic_counter": false,
				},
			}),
			expectedMode: CumulativeMonotonicSumModeRawValue,
			warnings: []string{
				"\"metrics::send_monotonic_counter\" has been deprecated in favor of \"metrics::sums::cumulative_monotonic_mode\" and will be removed in v0.50.0 or later. See github.com/open-telemetry/opentelemetry-collector-contrib/issues/8489",
			},
		},
		{
			name:         "metrics::send_monotonic and metrics::sums::cumulative_monotonic_mode unset",
			cfgMap:       confmap.NewFromStringMap(map[string]interface{}{}),
			expectedMode: CumulativeMonotonicSumModeToDelta,
		},
		{
			name: "metrics::sums::cumulative_monotonic_mode set",
			cfgMap: confmap.NewFromStringMap(map[string]interface{}{
				"metrics": map[string]interface{}{
					"sums": map[string]interface{}{
						"cumulative_monotonic_mode": "raw_value",
					},
				},
			}),
			expectedMode: CumulativeMonotonicSumModeRawValue,
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			cfg := futureDefaultConfig()
			err := cfg.Unmarshal(testInstance.cfgMap)
			if err != nil || testInstance.err != "" {
				assert.EqualError(t, err, testInstance.err)
			} else {
				assert.Equal(t, testInstance.expectedMode, cfg.Metrics.SumConfig.CumulativeMonotonicMode)
				var warningStr []string
				for _, warning := range cfg.warnings {
					warningStr = append(warningStr, warning.Error())
				}
				assert.ElementsMatch(t, testInstance.warnings, warningStr)
			}
		})
	}

}
