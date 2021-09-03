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

package internal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"go.opentelemetry.io/collector/model/pdata"
)

var (
	t1Ms = pdata.Timestamp(time.Unix(0, 1000000).UnixNano())
	t2Ms = pdata.Timestamp(time.Unix(0, 2000000).UnixNano())
	t3Ms = pdata.Timestamp(time.Unix(0, 3000000).UnixNano())
	t4Ms = pdata.Timestamp(time.Unix(0, 5000000).UnixNano())
	t5Ms = pdata.Timestamp(time.Unix(0, 5000000).UnixNano())

	bounds0  = []float64{1, 2, 4}
	percent0 = []float64{10, 50, 90}

	gd1      = "gaugedist1"
	c1       = "cumulative1"
	cd1      = "cumulativedist1"
	s1       = "summary1"
	k1       = []string{"k1"}
	k1k2     = []string{"k1", "k2"}
	k1k2k3   = []string{"k1", "k2", "k3"}
	v1v2     = []string{"v1", "v2"}
	v10v20   = []string{"v10", "v20"}
	v100v200 = []string{"v100", "v200"}
)

func Test_gauge(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			"Gauge: round 1 - gauge not adjusted",
			func() *pdata.MetricSlice {
				mL := pdata.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pdata.MetricDataTypeGauge)
				m0.SetName("gauge1")
				g0 := m0.Gauge()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(t1Ms)
				pt0.Attributes().InsertString("v1", "v2")
				pt0.SetTimestamp(t1Ms)
				pt0.SetDoubleVal(44)
				return &mL
			}(),
			func() *pdata.MetricSlice {
				mL := pdata.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pdata.MetricDataTypeGauge)
				m0.SetName("gauge1")
				g0 := m0.Gauge()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(t1Ms)
				pt0.Attributes().InsertString("v1", "v2")
				pt0.SetTimestamp(t1Ms)
				pt0.SetDoubleVal(44)
				return &mL
			}(),
			0,
		},
		{
			"Gauge: round 2 - gauge not adjusted",
			func() *pdata.MetricSlice {
				mL := pdata.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pdata.MetricDataTypeGauge)
				m0.SetName("gauge1")
				g0 := m0.Gauge()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(t2Ms)
				pt0.Attributes().InsertString("k1", "v1")
				pt0.Attributes().InsertString("k2", "v2")
				pt0.SetTimestamp(t2Ms)
				pt0.SetDoubleVal(66)

				return &mL
			}(),
			func() *pdata.MetricSlice {
				mL := pdata.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pdata.MetricDataTypeGauge)
				m0.SetName("gauge1")
				g0 := m0.Gauge()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(t2Ms)
				pt0.Attributes().InsertString("k1", "v1")
				pt0.Attributes().InsertString("k2", "v2")
				pt0.SetTimestamp(t2Ms)
				pt0.SetDoubleVal(66)
				return &mL
			}(),
			0,
		},
		{
			"Gauge: round 3 - value less than previous value - gauge is not adjusted",
			func() *pdata.MetricSlice {
				mL := pdata.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pdata.MetricDataTypeGauge)
				m0.SetName("gauge1")
				g0 := m0.Gauge()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(t3Ms)
				pt0.Attributes().InsertString("k1", "v1")
				pt0.Attributes().InsertString("k2", "v2")
				pt0.SetTimestamp(t3Ms)
				pt0.SetDoubleVal(55)

				return &mL
			}(),
			func() *pdata.MetricSlice {
				mL := pdata.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pdata.MetricDataTypeGauge)
				m0.SetName("gauge1")
				g0 := m0.Gauge()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(t3Ms)
				pt0.Attributes().InsertString("k1", "v1")
				pt0.Attributes().InsertString("k2", "v2")
				pt0.SetTimestamp(t3Ms)
				pt0.SetDoubleVal(55)

				return &mL
			}(),
			0,
		},
	}
	runScript(t, NewJobsMapPdata(time.Minute).get("job", "0"), script)
}

func Test_cumulative(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			"Cumulative: round 1 - initial instance, start time is established",
			func() *pdata.MetricSlice {
				mL := pdata.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pdata.MetricDataTypeSum)
				m0.SetName("cumulative1")
				g0 := m0.Sum()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(t1Ms)
				pt0.Attributes().InsertString("k1", "v1")
				pt0.Attributes().InsertString("k2", "v2")
				pt0.SetTimestamp(t1Ms)
				pt0.SetDoubleVal(44)

				return &mL
			}(),
			func() *pdata.MetricSlice {
				mL := pdata.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pdata.MetricDataTypeSum)
				m0.SetName("cumulative1")
				g0 := m0.Sum()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(t1Ms)
				pt0.Attributes().InsertString("k1", "v1")
				pt0.Attributes().InsertString("k2", "v2")
				pt0.SetTimestamp(t1Ms)
				pt0.SetDoubleVal(44)

				return &mL
			}(),
			1,
		},
		{
			"Cumulative: round 2 - instance adjusted based on round 1",
			func() *pdata.MetricSlice {
				mL := pdata.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pdata.MetricDataTypeSum)
				m0.SetName("cumulative1")
				g0 := m0.Sum()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(t2Ms)
				pt0.Attributes().InsertString("k1", "v1")
				pt0.Attributes().InsertString("k2", "v2")
				pt0.SetTimestamp(t2Ms)
				pt0.SetDoubleVal(66)

				return &mL
			}(),
			func() *pdata.MetricSlice {
				mL := pdata.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pdata.MetricDataTypeSum)
				m0.SetName("cumulative1")
				g0 := m0.Sum()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(t1Ms)
				pt0.Attributes().InsertString("k1", "v1")
				pt0.Attributes().InsertString("k2", "v2")
				pt0.SetTimestamp(t2Ms)
				pt0.SetDoubleVal(66)

				return &mL
			}(),
			0,
		},
		{
			"Cumulative: round 3 - instance reset (value less than previous value), start time is reset",
			func() *pdata.MetricSlice {
				mL := pdata.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pdata.MetricDataTypeSum)
				m0.SetName("cumulative1")
				g0 := m0.Sum()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(t3Ms)
				pt0.Attributes().InsertString("k1", "v1")
				pt0.Attributes().InsertString("k2", "v2")
				pt0.SetTimestamp(t3Ms)
				pt0.SetDoubleVal(55)

				return &mL
			}(),
			func() *pdata.MetricSlice {
				mL := pdata.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pdata.MetricDataTypeSum)
				m0.SetName("cumulative1")
				g0 := m0.Sum()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(t3Ms)
				pt0.Attributes().InsertString("k1", "v1")
				pt0.Attributes().InsertString("k2", "v2")
				pt0.SetTimestamp(t3Ms)
				pt0.SetDoubleVal(55)

				return &mL
			}(),
			1,
		},
		{
			"Cumulative: round 4 - instance adjusted based on round 3",
			func() *pdata.MetricSlice {
				mL := pdata.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pdata.MetricDataTypeSum)
				m0.SetName("cumulative1")
				g0 := m0.Sum()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(t4Ms)
				pt0.Attributes().InsertString("k1", "v1")
				pt0.Attributes().InsertString("k2", "v2")
				pt0.SetTimestamp(t4Ms)
				pt0.SetDoubleVal(72)

				return &mL
			}(),
			func() *pdata.MetricSlice {
				mL := pdata.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pdata.MetricDataTypeSum)
				m0.SetName("cumulative1")
				g0 := m0.Sum()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(t3Ms)
				pt0.Attributes().InsertString("k1", "v1")
				pt0.Attributes().InsertString("k2", "v2")
				pt0.SetTimestamp(t4Ms)
				pt0.SetDoubleVal(72)

				return &mL
			}(),
			0,
		},
	}
	runScript(t, NewJobsMapPdata(time.Minute).get("job", "0"), script)
}

func populateHistogram(hdp *pdata.HistogramDataPoint, timestamp pdata.Timestamp, bounds []float64, counts []uint64) {
	count := uint64(0)
	sum := float64(0)
	for i, counti := range counts {
		if i > 0 {
			sum += float64(counti) * bounds[i-1]
		}
		count += counti
	}
	hdp.SetBucketCounts(counts)
	hdp.SetSum(sum)
	hdp.SetCount(count)
	hdp.SetTimestamp(timestamp)
	hdp.SetExplicitBounds(bounds)
}

func Test_gaugeDistribution(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			"GaugeDist: round 1 - gauge distribution not adjusted",
			metricSlice(gaugeDistMetric(gd1, k1v1k2v2, t1Ms, distPoint(t1Ms, bounds0, []uint64{4, 2, 3, 7}))),
			metricSlice(gaugeDistMetric(gd1, k1v1k2v2, t1Ms, distPoint(t1Ms, bounds0, []uint64{4, 2, 3, 7}))),
			0,
		},
		{
			"GaugeDist: round 2 - gauge distribution not adjusted",
			metricSlice(gaugeDistMetric(gd1, k1v1k2v2, t2Ms, distPoint(t2Ms, bounds0, []uint64{6, 5, 8, 11}))),
			metricSlice(gaugeDistMetric(gd1, k1v1k2v2, t2Ms, distPoint(t2Ms, bounds0, []uint64{6, 5, 8, 11}))),
			0,
		},
		{
			"GaugeDist: round 3 - count/sum less than previous - gauge distribution not adjusted",
			metricSlice(gaugeDistMetric(gd1, k1v1k2v2, t3Ms, distPoint(t3Ms, bounds0, []uint64{2, 0, 1, 5}))),
			metricSlice(gaugeDistMetric(gd1, k1v1k2v2, t3Ms, distPoint(t3Ms, bounds0, []uint64{2, 0, 1, 5}))),
			0,
		},
	}
	runScript(t, NewJobsMapPdata(time.Minute).get("job", "0"), script)
}

func populateSummary(sdp *pdata.SummaryDataPoint, timestamp pdata.Timestamp, count uint64, sum float64, quantilePercents, quantileValues []float64) {
	quantiles := sdp.QuantileValues()
	for i := range quantilePercents {
		qv := quantiles.AppendEmpty()
		qv.SetQuantile(quantilePercents[i])
		qv.SetValue(quantileValues[i])
	}
	sdp.SetCount(count)
	sdp.SetTimestamp(timestamp)
	sdp.SetSum(sum)
}

func Test_summary_no_count(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			"Summary No Count: round 1 - initial instance, start time is established",
			func() *pdata.MetricSlice {
				mL := pdata.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pdata.MetricDataTypeSummary)
				m0.SetName("summary1")
				s0 := m0.Summary()
				pt0 := s0.DataPoints().AppendEmpty()
				pt0.Attributes().InsertString("v1", "v2")
				populateSummary(&pt0, t1Ms, 10, 40, percent0, []float64{1, 5, 8})
				return &mL
			}(),
			func() *pdata.MetricSlice {
				mL := pdata.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pdata.MetricDataTypeSummary)
				m0.SetName("summary1")
				s0 := m0.Summary()
				pt0 := s0.DataPoints().AppendEmpty()
				pt0.Attributes().InsertString("v1", "v2")
				populateSummary(&pt0, t1Ms, 10, 40, percent0, []float64{1, 5, 8})
				return &mL
			}(),
			1,
		},
		{
			"Summary No Count: round 2 - instance adjusted based on round 1",
			func() *pdata.MetricSlice {
				mL := pdata.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pdata.MetricDataTypeSummary)
				m0.SetName("summary1")
				s0 := m0.Summary()
				pt0 := s0.DataPoints().AppendEmpty()
				pt0.Attributes().InsertString("v1", "v2")
				populateSummary(&pt0, t2Ms, 15, 70, percent0, []float64{7, 44, 9})
				return &mL
			}(),
			func() *pdata.MetricSlice {
				mL := pdata.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pdata.MetricDataTypeSummary)
				m0.SetName("summary1")
				s0 := m0.Summary()
				pt0 := s0.DataPoints().AppendEmpty()
				pt0.Attributes().InsertString("v1", "v2")
				populateSummary(&pt0, t2Ms, 15, 70, percent0, []float64{7, 44, 9})
				return &mL
			}(),
			0,
		},
		{
			"Summary No Count: round 3 - instance reset (count less than previous), start time is reset",
			func() *pdata.MetricSlice {
				mL := pdata.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pdata.MetricDataTypeSummary)
				m0.SetName("summary1")
				s0 := m0.Summary()
				pt0 := s0.DataPoints().AppendEmpty()
				pt0.Attributes().InsertString("v1", "v2")
				populateSummary(&pt0, t3Ms, 12, 66, percent0, []float64{3, 22, 5})
				return &mL
			}(),
			func() *pdata.MetricSlice {
				mL := pdata.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pdata.MetricDataTypeSummary)
				m0.SetName("summary1")
				s0 := m0.Summary()
				pt0 := s0.DataPoints().AppendEmpty()
				pt0.Attributes().InsertString("v1", "v2")
				populateSummary(&pt0, t3Ms, 12, 66, percent0, []float64{3, 22, 5})
				return &mL
			}(),
			1,
		},
		{
			"Summary No Count: round 4 - instance adjusted based on round 3",
			func() *pdata.MetricSlice {
				mL := pdata.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pdata.MetricDataTypeSummary)
				m0.SetName("summary1")
				s0 := m0.Summary()
				pt0 := s0.DataPoints().AppendEmpty()
				pt0.Attributes().InsertString("v1", "v2")
				populateSummary(&pt0, t4Ms, 14, 96, percent0, []float64{9, 47, 8})
				pt0.SetStartTimestamp(t4Ms)
				return &mL
			}(),
			func() *pdata.MetricSlice {
				mL := pdata.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pdata.MetricDataTypeSummary)
				m0.SetName("summary1")
				s0 := m0.Summary()
				pt0 := s0.DataPoints().AppendEmpty()
				pt0.Attributes().InsertString("v1", "v2")
				populateSummary(&pt0, t4Ms, 14, 96, percent0, []float64{9, 47, 8})
				return &mL
			}(),
			0,
		},
	}

	runScript(t, NewJobsMapPdata(time.Minute).get("job", "0"), script)
}

func Test_summary(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			"Summary: round 1 - initial instance, start time is established",
			metricSlice(
				summaryMetric(s1, k1v1k2v2, t1Ms, summaryPoint(t1Ms, 10, 40, percent0, []float64{1, 5, 8})),
			),
			metricSlice(
				summaryMetric(s1, k1v1k2v2, t1Ms, summaryPoint(t1Ms, 10, 40, percent0, []float64{1, 5, 8})),
			),
			1,
		},
		{
			"Summary: round 2 - instance adjusted based on round 1",
			metricSlice(
				summaryMetric(s1, k1v1k2v2, t2Ms, summaryPoint(t2Ms, 15, 70, percent0, []float64{7, 44, 9})),
			),
			metricSlice(
				summaryMetric(s1, k1v1k2v2, t1Ms, summaryPoint(t2Ms, 15, 70, percent0, []float64{7, 44, 9})),
			),
			0,
		},
		{
			"Summary: round 3 - instance reset (count less than previous), start time is reset",
			metricSlice(
				summaryMetric(s1, k1v1k2v2, t3Ms, summaryPoint(t3Ms, 12, 66, percent0, []float64{3, 22, 5})),
			),
			metricSlice(
				summaryMetric(s1, k1v1k2v2, t3Ms, summaryPoint(t3Ms, 12, 66, percent0, []float64{3, 22, 5})),
			),
			1,
		},
		{
			"Summary: round 4 - instance adjusted based on round 3",
			metricSlice(
				summaryMetric(s1, k1v1k2v2, t4Ms, summaryPoint(t4Ms, 14, 96, percent0, []float64{9, 47, 8})),
			),
			metricSlice(
				summaryMetric(s1, k1v1k2v2, t3Ms, summaryPoint(t4Ms, 14, 96, percent0, []float64{9, 47, 8})),
			),
			0,
		},
	}

	runScript(t, NewJobsMapPdata(time.Minute).get("job", "0"), script)
}

func distPoint(ts pdata.Timestamp, bounds []float64, counts []uint64) *pdata.HistogramDataPoint {
	hdp := pdata.NewHistogramDataPoint()
	hdp.SetExplicitBounds(bounds)
	hdp.SetBucketCounts(counts)
	hdp.SetTimestamp(ts)
	var sum float64
	var count uint64
	for i, bcount := range counts {
		count += bcount
		if i > 0 {
			sum += float64(bcount) * bounds[i-1]
		}
	}
	hdp.SetCount(count)
	hdp.SetSum(sum)

	return &hdp
}

type kv struct {
	key, value string
}

func gaugeDistMetric(name string, kvp []*kv, startTs pdata.Timestamp, points ...*pdata.HistogramDataPoint) *pdata.Metric {
	hMetric := histogramMetric(name, kvp, startTs, points...)
	hMetric.Histogram().SetAggregationTemporality(pdata.AggregationTemporalityDelta)
	return hMetric
}

func histogramMetric(name string, kvp []*kv, startTs pdata.Timestamp, points ...*pdata.HistogramDataPoint) *pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeHistogram)
	histogram := metric.Histogram()
	histogram.SetAggregationTemporality(pdata.AggregationTemporalityCumulative)

	destPointL := histogram.DataPoints()
	// By default the AggregationTemporality is Cumulative until it'll be changed by the caller.
	for _, point := range points {
		destPoint := destPointL.AppendEmpty()
		point.CopyTo(destPoint)
		point.SetStartTimestamp(startTs)
		attrs := destPoint.Attributes()
		for _, kv := range kvp {
			attrs.InsertString(kv.key, kv.value)
		}
	}
	return &metric
}

func doublePoint(ts pdata.Timestamp, value float64) *pdata.NumberDataPoint {
	ndp := pdata.NewNumberDataPoint()
	ndp.SetTimestamp(ts)
	ndp.SetDoubleVal(value)

	return &ndp
}

func gaugeMetric(name string, kvp []*kv, startTs pdata.Timestamp, points ...*pdata.NumberDataPoint) *pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeGauge)

	destPointL := metric.Gauge().DataPoints()
	for _, point := range points {
		destPoint := destPointL.AppendEmpty()
		point.CopyTo(destPoint)
		point.SetStartTimestamp(startTs)
		attrs := destPoint.Attributes()
		for _, kv := range kvp {
			attrs.InsertString(kv.key, kv.value)
		}
	}
	return &metric
}

func summaryPoint(ts pdata.Timestamp, count uint64, sum float64, quantiles, values []float64) *pdata.SummaryDataPoint {
	sdp := pdata.NewSummaryDataPoint()
	sdp.SetTimestamp(ts)
	sdp.SetCount(count)
	sdp.SetSum(sum)
	qvL := sdp.QuantileValues()
	for i := 0; i < len(quantiles); i++ {
		qvi := qvL.AppendEmpty()
		qvi.SetQuantile(quantiles[i])
		qvi.SetValue(values[i])
	}
	return &sdp
}

func summaryMetric(name string, kvp []*kv, startTs pdata.Timestamp, points ...*pdata.SummaryDataPoint) *pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeSummary)

	destPointL := metric.Summary().DataPoints()
	for _, point := range points {
		destPoint := destPointL.AppendEmpty()
		point.CopyTo(destPoint)
		point.SetStartTimestamp(startTs)
		attrs := destPoint.Attributes()
		for _, kv := range kvp {
			attrs.InsertString(kv.key, kv.value)
		}
	}
	return &metric
}

func sumMetric(name string, kvp []*kv, startTs pdata.Timestamp, points ...*pdata.NumberDataPoint) *pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeSum)

	destPointL := metric.Sum().DataPoints()
	for _, point := range points {
		destPoint := destPointL.AppendEmpty()
		point.CopyTo(destPoint)
		point.SetStartTimestamp(startTs)
		attrs := destPoint.Attributes()
		for _, kv := range kvp {
			attrs.InsertString(kv.key, kv.value)
		}
	}
	return &metric
}

func metricSlice(metrics ...*pdata.Metric) *pdata.MetricSlice {
	ms := pdata.NewMetricSlice()
	for _, metric := range metrics {
		destMetric := ms.AppendEmpty()
		metric.CopyTo(destMetric)
	}
	return &ms
}

var (
	k1v1k2v2 = []*kv{
		{"k1", "v1"},
		{"k2", "v2"},
	}

	k1v10k2v20 = []*kv{
		{"k1", "v10"},
		{"k2", "v20"},
	}

	k1v100k2v200 = []*kv{
		{"k1", "v100"},
		{"k2", "v200"},
	}
)

func Test_cumulativeDistribution(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			"CumulativeDist: round 1 - initial instance, start time is established",
			metricSlice(histogramMetric(cd1, k1v1k2v2, t1Ms, distPoint(t1Ms, bounds0, []uint64{4, 2, 3, 7}))),
			metricSlice(histogramMetric(cd1, k1v1k2v2, t1Ms, distPoint(t1Ms, bounds0, []uint64{4, 2, 3, 7}))),
			1,
		}, {
			"CumulativeDist: round 2 - instance adjusted based on round 1",
			metricSlice(histogramMetric(cd1, k1v1k2v2, t2Ms, distPoint(t2Ms, bounds0, []uint64{6, 3, 4, 8}))),
			metricSlice(histogramMetric(cd1, k1v1k2v2, t1Ms, distPoint(t2Ms, bounds0, []uint64{6, 3, 4, 8}))),
			0,
		}, {
			"CumulativeDist: round 3 - instance reset (value less than previous value), start time is reset",
			metricSlice(histogramMetric(cd1, k1v1k2v2, t3Ms, distPoint(t3Ms, bounds0, []uint64{5, 3, 2, 7}))),
			metricSlice(histogramMetric(cd1, k1v1k2v2, t3Ms, distPoint(t3Ms, bounds0, []uint64{5, 3, 2, 7}))),
			1,
		}, {
			"CumulativeDist: round 4 - instance adjusted based on round 3",
			metricSlice(histogramMetric(cd1, k1v1k2v2, t4Ms, distPoint(t4Ms, bounds0, []uint64{7, 4, 2, 12}))),
			metricSlice(histogramMetric(cd1, k1v1k2v2, t3Ms, distPoint(t4Ms, bounds0, []uint64{7, 4, 2, 12}))),
			0,
		},
	}
	runScript(t, NewJobsMapPdata(time.Minute).get("job", "0"), script)
}

func Test_multiMetrics(t *testing.T) {
	g1 := "gauge1"
	script := []*metricsAdjusterTest{
		{
			"MultiMetrics: round 1 - combined round 1 of individual metrics",
			metricSlice(
				gaugeMetric(g1, k1v1k2v2, t1Ms, doublePoint(t1Ms, 44)),
				gaugeDistMetric(gd1, k1v1k2v2, t1Ms, distPoint(t1Ms, bounds0, []uint64{4, 2, 3, 7})),
				sumMetric(c1, k1v1k2v2, t1Ms, doublePoint(t1Ms, 44)),
				histogramMetric(cd1, k1v1k2v2, t1Ms, distPoint(t1Ms, bounds0, []uint64{4, 2, 3, 7})),
				summaryMetric(s1, k1v1k2v2, t1Ms, summaryPoint(t1Ms, 10, 40, percent0, []float64{1, 5, 8})),
			),
			metricSlice(
				gaugeMetric(g1, k1v1k2v2, t1Ms, doublePoint(t1Ms, 44)),
				gaugeDistMetric(gd1, k1v1k2v2, t1Ms, distPoint(t1Ms, bounds0, []uint64{4, 2, 3, 7})),
				sumMetric(c1, k1v1k2v2, t1Ms, doublePoint(t1Ms, 44)),
				histogramMetric(cd1, k1v1k2v2, t1Ms, distPoint(t1Ms, bounds0, []uint64{4, 2, 3, 7})),
				summaryMetric(s1, k1v1k2v2, t1Ms, summaryPoint(t1Ms, 10, 40, percent0, []float64{1, 5, 8})),
			),
			3,
		}, {
			"MultiMetrics: round 2 - combined round 2 of individual metrics",
			metricSlice(
				gaugeMetric(g1, k1v1k2v2, t2Ms, doublePoint(t2Ms, 66)),
				gaugeDistMetric(gd1, k1v1k2v2, t2Ms, distPoint(t2Ms, bounds0, []uint64{6, 5, 8, 11})),
				sumMetric(c1, k1v1k2v2, t2Ms, doublePoint(t2Ms, 66)),
				histogramMetric(cd1, k1v1k2v2, t2Ms, distPoint(t2Ms, bounds0, []uint64{6, 3, 4, 8})),
				summaryMetric(s1, k1v1k2v2, t2Ms, summaryPoint(t2Ms, 15, 70, percent0, []float64{7, 44, 9})),
			),
			metricSlice(
				gaugeMetric(g1, k1v1k2v2, t2Ms, doublePoint(t2Ms, 66)),
				gaugeDistMetric(gd1, k1v1k2v2, t2Ms, distPoint(t2Ms, bounds0, []uint64{6, 5, 8, 11})),
				sumMetric(c1, k1v1k2v2, t1Ms, doublePoint(t2Ms, 66)),
				histogramMetric(cd1, k1v1k2v2, t1Ms, distPoint(t2Ms, bounds0, []uint64{6, 3, 4, 8})),
				summaryMetric(s1, k1v1k2v2, t1Ms, summaryPoint(t2Ms, 15, 70, percent0, []float64{7, 44, 9})),
			),
			0,
		}, {
			"MultiMetrics: round 3 - combined round 3 of individual metrics",
			metricSlice(
				gaugeMetric(g1, k1v1k2v2, t3Ms, doublePoint(t3Ms, 55)),
				gaugeDistMetric(gd1, k1v1k2v2, t3Ms, distPoint(t3Ms, bounds0, []uint64{2, 0, 1, 5})),
				sumMetric(c1, k1v1k2v2, t3Ms, doublePoint(t3Ms, 55)),
				histogramMetric(cd1, k1v1k2v2, t3Ms, distPoint(t3Ms, bounds0, []uint64{5, 3, 2, 7})),
				summaryMetric(s1, k1v1k2v2, t3Ms, summaryPoint(t3Ms, 12, 66, percent0, []float64{3, 22, 5})),
			),
			metricSlice(
				gaugeMetric(g1, k1v1k2v2, t3Ms, doublePoint(t3Ms, 55)),
				gaugeDistMetric(gd1, k1v1k2v2, t3Ms, distPoint(t3Ms, bounds0, []uint64{2, 0, 1, 5})),
				sumMetric(c1, k1v1k2v2, t3Ms, doublePoint(t3Ms, 55)),
				histogramMetric(cd1, k1v1k2v2, t3Ms, distPoint(t3Ms, bounds0, []uint64{5, 3, 2, 7})),
				summaryMetric(s1, k1v1k2v2, t3Ms, summaryPoint(t3Ms, 12, 66, percent0, []float64{3, 22, 5})),
			),
			3,
		}, {
			"MultiMetrics: round 4 - combined round 4 of individual metrics",
			metricSlice(
				sumMetric(c1, k1v1k2v2, t4Ms, doublePoint(t4Ms, 72)),
				histogramMetric(cd1, k1v1k2v2, t4Ms, distPoint(t4Ms, bounds0, []uint64{7, 4, 2, 12})),
				summaryMetric(s1, k1v1k2v2, t4Ms, summaryPoint(t4Ms, 14, 96, percent0, []float64{9, 47, 8})),
			),
			metricSlice(
				sumMetric(c1, k1v1k2v2, t3Ms, doublePoint(t4Ms, 72)),
				histogramMetric(cd1, k1v1k2v2, t3Ms, distPoint(t4Ms, bounds0, []uint64{7, 4, 2, 12})),
				summaryMetric(s1, k1v1k2v2, t3Ms, summaryPoint(t4Ms, 14, 96, percent0, []float64{9, 47, 8})),
			),
			0,
		},
	}
	runScript(t, NewJobsMapPdata(time.Minute).get("job", "0"), script)
}

func Test_multiTimeseries(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			"MultiTimeseries: round 1 - initial first instance, start time is established",
			metricSlice(sumMetric(c1, k1v1k2v2, t1Ms, doublePoint(t1Ms, 44))),
			metricSlice(sumMetric(c1, k1v1k2v2, t1Ms, doublePoint(t1Ms, 44))),
			1,
		}, {
			"MultiTimeseries: round 2 - first instance adjusted based on round 1, initial second instance",
			metricSlice(
				sumMetric(c1, k1v1k2v2, t2Ms, doublePoint(t2Ms, 66)),
				sumMetric(c1, k1v10k2v20, t2Ms, doublePoint(t2Ms, 20.0)),
			),
			metricSlice(
				sumMetric(c1, k1v1k2v2, t1Ms, doublePoint(t2Ms, 66)),
				sumMetric(c1, k1v10k2v20, t2Ms, doublePoint(t2Ms, 20.0)),
			),
			1,
		}, {
			"MultiTimeseries: round 3 - first instance adjusted based on round 1, second based on round 2",
			metricSlice(
				sumMetric(c1, k1v1k2v2, t3Ms, doublePoint(t3Ms, 88.0)),
				sumMetric(c1, k1v10k2v20, t3Ms, doublePoint(t3Ms, 49.0)),
			),
			metricSlice(
				sumMetric(c1, k1v1k2v2, t1Ms, doublePoint(t3Ms, 88.0)),
				sumMetric(c1, k1v10k2v20, t2Ms, doublePoint(t3Ms, 49.0)),
			),
			0,
		}, {
			"MultiTimeseries: round 4 - first instance reset, second instance adjusted based on round 2, initial third instance",
			metricSlice(
				sumMetric(c1, k1v1k2v2, t4Ms, doublePoint(t4Ms, 87.0)),
				sumMetric(c1, k1v10k2v20, t4Ms, doublePoint(t4Ms, 57.0)),
				sumMetric(c1, k1v100k2v200, t4Ms, doublePoint(t4Ms, 10.0)),
			),
			metricSlice(
				sumMetric(c1, k1v1k2v2, t4Ms, doublePoint(t4Ms, 87.0)),
				sumMetric(c1, k1v10k2v20, t2Ms, doublePoint(t4Ms, 57.0)),
				sumMetric(c1, k1v100k2v200, t4Ms, doublePoint(t4Ms, 10.0)),
			),
			2,
		}, {
			"MultiTimeseries: round 5 - first instance adjusted based on round 4, second on round 2, third on round 4",
			metricSlice(
				sumMetric(c1, k1v1k2v2, t5Ms, doublePoint(t5Ms, 90.0)),
				sumMetric(c1, k1v10k2v20, t5Ms, doublePoint(t5Ms, 65.0)),
				sumMetric(c1, k1v100k2v200, t5Ms, doublePoint(t5Ms, 22.0)),
			),
			metricSlice(
				sumMetric(c1, k1v1k2v2, t4Ms, doublePoint(t5Ms, 90.0)),
				sumMetric(c1, k1v10k2v20, t2Ms, doublePoint(t5Ms, 65.0)),
				sumMetric(c1, k1v100k2v200, t4Ms, doublePoint(t5Ms, 22.0)),
			),
			0,
		},
	}
	runScript(t, NewJobsMapPdata(time.Minute).get("job", "0"), script)
}

var (
	emptyLabels              = []*kv{}
	k1vEmpty                 = []*kv{{"k1", ""}}
	k1vEmptyk2vEmptyk3vEmpty = []*kv{{"k1", ""}, {"k2", ""}, {"k3", ""}}
)

func Test_emptyLabels(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			"EmptyLabels: round 1 - initial instance, implicitly empty labels, start time is established",
			metricSlice(sumMetric(c1, emptyLabels, t1Ms, doublePoint(t1Ms, 44))),
			metricSlice(sumMetric(c1, emptyLabels, t1Ms, doublePoint(t1Ms, 44))),
			1,
		}, {
			"EmptyLabels: round 2 - instance adjusted based on round 1",
			metricSlice(sumMetric(c1, emptyLabels, t2Ms, doublePoint(t2Ms, 66))),
			metricSlice(sumMetric(c1, emptyLabels, t1Ms, doublePoint(t2Ms, 66))),
			0,
		}, {
			"EmptyLabels: round 3 - one explicitly empty label, instance adjusted based on round 1",
			metricSlice(sumMetric(c1, k1vEmpty, t3Ms, doublePoint(t3Ms, 77))),
			metricSlice(sumMetric(c1, k1vEmpty, t1Ms, doublePoint(t3Ms, 77))),
			0,
		}, {
			"EmptyLabels: round 4 - three explicitly empty labels, instance adjusted based on round 1",
			metricSlice(sumMetric(c1, k1vEmptyk2vEmptyk3vEmpty, t3Ms, doublePoint(t3Ms, 88))),
			metricSlice(sumMetric(c1, k1vEmptyk2vEmptyk3vEmpty, t1Ms, doublePoint(t3Ms, 88))),
			0,
		},
	}
	runScript(t, NewJobsMapPdata(time.Minute).get("job", "0"), script)
}

func Test_tsGC(t *testing.T) {
	script1 := []*metricsAdjusterTest{
		{
			"TsGC: round 1 - initial instances, start time is established",
			metricSlice(
				sumMetric(c1, k1v1k2v2, t1Ms, doublePoint(t1Ms, 44)),
				sumMetric(c1, k1v10k2v20, t1Ms, doublePoint(t1Ms, 20)),
				histogramMetric(cd1, k1v1k2v2, t1Ms, distPoint(t1Ms, bounds0, []uint64{4, 2, 3, 7})),
				histogramMetric(cd1, k1v10k2v20, t1Ms, distPoint(t1Ms, bounds0, []uint64{40, 20, 30, 70})),
			),
			metricSlice(
				sumMetric(c1, k1v1k2v2, t1Ms, doublePoint(t1Ms, 44)),
				sumMetric(c1, k1v10k2v20, t1Ms, doublePoint(t1Ms, 20)),
				histogramMetric(cd1, k1v1k2v2, t1Ms, distPoint(t1Ms, bounds0, []uint64{4, 2, 3, 7})),
				histogramMetric(cd1, k1v10k2v20, t1Ms, distPoint(t1Ms, bounds0, []uint64{40, 20, 30, 70})),
			),
			4,
		},
	}

	script2 := []*metricsAdjusterTest{
		{
			"TsGC: round 2 - metrics first timeseries adjusted based on round 2, second timeseries not updated",
			metricSlice(
				sumMetric(c1, k1v1k2v2, t2Ms, doublePoint(t2Ms, 88)),
				histogramMetric(cd1, k1v1k2v2, t2Ms, distPoint(t2Ms, bounds0, []uint64{8, 7, 9, 14})),
			),
			metricSlice(
				sumMetric(c1, k1v1k2v2, t1Ms, doublePoint(t2Ms, 88)),
				histogramMetric(cd1, k1v1k2v2, t1Ms, distPoint(t2Ms, bounds0, []uint64{8, 7, 9, 14})),
			),
			0,
		},
	}

	script3 := []*metricsAdjusterTest{
		{
			"TsGC: round 3 - metrics first timeseries adjusted based on round 2, second timeseries empty due to timeseries gc()",
			metricSlice(
				sumMetric(c1, k1v1k2v2, t3Ms, doublePoint(t3Ms, 99)),
				sumMetric(c1, k1v10k2v20, t3Ms, doublePoint(t3Ms, 80)),
				histogramMetric(cd1, k1v1k2v2, t3Ms, distPoint(t3Ms, bounds0, []uint64{9, 8, 10, 15})),
				histogramMetric(cd1, k1v10k2v20, t3Ms, distPoint(t3Ms, bounds0, []uint64{55, 66, 33, 77})),
			),
			metricSlice(
				sumMetric(c1, k1v1k2v2, t1Ms, doublePoint(t3Ms, 99)),
				sumMetric(c1, k1v10k2v20, t3Ms, doublePoint(t3Ms, 80)),
				histogramMetric(cd1, k1v1k2v2, t1Ms, distPoint(t3Ms, bounds0, []uint64{9, 8, 10, 15})),
				histogramMetric(cd1, k1v10k2v20, t3Ms, distPoint(t3Ms, bounds0, []uint64{55, 66, 33, 77})),
			),
			2,
		},
	}

	jobsMap := NewJobsMapPdata(time.Minute)

	// run round 1
	runScript(t, jobsMap.get("job", "0"), script1)
	// gc the tsmap, unmarking all entries
	jobsMap.get("job", "0").gc()
	// run round 2 - update metrics first timeseries only
	runScript(t, jobsMap.get("job", "0"), script2)
	// gc the tsmap, collecting umarked entries
	jobsMap.get("job", "0").gc()
	// run round 3 - verify that metrics second timeseries have been gc'd
	runScript(t, jobsMap.get("job", "0"), script3)
}

func Test_jobGC(t *testing.T) {
	job1Script1 := []*metricsAdjusterTest{
		{
			"JobGC: job 1, round 1 - initial instances, adjusted should be empty",
			metricSlice(
				sumMetric(c1, k1v1k2v2, t1Ms, doublePoint(t1Ms, 44)),
				sumMetric(c1, k1v10k2v20, t1Ms, doublePoint(t1Ms, 20)),
				histogramMetric(cd1, k1v1k2v2, t1Ms, distPoint(t1Ms, bounds0, []uint64{4, 2, 3, 7})),
				histogramMetric(cd1, k1v10k2v20, t1Ms, distPoint(t1Ms, bounds0, []uint64{40, 20, 30, 70})),
			),
			metricSlice(
				sumMetric(c1, k1v1k2v2, t1Ms, doublePoint(t1Ms, 44)),
				sumMetric(c1, k1v10k2v20, t1Ms, doublePoint(t1Ms, 20)),
				histogramMetric(cd1, k1v1k2v2, t1Ms, distPoint(t1Ms, bounds0, []uint64{4, 2, 3, 7})),
				histogramMetric(cd1, k1v10k2v20, t1Ms, distPoint(t1Ms, bounds0, []uint64{40, 20, 30, 70})),
			),
			4,
		},
	}

	emptyMetricSlice := func() *pdata.MetricSlice { ms := pdata.NewMetricSlice(); return &ms }
	job2Script1 := []*metricsAdjusterTest{
		{
			"JobGC: job2, round 1 - no metrics adjusted, just trigger gc",
			emptyMetricSlice(),
			emptyMetricSlice(),
			0,
		},
	}

	job1Script2 := []*metricsAdjusterTest{
		{
			"JobGC: job 1, round 2 - metrics timeseries empty due to job-level gc",
			metricSlice(
				sumMetric(c1, k1v1k2v2, t4Ms, doublePoint(t4Ms, 99)),
				sumMetric(c1, k1v10k2v20, t4Ms, doublePoint(t4Ms, 80)),
				histogramMetric(cd1, k1v1k2v2, t4Ms, distPoint(t4Ms, bounds0, []uint64{9, 8, 10, 15})),
				histogramMetric(cd1, k1v10k2v20, t4Ms, distPoint(t4Ms, bounds0, []uint64{55, 66, 33, 77})),
			),
			metricSlice(
				sumMetric(c1, k1v1k2v2, t4Ms, doublePoint(t4Ms, 99)),
				sumMetric(c1, k1v10k2v20, t4Ms, doublePoint(t4Ms, 80)),
				histogramMetric(cd1, k1v1k2v2, t4Ms, distPoint(t4Ms, bounds0, []uint64{9, 8, 10, 15})),
				histogramMetric(cd1, k1v10k2v20, t4Ms, distPoint(t4Ms, bounds0, []uint64{55, 66, 33, 77})),
			),
			4,
		},
	}

	gcInterval := 10 * time.Millisecond
	jobsMap := NewJobsMapPdata(gcInterval)

	// run job 1, round 1 - all entries marked
	runScript(t, jobsMap.get("job", "0"), job1Script1)
	// sleep longer than gcInterval to enable job gc in the next run
	time.Sleep(2 * gcInterval)
	// run job 2, round1 - trigger job gc, unmarking all entries
	runScript(t, jobsMap.get("job", "1"), job2Script1)
	// sleep longer than gcInterval to enable job gc in the next run
	time.Sleep(2 * gcInterval)
	// re-run job 2, round1 - trigger job gc, removing unmarked entries
	runScript(t, jobsMap.get("job", "1"), job2Script1)
	// ensure that at least one jobsMap.gc() completed
	jobsMap.gc()
	// run job 1, round 2 - verify that all job 1 timeseries have been gc'd
	runScript(t, jobsMap.get("job", "0"), job1Script2)
	return
}

type metricsAdjusterTest struct {
	description string
	metrics     *pdata.MetricSlice
	adjusted    *pdata.MetricSlice
	resets      int
}

func runScript(t *testing.T, tsm *timeseriesMapPdata, script []*metricsAdjusterTest) {
	l := zap.NewNop()
	t.Cleanup(func() { require.NoError(t, l.Sync()) }) // flushes buffer, if any
	ma := NewMetricsAdjusterPdata(tsm, l)

	for _, test := range script {
		expectedResets := test.resets
		resets := ma.AdjustMetrics(test.metrics)
		adjusted := test.metrics
		assert.EqualValuesf(t, test.adjusted, adjusted, "Test: %v - expected: %v, actual: %v", test.description, test.adjusted, adjusted)
		assert.Equalf(t, expectedResets, resets, "Test: %v", test.description)
	}
}
