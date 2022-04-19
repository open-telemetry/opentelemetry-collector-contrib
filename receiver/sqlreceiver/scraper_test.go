// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sqlreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetricsProvider(t *testing.T) {
	provider := scraper{
		client: fakeDBClient{
			[]metricRow{{
				"count":    "42",
				"foo_name": "baz",
				"bar_name": "quux",
			}},
		},
		query: Query{
			Metrics: []Metric{
				{
					MetricName:       "my.metric",
					ValueColumn:      "count",
					AttributeColumns: []string{"foo_name", "bar_name"},
				},
				{
					MetricName:  "my.monotonic.metric",
					ValueColumn: "count",
					IsMonotonic: true,
				},
			},
		},
	}
	metrics, err := provider.Scrape(context.Background())
	require.NoError(t, err)
	rms := metrics.ResourceMetrics()
	assert.Equal(t, 1, rms.Len())
	rm := rms.At(0)
	sms := rm.ScopeMetrics()
	assert.Equal(t, 1, sms.Len())
	sm := sms.At(0)
	ms := sm.Metrics()
	assert.Equal(t, 2, ms.Len())

	{
		gaugeMetric := ms.At(0)
		assert.Equal(t, "my.metric", gaugeMetric.Name())
		gauge := gaugeMetric.Gauge()
		dps := gauge.DataPoints()
		assert.Equal(t, 1, dps.Len())
		dp := dps.At(0)
		assert.EqualValues(t, 42, dp.IntVal())

		attrs := dp.Attributes()
		assert.Equal(t, 2, attrs.Len())
		fooVal, _ := attrs.Get("foo_name")
		assert.Equal(t, "baz", fooVal.AsString())
		barVal, _ := attrs.Get("bar_name")
		assert.Equal(t, "quux", barVal.AsString())
	}

	{
		sumMetric := ms.At(1)
		assert.Equal(t, "my.monotonic.metric", sumMetric.Name())
		sum := sumMetric.Sum()
		dps := sum.DataPoints()
		assert.Equal(t, 1, dps.Len())
		dp := dps.At(0)
		assert.EqualValues(t, 42, dp.IntVal())
		attrs := dp.Attributes()
		assert.Equal(t, 0, attrs.Len())
	}
}
