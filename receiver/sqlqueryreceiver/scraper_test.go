// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"
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
		stringMaps: [][]stringMap{
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
				DataType:    MetricTypeGauge,
			}},
		},
	}
	_, err := scrpr.Scrape(context.Background())
	assert.Error(t, err)
}

func TestScraper_RowToMetricErrorOnScrape_Int(t *testing.T) {
	client := &fakeDBClient{
		stringMaps: [][]stringMap{
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
				DataType:    MetricTypeGauge,
			}},
		},
	}
	_, err := scrpr.Scrape(context.Background())
	assert.Error(t, err)
}

func TestScraper_RowToMetricMultiErrorsOnScrape(t *testing.T) {
	client := &fakeDBClient{
		stringMaps: [][]stringMap{{
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
				DataType:    MetricTypeGauge,
			}},
		},
	}
	_, err := scrpr.Scrape(context.Background())
	assert.Error(t, err)
}

func TestScraper_SingleRow_MultiMetrics(t *testing.T) {
	scrpr := scraper{
		client: &fakeDBClient{
			stringMaps: [][]stringMap{{{
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
					DataType:         MetricTypeGauge,
				},
				{
					MetricName:       "my.metric.2",
					ValueColumn:      "count",
					AttributeColumns: []string{"foo_name", "bar_name"},
					ValueType:        MetricValueTypeInt,
					DataType:         MetricTypeSum,
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
		assert.EqualValues(t, 42, dp.IntValue())
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
		assert.EqualValues(t, 42, dp.IntValue())
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
		stringMaps: [][]stringMap{{
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
					DataType:         MetricTypeGauge,
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
		assert.EqualValues(t, 42, dp.IntValue())
		val, _ := dp.Attributes().Get("genre")
		assert.Equal(t, "action", val.Str())
	}
	{
		metric := ms.At(1)
		dp := metric.Gauge().DataPoints().At(0)
		assert.EqualValues(t, 111, dp.IntValue())
		val, _ := dp.Attributes().Get("genre")
		assert.Equal(t, "sci-fi", val.Str())
	}
}

func TestScraper_MultiResults_CumulativeSum(t *testing.T) {
	client := &fakeDBClient{
		stringMaps: [][]stringMap{
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
				DataType:    MetricTypeSum,
				Aggregation: MetricAggregationCumulative,
			}},
		},
	}
	assertTransactionCount(t, scrpr, 42, pmetric.AggregationTemporalityCumulative)
	assertTransactionCount(t, scrpr, 43, pmetric.AggregationTemporalityCumulative)
}

func TestScraper_MultiResults_DeltaSum(t *testing.T) {
	client := &fakeDBClient{
		stringMaps: [][]stringMap{
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
				DataType:    MetricTypeSum,
				Aggregation: MetricAggregationDelta,
			}},
		},
	}
	assertTransactionCount(t, scrpr, 42, pmetric.AggregationTemporalityDelta)
	assertTransactionCount(t, scrpr, 43, pmetric.AggregationTemporalityDelta)
}

func assertTransactionCount(t *testing.T, scrpr scraper, expected int, agg pmetric.AggregationTemporality) {
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
	assert.EqualValues(t, expected, sum.DataPoints().At(0).IntValue())
}

func TestScraper_Float(t *testing.T) {
	client := &fakeDBClient{
		stringMaps: [][]stringMap{
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
				DataType:    MetricTypeGauge,
			}},
		},
	}
	metrics, err := scrpr.Scrape(context.Background())
	require.NoError(t, err)
	metric := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	assert.Equal(t, 123.4, metric.Gauge().DataPoints().At(0).DoubleValue())
}

func TestScraper_DescriptionAndUnit(t *testing.T) {
	client := &fakeDBClient{
		stringMaps: [][]stringMap{
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

func TestScraper_FakeDB_Warnings(t *testing.T) {
	db := fakeDB{rowVals: [][]any{{42, nil}}}
	logger := zap.NewNop()
	scrpr := scraper{
		client: newDbClient(db, "", logger),
		logger: logger,
		query: Query{
			Metrics: []MetricCfg{{
				MetricName:  "my.name",
				ValueColumn: "col_0",
				Description: "my description",
				Unit:        "my-unit",
			}},
		},
	}
	_, err := scrpr.Scrape(context.Background())
	require.NoError(t, err)
}

func TestScraper_FakeDB_MultiRows_Warnings(t *testing.T) {
	db := fakeDB{rowVals: [][]any{{42, nil}, {43, nil}}}
	logger := zap.NewNop()
	scrpr := scraper{
		client: newDbClient(db, "", logger),
		logger: logger,
		query: Query{
			Metrics: []MetricCfg{{
				MetricName:  "my.col.0",
				ValueColumn: "col_0",
				Description: "my description 0",
				Unit:        "my-unit-0",
			}},
		},
	}
	_, err := scrpr.Scrape(context.Background())
	// No error is expected because we're not actually asking for metrics from the
	// NULL column. Instead the errors from the NULL reads should just log warnings.
	assert.NoError(t, err)
}

func TestScraper_FakeDB_MultiRows_Error(t *testing.T) {
	db := fakeDB{rowVals: [][]any{{42, nil}, {43, nil}}}
	logger := zap.NewNop()
	scrpr := scraper{
		client: newDbClient(db, "", logger),
		logger: logger,
		query: Query{
			Metrics: []MetricCfg{{
				MetricName:  "my.col.0",
				ValueColumn: "col_0",
				Description: "my description 0",
				Unit:        "my-unit-0",
			}, {
				MetricName:  "my.col.1",
				ValueColumn: "col_1",
				Description: "my description 1",
				Unit:        "my-unit-1",
			},
			},
		},
	}
	_, err := scrpr.Scrape(context.Background())
	// We expect an error here not directly because of the NULL values but because
	// the column was also requested in Query.Metrics[1] but wasn't found. It's just
	// a partial scrape error though so it shouldn't cause a scraper shutdown.
	assert.Error(t, err)
	assert.True(t, scrapererror.IsPartialScrapeError(err))
}
