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

package sqlqueryreceiver

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestScraper_ErrorOnStart(t *testing.T) {
	scrpr := scraper{
		dbProviderFunc: func() (*sql.DB, error) {
			return nil, errors.New("oops")
		},
	}
	err := scrpr.Start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err)
}

func TestScraper_ClientErrorOnScrape(t *testing.T) {
	client := &fakeDBClient{
		err: errors.New("oops"),
	}
	scrpr := scraper{
		client: client,
	}
	_, err := scrpr.Scrape(context.Background())
	require.Error(t, err)
}

func TestScraper_RowToMetricErrorOnScrape_Float(t *testing.T) {
	client := &fakeDBClient{
		responses: [][]metricRow{
			{{"myfloat": "blah"}},
		},
	}
	scrpr := scraper{
		client: client,
		query: Query{
			Metrics: []MetricCfg{{
				MetricName:  "my.float",
				ValueColumn: "myfloat",
				Monotonic:   true,
				ValueType:   MetricValueTypeDouble,
				DataType:    MetricDataTypeGauge,
			}},
		},
	}
	_, err := scrpr.Scrape(context.Background())
	const expected = "scraper.Scrape row conversion errors: row 0: rowToMetric: " +
		"setDataPointValue: error converting to double: " +
		"strconv.ParseFloat: parsing \"blah\": invalid syntax"
	assert.EqualError(t, err, expected)
}

func TestScraper_RowToMetricErrorOnScrape_Int(t *testing.T) {
	client := &fakeDBClient{
		responses: [][]metricRow{
			{{"myint": "blah"}},
		},
	}
	scrpr := scraper{
		client: client,
		query: Query{
			Metrics: []MetricCfg{{
				MetricName:  "my.int",
				ValueColumn: "myint",
				Monotonic:   true,
				ValueType:   MetricValueTypeInt,
				DataType:    MetricDataTypeGauge,
			}},
		},
	}
	_, err := scrpr.Scrape(context.Background())
	const expected = "scraper.Scrape row conversion errors: row 0: rowToMetric: " +
		"setDataPointValue: error converting to integer: " +
		"strconv.Atoi: parsing \"blah\": invalid syntax"
	assert.EqualError(t, err, expected)
}

func TestScraper_RowToMetricMultiErrorsOnScrape(t *testing.T) {
	client := &fakeDBClient{
		responses: [][]metricRow{{
			{"myint": "foo"},
			{"myint": "bar"},
		}},
	}
	scrpr := scraper{
		client: client,
		query: Query{
			Metrics: []MetricCfg{{
				MetricName:  "my.col",
				ValueColumn: "mycol",
				Monotonic:   true,
				ValueType:   MetricValueTypeInt,
				DataType:    MetricDataTypeGauge,
			}},
		},
	}
	_, err := scrpr.Scrape(context.Background())
	const expected = "scraper.Scrape row conversion errors: " +
		"row 0: rowToMetric: value_column 'mycol' not found in result set; " +
		"row 1: rowToMetric: value_column 'mycol' not found in result set"
	assert.EqualError(t, err, expected)
}

func TestScraper_SingleRow_MultiMetrics(t *testing.T) {
	scrpr := scraper{
		client: &fakeDBClient{
			responses: [][]metricRow{{{
				"count":    "42",
				"foo_name": "baz",
				"bar_name": "quux",
			}}},
		},
		query: Query{
			Metrics: []MetricCfg{
				{
					MetricName:       "my.metric.1",
					ValueColumn:      "count",
					AttributeColumns: []string{"foo_name", "bar_name"},
					ValueType:        MetricValueTypeInt,
					DataType:         MetricDataTypeGauge,
				},
				{
					MetricName:       "my.metric.2",
					ValueColumn:      "count",
					AttributeColumns: []string{"foo_name", "bar_name"},
					ValueType:        MetricValueTypeInt,
					DataType:         MetricDataTypeSum,
					Aggregation:      MetricAggregationCumulative,
				},
			},
		},
	}
	metrics, err := scrpr.Scrape(context.Background())
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
		assert.Equal(t, "my.metric.1", gaugeMetric.Name())
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
		assert.Equal(t, "my.metric.2", sumMetric.Name())
		sum := sumMetric.Sum()
		dps := sum.DataPoints()
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
}

func TestScraper_MultiRow(t *testing.T) {
	client := &fakeDBClient{
		responses: [][]metricRow{{
			{
				"count": "42",
				"genre": "action",
			},
			{
				"count": "111",
				"genre": "sci-fi",
			},
		}},
	}
	scrpr := scraper{
		client: client,
		query: Query{
			Metrics: []MetricCfg{
				{
					MetricName:       "movie.genre",
					ValueColumn:      "count",
					AttributeColumns: []string{"genre"},
					ValueType:        MetricValueTypeInt,
					DataType:         MetricDataTypeGauge,
				},
			},
		},
	}
	metrics, err := scrpr.Scrape(context.Background())
	require.NoError(t, err)
	ms := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	{
		metric := ms.At(0)
		dp := metric.Gauge().DataPoints().At(0)
		assert.EqualValues(t, 42, dp.IntVal())
		val, _ := dp.Attributes().Get("genre")
		assert.Equal(t, "action", val.StringVal())
	}
	{
		metric := ms.At(1)
		dp := metric.Gauge().DataPoints().At(0)
		assert.EqualValues(t, 111, dp.IntVal())
		val, _ := dp.Attributes().Get("genre")
		assert.Equal(t, "sci-fi", val.StringVal())
	}
}

func TestScraper_MultiResults_CumulativeSum(t *testing.T) {
	client := &fakeDBClient{
		responses: [][]metricRow{
			{{"count": "42"}},
			{{"count": "43"}},
		},
	}
	scrpr := scraper{
		client: client,
		query: Query{
			Metrics: []MetricCfg{{
				MetricName:  "transaction.count",
				ValueColumn: "count",
				ValueType:   MetricValueTypeInt,
				DataType:    MetricDataTypeSum,
				Aggregation: MetricAggregationCumulative,
			}},
		},
	}
	assertTransactionCount(t, scrpr, 42, pmetric.MetricAggregationTemporalityCumulative)
	assertTransactionCount(t, scrpr, 43, pmetric.MetricAggregationTemporalityCumulative)
}

func TestScraper_MultiResults_DeltaSum(t *testing.T) {
	client := &fakeDBClient{
		responses: [][]metricRow{
			{{"count": "42"}},
			{{"count": "43"}},
		},
	}
	scrpr := scraper{
		client: client,
		query: Query{
			Metrics: []MetricCfg{{
				MetricName:  "transaction.count",
				ValueColumn: "count",
				ValueType:   MetricValueTypeInt,
				DataType:    MetricDataTypeSum,
				Aggregation: MetricAggregationDelta,
			}},
		},
	}
	assertTransactionCount(t, scrpr, 42, pmetric.MetricAggregationTemporalityDelta)
	assertTransactionCount(t, scrpr, 43, pmetric.MetricAggregationTemporalityDelta)
}

func assertTransactionCount(t *testing.T, scrpr scraper, expected int, agg pmetric.MetricAggregationTemporality) {
	metrics, err := scrpr.Scrape(context.Background())
	require.NoError(t, err)
	metric := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	assert.Equal(t, "transaction.count", metric.Name())
	sum := metric.Sum()
	assert.Equal(
		t,
		agg,
		sum.AggregationTemporality(),
	)
	assert.EqualValues(t, expected, sum.DataPoints().At(0).IntVal())
}

func TestScraper_Float(t *testing.T) {
	client := &fakeDBClient{
		responses: [][]metricRow{
			{{"myfloat": "123.4"}},
		},
	}
	scrpr := scraper{
		client: client,
		query: Query{
			Metrics: []MetricCfg{{
				MetricName:  "my.float",
				ValueColumn: "myfloat",
				Monotonic:   true,
				ValueType:   MetricValueTypeDouble,
				DataType:    MetricDataTypeGauge,
			}},
		},
	}
	metrics, err := scrpr.Scrape(context.Background())
	require.NoError(t, err)
	metric := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	assert.Equal(t, 123.4, metric.Gauge().DataPoints().At(0).DoubleVal())
}

func TestScraper_DescriptionAndUnit(t *testing.T) {
	client := &fakeDBClient{
		responses: [][]metricRow{
			{{"mycol": "123"}},
		},
	}
	scrpr := scraper{
		client: client,
		query: Query{
			Metrics: []MetricCfg{{
				MetricName:  "my.name",
				ValueColumn: "mycol",
				Description: "my description",
				Unit:        "my-unit",
			}},
		},
	}
	metrics, err := scrpr.Scrape(context.Background())
	require.NoError(t, err)
	z := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	assert.Equal(t, "my-unit", z.Unit())
	assert.Equal(t, "my description", z.Description())
}
