// Copyright 2020, OpenTelemetry Authors
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

package translation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/translation/dpfilters"
)

func TestGetExcludeMetricsRule(t *testing.T) {
	rule := GetExcludeMetricsRule([]dpfilters.MetricFilter{
		{
			MetricName: "m0",
		},
		{
			MetricNames: []string{"m1"},
			Dimensions: map[string]interface{}{
				"d1": []string{"dv1"},
			},
		},
	})
	assert.Equal(t, rule.Action, ActionDropMetrics)
	require.Equal(t, 2, len(rule.MetricFilters))
	assert.Equal(t, "m0", rule.MetricFilters[0].MetricName)
	require.Equal(t, 1, len(rule.MetricFilters[1].MetricNames))
	assert.Equal(t, "m1", rule.MetricFilters[1].MetricNames[0])
	assert.Equal(t, []string{"dv1"}, rule.MetricFilters[1].Dimensions["d1"])
}
