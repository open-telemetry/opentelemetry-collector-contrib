// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"
)

func TestMetricsClusterConfig(t *testing.T) {
	testClusterConfig(t, func(t *testing.T, dsn string, clusterTest clusterTestConfig, fns ...func(*Config)) {
		fns = append(fns, withDriverName(t.Name()))
		exporter := newTestMetricsExporter(t, dsn, fns...)
		clusterTest.verifyConfig(t, exporter.cfg)
	})
}

func TestMetricsTableEngineConfig(t *testing.T) {
	testTableEngineConfig(t, func(t *testing.T, dsn string, engineTest tableEngineTestConfig, fns ...func(*Config)) {
		fns = append(fns, withDriverName(t.Name()))
		exporter := newTestMetricsExporter(t, dsn, fns...)
		engineTest.verifyConfig(t, exporter.cfg.TableEngine)
	})
}

func Test_generateMetricMetricTableNames(t *testing.T) {
	cfg := Config{
		MetricsTables: MetricTablesConfig{
			Gauge:                internal.MetricTypeConfig{Name: "otel_metrics_custom_gauge"},
			Sum:                  internal.MetricTypeConfig{Name: "otel_metrics_custom_sum"},
			Summary:              internal.MetricTypeConfig{Name: "otel_metrics_custom_summary"},
			Histogram:            internal.MetricTypeConfig{Name: "otel_metrics_custom_histogram"},
			ExponentialHistogram: internal.MetricTypeConfig{Name: "otel_metrics_custom_exp_histogram"},
		},
	}

	require.Equal(t, internal.MetricTablesConfigMapper{
		pmetric.MetricTypeGauge:                cfg.MetricsTables.Gauge,
		pmetric.MetricTypeSum:                  cfg.MetricsTables.Sum,
		pmetric.MetricTypeSummary:              cfg.MetricsTables.Summary,
		pmetric.MetricTypeHistogram:            cfg.MetricsTables.Histogram,
		pmetric.MetricTypeExponentialHistogram: cfg.MetricsTables.ExponentialHistogram,
	}, generateMetricTablesConfigMapper(&cfg))
}

func TestExporter_pushMetricsData(t *testing.T) {
	t.Parallel()
	t.Run("push success", func(t *testing.T) {
		items := &atomic.Int32{}
		initClickhouseTestServer(t, func(query string, _ []driver.Value) error {
			if strings.HasPrefix(query, "INSERT") {
				items.Add(1)
			}
			return nil
		})
		exporter := newTestMetricsExporter(t, defaultEndpoint, withDriverName(t.Name()))
		mustPushMetricsData(t, exporter, simpleMetrics(1))

		require.Equal(t, int32(15), items.Load())
	})
	t.Run("push failure", func(t *testing.T) {
		initClickhouseTestServer(t, func(query string, _ []driver.Value) error {
			if strings.HasPrefix(query, "INSERT") {
				return errors.New("mock insert error")
			}
			return nil
		})
		exporter := newTestMetricsExporter(t, defaultEndpoint, withDriverName(t.Name()))
		err := exporter.pushMetricsData(context.TODO(), simpleMetrics(2))
		require.Error(t, err)
	})
	t.Run("check Resource metadata and scope metadata (2nd resource contain 2 different scope metrics)", func(t *testing.T) {
		items := &atomic.Int32{}
		resourceSchemaIdx := []int{1, 2, 2}
		scopeNameIdx := []int{1, 2, 3}

		itemIdxs := map[string]*atomic.Uint32{
			"otel_metrics_exponential_histogram": {},
			"otel_metrics_gauge":                 {},
			"otel_metrics_histogram":             {},
			"otel_metrics_sum":                   {},
			"otel_metrics_summary":               {},
		}
		initClickhouseTestServer(t, func(query string, values []driver.Value) error {
			if strings.HasPrefix(query, "INSERT") {
				items.Add(1)
				if strings.HasPrefix(query, "INSERT INTO otel_metrics_exponential_histogram") {
					idx := itemIdxs["otel_metrics_exponential_histogram"]
					require.Equal(t, fmt.Sprintf("Resource SchemaUrl %d", resourceSchemaIdx[idx.Load()]), values[1])
					require.Equal(t, fmt.Sprintf("Scope name %d", scopeNameIdx[idx.Load()]), values[2])
					idx.Add(1)
				}
				if strings.HasPrefix(query, "INSERT INTO otel_metrics_gauge") {
					idx := itemIdxs["otel_metrics_gauge"]
					require.Equal(t, fmt.Sprintf("Resource SchemaUrl %d", resourceSchemaIdx[idx.Load()]), values[1])
					require.Equal(t, fmt.Sprintf("Scope name %d", scopeNameIdx[idx.Load()]), values[2])
					idx.Add(1)
				}
				if strings.HasPrefix(query, "INSERT INTO otel_metrics_histogram") {
					idx := itemIdxs["otel_metrics_histogram"]
					require.Equal(t, fmt.Sprintf("Resource SchemaUrl %d", resourceSchemaIdx[idx.Load()]), values[1])
					require.Equal(t, fmt.Sprintf("Scope name %d", scopeNameIdx[idx.Load()]), values[2])
					idx.Add(1)
				}
				if strings.HasPrefix(query, "INSERT INTO otel_metrics_sum (") {
					idx := itemIdxs["otel_metrics_sum"]
					require.Equal(t, fmt.Sprintf("Resource SchemaUrl %d", resourceSchemaIdx[idx.Load()]), values[1])
					require.Equal(t, fmt.Sprintf("Scope name %d", scopeNameIdx[idx.Load()]), values[2])
					idx.Add(1)
				}
				if strings.HasPrefix(query, "INSERT INTO otel_metrics_summary") {
					idx := itemIdxs["otel_metrics_summary"]
					require.Equal(t, fmt.Sprintf("Resource SchemaUrl %d", resourceSchemaIdx[idx.Load()]), values[1])
					require.Equal(t, fmt.Sprintf("Scope name %d", scopeNameIdx[idx.Load()]), values[2])
					idx.Add(1)
				}
			}
			return nil
		})
		exporter := newTestMetricsExporter(t, defaultEndpoint, withDriverName(t.Name()))
		mustPushMetricsData(t, exporter, simpleMetrics(1))

		require.Equal(t, int32(15), items.Load())
	})
	t.Run("check traceID and spanID", func(t *testing.T) {
		initClickhouseTestServer(t, func(query string, values []driver.Value) error {
			if strings.HasPrefix(query, "INSERT INTO otel_metrics_gauge") {
				require.Equal(t, clickhouse.ArraySet{"0102030000000000"}, values[19])
				require.Equal(t, clickhouse.ArraySet{"01020300000000000000000000000000"}, values[20])
			}
			if strings.HasPrefix(query, "INSERT INTO otel_metrics_histogram") {
				require.Equal(t, clickhouse.ArraySet{"0102030000000000"}, values[21])
				require.Equal(t, clickhouse.ArraySet{"01020300000000000000000000000000"}, values[22])
			}
			if strings.HasPrefix(query, "INSERT INTO otel_metrics_sum ") {
				require.Equal(t, clickhouse.ArraySet{"0102030000000000"}, values[19])
				require.Equal(t, clickhouse.ArraySet{"01020300000000000000000000000000"}, values[20])
			}
			if strings.HasPrefix(query, "INSERT INTO otel_metrics_exponential_histogram") {
				require.Equal(t, clickhouse.ArraySet{"0102030000000000"}, values[25])
				require.Equal(t, clickhouse.ArraySet{"01020300000000000000000000000000"}, values[26])
			}
			return nil
		})
		exporter := newTestMetricsExporter(t, defaultEndpoint, withDriverName(t.Name()))
		mustPushMetricsData(t, exporter, simpleMetrics(1))
	})
}

func Benchmark_pushMetricsData(b *testing.B) {
	pm := simpleMetrics(1)
	exporter := newTestMetricsExporter(&testing.T{}, defaultEndpoint)
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		err := exporter.pushMetricsData(context.TODO(), pm)
		require.NoError(b, err)
	}
}

// simpleMetrics there will be added two ResourceMetrics and each of them have count data point
func simpleMetrics(count int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "demo 1")
	rm.Resource().Attributes().PutStr("Resource Attributes 1", "value1")
	rm.Resource().SetDroppedAttributesCount(10)
	rm.SetSchemaUrl("Resource SchemaUrl 1")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("Scope SchemaUrl 1")
	sm.Scope().Attributes().PutStr("Scope Attributes 1", "value1")
	sm.Scope().SetDroppedAttributesCount(10)
	sm.Scope().SetName("Scope name 1")
	sm.Scope().SetVersion("Scope version 1")
	timestamp := time.Unix(1703498029, 0)
	for i := 0; i < count; i++ {
		// gauge
		m := sm.Metrics().AppendEmpty()
		m.SetName("gauge metrics")
		m.SetUnit("count")
		m.SetDescription("This is a gauge metrics")
		dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.SetFlags(pmetric.DefaultDataPointFlags)
		dp.Attributes().PutStr("gauge_label_1", "1")
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(timestamp))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		exemplars := dp.Exemplars().AppendEmpty()
		exemplars.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// sum
		m = sm.Metrics().AppendEmpty()
		m.SetName("sum metrics")
		m.SetUnit("count")
		m.SetDescription("This is a sum metrics")
		dp = m.SetEmptySum().DataPoints().AppendEmpty()
		dp.SetDoubleValue(11.234)
		dp.SetFlags(pmetric.DefaultDataPointFlags)
		dp.Attributes().PutStr("sum_label_1", "1")
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(timestamp))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		exemplars = dp.Exemplars().AppendEmpty()
		exemplars.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// histogram
		m = sm.Metrics().AppendEmpty()
		m.SetName("histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a histogram metrics")
		dpHisto := m.SetEmptyHistogram().DataPoints().AppendEmpty()
		dpHisto.SetStartTimestamp(pcommon.NewTimestampFromTime(timestamp))
		dpHisto.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		dpHisto.SetCount(1)
		dpHisto.SetSum(1)
		dpHisto.Attributes().PutStr("key", "value")
		dpHisto.Attributes().PutStr("key2", "value")
		dpHisto.ExplicitBounds().FromRaw([]float64{0, 0, 0, 0, 0})
		dpHisto.BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})
		dpHisto.SetMin(0)
		dpHisto.SetMax(1)
		exemplars = dpHisto.Exemplars().AppendEmpty()
		exemplars.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		exemplars.SetDoubleValue(55.22)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// exp histogram
		m = sm.Metrics().AppendEmpty()
		m.SetName("exp histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a exp histogram metrics")
		dpExpHisto := m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
		dpExpHisto.SetStartTimestamp(pcommon.NewTimestampFromTime(timestamp))
		dpExpHisto.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		dpExpHisto.SetSum(1)
		dpExpHisto.SetMin(0)
		dpExpHisto.SetMax(1)
		dpExpHisto.SetZeroCount(0)
		dpExpHisto.SetCount(1)
		dpExpHisto.Attributes().PutStr("key", "value")
		dpExpHisto.Attributes().PutStr("key2", "value")
		dpExpHisto.Negative().SetOffset(1)
		dpExpHisto.Negative().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})
		dpExpHisto.Positive().SetOffset(1)
		dpExpHisto.Positive().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})

		exemplars = dpExpHisto.Exemplars().AppendEmpty()
		exemplars.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// summary
		m = sm.Metrics().AppendEmpty()
		m.SetName("summary metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a summary metrics")
		summary := m.SetEmptySummary().DataPoints().AppendEmpty()
		summary.SetStartTimestamp(pcommon.NewTimestampFromTime(timestamp))
		summary.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		summary.Attributes().PutStr("key", "value")
		summary.Attributes().PutStr("key2", "value")
		summary.SetCount(1)
		summary.SetSum(1)
		quantileValues := summary.QuantileValues().AppendEmpty()
		quantileValues.SetValue(1)
		quantileValues.SetQuantile(1)
	}

	rm = metrics.ResourceMetrics().AppendEmpty()
	// Removed service.name from second metric to test both with/without ServiceName cases
	rm.Resource().Attributes().PutStr("Resource Attributes 2", "value2")
	rm.Resource().SetDroppedAttributesCount(20)
	rm.SetSchemaUrl("Resource SchemaUrl 2")
	sm = rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("Scope SchemaUrl 2")
	sm.Scope().Attributes().PutStr("Scope Attributes 2", "value2")
	sm.Scope().SetDroppedAttributesCount(20)
	sm.Scope().SetName("Scope name 2")
	sm.Scope().SetVersion("Scope version 2")
	for i := 0; i < count; i++ {
		// gauge
		m := sm.Metrics().AppendEmpty()
		m.SetName("gauge metrics")
		m.SetUnit("count")
		m.SetDescription("This is a gauge metrics")
		dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.Attributes().PutStr("gauge_label_2", "2")
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		exemplars := dp.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// sum
		m = sm.Metrics().AppendEmpty()
		m.SetName("sum metrics")
		m.SetUnit("count")
		m.SetDescription("This is a sum metrics")
		dp = m.SetEmptySum().DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.Attributes().PutStr("sum_label_2", "2")
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		exemplars = dp.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// histogram
		m = sm.Metrics().AppendEmpty()
		m.SetName("histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a histogram metrics")
		dpHisto := m.SetEmptyHistogram().DataPoints().AppendEmpty()
		dpHisto.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpHisto.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpHisto.SetCount(1)
		dpHisto.SetSum(1)
		dpHisto.Attributes().PutStr("key", "value")
		dpHisto.Attributes().PutStr("key", "value")
		dpHisto.ExplicitBounds().FromRaw([]float64{0, 0, 0, 0, 0})
		dpHisto.BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})
		dpHisto.SetMin(0)
		dpHisto.SetMax(1)
		exemplars = dpHisto.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// exp histogram
		m = sm.Metrics().AppendEmpty()
		m.SetName("exp histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a exp histogram metrics")
		dpExpHisto := m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
		dpExpHisto.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpExpHisto.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpExpHisto.SetSum(1)
		dpExpHisto.SetMin(0)
		dpExpHisto.SetMax(1)
		dpExpHisto.SetZeroCount(0)
		dpExpHisto.SetCount(1)
		dpExpHisto.Attributes().PutStr("key", "value")
		dpExpHisto.Attributes().PutStr("key", "value")
		dpExpHisto.Negative().SetOffset(1)
		dpExpHisto.Negative().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})
		dpExpHisto.Positive().SetOffset(1)
		dpExpHisto.Positive().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})

		exemplars = dpExpHisto.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// summary
		m = sm.Metrics().AppendEmpty()
		m.SetName("summary histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a summary metrics")
		summary := m.SetEmptySummary().DataPoints().AppendEmpty()
		summary.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		summary.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		summary.Attributes().PutStr("key", "value")
		summary.Attributes().PutStr("key2", "value2")
		summary.SetCount(1)
		summary.SetSum(1)
		quantileValues := summary.QuantileValues().AppendEmpty()
		quantileValues.SetValue(1)
		quantileValues.SetQuantile(1)
	}

	// add a different scope metrics
	sm = rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("Scope SchemaUrl 3")
	sm.Scope().Attributes().PutStr("Scope Attributes 3", "value3")
	sm.Scope().SetDroppedAttributesCount(20)
	sm.Scope().SetName("Scope name 3")
	sm.Scope().SetVersion("Scope version 3")
	for i := 0; i < count; i++ {
		// gauge
		m := sm.Metrics().AppendEmpty()
		m.SetName("gauge metrics")
		m.SetUnit("count")
		m.SetDescription("This is a gauge metrics")
		dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.Attributes().PutStr("gauge_label_3", "3")
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		exemplars := dp.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// sum
		m = sm.Metrics().AppendEmpty()
		m.SetName("sum metrics")
		m.SetUnit("count")
		m.SetDescription("This is a sum metrics")
		dp = m.SetEmptySum().DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.Attributes().PutStr("sum_label_2", "2")
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		exemplars = dp.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// histogram
		m = sm.Metrics().AppendEmpty()
		m.SetName("histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a histogram metrics")
		dpHisto := m.SetEmptyHistogram().DataPoints().AppendEmpty()
		dpHisto.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpHisto.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpHisto.SetCount(1)
		dpHisto.SetSum(1)
		dpHisto.Attributes().PutStr("key", "value")
		dpHisto.Attributes().PutStr("key", "value")
		dpHisto.ExplicitBounds().FromRaw([]float64{0, 0, 0, 0, 0})
		dpHisto.BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})
		dpHisto.SetMin(0)
		dpHisto.SetMax(1)
		exemplars = dpHisto.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// exp histogram
		m = sm.Metrics().AppendEmpty()
		m.SetName("exp histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a exp histogram metrics")
		dpExpHisto := m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
		dpExpHisto.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpExpHisto.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpExpHisto.SetSum(1)
		dpExpHisto.SetMin(0)
		dpExpHisto.SetMax(1)
		dpExpHisto.SetZeroCount(0)
		dpExpHisto.SetCount(1)
		dpExpHisto.Attributes().PutStr("key", "value")
		dpExpHisto.Attributes().PutStr("key", "value")
		dpExpHisto.Negative().SetOffset(1)
		dpExpHisto.Negative().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})
		dpExpHisto.Positive().SetOffset(1)
		dpExpHisto.Positive().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})

		exemplars = dpExpHisto.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// summary
		m = sm.Metrics().AppendEmpty()
		m.SetName("summary histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a summary metrics")
		summary := m.SetEmptySummary().DataPoints().AppendEmpty()
		summary.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		summary.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		summary.Attributes().PutStr("key", "value")
		summary.Attributes().PutStr("key2", "value2")
		summary.SetCount(1)
		summary.SetSum(1)
		quantileValues := summary.QuantileValues().AppendEmpty()
		quantileValues.SetValue(1)
		quantileValues.SetQuantile(1)
	}
	return metrics
}

func mustPushMetricsData(t *testing.T, exporter *metricsExporter, md pmetric.Metrics) {
	err := exporter.pushMetricsData(context.TODO(), md)
	require.NoError(t, err)
}

func newTestMetricsExporter(t *testing.T, dsn string, fns ...func(*Config)) *metricsExporter {
	exporter, err := newMetricsExporter(zaptest.NewLogger(t), withTestExporterConfig(fns...)(dsn))
	require.NoError(t, err)
	require.NoError(t, exporter.start(context.TODO(), nil))

	t.Cleanup(func() { _ = exporter.shutdown(context.TODO()) })
	return exporter
}
