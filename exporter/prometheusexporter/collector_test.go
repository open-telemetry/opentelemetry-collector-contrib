// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/prometheus/otlptranslator"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	prometheustranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

type mockAccumulator struct {
	metrics            []pmetric.Metric
	resourceAttributes pcommon.Map // Same attributes for all metrics.
	scopeNames         []string
	scopeVersions      []string
	scopeSchemaURLs    []string
	scopeAttributes    []pcommon.Map
}

func (*mockAccumulator) Accumulate(pmetric.ResourceMetrics) (n int) {
	return 0
}

func (*mockAccumulator) cleanupExpired() {}

func (a *mockAccumulator) Collect() ([]pmetric.Metric, []pcommon.Map, []string, []string, []string, []pcommon.Map) {
	rAttrs := make([]pcommon.Map, len(a.metrics))
	scopeNames := make([]string, len(a.metrics))
	scopeVersions := make([]string, len(a.metrics))
	scopeSchemaURLs := make([]string, len(a.metrics))
	scopeAttributes := make([]pcommon.Map, len(a.metrics))
	for i := range rAttrs {
		rAttrs[i] = a.resourceAttributes
		scopeNames[i] = a.scopeNames[i]
		scopeVersions[i] = a.scopeVersions[i]
		scopeSchemaURLs[i] = a.scopeSchemaURLs[i]
		scopeAttributes[i] = a.scopeAttributes[i]
	}

	return a.metrics, rAttrs, scopeNames, scopeVersions, scopeSchemaURLs, scopeAttributes
}

func TestConvertInvalidDataType(t *testing.T) {
	metric := pmetric.NewMetric()
	c := newCollector(&Config{}, zap.NewNop())
	c.accumulator = &mockAccumulator{
		[]pmetric.Metric{metric},
		pcommon.NewMap(),
		[]string{"test"},
		[]string{"1.0.0"},
		[]string{"http://test.com"},
		[]pcommon.Map{pcommon.NewMap()},
	}

	_, err := c.convertMetric(metric, pcommon.NewMap(), "test", "1.0.0", "http://test.com", pcommon.NewMap())
	require.Equal(t, errUnknownMetricType, err)

	ch := make(chan prometheus.Metric, 1)
	go func() {
		c.Collect(ch)
		close(ch)
	}()

	j := 0
	for range ch {
		require.Fail(t, "Expected no reported metrics")
		j++
	}
}

func TestConvertMetric(t *testing.T) {
	tests := []struct {
		description string
		mName       string
		mType       pmetric.MetricType
		mapVals     map[string]metricFamily
		err         bool
	}{
		{
			description: "invalid histogram metric",
			mType:       pmetric.MetricTypeHistogram,
			err:         true,
		},
		{
			description: "invalid sum metric",
			mType:       pmetric.MetricTypeSum,
			err:         true,
		},
		{
			description: "invalid gauge metric",
			mType:       pmetric.MetricTypeGauge,
			err:         true,
		},
		{
			description: "metric type conflict",
			mName:       "testgauge",
			mType:       pmetric.MetricTypeGauge,
			mapVals: map[string]metricFamily{
				"testgauge": {
					mf: &io_prometheus_client.MetricFamily{
						Name: new("testgauge"),
						Type: io_prometheus_client.MetricType_COUNTER.Enum(),
					},
				},
			},
			err: true,
		},
		{
			description: "metric description conflict",
			mName:       "testgauge",
			mType:       pmetric.MetricTypeGauge,
			mapVals: map[string]metricFamily{
				"testgauge": {
					mf: &io_prometheus_client.MetricFamily{
						Name: new("testgauge"),
						Type: io_prometheus_client.MetricType_GAUGE.Enum(),
						Help: new("test help value"),
					},
				},
			},
			err: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			metric := pmetric.NewMetric()
			metric.SetName(tt.mName)
			switch tt.mType {
			case pmetric.MetricTypeGauge:
				metric.SetEmptyGauge().DataPoints().AppendEmpty()
			case pmetric.MetricTypeSum:
				metric.SetEmptySum().DataPoints().AppendEmpty()
			case pmetric.MetricTypeHistogram:
				metric.SetEmptyHistogram().DataPoints().AppendEmpty()
			}
			c := newCollector(&Config{}, zap.NewNop())
			for k, v := range tt.mapVals {
				c.metricFamilies.Store(k, v)
			}

			_, err := c.convertMetric(metric, pcommon.NewMap(), "test", "1.0.0", "http://test.com", pcommon.NewMap())
			if tt.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func setUpTestExemplar(exemplar pmetric.Exemplar) {
	var tBytes [16]byte
	testTraceID, _ := hex.DecodeString("641d68e314a58152cc2581e7663435d1")
	copy(tBytes[:], testTraceID)
	traceID := pcommon.TraceID(tBytes)
	exemplar.SetTraceID(traceID)

	var sBytes [8]byte
	testSpanID, _ := hex.DecodeString("7436d6ac76178623")
	copy(sBytes[:], testSpanID)
	spanID := pcommon.SpanID(sBytes)
	exemplar.SetSpanID(spanID)

	exemplarTs, _ := time.Parse("unix", "Mon Jan _2 15:04:05 MST 2006")
	exemplar.SetTimestamp(pcommon.NewTimestampFromTime(exemplarTs))
}

func setTestExemplarWithDoubleValue(exemplar pmetric.Exemplar, value float64) {
	setUpTestExemplar(exemplar)
	exemplar.SetDoubleValue(value)
}

func setTextExemplarWithIntValue(exemplar pmetric.Exemplar, value int64) {
	setUpTestExemplar(exemplar)
	exemplar.SetIntValue(value)
}

func exemplarsEqual(t *testing.T, otelExemplar pmetric.Exemplar, promExemplar *io_prometheus_client.Exemplar) {
	var givenValue float64
	switch otelExemplar.ValueType() {
	case pmetric.ExemplarValueTypeDouble:
		givenValue = otelExemplar.DoubleValue()
	case pmetric.ExemplarValueTypeInt:
		givenValue = float64(otelExemplar.IntValue())
	default:
		t.Error("Unexpected value type for OTel exemplar", otelExemplar.ValueType())
	}

	require.Equal(t, givenValue, promExemplar.GetValue())
	require.Len(t, promExemplar.GetLabel(), 2)
	ml := make(map[string]string)
	for _, l := range promExemplar.GetLabel() {
		ml[l.GetName()] = l.GetValue()
	}
	traceID := otelExemplar.TraceID()
	spanID := otelExemplar.SpanID()
	require.Equal(t, hex.EncodeToString(traceID[:]), ml["trace_id"])
	require.Equal(t, hex.EncodeToString(spanID[:]), ml["span_id"])
}

func TestConvertDoubleHistogramExemplar(t *testing.T) {
	// initialize empty histogram
	metric := pmetric.NewMetric()
	metric.SetName("test_metric")
	metric.SetDescription("this is test metric")
	metric.SetUnit("T")

	// initialize empty datapoint
	histogramDataPoint := metric.SetEmptyHistogram().DataPoints().AppendEmpty()

	histogramDataPoint.ExplicitBounds().FromRaw([]float64{5, 25, 90})
	histogramDataPoint.BucketCounts().FromRaw([]uint64{2, 35, 70})

	// add test exemplar values to the metric
	promExporterExemplars := histogramDataPoint.Exemplars().AppendEmpty()
	setTestExemplarWithDoubleValue(promExporterExemplars, 3.0)

	pMap := pcommon.NewMap()

	c := newCollector(&Config{}, zap.NewNop())
	c.accumulator = &mockAccumulator{
		metrics:            []pmetric.Metric{metric},
		resourceAttributes: pMap,
	}

	pbMetric, _ := c.convertDoubleHistogram(metric, pMap, "test", "1.0.0", "http://test.com", pcommon.NewMap())
	m := io_prometheus_client.Metric{}
	err := pbMetric.Write(&m)
	if err != nil {
		return
	}

	buckets := m.GetHistogram().GetBucket()

	require.Len(t, buckets, 3)

	require.Equal(t, 3.0, buckets[0].GetExemplar().GetValue())
	exemplarsEqual(t, promExporterExemplars, buckets[0].GetExemplar())
}

func TestConvertDoubleHistogramEmptyBucketCounts(t *testing.T) {
	metric := pmetric.NewMetric()
	metric.SetName("test_metric")
	metric.SetDescription("this is test metric")
	metric.SetUnit("T")

	histogramDataPoint := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
	histogramDataPoint.ExplicitBounds().FromRaw([]float64{5, 25, 90})

	pMap := pcommon.NewMap()

	c := newCollector(&Config{}, zap.NewNop())
	c.accumulator = &mockAccumulator{
		metrics:            []pmetric.Metric{metric},
		resourceAttributes: pMap,
	}

	// Should not panic
	_, err := c.convertDoubleHistogram(metric, pMap, "test", "1.0.0", "http://test.com", pcommon.NewMap())
	require.NoError(t, err)
}

func TestConvertMonotonicSumExemplar(t *testing.T) {
	// initialize empty metric
	metric := pmetric.NewMetric()
	metric.SetName("test_monotonic_sum")
	metric.SetDescription("this is test monotonic sum metric")
	metric.SetUnit("T")

	sum := metric.SetEmptySum()
	sum.SetIsMonotonic(true)

	dataPoint := sum.DataPoints().AppendEmpty()
	dataPoint.SetIntValue(1)

	exemplar := dataPoint.Exemplars().AppendEmpty()
	setTextExemplarWithIntValue(exemplar, 1)

	pMap := pcommon.NewMap()

	c := newCollector(&Config{}, zap.NewNop())
	c.accumulator = &mockAccumulator{
		metrics:            []pmetric.Metric{metric},
		resourceAttributes: pMap,
	}

	promMetric, _ := c.convertSum(metric, pMap, "test", "1.0.0", "http://test.com", pcommon.NewMap())
	outMetric := io_prometheus_client.Metric{}
	err := promMetric.Write(&outMetric)
	if err != nil {
		t.Error("Unable to write metric to prometheus output form:", err)
	}

	promCounter := outMetric.GetCounter()
	require.Equal(t, 1.0, promCounter.GetValue())
	exemplarsEqual(t, exemplar, promCounter.GetExemplar())
}

func TestConvertExponentialHistogramExemplar(t *testing.T) {
	metric := pmetric.NewMetric()
	metric.SetName("test_metric")
	metric.SetDescription("this is test metric")
	metric.SetUnit("T")

	expHist := metric.SetEmptyExponentialHistogram()
	expHist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

	expDP := expHist.DataPoints().AppendEmpty()
	expDP.SetTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))
	expDP.SetCount(7)
	expDP.SetSum(1)
	expDP.SetScale(0)
	expDP.SetZeroThreshold(1)
	expDP.SetZeroCount(1)
	expDP.Positive().SetOffset(-1)
	expDP.Positive().BucketCounts().FromRaw([]uint64{2, 4})
	expDP.Attributes().PutStr("foo", "bar")

	ex := expDP.Exemplars().AppendEmpty()
	setTestExemplarWithDoubleValue(ex, 3.0)

	pMap := pcommon.NewMap()
	c := newCollector(&Config{}, zap.NewNop())
	c.accumulator = &mockAccumulator{
		metrics:            []pmetric.Metric{metric},
		resourceAttributes: pMap,
	}

	pbMetric, err := c.convertExponentialHistogram(metric, pMap, "test", "1.0.0", "http://test.com", pcommon.NewMap())
	require.NoError(t, err)

	m := io_prometheus_client.Metric{}
	require.NoError(t, pbMetric.Write(&m))

	h := m.GetHistogram()
	require.NotNil(t, h)
	require.Len(t, h.GetExemplars(), 1)
	exemplarsEqual(t, ex, h.GetExemplars()[0])
}

// errorCheckCore keeps track of logged errors
type errorCheckCore struct {
	errorMessages []string
}

func (*errorCheckCore) Enabled(zapcore.Level) bool      { return true }
func (c *errorCheckCore) With([]zap.Field) zapcore.Core { return c }
func (c *errorCheckCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

func (c *errorCheckCore) Write(ent zapcore.Entry, _ []zapcore.Field) error {
	if ent.Level == zapcore.ErrorLevel {
		c.errorMessages = append(c.errorMessages, ent.Message)
	}
	return nil
}
func (*errorCheckCore) Sync() error { return nil }

func TestCollectMetricsLabelSanitize(t *testing.T) {
	metric := pmetric.NewMetric()
	metric.SetName("test_metric")
	metric.SetDescription("test description")
	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetIntValue(42)
	dp.Attributes().PutStr("label.1", "1")
	dp.Attributes().PutStr("label/2", "2")
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	loggerCore := errorCheckCore{}

	c := newCollector(&Config{
		Namespace:      "test_space",
		SendTimestamps: false,
	}, zap.New(&loggerCore))
	// Replace accumulator with mock for test control
	c.accumulator = &mockAccumulator{
		[]pmetric.Metric{metric},
		pcommon.NewMap(),
		[]string{""},
		[]string{""},
		[]string{""},
		[]pcommon.Map{pcommon.NewMap()},
	}

	ch := make(chan prometheus.Metric, 1)
	go func() {
		c.Collect(ch)
		close(ch)
	}()

	for m := range ch {
		require.Contains(t, m.Desc().String(), "fqName: \"test_space_test_metric\"")
		require.Contains(t, m.Desc().String(), "label_1")
		require.Contains(t, m.Desc().String(), "label_2")

		pbMetric := io_prometheus_client.Metric{}
		require.NoError(t, m.Write(&pbMetric))

		labelsKeys := map[string]string{"label_1": "1", "label_2": "2"}
		for _, l := range pbMetric.Label {
			require.Equal(t, labelsKeys[*l.Name], *l.Value)
		}
	}

	require.Empty(t, loggerCore.errorMessages, "labels were not sanitized properly")
}

func TestWithoutScopeInfoFlag(t *testing.T) {
	metric := pmetric.NewMetric()
	metric.SetName("test_metric")
	metric.SetDescription("test description")
	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetIntValue(42)
	dp.Attributes().PutStr("somelabel", "1")
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	loggerCore := errorCheckCore{}
	// Replace accumulator with mock for test control
	scopeAttributes := pcommon.NewMap()
	scopeAttributes.PutStr("lib", "clickhouse")
	scopeAttributes.PutStr("repo", "https://gitlab/project/source")
	ma := &mockAccumulator{
		[]pmetric.Metric{metric},
		pcommon.NewMap(),
		[]string{"io.opentelemetry.contrib.clickhouse"},
		[]string{"1.0.0"},
		[]string{"https://opentelemetry.io/schemas/1.7.0"},
		[]pcommon.Map{scopeAttributes},
	}

	withoutScopeCollector := newCollector(&Config{
		Namespace:        "test_space",
		SendTimestamps:   false,
		WithoutScopeInfo: true,
	}, zap.New(&loggerCore))
	withoutScopeCollector.accumulator = ma
	withScopeCollector := newCollector(&Config{
		Namespace:        "test_space",
		SendTimestamps:   false,
		WithoutScopeInfo: false,
	}, zap.New(&loggerCore))
	withScopeCollector.accumulator = ma

	withoutScopeCh := make(chan prometheus.Metric, 1)
	withScopeCh := make(chan prometheus.Metric, 1)
	go func() {
		withoutScopeCollector.Collect(withoutScopeCh)
		close(withoutScopeCh)
	}()
	go func() {
		withScopeCollector.Collect(withScopeCh)
		close(withScopeCh)
	}()

	for m := range withoutScopeCh {
		pbMetric := io_prometheus_client.Metric{}
		require.NoError(t, m.Write(&pbMetric))
		actualLabels := []string{}
		for _, l := range pbMetric.Label {
			actualLabels = append(actualLabels, *l.Name)
		}
		require.ElementsMatch(t, actualLabels, []string{"somelabel"})
	}
	for m := range withScopeCh {
		pbMetric := io_prometheus_client.Metric{}
		require.NoError(t, m.Write(&pbMetric))
		actualLabels := []string{}
		for _, l := range pbMetric.Label {
			actualLabels = append(actualLabels, *l.Name)
		}
		require.ElementsMatch(t, actualLabels, []string{"somelabel", "otel_scope_name", "otel_scope_version", "otel_scope_schema_url" /* scope attributes*/, "otel_scope_lib", "otel_scope_repo"})
	}

	require.Empty(t, loggerCore.errorMessages, "collector unexpectedly returned an error")
}

func TestScopeAttributeConflictsDropped(t *testing.T) {
	metric := pmetric.NewMetric()
	metric.SetName("test_metric")
	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetIntValue(42)
	dp.Attributes().PutStr("somelabel", "1")

	scopeAttributes := pcommon.NewMap()
	scopeAttributes.PutStr("name", "should-be-dropped")
	scopeAttributes.PutStr("version", "should-be-dropped")
	scopeAttributes.PutStr("schema_url", "should-be-dropped")
	scopeAttributes.PutStr("custom", "should-be-kept")

	c := newCollector(&Config{}, zap.NewNop())
	m, err := c.convertMetric(metric, pcommon.NewMap(), "test.scope", "1.0.0", "https://opentelemetry.io/schemas/1.7.0", scopeAttributes)
	require.NoError(t, err)

	pbMetric := io_prometheus_client.Metric{}
	require.NoError(t, m.Write(&pbMetric))

	labels := make(map[string]string, len(pbMetric.Label))
	for _, l := range pbMetric.Label {
		labels[l.GetName()] = l.GetValue()
	}

	require.Equal(t, map[string]string{
		"somelabel":             "1",
		"otel_scope_name":       "test.scope",
		"otel_scope_version":    "1.0.0",
		"otel_scope_schema_url": "https://opentelemetry.io/schemas/1.7.0",
		"otel_scope_custom":     "should-be-kept",
	}, labels)
}

func TestCollectMetrics(t *testing.T) {
	tests := []struct {
		name       string
		metric     func(time.Time, bool) pmetric.Metric
		metricType prometheus.ValueType
		value      float64
	}{
		{
			name:       "IntGauge",
			metricType: prometheus.GaugeValue,
			value:      42.0,
			metric: func(ts time.Time, withStartTime bool) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_metric")
				metric.SetDescription("test description")
				dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
				dp.SetIntValue(42)
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				if withStartTime {
					dp.SetStartTimestamp(pcommon.NewTimestampFromTime(ts))
				}

				return metric
			},
		},
		{
			name:       "Gauge",
			metricType: prometheus.GaugeValue,
			value:      42.42,
			metric: func(ts time.Time, withStartTime bool) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_metric")
				metric.SetDescription("test description")
				dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
				dp.SetDoubleValue(42.42)
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				if withStartTime {
					dp.SetStartTimestamp(pcommon.NewTimestampFromTime(ts))
				}

				return metric
			},
		},
		{
			name:       "IntSum",
			metricType: prometheus.GaugeValue,
			value:      42.0,
			metric: func(ts time.Time, withStartTime bool) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_metric")
				metric.SetEmptySum().SetIsMonotonic(false)
				metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				metric.SetDescription("test description")
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetIntValue(42)
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				if withStartTime {
					dp.SetStartTimestamp(pcommon.NewTimestampFromTime(ts))
				}

				return metric
			},
		},
		{
			name:       "Sum",
			metricType: prometheus.GaugeValue,
			value:      42.42,
			metric: func(ts time.Time, withStartTime bool) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_metric")
				metric.SetEmptySum().SetIsMonotonic(false)
				metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				metric.SetDescription("test description")
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(42.42)
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				if withStartTime {
					dp.SetStartTimestamp(pcommon.NewTimestampFromTime(ts))
				}

				return metric
			},
		},
		{
			name:       "MonotonicIntSum",
			metricType: prometheus.CounterValue,
			value:      42.0,
			metric: func(ts time.Time, withStartTime bool) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_metric")
				metric.SetEmptySum().SetIsMonotonic(true)
				metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				metric.SetDescription("test description")
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetIntValue(42)
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				if withStartTime {
					dp.SetStartTimestamp(pcommon.NewTimestampFromTime(ts))
				}

				return metric
			},
		},
		{
			name:       "MonotonicSum",
			metricType: prometheus.CounterValue,
			value:      42.42,
			metric: func(ts time.Time, withStartTime bool) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_metric")
				metric.SetEmptySum().SetIsMonotonic(true)
				metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				metric.SetDescription("test description")
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(42.42)
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				if withStartTime {
					dp.SetStartTimestamp(pcommon.NewTimestampFromTime(ts))
				}

				return metric
			},
		},
		{
			name:       "Unknown",
			metricType: prometheus.UntypedValue,
			value:      42.42,
			metric: func(ts time.Time, withStartTime bool) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_metric")
				metric.SetDescription("test description")
				metric.Metadata().PutStr(prometheustranslator.MetricMetadataTypeKey, "unknown")
				dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
				dp.SetDoubleValue(42.42)
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				if withStartTime {
					dp.SetStartTimestamp(pcommon.NewTimestampFromTime(ts))
				}

				return metric
			},
		},
	}

	for _, tt := range tests {
		for _, sendTimestamp := range []bool{true, false} {
			name := tt.name
			// In this test, sendTimestamp is used to test
			// both prometheus regular timestamp and "created timestamp".
			if sendTimestamp {
				name += "/WithTimestamp"
			}

			rAttrs := pcommon.NewMap()
			rAttrs.PutStr("service.instance.id", "localhost:9090")
			rAttrs.PutStr("service.name", "testapp")
			rAttrs.PutStr("service.namespace", "prod")

			t.Run(name, func(t *testing.T) {
				ts := time.Now()
				metric := tt.metric(ts, sendTimestamp)

				c := newCollector(&Config{
					Namespace:      "test_space",
					SendTimestamps: sendTimestamp,
				}, zap.NewNop())
				// Replace accumulator with mock for test control
				c.accumulator = &mockAccumulator{
					[]pmetric.Metric{metric},
					rAttrs,
					[]string{"test"},
					[]string{"1.0.0"},
					[]string{"http://test.com"},
					[]pcommon.Map{pcommon.NewMap()},
				}

				ch := make(chan prometheus.Metric, 1)
				go func() {
					c.Collect(ch)
					close(ch)
				}()

				j := 0
				for m := range ch {
					j++

					if strings.Contains(m.Desc().String(), "fqName: \"test_space_target_info\"") {
						pbMetric := io_prometheus_client.Metric{}
						require.NoError(t, m.Write(&pbMetric))

						labelsKeys := map[string]string{"job": "prod/testapp", "instance": "localhost:9090"}
						for _, l := range pbMetric.Label {
							require.Equal(t, labelsKeys[*l.Name], *l.Value)
						}

						continue
					}

					expectedName := "test_space_test_metric"
					if tt.metricType == prometheus.CounterValue {
						expectedName += "_total"
					}
					require.Contains(t, m.Desc().String(), `fqName: "`+expectedName+`"`)
					require.Contains(t, m.Desc().String(), `variableLabels: {label_1,label_2,otel_scope_name,otel_scope_version,otel_scope_schema_url,job,instance}`)

					pbMetric := io_prometheus_client.Metric{}
					require.NoError(t, m.Write(&pbMetric))

					labelsKeys := map[string]string{"label_1": "1", "label_2": "2", "otel_scope_name": "test", "otel_scope_version": "1.0.0", "otel_scope_schema_url": "http://test.com", "job": "prod/testapp", "instance": "localhost:9090"}
					for _, l := range pbMetric.Label {
						require.Equal(t, labelsKeys[*l.Name], *l.Value)
					}

					if sendTimestamp {
						require.Equal(t, ts.UnixNano()/1e6, *pbMetric.TimestampMs)
						// Prometheus gauges don't have created timestamp.
						if tt.metricType == prometheus.CounterValue {
							require.Equal(t, timestamppb.New(ts), pbMetric.Counter.CreatedTimestamp)
						}
					} else {
						require.Nil(t, pbMetric.TimestampMs)
						if tt.metricType == prometheus.CounterValue {
							require.Nil(t, pbMetric.Counter.CreatedTimestamp)
						}
					}

					switch tt.metricType {
					case prometheus.CounterValue:
						require.Equal(t, tt.value, *pbMetric.Counter.Value)
						require.Nil(t, pbMetric.Gauge)
						require.Nil(t, pbMetric.Histogram)
						require.Nil(t, pbMetric.Summary)
					case prometheus.GaugeValue:
						require.Equal(t, tt.value, *pbMetric.Gauge.Value)
						require.Nil(t, pbMetric.Counter)
						require.Nil(t, pbMetric.Histogram)
						require.Nil(t, pbMetric.Summary)
					}
				}
				require.Equal(t, 2, j)
			})
		}
	}
}

func TestAccumulateHistograms(t *testing.T) {
	tests := []struct {
		name   string
		metric func(time.Time, bool) pmetric.Metric

		histogramPoints map[float64]uint64
		histogramSum    float64
		histogramCount  uint64
	}{
		{
			name: "Histogram",
			histogramPoints: map[float64]uint64{
				3.5:  5,
				10.0: 7,
			},
			histogramSum:   42.42,
			histogramCount: 7,
			metric: func(ts time.Time, withStartTime bool) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_metric")
				metric.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				metric.SetDescription("test description")
				dp := metric.Histogram().DataPoints().AppendEmpty()
				dp.BucketCounts().FromRaw([]uint64{5, 2})
				dp.SetCount(7)
				dp.ExplicitBounds().FromRaw([]float64{3.5, 10.0})
				dp.SetSum(42.42)
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				if withStartTime {
					dp.SetStartTimestamp(pcommon.NewTimestampFromTime(ts))
				}
				return metric
			},
		},
	}

	for _, tt := range tests {
		for _, sendTimestamp := range []bool{true, false} {
			name := tt.name
			// In this test, sendTimestamp is used to test
			// both prometheus regular timestamp and "created timestamp".
			if sendTimestamp {
				name += "/WithTimestamp"
			}
			t.Run(name, func(t *testing.T) {
				ts := time.Now()
				metric := tt.metric(ts, sendTimestamp)
				c := newCollector(&Config{
					SendTimestamps: sendTimestamp,
				}, zap.NewNop())
				// Replace accumulator with mock for test control
				c.accumulator = &mockAccumulator{
					[]pmetric.Metric{metric},
					pcommon.NewMap(),
					[]string{""},
					[]string{""},
					[]string{""},
					[]pcommon.Map{pcommon.NewMap()},
				}

				ch := make(chan prometheus.Metric, 1)
				go func() {
					c.Collect(ch)
					close(ch)
				}()

				n := 0
				for m := range ch {
					n++
					require.Contains(t, m.Desc().String(), "fqName: \"test_metric\"")
					require.Contains(t, m.Desc().String(), "label_1")
					require.Contains(t, m.Desc().String(), "label_2")

					pbMetric := io_prometheus_client.Metric{}
					require.NoError(t, m.Write(&pbMetric))

					labelsKeys := map[string]string{"label_1": "1", "label_2": "2"}
					for _, l := range pbMetric.Label {
						require.Equal(t, labelsKeys[*l.Name], *l.Value)
					}

					if sendTimestamp {
						require.Equal(t, ts.UnixNano()/1e6, *pbMetric.TimestampMs)
						require.Equal(t, timestamppb.New(ts), pbMetric.Histogram.CreatedTimestamp)
					} else {
						require.Nil(t, pbMetric.TimestampMs)
						require.Nil(t, pbMetric.Histogram.CreatedTimestamp)
					}

					require.Nil(t, pbMetric.Gauge)
					require.Nil(t, pbMetric.Counter)

					h := pbMetric.Histogram
					require.Equal(t, tt.histogramCount, h.GetSampleCount())
					require.Equal(t, tt.histogramSum, h.GetSampleSum())
					require.Len(t, h.Bucket, len(tt.histogramPoints))

					for _, b := range h.Bucket {
						require.Equal(t, tt.histogramPoints[(*b).GetUpperBound()], b.GetCumulativeCount())
					}
				}
				require.Equal(t, 1, n)
			})
		}
	}
}

func TestAccumulateExponentialHistograms(t *testing.T) {
	tests := []struct {
		name           string
		metric         func(time.Time, bool) pmetric.Metric
		wantCount      uint64
		wantSum        float64
		wantSchema     int32
		wantZeroCount  uint64
		wantZeroThresh float64
	}{
		{
			name:           "NativeHistogram basic (zero threshold)",
			wantCount:      10,
			wantSum:        42.0,
			wantSchema:     0,
			wantZeroCount:  4,
			wantZeroThresh: 0,
			metric: func(ts time.Time, withStartTime bool) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_native_hist")
				metric.SetEmptyExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := metric.ExponentialHistogram().DataPoints().AppendEmpty()
				dp.SetScale(0)
				dp.Positive().SetOffset(0)
				dp.Positive().BucketCounts().FromRaw([]uint64{1, 2})
				dp.Negative().SetOffset(0)
				dp.Negative().BucketCounts().FromRaw([]uint64{3})
				dp.SetZeroCount(4)
				dp.SetCount(10)
				dp.SetSum(42.0)
				dp.SetZeroThreshold(0)
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				if withStartTime {
					dp.SetStartTimestamp(pcommon.NewTimestampFromTime(ts))
				}

				return metric
			},
		},
		{
			name:           "NativeHistogram scale down (>8 to 8)",
			wantCount:      6,
			wantSum:        5.5,
			wantSchema:     8,
			wantZeroCount:  1,
			wantZeroThresh: 0.25,
			metric: func(ts time.Time, withStartTime bool) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_native_hist_scaled")
				metric.SetEmptyExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := metric.ExponentialHistogram().DataPoints().AppendEmpty()
				dp.SetScale(10) // will be downscaled to 8
				dp.Positive().SetOffset(0)
				dp.Positive().BucketCounts().FromRaw([]uint64{2, 3})
				dp.Negative().SetOffset(-1)
				dp.Negative().BucketCounts().FromRaw([]uint64{0, 0})
				dp.SetZeroCount(1)
				dp.SetCount(6)
				dp.SetSum(5.5)
				dp.SetZeroThreshold(0.25)
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				if withStartTime {
					dp.SetStartTimestamp(pcommon.NewTimestampFromTime(ts))
				}
				return metric
			},
		},
	}

	for _, tt := range tests {
		for _, sendTimestamp := range []bool{true, false} {
			name := tt.name
			if sendTimestamp {
				name += "/WithTimestamp"
			}
			t.Run(name, func(t *testing.T) {
				ts := time.Now()
				metric := tt.metric(ts, sendTimestamp)
				c := newCollector(&Config{SendTimestamps: sendTimestamp}, zap.NewNop())
				// Replace accumulator with mock for test control
				c.accumulator = &mockAccumulator{
					[]pmetric.Metric{metric},
					pcommon.NewMap(),
					[]string{""},
					[]string{""},
					[]string{""},
					[]pcommon.Map{pcommon.NewMap()},
				}

				ch := make(chan prometheus.Metric, 1)
				go func() {
					c.Collect(ch)
					close(ch)
				}()

				n := 0
				for m := range ch {
					n++
					require.Contains(t, m.Desc().String(), "fqName: \""+metric.Name()+"\"")

					pbMetric := io_prometheus_client.Metric{}
					require.NoError(t, m.Write(&pbMetric))

					// Assert timestamp behavior
					if sendTimestamp {
						require.Equal(t, ts.UnixNano()/1e6, *pbMetric.TimestampMs)
						// withStartTime is tied to sendTimestamp in this test
						require.Equal(t, timestamppb.New(ts), pbMetric.Histogram.CreatedTimestamp)
					} else {
						require.Nil(t, pbMetric.TimestampMs)
						// Native histograms always include CreatedTimestamp; when no start
						// time is set, it encodes the zero time (0001-01-01) rather than nil.
						require.NotNil(t, pbMetric.Histogram.CreatedTimestamp)
						require.True(t, pbMetric.Histogram.CreatedTimestamp.AsTime().IsZero())
					}

					h := pbMetric.Histogram
					require.NotNil(t, h)
					require.Equal(t, tt.wantCount, h.GetSampleCount())
					require.InDelta(t, tt.wantSum, h.GetSampleSum(), 1e-12)
					require.Equal(t, tt.wantSchema, h.GetSchema())
					require.Equal(t, tt.wantZeroCount, h.GetZeroCount())
					requireNoZeroBucketOverlap(t, h)
					require.InDelta(t, tt.wantZeroThresh, h.GetZeroThreshold(), 0)
				}
				require.Equal(t, 1, n)
			})
		}
	}
}

// requireNoZeroBucketOverlap checks that no populated bucket overlaps the zero bucket.
func requireNoZeroBucketOverlap(t *testing.T, h *io_prometheus_client.Histogram) {
	toSpans := func(spans []*io_prometheus_client.BucketSpan) []histogram.Span {
		out := make([]histogram.Span, 0, len(spans))
		for _, s := range spans {
			out = append(out, histogram.Span{Offset: s.GetOffset(), Length: s.GetLength()})
		}
		return out
	}
	nh := &histogram.Histogram{
		Schema:          h.GetSchema(),
		ZeroThreshold:   h.GetZeroThreshold(),
		PositiveSpans:   toSpans(h.GetPositiveSpan()),
		PositiveBuckets: h.GetPositiveDelta(),
		NegativeSpans:   toSpans(h.GetNegativeSpan()),
		NegativeBuckets: h.GetNegativeDelta(),
	}
	for it := nh.PositiveBucketIterator(); it.Next(); {
		b := it.At()
		require.GreaterOrEqualf(t, b.Lower, nh.ZeroThreshold, "positive bucket %s overlaps the zero bucket with threshold %g", b.String(), nh.ZeroThreshold)
	}
	for it := nh.NegativeBucketIterator(); it.Next(); {
		b := it.At()
		require.LessOrEqualf(t, b.Upper, -nh.ZeroThreshold, "negative bucket %s overlaps the zero bucket with threshold %g", b.String(), nh.ZeroThreshold)
	}
}

func TestConvertExponentialHistogramInvalidScale(t *testing.T) {
	metric := pmetric.NewMetric()
	metric.SetName("invalid_native_hist")
	metric.SetEmptyExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	dp := metric.ExponentialHistogram().DataPoints().AppendEmpty()
	dp.SetScale(-5) // invalid: must be >= -4
	dp.SetCount(0)
	dp.SetZeroThreshold(0)

	c := newCollector(&Config{}, zap.NewNop())
	_, err := c.convertExponentialHistogram(metric, pcommon.NewMap(), "", "", "", pcommon.NewMap())
	require.Error(t, err)
	require.Contains(t, err.Error(), "scale must be >= -4")
}

func TestCollectExponentialHistogramSum(t *testing.T) {
	for _, tt := range []struct {
		name   string
		hasSum bool
		sum    float64
	}{
		{name: "absent sum"},
		{name: "zero sum", hasSum: true},
		{name: "nonzero sum", hasSum: true, sum: -0.5},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rm := pmetric.NewResourceMetrics()
			metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
			metric.SetName("test_native_hist")
			h := metric.SetEmptyExponentialHistogram()
			h.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			dp := h.DataPoints().AppendEmpty()
			dp.SetCount(2)
			dp.Positive().BucketCounts().FromRaw([]uint64{1})
			dp.Negative().BucketCounts().FromRaw([]uint64{1})
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			if tt.hasSum {
				dp.SetSum(tt.sum)
			}
			// Another series must still be exported when the first one is dropped.
			valid := h.DataPoints().AppendEmpty()
			dp.CopyTo(valid)
			valid.Attributes().PutStr("series", "valid")
			valid.SetSum(0.5)

			c := newCollector(&Config{MetricExpiration: time.Minute}, zap.NewNop())
			c.accumulator.Accumulate(rm)
			registry := prometheus.NewRegistry()
			registry.MustRegister(c)
			families, err := registry.Gather()
			require.NoError(t, err)
			require.Len(t, families, 1)
			wantSums := []float64{0.5}
			if tt.hasSum {
				wantSums = append(wantSums, tt.sum)
			}
			var sums []float64
			for _, m := range families[0].GetMetric() {
				require.NotNil(t, m.GetHistogram().Schema)
				sums = append(sums, m.GetHistogram().GetSampleSum())
			}
			require.ElementsMatch(t, wantSums, sums)
		})
	}
}

func TestAccumulateSummary(t *testing.T) {
	fillQuantileValue := func(pN, value float64, dest pmetric.SummaryDataPointValueAtQuantile) {
		dest.SetQuantile(pN)
		dest.SetValue(value)
	}
	tests := []struct {
		name          string
		metric        func(time.Time, bool) pmetric.Metric
		wantSum       float64
		wantCount     uint64
		wantQuantiles map[float64]float64
	}{
		{
			name:      "Summary with single point",
			wantSum:   0.012,
			wantCount: 10,
			wantQuantiles: map[float64]float64{
				0.50: 190,
				0.99: 817,
			},
			metric: func(ts time.Time, withStartTime bool) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_metric")
				metric.SetDescription("test description")
				sp := metric.SetEmptySummary().DataPoints().AppendEmpty()
				sp.SetCount(10)
				sp.SetSum(0.012)
				sp.SetCount(10)
				sp.Attributes().PutStr("label_1", "1")
				sp.Attributes().PutStr("label_2", "2")
				sp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				if withStartTime {
					sp.SetStartTimestamp(pcommon.NewTimestampFromTime(ts))
				}

				fillQuantileValue(0.50, 190, sp.QuantileValues().AppendEmpty())
				fillQuantileValue(0.99, 817, sp.QuantileValues().AppendEmpty())

				return metric
			},
		},
	}

	for _, tt := range tests {
		for _, sendTimestamp := range []bool{true, false} {
			name := tt.name
			// In this test, sendTimestamp is used to test
			// both prometheus regular timestamp and "created timestamp".
			if sendTimestamp {
				name += "/WithTimestamp"
			}
			t.Run(name, func(t *testing.T) {
				ts := time.Now()
				metric := tt.metric(ts, sendTimestamp)
				c := newCollector(&Config{
					SendTimestamps: sendTimestamp,
				}, zap.NewNop())
				// Replace accumulator with mock for test control
				c.accumulator = &mockAccumulator{
					[]pmetric.Metric{metric},
					pcommon.NewMap(),
					[]string{""},
					[]string{""},
					[]string{""},
					[]pcommon.Map{pcommon.NewMap()},
				}

				ch := make(chan prometheus.Metric, 1)
				go func() {
					c.Collect(ch)
					close(ch)
				}()

				n := 0
				for m := range ch {
					n++
					require.Contains(t, m.Desc().String(), "fqName: \"test_metric\"")
					require.Contains(t, m.Desc().String(), "label_1")
					require.Contains(t, m.Desc().String(), "label_2")

					pbMetric := io_prometheus_client.Metric{}
					require.NoError(t, m.Write(&pbMetric))

					labelsKeys := map[string]string{"label_1": "1", "label_2": "2"}
					for _, l := range pbMetric.Label {
						require.Equal(t, labelsKeys[*l.Name], *l.Value)
					}

					if sendTimestamp {
						require.Equal(t, ts.UnixNano()/1e6, *pbMetric.TimestampMs)
						require.Equal(t, timestamppb.New(ts), pbMetric.Summary.CreatedTimestamp)
					} else {
						require.Nil(t, pbMetric.TimestampMs)
						require.Nil(t, pbMetric.Summary.CreatedTimestamp)
					}

					require.Nil(t, pbMetric.Gauge)
					require.Nil(t, pbMetric.Counter)
					require.Nil(t, pbMetric.Histogram)

					s := pbMetric.Summary
					require.Equal(t, tt.wantCount, *s.SampleCount)
					require.Equal(t, tt.wantSum, *s.SampleSum)
					// To ensure that we can compare quantiles, we need to just extract their values.
					gotQuantiles := make(map[float64]float64)
					for _, q := range s.Quantile {
						gotQuantiles[q.GetQuantile()] = q.GetValue()
					}
					require.Equal(t, tt.wantQuantiles, gotQuantiles)
				}
				require.Equal(t, 1, n)
			})
		}
	}
}

func TestNormalizeNamespaceEmpty(t *testing.T) {
	logger := zap.NewNop()
	labelNamer := otlptranslator.LabelNamer{UTF8Allowed: false}

	ns := normalizeNamespace("", labelNamer, logger)
	require.Empty(t, ns, "empty configNamespace should yield empty namespace")
}

func TestNormalizeNamespaceInvalid(t *testing.T) {
	logger := zap.NewNop()
	labelNamer := otlptranslator.LabelNamer{UTF8Allowed: false}

	ns := normalizeNamespace("----", labelNamer, logger)
	require.Empty(t, ns, "configNamespace with no valid characters should yield empty namespace")
}

func TestNormalizeNamespaceSanitizes(t *testing.T) {
	logger := zap.NewNop()

	// With UTF-8 not allowed, chars like '.' should be mapped to '_'
	labelNamer := otlptranslator.LabelNamer{UTF8Allowed: false}
	ns1 := normalizeNamespace("my_namespace.1", labelNamer, logger)
	require.Equal(t, "my_namespace_1", ns1, "UTF-8 not allowed should sanitize '.' to '_'")

	// With UTF-8 allowed, ASCII characters remain unchanged
	labelNamerUTF8 := otlptranslator.LabelNamer{UTF8Allowed: true}
	ns2 := normalizeNamespace("my_namespace.1", labelNamerUTF8, logger)

	require.Equal(t, "my_namespace.1", ns2, "UTF-8 allowed should not sanitize ASCII characters")
}

func TestTargetInfoIdentifyingResourceAttributes(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("enabled=%t", enabled), func(t *testing.T) {
			defer testutil.SetFeatureGateForTest(t, prometheustranslator.KeepIdentifyingResourceAttributesGate, enabled)()
			for _, strategy := range []translationStrategy{underscoreEscapingWithSuffixes, noTranslation} {
				for _, withOtherAttribute := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/other_attribute=%t", strategy, withOtherAttribute), func(t *testing.T) {
						attrs := pcommon.NewMap()
						attrs.PutStr("service.name", "api")
						attrs.PutStr("service.namespace", "shop")
						attrs.PutStr("service.instance.id", "replica-1")
						want := map[string]string{"job": "shop/api", "instance": "replica-1"}
						if withOtherAttribute {
							attrs.PutStr("region", "west")
							want["region"] = "west"
						}
						original := attrs.AsRaw()
						if enabled {
							for key, value := range original {
								if strings.HasPrefix(key, "service.") {
									if strategy != noTranslation {
										key = strings.ReplaceAll(key, ".", "_")
									}
									want[key] = value.(string)
								}
							}
						}
						c := newCollector(&Config{TranslationStrategy: strategy}, zap.NewNop())
						metrics, err := c.createTargetInfoMetrics([]pcommon.Map{attrs})
						require.NoError(t, err)
						require.Len(t, metrics, 1)
						var metric io_prometheus_client.Metric
						require.NoError(t, metrics[0].Write(&metric))
						got := map[string]string{}
						for _, label := range metric.Label {
							got[label.GetName()] = label.GetValue()
						}
						require.Equal(t, want, got)
						require.Equal(t, float64(1), metric.Gauge.GetValue())
						require.Equal(t, original, attrs.AsRaw())
					})
				}
			}
		})
	}
}

func TestConvertExponentialHistogramZeroThreshold(t *testing.T) {
	tests := []struct {
		name         string
		threshold    float64
		setThreshold bool
		offset       int32
	}{
		{name: "unset threshold", offset: -432},
		{name: "explicit zero threshold", setThreshold: true, offset: -432},
		{name: "nonzero threshold", threshold: 0.5, setThreshold: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := pmetric.NewMetric()
			metric.SetName("test_native_histogram")
			hist := metric.SetEmptyExponentialHistogram()
			hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			dp := hist.DataPoints().AppendEmpty()
			dp.SetScale(0)
			dp.SetCount(3)
			dp.SetSum(0)
			dp.SetZeroCount(1)
			if tt.setThreshold {
				dp.SetZeroThreshold(tt.threshold)
			}
			dp.Positive().SetOffset(tt.offset)
			dp.Positive().BucketCounts().FromRaw([]uint64{1})
			dp.Negative().SetOffset(tt.offset)
			dp.Negative().BucketCounts().FromRaw([]uint64{1})
			c := newCollector(&Config{}, zap.NewNop())
			m, err := c.convertExponentialHistogram(metric, pcommon.NewMap(), "", "", "", pcommon.NewMap())
			require.NoError(t, err)
			out := &io_prometheus_client.Metric{}
			require.NoError(t, m.Write(out))
			h := out.GetHistogram()
			requireNoZeroBucketOverlap(t, h)
			require.Equal(t, tt.threshold, h.GetZeroThreshold())
			require.Equal(t, uint64(3), h.GetSampleCount())
			require.Equal(t, uint64(1), h.GetZeroCount())
			require.Equal(t, 0.0, h.GetSampleSum())
			require.Len(t, h.PositiveSpan, 1)
			require.Len(t, h.NegativeSpan, 1)
			require.Equal(t, tt.offset+1, h.PositiveSpan[0].GetOffset())
			require.Equal(t, tt.offset+1, h.NegativeSpan[0].GetOffset())
			require.Equal(t, uint32(1), h.PositiveSpan[0].GetLength())
			require.Equal(t, uint32(1), h.NegativeSpan[0].GetLength())
			require.Equal(t, []int64{1}, h.PositiveDelta)
			require.Equal(t, []int64{1}, h.NegativeDelta)
		})
	}
}

func extractMetricLabels(t *testing.T, m prometheus.Metric) map[string]string {
	t.Helper()
	pbMetric := &io_prometheus_client.Metric{}
	require.NoError(t, m.Write(pbMetric))
	labels := make(map[string]string, len(pbMetric.Label))
	for _, l := range pbMetric.Label {
		labels[l.GetName()] = l.GetValue()
	}
	return labels
}

func TestMetricLabelCollisions(t *testing.T) {
	tests := []struct {
		name            string
		cfg             *Config
		dpAttrs         func(pcommon.Map)
		resourceAttrs   func(pcommon.Map)
		scopeName       string
		scopeVersion    string
		scopeSchemaURL  string
		scopeAttrs      func(pcommon.Map)
		wantLabels      map[string]string
		wantErrContains string
	}{
		{
			name: "colliding datapoint attributes sorted by original key",
			cfg:  &Config{WithoutScopeInfo: true},
			dpAttrs: func(m pcommon.Map) {
				// Insert in reverse lexicographical order to verify sorting by original key.
				m.PutStr("a_b", "v2")
				m.PutStr("a.b", "v1")
				m.PutStr("a-b", "v0")
			},
			wantLabels: map[string]string{
				"a_b": "v0;v1;v2",
			},
		},
		{
			name: "colliding datapoint attributes deduplicate equal values",
			cfg:  &Config{WithoutScopeInfo: true},
			dpAttrs: func(m pcommon.Map) {
				m.PutStr("dup_label", "same")
				m.PutStr("dup.label", "same")
				m.PutStr("tri_c", "b")
				m.PutStr("tri.c", "b")
				m.PutStr("tri-c", "a")
				m.PutStr("tri2_c", "a")
				m.PutStr("tri2.c", "b")
				m.PutStr("tri2-c", "a")
			},
			wantLabels: map[string]string{
				"dup_label": "same",
				"tri_c":     "a;b",
				"tri2_c":    "a;b",
			},
		},
		{
			name: "colliding datapoint values containing semicolons are not falsely deduplicated",
			cfg:  &Config{WithoutScopeInfo: true},
			dpAttrs: func(m pcommon.Map) {
				m.PutStr("semi_k", "y")
				m.PutStr("semi.k", "x;y")
			},
			wantLabels: map[string]string{
				"semi_k": "x;y;y",
			},
		},
		{
			name: "colliding leading-digit and key-prefixed attributes",
			cfg:  &Config{WithoutScopeInfo: true},
			dpAttrs: func(m pcommon.Map) {
				m.PutStr("key.123", "second")
				m.PutStr("123", "first")
			},
			wantLabels: map[string]string{
				"key_123": "first;second",
			},
		},
		{
			name: "colliding non-string datapoint attributes",
			cfg:  &Config{WithoutScopeInfo: true},
			dpAttrs: func(m pcommon.Map) {
				m.PutInt("num.k", 42)
				m.PutStr("num_k", "42")
				m.PutBool("mix_k", true)
				m.PutInt("mix.k", 1)
			},
			wantLabels: map[string]string{
				"num_k": "42",
				"mix_k": "1;true",
			},
		},
		{
			name: "post-normalization reserved scope attributes dropped while keeping empty scope emission",
			cfg:  &Config{},
			scopeAttrs: func(m pcommon.Map) {
				m.PutStr("name", "drop-name")
				m.PutStr("version", "drop-version")
				m.PutStr("schema_url", "drop-schema-underscore")
				m.PutStr("schema.url", "drop-schema-dot")
				m.PutStr("schema-url", "drop-schema-dash")
				m.PutStr("schema/url", "drop-schema-slash")
				m.PutStr("custom", "keep")
			},
			wantLabels: map[string]string{
				"otel_scope_name":       "",
				"otel_scope_version":    "",
				"otel_scope_schema_url": "",
				"otel_scope_custom":     "keep",
			},
		},
		{
			name: "NoTranslation drops exact reserved scope attributes but keeps schema.url",
			cfg:  &Config{TranslationStrategy: noTranslation},
			scopeAttrs: func(m pcommon.Map) {
				m.PutStr("name", "drop-name")
				m.PutStr("version", "drop-version")
				m.PutStr("schema_url", "drop-schema-url")
				m.PutStr("schema.url", "keep-utf8-schema-dot-url")
			},
			scopeName:      "my.scope",
			scopeVersion:   "2.0.0",
			scopeSchemaURL: "https://opentelemetry.io/schemas/1.26.0",
			wantLabels: map[string]string{
				"otel_scope_name":       "my.scope",
				"otel_scope_version":    "2.0.0",
				"otel_scope_schema_url": "https://opentelemetry.io/schemas/1.26.0",
				"otel_scope_schema.url": "keep-utf8-schema-dot-url",
			},
		},
		{
			name: "colliding scope attributes concatenated sorted by original key with equal-value dedup",
			cfg:  &Config{},
			scopeAttrs: func(m pcommon.Map) {
				m.PutStr("a_b", "s2")
				m.PutStr("a.b", "s1")
				m.PutStr("eq_k", "same")
				m.PutStr("eq.k", "same")
				m.PutStr("t_k", "b")
				m.PutStr("t.k", "b")
				m.PutStr("t-k", "a")
			},
			scopeName: "scope",
			wantLabels: map[string]string{
				"otel_scope_name":       "scope",
				"otel_scope_version":    "",
				"otel_scope_schema_url": "",
				"otel_scope_a_b":        "s1;s2",
				"otel_scope_eq_k":       "same",
				"otel_scope_t_k":        "a;b",
			},
		},
		{
			name: "resource job and instance overwrite datapoint job and instance",
			cfg:  &Config{WithoutScopeInfo: true},
			dpAttrs: func(m pcommon.Map) {
				m.PutStr("job", "dp-job")
				m.PutStr("instance", "dp-inst")
				m.PutStr("env", "prod")
			},
			resourceAttrs: func(m pcommon.Map) {
				m.PutStr("service.name", "api")
				m.PutStr("service.namespace", "shop")
				m.PutStr("service.instance.id", "replica-1")
			},
			wantLabels: map[string]string{
				"job":      "shop/api",
				"instance": "replica-1",
				"env":      "prod",
			},
		},
		{
			name: "datapoint job and instance preserved when resource lacks service attributes",
			cfg:  &Config{WithoutScopeInfo: true},
			dpAttrs: func(m pcommon.Map) {
				m.PutStr("job", "dp-job")
				m.PutStr("instance", "dp-inst")
			},
			wantLabels: map[string]string{
				"job":      "dp-job",
				"instance": "dp-inst",
			},
		},
		{
			name: "scope labels overwrite colliding datapoint attributes without cross-source concatenation",
			cfg:  &Config{},
			dpAttrs: func(m pcommon.Map) {
				m.PutStr("otel_scope_name", "dp-name")
				m.PutStr("otel.scope.version", "dp-ver")
				m.PutStr("otel.scope.schema.url", "dp-schema")
				m.PutStr("otel.scope.custom", "dp-custom")
				m.PutStr("otel_scope_a_b", "dp-ab")
				m.PutStr("otel_scope_only_dp", "keep-dp")
			},
			scopeName: "real-scope",
			scopeAttrs: func(m pcommon.Map) {
				m.PutStr("custom", "sc-custom")
				m.PutStr("a_b", "s2")
				m.PutStr("a.b", "s1")
			},
			wantLabels: map[string]string{
				"otel_scope_name":       "real-scope",
				"otel_scope_version":    "",
				"otel_scope_schema_url": "",
				"otel_scope_custom":     "sc-custom",
				"otel_scope_a_b":        "s1;s2",
				"otel_scope_only_dp":    "keep-dp",
			},
		},
		{
			name: "without_scope_info preserves datapoint and const_labels otel_scope labels",
			cfg: &Config{
				WithoutScopeInfo: true,
				ConstLabels: prometheus.Labels{
					"otel_scope_version": "const-ver",
				},
			},
			dpAttrs: func(m pcommon.Map) {
				m.PutStr("otel_scope_name", "dp-name")
			},
			scopeName:    "ignored-scope",
			scopeVersion: "ignored-ver",
			wantLabels: map[string]string{
				"otel_scope_name":    "dp-name",
				"otel_scope_version": "const-ver",
			},
		},
		{
			name: "variable labels take precedence over const_labels",
			cfg: &Config{
				ConstLabels: prometheus.Labels{
					"env":               "const-env",
					"foo_bar":           "const-fb",
					"job":               "const-job",
					"instance":          "const-inst",
					"otel_scope_name":   "const-scope",
					"otel_scope_custom": "const-sc",
					"kept_const":        "kept",
				},
			},
			dpAttrs: func(m pcommon.Map) {
				m.PutStr("env", "dp-env")
				m.PutStr("foo.bar", "dp-fb")
			},
			resourceAttrs: func(m pcommon.Map) {
				m.PutStr("service.name", "svc")
				m.PutStr("service.instance.id", "inst-1")
			},
			scopeName: "sc-name",
			scopeAttrs: func(m pcommon.Map) {
				m.PutStr("custom", "sc-val")
			},
			wantLabels: map[string]string{
				"env":                   "dp-env",
				"foo_bar":               "dp-fb",
				"job":                   "svc",
				"instance":              "inst-1",
				"otel_scope_name":       "sc-name",
				"otel_scope_version":    "",
				"otel_scope_schema_url": "",
				"otel_scope_custom":     "sc-val",
				"kept_const":            "kept",
			},
		},
		{
			name: "colliding datapoint attributes preserve non-colliding const_labels",
			cfg: &Config{
				WithoutScopeInfo: true,
				ConstLabels: prometheus.Labels{
					"region": "us-east1",
				},
			},
			dpAttrs: func(m pcommon.Map) {
				m.PutStr("a_b", "v2")
				m.PutStr("a.b", "v1")
			},
			wantLabels: map[string]string{
				"a_b":    "v1;v2",
				"region": "us-east1",
			},
		},
		{
			name: "all const_labels overridden by variable labels",
			cfg: &Config{
				WithoutScopeInfo: true,
				ConstLabels: prometheus.Labels{
					"env": "const-env",
				},
			},
			dpAttrs: func(m pcommon.Map) {
				m.PutStr("env", "dp-env")
			},
			wantLabels: map[string]string{
				"env": "dp-env",
			},
		},
		{
			name: "invalid datapoint attribute key still returns error",
			cfg:  &Config{WithoutScopeInfo: true},
			dpAttrs: func(m pcommon.Map) {
				m.PutStr("---", "bad")
			},
			wantErrContains: "resulted in invalid name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := pmetric.NewMetric()
			metric.SetName("test_metric")
			dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
			dp.SetDoubleValue(1)
			if tt.dpAttrs != nil {
				tt.dpAttrs(dp.Attributes())
			}

			rAttrs := pcommon.NewMap()
			if tt.resourceAttrs != nil {
				tt.resourceAttrs(rAttrs)
			}

			sAttrs := pcommon.NewMap()
			if tt.scopeAttrs != nil {
				tt.scopeAttrs(sAttrs)
			}

			c := newCollector(tt.cfg, zap.NewNop())
			m, err := c.convertMetric(metric, rAttrs, tt.scopeName, tt.scopeVersion, tt.scopeSchemaURL, sAttrs)
			if tt.wantErrContains != "" {
				require.ErrorContains(t, err, tt.wantErrContains)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantLabels, extractMetricLabels(t, m))
		})
	}
}

func TestConstLabelsNotMutatedOnCollision(t *testing.T) {
	c := newCollector(&Config{
		WithoutScopeInfo: true,
		ConstLabels: prometheus.Labels{
			"env":    "const-env",
			"region": "us-west",
		},
	}, zap.NewNop())

	m1 := pmetric.NewMetric()
	m1.SetName("metric_colliding")
	dp1 := m1.SetEmptyGauge().DataPoints().AppendEmpty()
	dp1.SetDoubleValue(1)
	dp1.Attributes().PutStr("env", "dp-env")

	prom1, err := c.convertMetric(m1, pcommon.NewMap(), "", "", "", pcommon.NewMap())
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"env":    "dp-env",
		"region": "us-west",
	}, extractMetricLabels(t, prom1))

	m2 := pmetric.NewMetric()
	m2.SetName("metric_non_colliding")
	dp2 := m2.SetEmptyGauge().DataPoints().AppendEmpty()
	dp2.SetDoubleValue(2)

	prom2, err := c.convertMetric(m2, pcommon.NewMap(), "", "", "", pcommon.NewMap())
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"env":    "const-env",
		"region": "us-west",
	}, extractMetricLabels(t, prom2))
}

func TestCollectAllMetricTypesWithLabelCollisions(t *testing.T) {
	populateAttrs := func(m pcommon.Map) {
		m.PutStr("a_b", "v2")
		m.PutStr("a.b", "v1")
		m.PutStr("job", "dp-job")
		m.PutStr("env", "dp-env")
		m.PutStr("otel_scope_name", "dp-scope")
	}

	rm := pmetric.NewResourceMetrics()
	rm.Resource().Attributes().PutStr("service.name", "svc")
	rm.Resource().Attributes().PutStr("service.instance.id", "inst-1")

	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("test-scope")
	sm.Scope().SetVersion("1.0.0")
	sm.SetSchemaUrl("https://opentelemetry.io/schemas/1.26.0")
	sm.Scope().Attributes().PutStr("schema.url", "drop-me")
	sm.Scope().Attributes().PutStr("x_y", "s2")
	sm.Scope().Attributes().PutStr("x.y", "s1")

	ts := pcommon.NewTimestampFromTime(time.Now())

	g := sm.Metrics().AppendEmpty()
	g.SetName("test_gauge")
	gdp := g.SetEmptyGauge().DataPoints().AppendEmpty()
	gdp.SetDoubleValue(1)
	gdp.SetTimestamp(ts)
	populateAttrs(gdp.Attributes())

	s := sm.Metrics().AppendEmpty()
	s.SetName("test_counter")
	sum := s.SetEmptySum()
	sum.SetIsMonotonic(true)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	sdp := sum.DataPoints().AppendEmpty()
	sdp.SetDoubleValue(2)
	sdp.SetTimestamp(ts)
	populateAttrs(sdp.Attributes())

	h := sm.Metrics().AppendEmpty()
	h.SetName("test_hist")
	hist := h.SetEmptyHistogram()
	hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	hdp := hist.DataPoints().AppendEmpty()
	hdp.SetCount(1)
	hdp.SetSum(1)
	hdp.ExplicitBounds().FromRaw([]float64{1})
	hdp.BucketCounts().FromRaw([]uint64{1, 0})
	hdp.SetTimestamp(ts)
	populateAttrs(hdp.Attributes())

	eh := sm.Metrics().AppendEmpty()
	eh.SetName("test_exp_hist")
	expHist := eh.SetEmptyExponentialHistogram()
	expHist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	ehdp := expHist.DataPoints().AppendEmpty()
	ehdp.SetCount(1)
	ehdp.SetSum(1)
	ehdp.SetScale(0)
	ehdp.SetZeroCount(1)
	ehdp.SetTimestamp(ts)
	populateAttrs(ehdp.Attributes())

	sumry := sm.Metrics().AppendEmpty()
	sumry.SetName("test_summary")
	sumdp := sumry.SetEmptySummary().DataPoints().AppendEmpty()
	sumdp.SetCount(1)
	sumdp.SetSum(1)
	sumdp.SetTimestamp(ts)
	populateAttrs(sumdp.Attributes())

	loggerCore := &errorCheckCore{}
	c := newCollector(&Config{
		MetricExpiration: time.Minute,
		ConstLabels: prometheus.Labels{
			"env":   "const-env",
			"extra": "const-extra",
		},
	}, zap.New(loggerCore))
	c.accumulator.Accumulate(rm)

	registry := prometheus.NewRegistry()
	registry.MustRegister(c)
	families, err := registry.Gather()
	require.NoError(t, err)
	require.Empty(t, loggerCore.errorMessages)

	wantLabels := map[string]string{
		"a_b":                   "v1;v2",
		"env":                   "dp-env",
		"extra":                 "const-extra",
		"job":                   "svc",
		"instance":              "inst-1",
		"otel_scope_name":       "test-scope",
		"otel_scope_version":    "1.0.0",
		"otel_scope_schema_url": "https://opentelemetry.io/schemas/1.26.0",
		"otel_scope_x_y":        "s1;s2",
	}

	gotFamilies := make(map[string]map[string]string, len(families))
	for _, mf := range families {
		require.Len(t, mf.GetMetric(), 1)
		lbls := make(map[string]string, len(mf.GetMetric()[0].GetLabel()))
		for _, l := range mf.GetMetric()[0].GetLabel() {
			lbls[l.GetName()] = l.GetValue()
		}
		gotFamilies[mf.GetName()] = lbls
	}

	for _, name := range []string{"test_gauge", "test_counter_total", "test_hist", "test_exp_hist", "test_summary"} {
		require.Equal(t, wantLabels, gotFamilies[name], "metric %s labels mismatch", name)
	}
	require.Equal(t, map[string]string{"job": "svc", "instance": "inst-1"}, gotFamilies["target_info"])
}

func TestTargetInfoLabelCollisions(t *testing.T) {
	t.Run("sorted concatenation, equal-value dedup, and job/instance overwrite", func(t *testing.T) {
		attrs := pcommon.NewMap()
		attrs.PutStr("service.name", "api")
		attrs.PutStr("service.namespace", "shop")
		attrs.PutStr("service.instance.id", "replica-1")
		attrs.PutStr("job", "attr-job")
		attrs.PutStr("instance", "attr-inst")
		// Insert in reverse lexicographical order to verify sorting by original key.
		attrs.PutStr("k8s_pod_name", "pod-b")
		attrs.PutStr("k8s.pod.name", "pod-a")
		attrs.PutStr("env_name", "prod")
		attrs.PutStr("env.name", "prod")
		attrs.PutStr("tier_id", "b")
		attrs.PutStr("tier.id", "b")
		attrs.PutStr("tier-id", "a")

		original := attrs.AsRaw()
		c := newCollector(&Config{}, zap.NewNop())
		metrics, err := c.createTargetInfoMetrics([]pcommon.Map{attrs})
		require.NoError(t, err)
		require.Len(t, metrics, 1)

		require.Equal(t, map[string]string{
			"job":          "shop/api",
			"instance":     "replica-1",
			"k8s_pod_name": "pod-a;pod-b",
			"env_name":     "prod",
			"tier_id":      "a;b",
		}, extractMetricLabels(t, metrics[0]))
		require.Equal(t, original, attrs.AsRaw())
	})

	t.Run("KeepIdentifyingResourceAttributes gate merges colliding service_name sorted", func(t *testing.T) {
		defer testutil.SetFeatureGateForTest(t, prometheustranslator.KeepIdentifyingResourceAttributesGate, true)()

		attrs := pcommon.NewMap()
		attrs.PutStr("service_name", "other")
		attrs.PutStr("service.name", "api")
		attrs.PutStr("service.instance.id", "replica-1")

		original := attrs.AsRaw()
		c := newCollector(&Config{}, zap.NewNop())
		metrics, err := c.createTargetInfoMetrics([]pcommon.Map{attrs})
		require.NoError(t, err)
		require.Len(t, metrics, 1)

		require.Equal(t, map[string]string{
			"job":                 "api",
			"instance":            "replica-1",
			"service_name":        "api;other",
			"service_instance_id": "replica-1",
		}, extractMetricLabels(t, metrics[0]))
		require.Equal(t, original, attrs.AsRaw())
	})

	t.Run("invalid resource attribute key returns error", func(t *testing.T) {
		attrs := pcommon.NewMap()
		attrs.PutStr("service.name", "api")
		attrs.PutStr("service.instance.id", "replica-1")
		attrs.PutStr("---", "bad")

		c := newCollector(&Config{}, zap.NewNop())
		_, err := c.createTargetInfoMetrics([]pcommon.Map{attrs})
		require.ErrorContains(t, err, "resulted in invalid name")
	})
}
