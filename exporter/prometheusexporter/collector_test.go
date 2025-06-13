// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.25.0"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	prometheustranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

type mockAccumulator struct {
	metrics            []pmetric.Metric
	resourceAttributes pcommon.Map // Same attributes for all metrics.
}

func (a *mockAccumulator) Accumulate(pmetric.ResourceMetrics) (n int) {
	return 0
}

func (a *mockAccumulator) Collect() ([]pmetric.Metric, []pcommon.Map) {
	rAttrs := make([]pcommon.Map, len(a.metrics))
	for i := range rAttrs {
		rAttrs[i] = a.resourceAttributes
	}

	return a.metrics, rAttrs
}

func TestConvertInvalidDataType(t *testing.T) {
	metric := pmetric.NewMetric()
	c := collector{
		accumulator: &mockAccumulator{
			[]pmetric.Metric{metric},
			pcommon.NewMap(),
		},
		logger: zap.NewNop(),
	}

	_, err := c.convertMetric(metric, pcommon.NewMap())
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
						Name: proto.String("testgauge"),
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
						Name: proto.String("testgauge"),
						Type: io_prometheus_client.MetricType_GAUGE.Enum(),
						Help: proto.String("test help value"),
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
			c := collector{
				logger: zap.NewNop(),
			}
			for k, v := range tt.mapVals {
				c.metricFamilies.Store(k, v)
			}

			_, err := c.convertMetric(metric, pcommon.NewMap())
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

	c := collector{
		accumulator: &mockAccumulator{
			metrics:            []pmetric.Metric{metric},
			resourceAttributes: pMap,
		},
		logger: zap.NewNop(),
	}

	pbMetric, _ := c.convertDoubleHistogram(metric, pMap)
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

	c := collector{
		accumulator: &mockAccumulator{
			metrics:            []pmetric.Metric{metric},
			resourceAttributes: pMap,
		},
		logger: zap.NewNop(),
	}

	promMetric, _ := c.convertSum(metric, pMap)
	outMetric := io_prometheus_client.Metric{}
	err := promMetric.Write(&outMetric)
	if err != nil {
		t.Error("Unable to write metric to prometheus output form:", err)
	}

	promCounter := outMetric.GetCounter()
	require.Equal(t, 1.0, promCounter.GetValue())
	exemplarsEqual(t, exemplar, promCounter.GetExemplar())
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
	c := collector{
		namespace: "test_space",
		accumulator: &mockAccumulator{
			[]pmetric.Metric{metric},
			pcommon.NewMap(),
		},
		sendTimestamps: false,
		logger:         zap.New(&loggerCore),
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

				return
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

				return
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

				return
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

				return
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

				return
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

				return
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

				return
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
			rAttrs.PutStr(conventions.AttributeServiceInstanceID, "localhost:9090")
			rAttrs.PutStr(conventions.AttributeServiceName, "testapp")
			rAttrs.PutStr(conventions.AttributeServiceNamespace, "prod")

			t.Run(name, func(t *testing.T) {
				ts := time.Now()
				metric := tt.metric(ts, sendTimestamp)
				c := collector{
					namespace: "test_space",
					accumulator: &mockAccumulator{
						[]pmetric.Metric{metric},
						rAttrs,
					},
					sendTimestamps: sendTimestamp,
					logger:         zap.NewNop(),
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

					require.Contains(t, m.Desc().String(), "fqName: \"test_space_test_metric\"")
					require.Contains(t, m.Desc().String(), `variableLabels: {label_1,label_2,job,instance}`)

					pbMetric := io_prometheus_client.Metric{}
					require.NoError(t, m.Write(&pbMetric))

					labelsKeys := map[string]string{"label_1": "1", "label_2": "2", "job": "prod/testapp", "instance": "localhost:9090"}
					for _, l := range pbMetric.Label {
						require.Equal(t, labelsKeys[*l.Name], *l.Value)
					}

					if sendTimestamp {
						require.Equal(t, ts.UnixNano()/1e6, *(pbMetric.TimestampMs))
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
				return
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
				c := collector{
					accumulator: &mockAccumulator{
						[]pmetric.Metric{metric},
						pcommon.NewMap(),
					},
					sendTimestamps: sendTimestamp,
					logger:         zap.NewNop(),
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
						require.Equal(t, ts.UnixNano()/1e6, *(pbMetric.TimestampMs))
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

				return
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
				c := collector{
					accumulator: &mockAccumulator{
						[]pmetric.Metric{metric},
						pcommon.NewMap(),
					},
					sendTimestamps: sendTimestamp,
					logger:         zap.NewNop(),
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
						require.Equal(t, ts.UnixNano()/1e6, *(pbMetric.TimestampMs))
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
