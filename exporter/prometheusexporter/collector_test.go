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

package prometheusexporter

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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

func TestConvertInvalidMetric(t *testing.T) {
	for _, mType := range []pmetric.MetricDataType{
		pmetric.MetricDataTypeHistogram,
		pmetric.MetricDataTypeSum,
		pmetric.MetricDataTypeGauge,
	} {
		metric := pmetric.NewMetric()
		switch mType {
		case pmetric.MetricDataTypeGauge:
			metric.SetEmptyGauge().DataPoints().AppendEmpty()
		case pmetric.MetricDataTypeSum:
			metric.SetEmptySum().DataPoints().AppendEmpty()
		case pmetric.MetricDataTypeHistogram:
			metric.SetEmptyHistogram().DataPoints().AppendEmpty()
		}
		c := collector{}

		_, err := c.convertMetric(metric, pcommon.NewMap())
		require.Error(t, err)
	}
}

func TestConvertDoubleHistogramExemplar(t *testing.T) {
	// initialize empty histogram
	metric := pmetric.NewMetric()
	metric.SetName("test_metric")
	metric.SetDescription("this is test metric")
	metric.SetUnit("T")

	// initialize empty datapoint
	hd := metric.SetEmptyHistogram().DataPoints().AppendEmpty()

	hd.ExplicitBounds().FromRaw([]float64{5, 25, 90})
	hd.BucketCounts().FromRaw([]uint64{2, 35, 70})

	exemplarTs, _ := time.Parse("unix", "Mon Jan _2 15:04:05 MST 2006")
	exemplars := []prometheus.Exemplar{
		{
			Timestamp: exemplarTs,
			Value:     3,
			Labels:    prometheus.Labels{"test_label_0": "label_value_0"},
		},
		{
			Timestamp: exemplarTs,
			Value:     50,
			Labels:    prometheus.Labels{"test_label_1": "label_value_1"},
		},
		{
			Timestamp: exemplarTs,
			Value:     78,
			Labels:    prometheus.Labels{"test_label_2": "label_value_2"},
		},
		{
			Timestamp: exemplarTs,
			Value:     100,
			Labels:    prometheus.Labels{"test_label_3": "label_value_3"},
		},
	}

	// add each exemplar value to the metric
	for _, e := range exemplars {
		pde := hd.Exemplars().AppendEmpty()
		pde.SetDoubleVal(e.Value)
		for k, v := range e.Labels {
			pde.FilteredAttributes().PutString(k, v)
		}
		pde.SetTimestamp(pcommon.NewTimestampFromTime(e.Timestamp))
	}

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

	require.Equal(t, 4, len(buckets))

	require.Equal(t, 3.0, buckets[0].GetExemplar().GetValue())
	require.Equal(t, int32(128654848), buckets[0].GetExemplar().GetTimestamp().GetNanos())
	require.Equal(t, 1, len(buckets[0].GetExemplar().GetLabel()))
	require.Equal(t, "test_label_0", buckets[0].GetExemplar().GetLabel()[0].GetName())
	require.Equal(t, "label_value_0", buckets[0].GetExemplar().GetLabel()[0].GetValue())

	require.Equal(t, 0.0, buckets[1].GetExemplar().GetValue())
	require.Equal(t, int32(0), buckets[1].GetExemplar().GetTimestamp().GetNanos())
	require.Equal(t, 0, len(buckets[1].GetExemplar().GetLabel()))

	require.Equal(t, 78.0, buckets[2].GetExemplar().GetValue())
	require.Equal(t, int32(128654848), buckets[2].GetExemplar().GetTimestamp().GetNanos())
	require.Equal(t, 1, len(buckets[2].GetExemplar().GetLabel()))
	require.Equal(t, "test_label_2", buckets[2].GetExemplar().GetLabel()[0].GetName())
	require.Equal(t, "label_value_2", buckets[2].GetExemplar().GetLabel()[0].GetValue())

	require.Equal(t, 100.0, buckets[3].GetExemplar().GetValue())
	require.Equal(t, int32(128654848), buckets[3].GetExemplar().GetTimestamp().GetNanos())
	require.Equal(t, 1, len(buckets[3].GetExemplar().GetLabel()))
	require.Equal(t, "test_label_3", buckets[3].GetExemplar().GetLabel()[0].GetName())
	require.Equal(t, "label_value_3", buckets[3].GetExemplar().GetLabel()[0].GetValue())
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
	dp.SetIntVal(42)
	dp.Attributes().PutString("label.1", "1")
	dp.Attributes().PutString("label/2", "2")
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
		require.Contains(t, m.Desc().String(), "variableLabels: [label_1 label_2]")

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
		metric     func(time.Time) pmetric.Metric
		metricType prometheus.ValueType
		value      float64
	}{
		{
			name:       "IntGauge",
			metricType: prometheus.GaugeValue,
			value:      42.0,
			metric: func(ts time.Time) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_metric")
				metric.SetDescription("test description")
				dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
				dp.SetIntVal(42)
				dp.Attributes().PutString("label_1", "1")
				dp.Attributes().PutString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))

				return
			},
		},
		{
			name:       "Gauge",
			metricType: prometheus.GaugeValue,
			value:      42.42,
			metric: func(ts time.Time) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_metric")
				metric.SetDescription("test description")
				dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
				dp.SetDoubleVal(42.42)
				dp.Attributes().PutString("label_1", "1")
				dp.Attributes().PutString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))

				return
			},
		},
		{
			name:       "IntSum",
			metricType: prometheus.GaugeValue,
			value:      42.0,
			metric: func(ts time.Time) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_metric")
				metric.SetEmptySum().SetIsMonotonic(false)
				metric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				metric.SetDescription("test description")
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetIntVal(42)
				dp.Attributes().PutString("label_1", "1")
				dp.Attributes().PutString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))

				return
			},
		},
		{
			name:       "Sum",
			metricType: prometheus.GaugeValue,
			value:      42.42,
			metric: func(ts time.Time) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_metric")
				metric.SetEmptySum().SetIsMonotonic(false)
				metric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				metric.SetDescription("test description")
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleVal(42.42)
				dp.Attributes().PutString("label_1", "1")
				dp.Attributes().PutString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))

				return
			},
		},
		{
			name:       "MonotonicIntSum",
			metricType: prometheus.CounterValue,
			value:      42.0,
			metric: func(ts time.Time) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_metric")
				metric.SetEmptySum().SetIsMonotonic(true)
				metric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				metric.SetDescription("test description")
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetIntVal(42)
				dp.Attributes().PutString("label_1", "1")
				dp.Attributes().PutString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))

				return
			},
		},
		{
			name:       "MonotonicSum",
			metricType: prometheus.CounterValue,
			value:      42.42,
			metric: func(ts time.Time) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_metric")
				metric.SetEmptySum().SetIsMonotonic(true)
				metric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				metric.SetDescription("test description")
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleVal(42.42)
				dp.Attributes().PutString("label_1", "1")
				dp.Attributes().PutString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))

				return
			},
		},
	}

	for _, tt := range tests {
		for _, sendTimestamp := range []bool{true, false} {
			name := tt.name
			if sendTimestamp {
				name += "/WithTimestamp"
			}

			rAttrs := pcommon.NewMap()
			rAttrs.PutString(conventions.AttributeServiceInstanceID, "localhost:9090")
			rAttrs.PutString(conventions.AttributeServiceName, "testapp")
			rAttrs.PutString(conventions.AttributeServiceNamespace, "prod")

			t.Run(name, func(t *testing.T) {
				ts := time.Now()
				metric := tt.metric(ts)
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
					require.Contains(t, m.Desc().String(), "variableLabels: [label_1 label_2 job instance]")

					pbMetric := io_prometheus_client.Metric{}
					require.NoError(t, m.Write(&pbMetric))

					labelsKeys := map[string]string{"label_1": "1", "label_2": "2", "job": "prod/testapp", "instance": "localhost:9090"}
					for _, l := range pbMetric.Label {
						require.Equal(t, labelsKeys[*l.Name], *l.Value)
					}

					if sendTimestamp {
						require.Equal(t, ts.UnixNano()/1e6, *(pbMetric.TimestampMs))
					} else {
						require.Nil(t, pbMetric.TimestampMs)
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
		metric func(time.Time) pmetric.Metric

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
			metric: func(ts time.Time) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_metric")
				metric.SetEmptyHistogram().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				metric.SetDescription("test description")
				dp := metric.Histogram().DataPoints().AppendEmpty()
				dp.BucketCounts().FromRaw([]uint64{5, 2})
				dp.SetCount(7)
				dp.ExplicitBounds().FromRaw([]float64{3.5, 10.0})
				dp.SetSum(42.42)
				dp.Attributes().PutString("label_1", "1")
				dp.Attributes().PutString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				return
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
				metric := tt.metric(ts)
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
					require.Contains(t, m.Desc().String(), "variableLabels: [label_1 label_2]")

					pbMetric := io_prometheus_client.Metric{}
					require.NoError(t, m.Write(&pbMetric))

					labelsKeys := map[string]string{"label_1": "1", "label_2": "2"}
					for _, l := range pbMetric.Label {
						require.Equal(t, labelsKeys[*l.Name], *l.Value)
					}

					if sendTimestamp {
						require.Equal(t, ts.UnixNano()/1e6, *(pbMetric.TimestampMs))
					} else {
						require.Nil(t, pbMetric.TimestampMs)
					}

					require.Nil(t, pbMetric.Gauge)
					require.Nil(t, pbMetric.Counter)

					h := *pbMetric.Histogram
					require.Equal(t, tt.histogramCount, h.GetSampleCount())
					require.Equal(t, tt.histogramSum, h.GetSampleSum())
					require.Equal(t, len(tt.histogramPoints), len(h.Bucket))

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
	fillQuantileValue := func(pN, value float64, dest pmetric.ValueAtQuantile) {
		dest.SetQuantile(pN)
		dest.SetValue(value)
	}
	tests := []struct {
		name          string
		metric        func(time.Time) pmetric.Metric
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
			metric: func(ts time.Time) (metric pmetric.Metric) {
				metric = pmetric.NewMetric()
				metric.SetName("test_metric")
				metric.SetDescription("test description")
				sp := metric.SetEmptySummary().DataPoints().AppendEmpty()
				sp.SetCount(10)
				sp.SetSum(0.012)
				sp.SetCount(10)
				sp.Attributes().PutString("label_1", "1")
				sp.Attributes().PutString("label_2", "2")
				sp.SetTimestamp(pcommon.NewTimestampFromTime(ts))

				fillQuantileValue(0.50, 190, sp.QuantileValues().AppendEmpty())
				fillQuantileValue(0.99, 817, sp.QuantileValues().AppendEmpty())

				return
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
				metric := tt.metric(ts)
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
					require.Contains(t, m.Desc().String(), "variableLabels: [label_1 label_2]")

					pbMetric := io_prometheus_client.Metric{}
					require.NoError(t, m.Write(&pbMetric))

					labelsKeys := map[string]string{"label_1": "1", "label_2": "2"}
					for _, l := range pbMetric.Label {
						require.Equal(t, labelsKeys[*l.Name], *l.Value)
					}

					if sendTimestamp {
						require.Equal(t, ts.UnixNano()/1e6, *(pbMetric.TimestampMs))
					} else {
						require.Nil(t, pbMetric.TimestampMs)
					}

					require.Nil(t, pbMetric.Gauge)
					require.Nil(t, pbMetric.Counter)
					require.Nil(t, pbMetric.Histogram)

					s := *pbMetric.Summary
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
