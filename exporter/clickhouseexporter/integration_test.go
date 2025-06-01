// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package clickhouseexporter

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strconv"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestIntegration(t *testing.T) {
	testCase := []struct {
		name  string
		image string
	}{
		{
			name:  "test clickhouse 24-alpine",
			image: "clickhouse/clickhouse-server:24-alpine",
		},
		{
			name:  "test clickhouse 23-alpine",
			image: "clickhouse/clickhouse-server:23-alpine",
		},
		{
			name:  "test clickhouse 22-alpine",
			image: "clickhouse/clickhouse-server:22-alpine",
		},
	}

	for _, c := range testCase {
		// Container creation is slow, so create it once per test
		endpoint := createTestClickhouseContainer(t, c.image)
		t.Run(c.name+" exporting", func(t *testing.T) {
			logExporter := newTestLogsExporter(t, endpoint)
			verifyExportLog(t, logExporter)

			traceExporter := newTestTracesExporter(t, endpoint)
			verifyExporterTrace(t, traceExporter)

			metricExporter := newTestMetricsExporter(t, endpoint)
			verifyExporterMetric(t, metricExporter)
		})
		t.Run(c.name+" database creation", func(t *testing.T) {
			// Verify that start function returns no error
			logExporter := newTestLogsExporter(t, endpoint, func(c *Config) { c.Database = "otel" })
			// Verify that the table was created
			verifyLogTable(t, logExporter)

			traceExporter := newTestTracesExporter(t, endpoint, func(c *Config) { c.Database = "otel" })
			verifyTraceTable(t, traceExporter)

			metricsExporter := newTestMetricsExporter(t, endpoint, func(c *Config) { c.Database = "otel" })
			verifyMetricTable(t, metricsExporter)
		})
	}
}

// returns endpoint
func createTestClickhouseContainer(t *testing.T, image string) string {
	port := randPort()
	req := testcontainers.ContainerRequest{
		Image:        image,
		ExposedPorts: []string{fmt.Sprintf("%s:9000", port)},
		WaitingFor: wait.ForListeningPort("9000").
			WithStartupTimeout(2 * time.Minute),
	}
	c := getContainer(t, req)
	t.Cleanup(func() {
		err := c.Terminate(context.Background())
		require.NoError(t, err)
	})

	host, err := c.Host(context.Background())
	require.NoError(t, err)
	endpoint := fmt.Sprintf("tcp://%s:%s", host, port)

	return endpoint
}

func getContainer(t *testing.T, req testcontainers.ContainerRequest) testcontainers.Container {
	require.NoError(t, req.Validate())
	container, err := testcontainers.GenericContainer(
		context.Background(),
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
	require.NoError(t, err)

	err = container.Start(context.Background())
	require.NoError(t, err)
	return container
}

func verifyExportLog(t *testing.T, logExporter *logsExporter) {
	mustPushLogsData(t, logExporter, simpleLogs(1))
	db := sqlx.NewDb(logExporter.client, clickhouseDriverName)

	type log struct {
		Timestamp          string            `db:"Timestamp"`
		TimestampTime      string            `db:"TimestampTime"`
		TraceID            string            `db:"TraceId"`
		SpanID             string            `db:"SpanId"`
		TraceFlags         uint32            `db:"TraceFlags"`
		SeverityText       string            `db:"SeverityText"`
		SeverityNumber     int32             `db:"SeverityNumber"`
		ServiceName        string            `db:"ServiceName"`
		Body               string            `db:"Body"`
		ResourceSchemaURL  string            `db:"ResourceSchemaUrl"`
		ResourceAttributes map[string]string `db:"ResourceAttributes"`
		ScopeSchemaURL     string            `db:"ScopeSchemaUrl"`
		ScopeName          string            `db:"ScopeName"`
		ScopeVersion       string            `db:"ScopeVersion"`
		ScopeAttributes    map[string]string `db:"ScopeAttributes"`
		LogAttributes      map[string]string `db:"LogAttributes"`
	}

	var actualLog log

	expectLog := log{
		Timestamp:         "2023-12-25T09:53:49Z",
		TimestampTime:     "2023-12-25T09:53:49Z",
		TraceID:           "01020300000000000000000000000000",
		SpanID:            "0102030000000000",
		SeverityText:      "error",
		SeverityNumber:    18,
		ServiceName:       "test-service",
		Body:              "error message",
		ResourceSchemaURL: "https://opentelemetry.io/schemas/1.4.0",
		ResourceAttributes: map[string]string{
			"service.name": "test-service",
		},
		ScopeSchemaURL: "https://opentelemetry.io/schemas/1.7.0",
		ScopeName:      "io.opentelemetry.contrib.clickhouse",
		ScopeVersion:   "1.0.0",
		ScopeAttributes: map[string]string{
			"lib": "clickhouse",
		},
		LogAttributes: map[string]string{
			"service.namespace": "default",
		},
	}

	err := db.Get(&actualLog, "select * from default.otel_logs")
	require.NoError(t, err)
	require.Equal(t, expectLog, actualLog)
}

func verifyExporterTrace(t *testing.T, traceExporter *tracesExporter) {
	mustPushTracesData(t, traceExporter, simpleTraces(1))
	db := sqlx.NewDb(traceExporter.client, clickhouseDriverName)

	type trace struct {
		Timestamp          string              `db:"Timestamp"`
		TraceID            string              `db:"TraceId"`
		SpanID             string              `db:"SpanId"`
		ParentSpanID       string              `db:"ParentSpanId"`
		TraceState         string              `db:"TraceState"`
		SpanName           string              `db:"SpanName"`
		SpanKind           string              `db:"SpanKind"`
		ServiceName        string              `db:"ServiceName"`
		ResourceAttributes map[string]string   `db:"ResourceAttributes"`
		ScopeName          string              `db:"ScopeName"`
		ScopeVersion       string              `db:"ScopeVersion"`
		SpanAttributes     map[string]string   `db:"SpanAttributes"`
		Duration           int64               `db:"Duration"`
		StatusCode         string              `db:"StatusCode"`
		StatusMessage      string              `db:"StatusMessage"`
		EventsTimestamp    []time.Time         `db:"Events.Timestamp"`
		EventsName         []string            `db:"Events.Name"`
		EventsAttributes   []map[string]string `db:"Events.Attributes"`
		LinksTraceID       []string            `db:"Links.TraceId"`
		LinksSpanID        []string            `db:"Links.SpanId"`
		LinksTraceState    []string            `db:"Links.TraceState"`
		LinksAttributes    []map[string]string `db:"Links.Attributes"`
	}

	var actualTrace trace

	expectTrace := trace{
		Timestamp:    "2023-12-25T09:53:49Z",
		TraceID:      "01020300000000000000000000000000",
		SpanID:       "0102030000000000",
		ParentSpanID: "0102040000000000",
		TraceState:   "trace state",
		SpanName:     "call db",
		SpanKind:     "Internal",
		ServiceName:  "test-service",
		ResourceAttributes: map[string]string{
			"service.name": "test-service",
		},
		ScopeName:    "io.opentelemetry.contrib.clickhouse",
		ScopeVersion: "1.0.0",
		SpanAttributes: map[string]string{
			"service.name": "v",
		},
		Duration:      60000000000,
		StatusCode:    "Error",
		StatusMessage: "error",
		EventsTimestamp: []time.Time{
			time.Unix(1703498029, 0).UTC(),
		},
		EventsName: []string{"event1"},
		EventsAttributes: []map[string]string{
			{
				"level": "info",
			},
		},
		LinksTraceID: []string{
			"01020500000000000000000000000000",
		},
		LinksSpanID: []string{
			"0102050000000000",
		},
		LinksTraceState: []string{
			"error",
		},
		LinksAttributes: []map[string]string{
			{
				"k": "v",
			},
		},
	}

	err := db.Get(&actualTrace, "select * from default.otel_traces")
	require.NoError(t, err)
	require.Equal(t, expectTrace, actualTrace)
}

func verifyExporterMetric(t *testing.T, metricExporter *metricsExporter) {
	metric := pmetric.NewMetrics()
	rm := metric.ResourceMetrics().AppendEmpty()
	simpleMetrics(1).ResourceMetrics().At(0).CopyTo(rm)

	mustPushMetricsData(t, metricExporter, metric)
	db := sqlx.NewDb(metricExporter.client, clickhouseDriverName)

	verifyGaugeMetric(t, db)
	verifySumMetric(t, db)
	verifyHistogramMetric(t, db)
	verifyExphistogramMetric(t, db)
	verifySummaryMetric(t, db)
}

func verifyGaugeMetric(t *testing.T, db *sqlx.DB) {
	type gauge struct {
		ResourceAttributes          map[string]string   `db:"ResourceAttributes"`
		ResourceSchemaURL           string              `db:"ResourceSchemaUrl"`
		ScopeName                   string              `db:"ScopeName"`
		ScopeVersion                string              `db:"ScopeVersion"`
		ScopeAttributes             map[string]string   `db:"ScopeAttributes"`
		ScopeDroppedAttrCount       uint32              `db:"ScopeDroppedAttrCount"`
		ScopeSchemaURL              string              `db:"ScopeSchemaUrl"`
		ServiceName                 string              `db:"ServiceName"`
		MetricName                  string              `db:"MetricName"`
		MetricDescription           string              `db:"MetricDescription"`
		MetricUnit                  string              `db:"MetricUnit"`
		Attributes                  map[string]string   `db:"Attributes"`
		StartTimeUnix               string              `db:"StartTimeUnix"`
		TimeUnix                    string              `db:"TimeUnix"`
		Value                       float64             `db:"Value"`
		Flags                       uint32              `db:"Flags"`
		ExemplarsFilteredAttributes []map[string]string `db:"Exemplars.FilteredAttributes"`
		ExemplarsTimeUnix           []time.Time         `db:"Exemplars.TimeUnix"`
		ExemplarsValue              []float64           `db:"Exemplars.Value"`
		ExemplarsSpanID             []string            `db:"Exemplars.SpanId"`
		ExemplarsTraceID            []string            `db:"Exemplars.TraceId"`
	}

	var actualGauge gauge

	expectGauge := gauge{
		ResourceAttributes: map[string]string{
			"service.name":          "demo 1",
			"Resource Attributes 1": "value1",
		},
		ResourceSchemaURL:     "Resource SchemaUrl 1",
		ScopeName:             "Scope name 1",
		ScopeVersion:          "Scope version 1",
		ScopeDroppedAttrCount: 10,
		ScopeSchemaURL:        "Scope SchemaUrl 1",
		ScopeAttributes: map[string]string{
			"Scope Attributes 1": "value1",
		},
		ServiceName:       "demo 1",
		MetricName:        "gauge metrics",
		MetricDescription: "This is a gauge metrics",
		MetricUnit:        "count",
		Attributes: map[string]string{
			"gauge_label_1": "1",
		},
		StartTimeUnix: "2023-12-25T09:53:49Z",
		TimeUnix:      "2023-12-25T09:53:49Z",
		Value:         0,
		Flags:         0,
		ExemplarsFilteredAttributes: []map[string]string{
			{
				"key":  "value",
				"key2": "value2",
			},
		},
		ExemplarsTimeUnix: []time.Time{time.Unix(1703498029, 0).UTC()},
		ExemplarsTraceID:  []string{"01020300000000000000000000000000"},
		ExemplarsSpanID:   []string{"0102030000000000"},
		ExemplarsValue:    []float64{54},
	}
	err := db.Get(&actualGauge, "select * from default.otel_metrics_gauge")
	require.NoError(t, err)
	require.Equal(t, expectGauge, actualGauge)
}

func verifySumMetric(t *testing.T, db *sqlx.DB) {
	type sum struct {
		ResourceAttributes          map[string]string   `db:"ResourceAttributes"`
		ResourceSchemaURL           string              `db:"ResourceSchemaUrl"`
		ScopeName                   string              `db:"ScopeName"`
		ScopeVersion                string              `db:"ScopeVersion"`
		ScopeAttributes             map[string]string   `db:"ScopeAttributes"`
		ScopeDroppedAttrCount       uint32              `db:"ScopeDroppedAttrCount"`
		ScopeSchemaURL              string              `db:"ScopeSchemaUrl"`
		ServiceName                 string              `db:"ServiceName"`
		MetricName                  string              `db:"MetricName"`
		MetricDescription           string              `db:"MetricDescription"`
		MetricUnit                  string              `db:"MetricUnit"`
		Attributes                  map[string]string   `db:"Attributes"`
		StartTimeUnix               string              `db:"StartTimeUnix"`
		TimeUnix                    string              `db:"TimeUnix"`
		Value                       float64             `db:"Value"`
		Flags                       uint32              `db:"Flags"`
		ExemplarsFilteredAttributes []map[string]string `db:"Exemplars.FilteredAttributes"`
		ExemplarsTimeUnix           []time.Time         `db:"Exemplars.TimeUnix"`
		ExemplarsValue              []float64           `db:"Exemplars.Value"`
		ExemplarsSpanID             []string            `db:"Exemplars.SpanId"`
		ExemplarsTraceID            []string            `db:"Exemplars.TraceId"`
		AggregationTemporality      int32               `db:"AggregationTemporality"`
		IsMonotonic                 bool                `db:"IsMonotonic"`
	}

	var actualSum sum

	expectSum := sum{
		ResourceAttributes: map[string]string{
			"service.name":          "demo 1",
			"Resource Attributes 1": "value1",
		},
		ResourceSchemaURL:     "Resource SchemaUrl 1",
		ScopeName:             "Scope name 1",
		ScopeVersion:          "Scope version 1",
		ScopeDroppedAttrCount: 10,
		ScopeSchemaURL:        "Scope SchemaUrl 1",
		ScopeAttributes: map[string]string{
			"Scope Attributes 1": "value1",
		},
		ServiceName:       "demo 1",
		MetricName:        "sum metrics",
		MetricDescription: "This is a sum metrics",
		MetricUnit:        "count",
		Attributes: map[string]string{
			"sum_label_1": "1",
		},
		StartTimeUnix: "2023-12-25T09:53:49Z",
		TimeUnix:      "2023-12-25T09:53:49Z",
		Value:         11.234,
		Flags:         0,
		ExemplarsFilteredAttributes: []map[string]string{
			{
				"key":  "value",
				"key2": "value2",
			},
		},
		ExemplarsTimeUnix: []time.Time{time.Unix(1703498029, 0).UTC()},
		ExemplarsTraceID:  []string{"01020300000000000000000000000000"},
		ExemplarsSpanID:   []string{"0102030000000000"},
		ExemplarsValue:    []float64{54},
	}

	err := db.Get(&actualSum, "select * from default.otel_metrics_sum")
	require.NoError(t, err)
	require.Equal(t, expectSum, actualSum)
}

func verifyHistogramMetric(t *testing.T, db *sqlx.DB) {
	type histogram struct {
		ResourceAttributes          map[string]string   `db:"ResourceAttributes"`
		ResourceSchemaURL           string              `db:"ResourceSchemaUrl"`
		ScopeName                   string              `db:"ScopeName"`
		ScopeVersion                string              `db:"ScopeVersion"`
		ScopeAttributes             map[string]string   `db:"ScopeAttributes"`
		ScopeDroppedAttrCount       uint32              `db:"ScopeDroppedAttrCount"`
		ScopeSchemaURL              string              `db:"ScopeSchemaUrl"`
		ServiceName                 string              `db:"ServiceName"`
		MetricName                  string              `db:"MetricName"`
		MetricDescription           string              `db:"MetricDescription"`
		MetricUnit                  string              `db:"MetricUnit"`
		Attributes                  map[string]string   `db:"Attributes"`
		StartTimeUnix               string              `db:"StartTimeUnix"`
		TimeUnix                    string              `db:"TimeUnix"`
		Count                       float64             `db:"Count"`
		Sum                         float64             `db:"Sum"`
		BucketCounts                []uint64            `db:"BucketCounts"`
		ExplicitBounds              []float64           `db:"ExplicitBounds"`
		ExemplarsFilteredAttributes []map[string]string `db:"Exemplars.FilteredAttributes"`
		ExemplarsTimeUnix           []time.Time         `db:"Exemplars.TimeUnix"`
		ExemplarsValue              []float64           `db:"Exemplars.Value"`
		ExemplarsSpanID             []string            `db:"Exemplars.SpanId"`
		ExemplarsTraceID            []string            `db:"Exemplars.TraceId"`
		AggregationTemporality      int32               `db:"AggregationTemporality"`
		Flags                       uint32              `db:"Flags"`
		Min                         float64             `db:"Min"`
		Max                         float64             `db:"Max"`
	}

	var actualHistogram histogram

	expectHistogram := histogram{
		ResourceAttributes: map[string]string{
			"service.name":          "demo 1",
			"Resource Attributes 1": "value1",
		},
		ResourceSchemaURL:     "Resource SchemaUrl 1",
		ScopeName:             "Scope name 1",
		ScopeVersion:          "Scope version 1",
		ScopeDroppedAttrCount: 10,
		ScopeSchemaURL:        "Scope SchemaUrl 1",
		ScopeAttributes: map[string]string{
			"Scope Attributes 1": "value1",
		},
		ServiceName:       "demo 1",
		MetricName:        "histogram metrics",
		MetricDescription: "This is a histogram metrics",
		MetricUnit:        "ms",
		Attributes: map[string]string{
			"key":  "value",
			"key2": "value",
		},
		StartTimeUnix:  "2023-12-25T09:53:49Z",
		TimeUnix:       "2023-12-25T09:53:49Z",
		Count:          1,
		Sum:            1,
		BucketCounts:   []uint64{0, 0, 0, 1, 0},
		ExplicitBounds: []float64{0, 0, 0, 0, 0},
		Flags:          0,
		Min:            0,
		Max:            1,
		ExemplarsFilteredAttributes: []map[string]string{
			{
				"key":  "value",
				"key2": "value2",
			},
		},
		ExemplarsTimeUnix: []time.Time{time.Unix(1703498029, 0).UTC()},
		ExemplarsTraceID:  []string{"01020300000000000000000000000000"},
		ExemplarsSpanID:   []string{"0102030000000000"},
		ExemplarsValue:    []float64{55.22},
	}

	err := db.Get(&actualHistogram, "select * from default.otel_metrics_histogram")
	require.NoError(t, err)
	require.Equal(t, expectHistogram, actualHistogram)
}

func verifyExphistogramMetric(t *testing.T, db *sqlx.DB) {
	type expHistogram struct {
		ResourceAttributes          map[string]string   `db:"ResourceAttributes"`
		ResourceSchemaURL           string              `db:"ResourceSchemaUrl"`
		ScopeName                   string              `db:"ScopeName"`
		ScopeVersion                string              `db:"ScopeVersion"`
		ScopeAttributes             map[string]string   `db:"ScopeAttributes"`
		ScopeDroppedAttrCount       uint32              `db:"ScopeDroppedAttrCount"`
		ScopeSchemaURL              string              `db:"ScopeSchemaUrl"`
		ServiceName                 string              `db:"ServiceName"`
		MetricName                  string              `db:"MetricName"`
		MetricDescription           string              `db:"MetricDescription"`
		MetricUnit                  string              `db:"MetricUnit"`
		Attributes                  map[string]string   `db:"Attributes"`
		StartTimeUnix               string              `db:"StartTimeUnix"`
		TimeUnix                    string              `db:"TimeUnix"`
		Count                       float64             `db:"Count"`
		Sum                         float64             `db:"Sum"`
		Scale                       int32               `db:"Scale"`
		ZeroCount                   uint64              `db:"ZeroCount"`
		PositiveOffset              int32               `db:"PositiveOffset"`
		PositiveBucketCounts        []uint64            `db:"PositiveBucketCounts"`
		NegativeOffset              int32               `db:"NegativeOffset"`
		NegativeBucketCounts        []uint64            `db:"NegativeBucketCounts"`
		ExemplarsFilteredAttributes []map[string]string `db:"Exemplars.FilteredAttributes"`
		ExemplarsTimeUnix           []time.Time         `db:"Exemplars.TimeUnix"`
		ExemplarsValue              []float64           `db:"Exemplars.Value"`
		ExemplarsSpanID             []string            `db:"Exemplars.SpanId"`
		ExemplarsTraceID            []string            `db:"Exemplars.TraceId"`
		AggregationTemporality      int32               `db:"AggregationTemporality"`
		Flags                       uint32              `db:"Flags"`
		Min                         float64             `db:"Min"`
		Max                         float64             `db:"Max"`
	}

	var actualExpHistogram expHistogram

	expectExpHistogram := expHistogram{
		ResourceAttributes: map[string]string{
			"service.name":          "demo 1",
			"Resource Attributes 1": "value1",
		},
		ResourceSchemaURL:     "Resource SchemaUrl 1",
		ScopeName:             "Scope name 1",
		ScopeVersion:          "Scope version 1",
		ScopeDroppedAttrCount: 10,
		ScopeSchemaURL:        "Scope SchemaUrl 1",
		ScopeAttributes: map[string]string{
			"Scope Attributes 1": "value1",
		},
		ServiceName:       "demo 1",
		MetricName:        "exp histogram metrics",
		MetricDescription: "This is a exp histogram metrics",
		MetricUnit:        "ms",
		Attributes: map[string]string{
			"key":  "value",
			"key2": "value",
		},
		StartTimeUnix:        "2023-12-25T09:53:49Z",
		TimeUnix:             "2023-12-25T09:53:49Z",
		Count:                1,
		Sum:                  1,
		Scale:                0,
		ZeroCount:            0,
		PositiveOffset:       1,
		PositiveBucketCounts: []uint64{0, 0, 0, 1, 0},
		NegativeOffset:       1,
		NegativeBucketCounts: []uint64{0, 0, 0, 1, 0},
		Flags:                0,
		Min:                  0,
		Max:                  1,
		ExemplarsFilteredAttributes: []map[string]string{
			{
				"key":  "value",
				"key2": "value2",
			},
		},
		ExemplarsTimeUnix: []time.Time{time.Unix(1703498029, 0).UTC()},
		ExemplarsTraceID:  []string{"01020300000000000000000000000000"},
		ExemplarsSpanID:   []string{"0102030000000000"},
		ExemplarsValue:    []float64{54},
	}

	err := db.Get(&actualExpHistogram, "select * from default.otel_metrics_exponential_histogram")
	require.NoError(t, err)
	require.Equal(t, expectExpHistogram, actualExpHistogram)
}

func verifySummaryMetric(t *testing.T, db *sqlx.DB) {
	type summary struct {
		ResourceAttributes    map[string]string `db:"ResourceAttributes"`
		ResourceSchemaURL     string            `db:"ResourceSchemaUrl"`
		ScopeName             string            `db:"ScopeName"`
		ScopeVersion          string            `db:"ScopeVersion"`
		ScopeAttributes       map[string]string `db:"ScopeAttributes"`
		ScopeDroppedAttrCount uint32            `db:"ScopeDroppedAttrCount"`
		ScopeSchemaURL        string            `db:"ScopeSchemaUrl"`
		ServiceName           string            `db:"ServiceName"`
		MetricName            string            `db:"MetricName"`
		MetricDescription     string            `db:"MetricDescription"`
		MetricUnit            string            `db:"MetricUnit"`
		Attributes            map[string]string `db:"Attributes"`
		StartTimeUnix         string            `db:"StartTimeUnix"`
		TimeUnix              string            `db:"TimeUnix"`
		Count                 float64           `db:"Count"`
		Sum                   float64           `db:"Sum"`
		Quantile              []float64         `db:"ValueAtQuantiles.Quantile"`
		QuantilesValue        []float64         `db:"ValueAtQuantiles.Value"`
		Flags                 uint32            `db:"Flags"`
	}

	var actualSummary summary

	expectSummary := summary{
		ResourceAttributes: map[string]string{
			"service.name":          "demo 1",
			"Resource Attributes 1": "value1",
		},
		ResourceSchemaURL:     "Resource SchemaUrl 1",
		ScopeName:             "Scope name 1",
		ScopeVersion:          "Scope version 1",
		ScopeDroppedAttrCount: 10,
		ScopeSchemaURL:        "Scope SchemaUrl 1",
		ScopeAttributes: map[string]string{
			"Scope Attributes 1": "value1",
		},
		ServiceName:       "demo 1",
		MetricName:        "summary metrics",
		MetricDescription: "This is a summary metrics",
		MetricUnit:        "ms",
		Attributes: map[string]string{
			"key":  "value",
			"key2": "value",
		},
		StartTimeUnix:  "2023-12-25T09:53:49Z",
		TimeUnix:       "2023-12-25T09:53:49Z",
		Count:          1,
		Sum:            1,
		Quantile:       []float64{1},
		QuantilesValue: []float64{1},
		Flags:          0,
	}

	err := db.Get(&actualSummary, "select * from default.otel_metrics_summary")
	require.NoError(t, err)
	require.Equal(t, expectSummary, actualSummary)
}

func verifyLogTable(t *testing.T, logExporter *logsExporter) {
	db := sqlx.NewDb(logExporter.client, clickhouseDriverName)
	_, err := db.Query("select * from otel.otel_logs")
	require.NoError(t, err)
}

func verifyTraceTable(t *testing.T, traceExporter *tracesExporter) {
	db := sqlx.NewDb(traceExporter.client, clickhouseDriverName)
	_, err := db.Query("select * from otel.otel_traces")
	require.NoError(t, err)
}

func verifyMetricTable(t *testing.T, metricExporter *metricsExporter) {
	db := sqlx.NewDb(metricExporter.client, clickhouseDriverName)
	_, err := db.Query("select * from otel.otel_metrics_gauge")
	require.NoError(t, err)
}

func randPort() string {
	return strconv.Itoa(rand.IntN(999) + 9000)
}
