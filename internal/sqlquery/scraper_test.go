// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlquery // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"
)

func TestScraper_ErrorOnStart(t *testing.T) {
	scrpr := Scraper{
		InstrumentationScope: pcommon.NewInstrumentationScope(),
		DbProviderFunc: func() (*sql.DB, error) {
			return nil, errors.New("oops")
		},
	}
	err := scrpr.Start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err)
}

func TestScraper_ClientErrorOnScrape(t *testing.T) {
	client := &FakeDBClient{
		Err: errors.New("oops"),
	}
	scrpr := Scraper{
		InstrumentationScope: pcommon.NewInstrumentationScope(),
		Client:               client,
	}
	_, err := scrpr.ScrapeMetrics(context.Background())
	require.Error(t, err)
}

func TestScraper_RowToMetricErrorOnScrape_Float(t *testing.T) {
	client := &FakeDBClient{
		StringMaps: [][]StringMap{
			{{"myfloat": "blah"}},
		},
	}
	scrpr := Scraper{
		InstrumentationScope: pcommon.NewInstrumentationScope(),
		Client:               client,
		Query: Query{
			Metrics: []MetricCfg{{
				MetricName:  "my.float",
				ValueColumn: "myfloat",
				Monotonic:   true,
				ValueType:   MetricValueTypeDouble,
				DataType:    MetricTypeGauge,
			}},
		},
	}
	_, err := scrpr.ScrapeMetrics(context.Background())
	assert.Error(t, err)
}

func TestScraper_RowToMetricErrorOnScrape_Int(t *testing.T) {
	client := &FakeDBClient{
		StringMaps: [][]StringMap{
			{{"myint": "blah"}},
		},
	}
	scrpr := Scraper{
		InstrumentationScope: pcommon.NewInstrumentationScope(),
		Client:               client,
		Query: Query{
			Metrics: []MetricCfg{{
				MetricName:  "my.int",
				ValueColumn: "myint",
				Monotonic:   true,
				ValueType:   MetricValueTypeInt,
				DataType:    MetricTypeGauge,
			}},
		},
	}
	_, err := scrpr.ScrapeMetrics(context.Background())
	assert.Error(t, err)
}

func TestScraper_RowToMetricMultiErrorsOnScrape(t *testing.T) {
	client := &FakeDBClient{
		StringMaps: [][]StringMap{{
			{"myint": "foo"},
			{"myint": "bar"},
		}},
	}
	scrpr := Scraper{
		InstrumentationScope: pcommon.NewInstrumentationScope(),
		Client:               client,
		Query: Query{
			Metrics: []MetricCfg{{
				MetricName:  "my.col",
				ValueColumn: "mycol",
				Monotonic:   true,
				ValueType:   MetricValueTypeInt,
				DataType:    MetricTypeGauge,
			}},
		},
	}
	_, err := scrpr.ScrapeMetrics(context.Background())
	assert.Error(t, err)
}

func TestScraper_SingleRow_MultiMetrics(t *testing.T) {
	scrpr := Scraper{
		InstrumentationScope: pcommon.NewInstrumentationScope(),
		Client: &FakeDBClient{
			StringMaps: [][]StringMap{{{
				"count":    "42",
				"foo_name": "baz",
				"bar_name": "quux",
			}}},
		},
		Query: Query{
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
	metrics, err := scrpr.ScrapeMetrics(context.Background())
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
	client := &FakeDBClient{
		StringMaps: [][]StringMap{{
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
	scrpr := Scraper{
		InstrumentationScope: pcommon.NewInstrumentationScope(),
		Client:               client,
		Query: Query{
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
	metrics, err := scrpr.ScrapeMetrics(context.Background())
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
	client := &FakeDBClient{
		StringMaps: [][]StringMap{
			{{"count": "42"}},
			{{"count": "43"}},
		},
	}
	scrpr := Scraper{
		InstrumentationScope: pcommon.NewInstrumentationScope(),
		Client:               client,
		Query: Query{
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
	client := &FakeDBClient{
		StringMaps: [][]StringMap{
			{{"count": "42"}},
			{{"count": "43"}},
		},
	}
	scrpr := Scraper{
		InstrumentationScope: pcommon.NewInstrumentationScope(),
		Client:               client,
		Query: Query{
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

func assertTransactionCount(t *testing.T, scrpr Scraper, expected int, agg pmetric.AggregationTemporality) {
	metrics, err := scrpr.ScrapeMetrics(context.Background())
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
	client := &FakeDBClient{
		StringMaps: [][]StringMap{
			{{"myfloat": "123.4"}},
		},
	}
	scrpr := Scraper{
		InstrumentationScope: pcommon.NewInstrumentationScope(),
		Client:               client,
		Query: Query{
			Metrics: []MetricCfg{{
				MetricName:  "my.float",
				ValueColumn: "myfloat",
				Monotonic:   true,
				ValueType:   MetricValueTypeDouble,
				DataType:    MetricTypeGauge,
			}},
		},
	}
	metrics, err := scrpr.ScrapeMetrics(context.Background())
	require.NoError(t, err)
	metric := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	assert.Equal(t, 123.4, metric.Gauge().DataPoints().At(0).DoubleValue())
}

func TestScraper_DescriptionAndUnit(t *testing.T) {
	client := &FakeDBClient{
		StringMaps: [][]StringMap{
			{{"mycol": "123"}},
		},
	}
	scrpr := Scraper{
		InstrumentationScope: pcommon.NewInstrumentationScope(),
		Client:               client,
		Query: Query{
			Metrics: []MetricCfg{{
				MetricName:  "my.name",
				ValueColumn: "mycol",
				Description: "my description",
				Unit:        "my-unit",
			}},
		},
	}
	metrics, err := scrpr.ScrapeMetrics(context.Background())
	require.NoError(t, err)
	z := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	assert.Equal(t, "my-unit", z.Unit())
	assert.Equal(t, "my description", z.Description())
}

func TestScraper_FakeDB_Warnings(t *testing.T) {
	db := fakeDB{rowVals: [][]any{{42, nil}}}
	logger := zap.NewNop()
	scrpr := Scraper{
		InstrumentationScope: pcommon.NewInstrumentationScope(),
		Client:               NewDbClient(db, "", logger, TelemetryConfig{}),
		Logger:               logger,
		Query: Query{
			Metrics: []MetricCfg{{
				MetricName:  "my.name",
				ValueColumn: "col_0",
				Description: "my description",
				Unit:        "my-unit",
			}},
		},
	}
	_, err := scrpr.ScrapeMetrics(context.Background())
	require.NoError(t, err)
}

func TestScraper_FakeDB_MultiRows_Warnings(t *testing.T) {
	db := fakeDB{rowVals: [][]any{{42, nil}, {43, nil}}}
	logger := zap.NewNop()
	scrpr := Scraper{
		InstrumentationScope: pcommon.NewInstrumentationScope(),
		Client:               NewDbClient(db, "", logger, TelemetryConfig{}),
		Logger:               logger,
		Query: Query{
			Metrics: []MetricCfg{{
				MetricName:  "my.col.0",
				ValueColumn: "col_0",
				Description: "my description 0",
				Unit:        "my-unit-0",
			}},
		},
	}
	_, err := scrpr.ScrapeMetrics(context.Background())
	// No error is expected because we're not actually asking for metrics from the
	// NULL column. Instead the errors from the NULL reads should just log warnings.
	assert.NoError(t, err)
}

func TestScraper_FakeDB_MultiRows_Error(t *testing.T) {
	db := fakeDB{rowVals: [][]any{{42, nil}, {43, nil}}}
	logger := zap.NewNop()
	scrpr := Scraper{
		InstrumentationScope: pcommon.NewInstrumentationScope(),
		Client:               NewDbClient(db, "", logger, TelemetryConfig{}),
		Logger:               logger,
		Query: Query{
			Metrics: []MetricCfg{
				{
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
	_, err := scrpr.ScrapeMetrics(context.Background())
	// We expect an error here not directly because of the NULL values but because
	// the column was also requested in Query.Metrics[1] but wasn't found. It's just
	// a partial scrape error though so it shouldn't cause a Scraper shutdown.
	assert.Error(t, err)
	assert.True(t, scrapererror.IsPartialScrapeError(err))
}

func TestScraper_StartAndTSColumn(t *testing.T) {
	client := &FakeDBClient{
		StringMaps: [][]StringMap{{
			{
				"mycol":   "42",
				"StartTs": "1682417791",
				"Ts":      "1682418264",
			},
		}},
	}
	scrpr := Scraper{
		InstrumentationScope: pcommon.NewInstrumentationScope(),
		Client:               client,
		Query: Query{
			Metrics: []MetricCfg{{
				MetricName:    "my.name",
				ValueColumn:   "mycol",
				TsColumn:      "Ts",
				StartTsColumn: "StartTs",
				DataType:      MetricTypeSum,
				Aggregation:   MetricAggregationCumulative,
			}},
		},
	}
	metrics, err := scrpr.ScrapeMetrics(context.Background())
	require.NoError(t, err)
	metric := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	assert.Equal(t, pcommon.Timestamp(1682417791), metric.Sum().DataPoints().At(0).StartTimestamp())
	assert.Equal(t, pcommon.Timestamp(1682418264), metric.Sum().DataPoints().At(0).Timestamp())
}

func TestScraper_StartAndTS_ErrorOnColumnNotFound(t *testing.T) {
	client := &FakeDBClient{
		StringMaps: [][]StringMap{{
			{
				"mycol":   "42",
				"StartTs": "1682417791",
			},
		}},
	}
	scrpr := Scraper{
		InstrumentationScope: pcommon.NewInstrumentationScope(),
		Client:               client,
		Query: Query{
			Metrics: []MetricCfg{{
				MetricName:    "my.name",
				ValueColumn:   "mycol",
				TsColumn:      "Ts",
				StartTsColumn: "StartTs",
				DataType:      MetricTypeSum,
				Aggregation:   MetricAggregationCumulative,
			}},
		},
	}
	_, err := scrpr.ScrapeMetrics(context.Background())
	assert.Error(t, err)
}

func TestScraper_CollectRowToMetricsErrors(t *testing.T) {
	client := &FakeDBClient{
		StringMaps: [][]StringMap{{
			{
				"mycol": "42",
			},
		}},
	}
	scrpr := Scraper{
		InstrumentationScope: pcommon.NewInstrumentationScope(),
		Client:               client,
		Query: Query{
			Metrics: []MetricCfg{{
				MetricName:       "my.name",
				ValueColumn:      "mycol_na",
				TsColumn:         "Ts",
				StartTsColumn:    "StartTs",
				AttributeColumns: []string{"attr_na"},
				DataType:         MetricTypeSum,
				Aggregation:      MetricAggregationCumulative,
			}},
		},
	}
	_, err := scrpr.ScrapeMetrics(context.Background())
	assert.ErrorContains(t, err, "rowToMetric: start_ts_column not found")
	assert.ErrorContains(t, err, "rowToMetric: ts_column not found")
	assert.ErrorContains(t, err, "rowToMetric: value_column 'mycol_na' not found in result set")
	assert.ErrorContains(t, err, "rowToMetric: attribute_column 'attr_na' not found in result set")
}

func TestScraper_StartAndTS_ErrorOnParse(t *testing.T) {
	client := &FakeDBClient{
		StringMaps: [][]StringMap{{
			{
				"mycol":   "42",
				"StartTs": "blah",
			},
		}},
	}
	scrpr := Scraper{
		InstrumentationScope: pcommon.NewInstrumentationScope(),
		Client:               client,
		Query: Query{
			Metrics: []MetricCfg{{
				MetricName:    "my.name",
				ValueColumn:   "mycol",
				StartTsColumn: "StartTs",
				DataType:      MetricTypeSum,
				Aggregation:   MetricAggregationCumulative,
			}},
		},
	}
	_, err := scrpr.ScrapeMetrics(context.Background())
	assert.Error(t, err)
}

func TestBuildDataSourceString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		driver      string
		host        string
		port        int
		database    string
		username    string
		password    string
		queryParams map[string]any
		expected    string
		wantErr     bool
	}{
		{
			name:     "postgresql basic",
			driver:   "postgres",
			host:     "localhost",
			port:     5432,
			database: "postgres",
			expected: "postgresql://localhost:5432/postgres",
		},
		{
			name:     "postgresql with username and password",
			driver:   "postgres",
			host:     "localhost",
			port:     5432,
			database: "postgres",
			username: "user",
			password: "pass",
			expected: "postgresql://user:pass@localhost:5432/postgres",
		},
		{
			name:        "postgresql with ssl",
			driver:      "postgres",
			host:        "localhost",
			port:        5432,
			database:    "postgres",
			username:    "user",
			password:    "pass",
			queryParams: map[string]any{"sslmode": "require"},
			expected:    "postgresql://user:pass@localhost:5432/postgres?sslmode=require",
		},
		{
			name:        "postgresql with ssl and additional params",
			driver:      "postgres",
			host:        "localhost",
			port:        5432,
			database:    "postgres",
			username:    "user",
			password:    "pass",
			queryParams: map[string]any{"sslmode": "require", "param1": "value1", "param2": "value2"},
			expected:    "postgresql://user:pass@localhost:5432/postgres?param1=value1&param2=value2&sslmode=require",
		},
		{
			name:     "mysql basic",
			driver:   "mysql",
			host:     "localhost",
			port:     3306,
			database: "mydb",
			expected: "tcp(localhost:3306)/mydb",
		},
		{
			name:        "mysql with tls",
			driver:      "mysql",
			host:        "localhost",
			port:        3306,
			database:    "mydb",
			username:    "user",
			password:    "pass",
			queryParams: map[string]any{"tls": true},
			expected:    "user:pass@tcp(localhost:3306)/mydb?tls=true",
		},
		{
			name:        "snowflake basic",
			driver:      "snowflake",
			host:        "account",
			port:        443,
			database:    "mydb",
			username:    "user",
			password:    "pass",
			queryParams: map[string]any{"schema": "PUBLIC", "warehouse": "WH", "role": "SYSADMIN"},
			expected:    "user:pass@account:443/mydb?role=SYSADMIN&schema=PUBLIC&warehouse=WH",
		},
		{
			name:     "sqlserver basic",
			driver:   "sqlserver",
			host:     "localhost",
			port:     1433,
			database: "mydb",
			expected: "sqlserver://localhost:1433?database=mydb",
		},
		{
			name:        "sqlserver with username and password",
			driver:      "sqlserver",
			host:        "localhost",
			port:        1433,
			database:    "mydb",
			username:    "user",
			password:    "pass",
			queryParams: map[string]any{"encrypt": true},
			expected:    "sqlserver://user:pass@localhost:1433?database=mydb&encrypt=true",
		},
		{
			name:     "oracle basic",
			driver:   "oracle",
			host:     "localhost",
			port:     2484,
			database: "service_name",
			expected: "oracle://localhost:2484/service_name",
		},
		{
			name:        "oracle with username and password",
			driver:      "oracle",
			host:        "localhost",
			port:        2484,
			database:    "service_name",
			username:    "user2",
			password:    "pass2",
			queryParams: map[string]any{"retry_count": 3, "timeout": 30},
			expected:    "oracle://user2:pass2@localhost:2484/service_name?retry_count=3&timeout=30",
		},
		{
			name:     "unsupported database type",
			driver:   "unsupported-db",
			host:     "localhost",
			port:     1234,
			database: "mydb",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "postgresql with invalid username",
			driver:   "postgres",
			host:     "localhost",
			port:     5432,
			database: "mydb",
			username: "user@domain",
			password: "pass",
			expected: "postgresql://user%40domain:pass@localhost:5432/mydb",
		},
		{
			name:     "postgresql with invalid password",
			driver:   "postgres",
			host:     "localhost",
			port:     5432,
			database: "mydb",
			username: "user",
			password: "pass#word@123%456&789=0",
			expected: "postgresql://user:pass%23word%40123%25456%26789%3D0@localhost:5432/mydb",
		},
		{
			name:     "postgresql with invalid username and password",
			driver:   "postgres",
			host:     "localhost",
			port:     5432,
			database: "mydb",
			username: "user@domain",
			password: "pass#word@123",
			expected: "postgresql://user%40domain:pass%23word%40123@localhost:5432/mydb",
		},
		{
			name:        "postgresql with invalid username, password and query params",
			driver:      "postgres",
			host:        "localhost",
			port:        5432,
			database:    "mydb",
			username:    "user@domain",
			password:    "pass#word@123",
			queryParams: map[string]any{"sslmode": "disable", "app": "my#app"},
			expected:    "postgresql://user%40domain:pass%23word%40123@localhost:5432/mydb?app=my%23app&sslmode=disable",
		},
		{
			name:        "mysql with invalid username and password, no escaping",
			driver:      "mysql",
			host:        "localhost",
			port:        3306,
			database:    "mydb",
			username:    "user@domain+",
			password:    "pass#word@123",
			queryParams: map[string]any{"sslmode": "disable", "app": "my#app"},
			expected:    "user@domain+:pass#word@123@tcp(localhost:3306)/mydb?app=my%23app&sslmode=disable",
		},
		{
			name:        "snowflake with invalid username and password",
			driver:      "snowflake",
			host:        "account",
			port:        443,
			database:    "mydb",
			username:    "user@domain%",
			password:    "pass#word@123",
			queryParams: map[string]any{"sslmode": "disable", "app": "my#app"},
			expected:    "user%40domain%25:pass%23word%40123@account:443/mydb?app=my%23app&sslmode=disable",
		},
		{
			name:        "sqlserver with invalid username and password",
			driver:      "sqlserver",
			host:        "localhost",
			port:        1433,
			database:    "mydb",
			username:    "user@domain^",
			password:    "pass#word@123",
			queryParams: map[string]any{"sslmode": "disable", "app": "my#app"},
			expected:    "sqlserver://user%40domain%5E:pass%23word%40123@localhost:1433?app=my%23app&database=mydb&sslmode=disable",
		},
		{
			name:        "oracle with invalid username and password",
			driver:      "oracle",
			host:        "localhost",
			port:        2484,
			database:    "service_name",
			username:    "user@domain<",
			password:    "pass#word@123",
			queryParams: map[string]any{"sslmode": "disable", "app": "my#app"},
			expected:    "oracle://user%40domain%3C:pass%23word%40123@localhost:2484/service_name?app=my%23app&sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildDataSourceString(Config{
				Driver:           tt.driver,
				Host:             tt.host,
				Port:             tt.port,
				Database:         tt.database,
				Username:         tt.username,
				Password:         configopaque.String(tt.password),
				AdditionalParams: tt.queryParams,
			})
			if tt.wantErr {
				require.Error(t, err)
				require.Empty(t, got)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, got)
			}
		})
	}
}

func TestBuildDataSourceStringSQLServer(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected string
		wantErr  bool
	}{
		{
			name: "SQL Server without instance",
			config: Config{
				Host:     "localhost",
				Port:     1433,
				Database: "mydb",
				Driver:   "sqlserver",
				Username: "test",
				Password: "password",
			},
			expected: "sqlserver://test:password@localhost:1433?database=mydb",
			wantErr:  false,
		},
		{
			name: "SQL Server with instance",
			config: Config{
				Host:     "localhost\\instance",
				Port:     1433,
				Database: "mydb",
				Username: "test",
				Driver:   "sqlserver",
				Password: "password",
			},
			expected: "sqlserver://test:password@localhost:1433/instance?database=mydb",
			wantErr:  false,
		},
		{
			name: "SQL Server with instance and additional params",
			config: Config{
				Host:     "localhost\\instance",
				Port:     1433,
				Database: "mydb",
				Driver:   "sqlserver",
				Username: "test",
				Password: "password",
				AdditionalParams: map[string]any{
					"encrypt": true,
				},
			},
			expected: "sqlserver://test:password@localhost:1433/instance?database=mydb&encrypt=true",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildDataSourceString(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}
