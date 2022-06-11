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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

var (
	pdt1Ms = pcommon.Timestamp(time.Unix(0, 1000000).UnixNano())
	pdt2Ms = pcommon.Timestamp(time.Unix(0, 2000000).UnixNano())
	pdt3Ms = pcommon.Timestamp(time.Unix(0, 3000000).UnixNano())
	pdt4Ms = pcommon.Timestamp(time.Unix(0, 5000000).UnixNano())
	pdt5Ms = pcommon.Timestamp(time.Unix(0, 5000000).UnixNano())

	bounds0  = []float64{1, 2, 4}
	percent0 = []float64{10, 50, 90}

	c1  = "cumulative1"
	cd1 = "cumulativedist1"
	s1  = "summary1"
)

func Test_gauge_pdata(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			"Gauge: round 1 - gauge not adjusted",
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeGauge)
				m0.SetName("gauge1")
				g0 := m0.Gauge()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(pdt1Ms)
				pt0.Attributes().InsertString("v1", "v2")
				pt0.SetTimestamp(pdt1Ms)
				pt0.SetDoubleVal(44)
				return &mL
			}(),
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeGauge)
				m0.SetName("gauge1")
				g0 := m0.Gauge()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(pdt1Ms)
				pt0.Attributes().InsertString("v1", "v2")
				pt0.SetTimestamp(pdt1Ms)
				pt0.SetDoubleVal(44)
				return &mL
			}(),
			0,
		},
		{
			"Gauge: round 2 - gauge not adjusted",
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeGauge)
				m0.SetName("gauge1")
				g0 := m0.Gauge()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(pdt2Ms)
				pt0.Attributes().InsertString("k1", "v1")
				pt0.Attributes().InsertString("k2", "v2")
				pt0.SetTimestamp(pdt2Ms)
				pt0.SetDoubleVal(66)

				return &mL
			}(),
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeGauge)
				m0.SetName("gauge1")
				g0 := m0.Gauge()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(pdt2Ms)
				pt0.Attributes().InsertString("k1", "v1")
				pt0.Attributes().InsertString("k2", "v2")
				pt0.SetTimestamp(pdt2Ms)
				pt0.SetDoubleVal(66)
				return &mL
			}(),
			0,
		},
		{
			"Gauge: round 3 - value less than previous value - gauge is not adjusted",
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeGauge)
				m0.SetName("gauge1")
				g0 := m0.Gauge()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(pdt3Ms)
				pt0.Attributes().InsertString("k1", "v1")
				pt0.Attributes().InsertString("k2", "v2")
				pt0.SetTimestamp(pdt3Ms)
				pt0.SetDoubleVal(55)

				return &mL
			}(),
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeGauge)
				m0.SetName("gauge1")
				g0 := m0.Gauge()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(pdt3Ms)
				pt0.Attributes().InsertString("k1", "v1")
				pt0.Attributes().InsertString("k2", "v2")
				pt0.SetTimestamp(pdt3Ms)
				pt0.SetDoubleVal(55)

				return &mL
			}(),
			0,
		},
	}
	runScript(t, NewJobsMap(time.Minute).get("job", "0"), script)
}

func Test_cumulative_pdata(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			"Cumulative: round 1 - initial instance, start time is established",
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeSum)
				m0.SetName("cumulative1")
				g0 := m0.Sum()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(pdt1Ms)
				pt0.Attributes().InsertString("k1", "v1")
				pt0.Attributes().InsertString("k2", "v2")
				pt0.SetTimestamp(pdt1Ms)
				pt0.SetDoubleVal(44)

				return &mL
			}(),
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeSum)
				m0.SetName("cumulative1")
				g0 := m0.Sum()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(pdt1Ms)
				pt0.Attributes().InsertString("k1", "v1")
				pt0.Attributes().InsertString("k2", "v2")
				pt0.SetTimestamp(pdt1Ms)
				pt0.SetDoubleVal(44)

				return &mL
			}(),
			1,
		},
		{
			"Cumulative: round 2 - instance adjusted based on round 1",
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeSum)
				m0.SetName("cumulative1")
				g0 := m0.Sum()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(pdt2Ms)
				pt0.Attributes().InsertString("k1", "v1")
				pt0.Attributes().InsertString("k2", "v2")
				pt0.SetTimestamp(pdt2Ms)
				pt0.SetDoubleVal(66)

				return &mL
			}(),
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeSum)
				m0.SetName("cumulative1")
				g0 := m0.Sum()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(pdt1Ms)
				pt0.Attributes().InsertString("k1", "v1")
				pt0.Attributes().InsertString("k2", "v2")
				pt0.SetTimestamp(pdt2Ms)
				pt0.SetDoubleVal(66)

				return &mL
			}(),
			0,
		},
		{
			"Cumulative: round 3 - instance reset (value less than previous value), start time is reset",
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeSum)
				m0.SetName("cumulative1")
				g0 := m0.Sum()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(pdt3Ms)
				pt0.Attributes().InsertString("k1", "v1")
				pt0.Attributes().InsertString("k2", "v2")
				pt0.SetTimestamp(pdt3Ms)
				pt0.SetDoubleVal(55)

				return &mL
			}(),
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeSum)
				m0.SetName("cumulative1")
				g0 := m0.Sum()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(pdt3Ms)
				pt0.Attributes().InsertString("k1", "v1")
				pt0.Attributes().InsertString("k2", "v2")
				pt0.SetTimestamp(pdt3Ms)
				pt0.SetDoubleVal(55)

				return &mL
			}(),
			1,
		},
		{
			"Cumulative: round 4 - instance adjusted based on round 3",
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeSum)
				m0.SetName("cumulative1")
				g0 := m0.Sum()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(pdt4Ms)
				pt0.Attributes().InsertString("k1", "v1")
				pt0.Attributes().InsertString("k2", "v2")
				pt0.SetTimestamp(pdt4Ms)
				pt0.SetDoubleVal(72)

				return &mL
			}(),
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeSum)
				m0.SetName("cumulative1")
				g0 := m0.Sum()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(pdt3Ms)
				pt0.Attributes().InsertString("k1", "v1")
				pt0.Attributes().InsertString("k2", "v2")
				pt0.SetTimestamp(pdt4Ms)
				pt0.SetDoubleVal(72)

				return &mL
			}(),
			0,
		},
		{
			"Cumulative: round 5 - instance adjusted based on round 4",
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeSum)
				m0.SetName("cumulative1")
				g0 := m0.Sum()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(pdt5Ms)
				pt0.Attributes().InsertString("k1", "v1")
				pt0.Attributes().InsertString("k2", "v2")
				pt0.SetTimestamp(pdt5Ms)
				pt0.SetFlags(1)

				return &mL
			}(),
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeSum)
				m0.SetName("cumulative1")
				g0 := m0.Sum()
				pt0 := g0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(pdt3Ms)
				pt0.Attributes().InsertString("k1", "v1")
				pt0.Attributes().InsertString("k2", "v2")
				pt0.SetTimestamp(pdt5Ms)
				pt0.SetFlags(1)

				return &mL
			}(),
			0,
		},
	}
	runScript(t, NewJobsMap(time.Minute).get("job", "0"), script)
}

func populateSummary(sdp *pmetric.SummaryDataPoint, timestamp pcommon.Timestamp, count uint64, sum float64, quantilePercents, quantileValues []float64) {
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

func Test_summary_no_count_pdata(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			"Summary No Count: round 1 - initial instance, start time is established",
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeSummary)
				m0.SetName("summary1")
				s0 := m0.Summary()
				pt0 := s0.DataPoints().AppendEmpty()
				pt0.Attributes().InsertString("v1", "v2")
				populateSummary(&pt0, pdt1Ms, 10, 40, percent0, []float64{1, 5, 8})
				return &mL
			}(),
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeSummary)
				m0.SetName("summary1")
				s0 := m0.Summary()
				pt0 := s0.DataPoints().AppendEmpty()
				pt0.Attributes().InsertString("v1", "v2")
				populateSummary(&pt0, pdt1Ms, 10, 40, percent0, []float64{1, 5, 8})
				return &mL
			}(),
			1,
		},
		{
			"Summary No Count: round 2 - instance adjusted based on round 1",
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeSummary)
				m0.SetName("summary1")
				s0 := m0.Summary()
				pt0 := s0.DataPoints().AppendEmpty()
				pt0.Attributes().InsertString("v1", "v2")
				populateSummary(&pt0, pdt2Ms, 15, 70, percent0, []float64{7, 44, 9})
				return &mL
			}(),
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeSummary)
				m0.SetName("summary1")
				s0 := m0.Summary()
				pt0 := s0.DataPoints().AppendEmpty()
				pt0.Attributes().InsertString("v1", "v2")
				populateSummary(&pt0, pdt2Ms, 15, 70, percent0, []float64{7, 44, 9})
				return &mL
			}(),
			0,
		},
		{
			"Summary No Count: round 3 - instance reset (count less than previous), start time is reset",
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeSummary)
				m0.SetName("summary1")
				s0 := m0.Summary()
				pt0 := s0.DataPoints().AppendEmpty()
				pt0.Attributes().InsertString("v1", "v2")
				populateSummary(&pt0, pdt3Ms, 12, 66, percent0, []float64{3, 22, 5})
				return &mL
			}(),
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeSummary)
				m0.SetName("summary1")
				s0 := m0.Summary()
				pt0 := s0.DataPoints().AppendEmpty()
				pt0.Attributes().InsertString("v1", "v2")
				populateSummary(&pt0, pdt3Ms, 12, 66, percent0, []float64{3, 22, 5})
				return &mL
			}(),
			1,
		},
		{
			"Summary No Count: round 4 - instance adjusted based on round 3",
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeSummary)
				m0.SetName("summary1")
				s0 := m0.Summary()
				pt0 := s0.DataPoints().AppendEmpty()
				pt0.Attributes().InsertString("v1", "v2")
				populateSummary(&pt0, pdt4Ms, 14, 96, percent0, []float64{9, 47, 8})
				pt0.SetStartTimestamp(pdt4Ms)
				return &mL
			}(),
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeSummary)
				m0.SetName("summary1")
				s0 := m0.Summary()
				pt0 := s0.DataPoints().AppendEmpty()
				pt0.Attributes().InsertString("v1", "v2")
				populateSummary(&pt0, pdt4Ms, 14, 96, percent0, []float64{9, 47, 8})
				return &mL
			}(),
			0,
		},
	}

	runScript(t, NewJobsMap(time.Minute).get("job", "0"), script)
}

func Test_summary_flag_norecordedvalue(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			"Summary No Count: round 1 - initial instance, start time is established",
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeSummary)
				m0.SetName("summary1")
				s0 := m0.Summary()
				pt0 := s0.DataPoints().AppendEmpty()
				pt0.Attributes().InsertString("v1", "v2")
				pt0.SetStartTimestamp(pdt1Ms)
				pt0.SetTimestamp(pdt1Ms)
				populateSummary(&pt0, pdt1Ms, 10, 40, percent0, []float64{1, 5, 8})
				return &mL
			}(),
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeSummary)
				m0.SetName("summary1")
				s0 := m0.Summary()
				pt0 := s0.DataPoints().AppendEmpty()
				pt0.Attributes().InsertString("v1", "v2")
				pt0.SetStartTimestamp(pdt1Ms)
				pt0.SetTimestamp(pdt1Ms)
				populateSummary(&pt0, pdt1Ms, 10, 40, percent0, []float64{1, 5, 8})
				return &mL
			}(),
			1,
		},
		{
			"Summary Flag NoRecordedValue: round 2 - instance adjusted based on round 1",
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeSummary)
				m0.SetName("summary1")
				s0 := m0.Summary()
				pt0 := s0.DataPoints().AppendEmpty()
				pt0.Attributes().InsertString("v1", "v2")
				pt0.SetStartTimestamp(pdt2Ms)
				pt0.SetTimestamp(pdt2Ms)
				pt0.SetFlags(1)
				return &mL
			}(),
			func() *pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
				m0.SetDataType(pmetric.MetricDataTypeSummary)
				m0.SetName("summary1")
				s0 := m0.Summary()
				pt0 := s0.DataPoints().AppendEmpty()
				pt0.Attributes().InsertString("v1", "v2")
				pt0.SetStartTimestamp(pdt1Ms)
				pt0.SetTimestamp(pdt2Ms)
				pt0.SetFlags(1)
				return &mL
			}(),
			0,
		},
	}

	runScript(t, NewJobsMap(time.Minute).get("job", "0"), script)
}

func Test_summary_pdata(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			"Summary: round 1 - initial instance, start time is established",
			metricSlice(
				summaryMetric(s1, k1v1k2v2, pdt1Ms, summaryPoint(pdt1Ms, 10, 40, percent0, []float64{1, 5, 8})),
			),
			metricSlice(
				summaryMetric(s1, k1v1k2v2, pdt1Ms, summaryPoint(pdt1Ms, 10, 40, percent0, []float64{1, 5, 8})),
			),
			1,
		},
		{
			"Summary: round 2 - instance adjusted based on round 1",
			metricSlice(
				summaryMetric(s1, k1v1k2v2, pdt2Ms, summaryPoint(pdt2Ms, 15, 70, percent0, []float64{7, 44, 9})),
			),
			metricSlice(
				summaryMetric(s1, k1v1k2v2, pdt1Ms, summaryPoint(pdt2Ms, 15, 70, percent0, []float64{7, 44, 9})),
			),
			0,
		},
		{
			"Summary: round 3 - instance reset (count less than previous), start time is reset",
			metricSlice(
				summaryMetric(s1, k1v1k2v2, pdt3Ms, summaryPoint(pdt3Ms, 12, 66, percent0, []float64{3, 22, 5})),
			),
			metricSlice(
				summaryMetric(s1, k1v1k2v2, pdt3Ms, summaryPoint(pdt3Ms, 12, 66, percent0, []float64{3, 22, 5})),
			),
			1,
		},
		{
			"Summary: round 4 - instance adjusted based on round 3",
			metricSlice(
				summaryMetric(s1, k1v1k2v2, pdt4Ms, summaryPoint(pdt4Ms, 14, 96, percent0, []float64{9, 47, 8})),
			),
			metricSlice(
				summaryMetric(s1, k1v1k2v2, pdt3Ms, summaryPoint(pdt4Ms, 14, 96, percent0, []float64{9, 47, 8})),
			),
			0,
		},
	}

	runScript(t, NewJobsMap(time.Minute).get("job", "0"), script)
}

func metricSlice(metrics ...*pmetric.Metric) *pmetric.MetricSlice {
	ms := pmetric.NewMetricSlice()
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

func Test_cumulativeDistribution_pdata(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			"CumulativeDist: round 1 - initial instance, start time is established",
			metricSlice(cumulativeDistMetric(cd1, k1v1k2v2, pdt1Ms, distPoint(pdt1Ms, bounds0, []uint64{4, 2, 3, 7}))),
			metricSlice(cumulativeDistMetric(cd1, k1v1k2v2, pdt1Ms, distPoint(pdt1Ms, bounds0, []uint64{4, 2, 3, 7}))),
			1,
		}, {
			"CumulativeDist: round 2 - instance adjusted based on round 1",
			metricSlice(cumulativeDistMetric(cd1, k1v1k2v2, pdt2Ms, distPoint(pdt2Ms, bounds0, []uint64{6, 3, 4, 8}))),
			metricSlice(cumulativeDistMetric(cd1, k1v1k2v2, pdt1Ms, distPoint(pdt2Ms, bounds0, []uint64{6, 3, 4, 8}))),
			0,
		}, {
			"CumulativeDist: round 3 - instance reset (value less than previous value), start time is reset",
			metricSlice(cumulativeDistMetric(cd1, k1v1k2v2, pdt3Ms, distPoint(pdt3Ms, bounds0, []uint64{5, 3, 2, 7}))),
			metricSlice(cumulativeDistMetric(cd1, k1v1k2v2, pdt3Ms, distPoint(pdt3Ms, bounds0, []uint64{5, 3, 2, 7}))),
			1,
		}, {
			"CumulativeDist: round 4 - instance adjusted based on round 3",
			metricSlice(cumulativeDistMetric(cd1, k1v1k2v2, pdt4Ms, distPoint(pdt4Ms, bounds0, []uint64{7, 4, 2, 12}))),
			metricSlice(cumulativeDistMetric(cd1, k1v1k2v2, pdt3Ms, distPoint(pdt4Ms, bounds0, []uint64{7, 4, 2, 12}))),
			0,
		},
	}
	runScript(t, NewJobsMap(time.Minute).get("job", "0"), script)
}

func Test_histogram_flag_norecordedvalue(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			"Histogram: round 1 - initial instance, start time is established",
			metricSlice(cumulativeDistMetric(cd1, k1v1k2v2, pdt1Ms, distPoint(pdt1Ms, bounds0, []uint64{7, 4, 2, 12}))),
			metricSlice(cumulativeDistMetric(cd1, k1v1k2v2, pdt1Ms, distPoint(pdt1Ms, bounds0, []uint64{7, 4, 2, 12}))),
			1,
		},
		{
			"Histogram: round 2 - instance adjusted based on round 1",
			func() *pmetric.MetricSlice {
				metric := pmetric.NewMetric()
				metric.SetName(cd1)
				metric.SetDataType(pmetric.MetricDataTypeHistogram)
				histogram := metric.Histogram()
				histogram.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				destPointL := histogram.DataPoints()
				dp := destPointL.AppendEmpty()
				dp.SetTimestamp(pdt2Ms)
				dp.SetFlags(1)
				return metricSlice(cumulativeDistMetric(cd1, k1v1k2v2, pdt2Ms, &dp))
			}(),
			func() *pmetric.MetricSlice {
				metric := pmetric.NewMetric()
				metric.SetName(cd1)
				metric.SetDataType(pmetric.MetricDataTypeHistogram)
				histogram := metric.Histogram()
				histogram.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				destPointL := histogram.DataPoints()
				dp := destPointL.AppendEmpty()
				dp.SetTimestamp(pdt2Ms)
				dp.SetFlags(1)
				return metricSlice(cumulativeDistMetric(cd1, k1v1k2v2, pdt1Ms, &dp))
			}(),
			0,
		},
	}

	runScript(t, NewJobsMap(time.Minute).get("job", "0"), script)
}

func Test_histogram_flag_norecordedvalue_first_observation(t *testing.T) {
	m1 := func() *pmetric.MetricSlice {
		metric := pmetric.NewMetric()
		metric.SetName(cd1)
		metric.SetDataType(pmetric.MetricDataTypeHistogram)
		histogram := metric.Histogram()
		histogram.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
		destPointL := histogram.DataPoints()
		dp := destPointL.AppendEmpty()
		dp.SetTimestamp(pdt1Ms)
		dp.SetFlags(1)
		return metricSlice(cumulativeDistMetric(cd1, k1v1k2v2, pdt1Ms, &dp))
	}()
	m2 := func() *pmetric.MetricSlice {
		metric := pmetric.NewMetric()
		metric.SetName(cd1)
		metric.SetDataType(pmetric.MetricDataTypeHistogram)
		histogram := metric.Histogram()
		histogram.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
		destPointL := histogram.DataPoints()
		dp := destPointL.AppendEmpty()
		dp.SetTimestamp(pdt2Ms)
		dp.SetFlags(1)
		return metricSlice(cumulativeDistMetric(cd1, k1v1k2v2, pdt2Ms, &dp))
	}()
	script := []*metricsAdjusterTest{
		{
			"Histogram: round 1 - initial instance, start time is unknown",
			m1,
			m1,
			1,
		},
		{
			"Histogram: round 2 - instance unchanged",
			m2,
			m2,
			0,
		},
	}

	runScript(t, NewJobsMap(time.Minute).get("job", "0"), script)
}

func Test_summary_flag_norecordedvalue_first_observation(t *testing.T) {
	m1 := func() *pmetric.MetricSlice {
		metric := pmetric.NewMetric()
		metric.SetName(cd1)
		metric.SetDataType(pmetric.MetricDataTypeSummary)
		summary := metric.Summary()
		destPointL := summary.DataPoints()
		dp := destPointL.AppendEmpty()
		dp.SetTimestamp(pdt1Ms)
		dp.SetFlags(1)
		return metricSlice(summaryMetric(cd1, k1v1k2v2, pdt1Ms, &dp))
	}()
	m2 := func() *pmetric.MetricSlice {
		metric := pmetric.NewMetric()
		metric.SetName(cd1)
		metric.SetDataType(pmetric.MetricDataTypeSummary)
		summary := metric.Summary()
		destPointL := summary.DataPoints()
		dp := destPointL.AppendEmpty()
		dp.SetTimestamp(pdt2Ms)
		dp.SetFlags(1)
		return metricSlice(summaryMetric(cd1, k1v1k2v2, pdt2Ms, &dp))
	}()
	script := []*metricsAdjusterTest{
		{
			"Summary: round 1 - initial instance, start time is unknown",
			m1,
			m1,
			1,
		},
		{
			"Summary: round 2 - instance unchanged",
			m2,
			m2,
			0,
		},
	}

	runScript(t, NewJobsMap(time.Minute).get("job", "0"), script)
}

func Test_gauge_flag_norecordedvalue_first_observation(t *testing.T) {
	m1 := func() *pmetric.MetricSlice {
		metric := pmetric.NewMetric()
		metric.SetName(cd1)
		metric.SetDataType(pmetric.MetricDataTypeGauge)
		gauge := metric.Gauge()
		destPointL := gauge.DataPoints()
		dp := destPointL.AppendEmpty()
		dp.SetTimestamp(pdt1Ms)
		dp.SetFlags(1)
		return metricSlice(gaugeMetric(cd1, k1v1k2v2, pdt1Ms, &dp))
	}()
	m2 := func() *pmetric.MetricSlice {
		metric := pmetric.NewMetric()
		metric.SetName(cd1)
		metric.SetDataType(pmetric.MetricDataTypeGauge)
		gauge := metric.Gauge()
		destPointL := gauge.DataPoints()
		dp := destPointL.AppendEmpty()
		dp.SetTimestamp(pdt2Ms)
		dp.SetFlags(1)
		return metricSlice(gaugeMetric(cd1, k1v1k2v2, pdt2Ms, &dp))
	}()
	script := []*metricsAdjusterTest{
		{
			"Gauge: round 1 - initial instance, start time is unknown",
			m1,
			m1,
			0,
		},
		{
			"Gauge: round 2 - instance unchanged",
			m2,
			m2,
			0,
		},
	}

	runScript(t, NewJobsMap(time.Minute).get("job", "0"), script)
}

func Test_sum_flag_norecordedvalue_first_observation(t *testing.T) {
	m1 := func() *pmetric.MetricSlice {
		metric := pmetric.NewMetric()
		metric.SetName(cd1)
		metric.SetDataType(pmetric.MetricDataTypeSum)
		sum := metric.Sum()
		sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
		destPointL := sum.DataPoints()
		dp := destPointL.AppendEmpty()
		dp.SetTimestamp(pdt1Ms)
		dp.SetFlags(1)
		return metricSlice(sumMetric(cd1, k1v1k2v2, pdt1Ms, &dp))
	}()
	m2 := func() *pmetric.MetricSlice {
		metric := pmetric.NewMetric()
		metric.SetName(cd1)
		metric.SetDataType(pmetric.MetricDataTypeSum)
		sum := metric.Sum()
		sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
		destPointL := sum.DataPoints()
		dp := destPointL.AppendEmpty()
		dp.SetTimestamp(pdt2Ms)
		dp.SetFlags(1)
		return metricSlice(sumMetric(cd1, k1v1k2v2, pdt2Ms, &dp))
	}()
	script := []*metricsAdjusterTest{
		{
			"Sum: round 1 - initial instance, start time is unknown",
			m1,
			m1,
			1,
		},
		{
			"Sum: round 2 - instance unchanged",
			m2,
			m2,
			0,
		},
	}

	runScript(t, NewJobsMap(time.Minute).get("job", "0"), script)
}

func Test_multiMetrics_pdata(t *testing.T) {
	g1 := "gauge1"
	script := []*metricsAdjusterTest{
		{
			"MultiMetrics: round 1 - combined round 1 of individual metrics",
			metricSlice(
				gaugeMetric(g1, k1v1k2v2, pdt1Ms, doublePoint(pdt1Ms, 44)),
				sumMetric(c1, k1v1k2v2, pdt1Ms, doublePoint(pdt1Ms, 44)),
				cumulativeDistMetric(cd1, k1v1k2v2, pdt1Ms, distPoint(pdt1Ms, bounds0, []uint64{4, 2, 3, 7})),
				summaryMetric(s1, k1v1k2v2, pdt1Ms, summaryPoint(pdt1Ms, 10, 40, percent0, []float64{1, 5, 8})),
			),
			metricSlice(
				gaugeMetric(g1, k1v1k2v2, pdt1Ms, doublePoint(pdt1Ms, 44)),
				sumMetric(c1, k1v1k2v2, pdt1Ms, doublePoint(pdt1Ms, 44)),
				cumulativeDistMetric(cd1, k1v1k2v2, pdt1Ms, distPoint(pdt1Ms, bounds0, []uint64{4, 2, 3, 7})),
				summaryMetric(s1, k1v1k2v2, pdt1Ms, summaryPoint(pdt1Ms, 10, 40, percent0, []float64{1, 5, 8})),
			),
			3,
		}, {
			"MultiMetrics: round 2 - combined round 2 of individual metrics",
			metricSlice(
				gaugeMetric(g1, k1v1k2v2, pdt2Ms, doublePoint(pdt2Ms, 66)),
				sumMetric(c1, k1v1k2v2, pdt2Ms, doublePoint(pdt2Ms, 66)),
				cumulativeDistMetric(cd1, k1v1k2v2, pdt2Ms, distPoint(pdt2Ms, bounds0, []uint64{6, 3, 4, 8})),
				summaryMetric(s1, k1v1k2v2, pdt2Ms, summaryPoint(pdt2Ms, 15, 70, percent0, []float64{7, 44, 9})),
			),
			metricSlice(
				gaugeMetric(g1, k1v1k2v2, pdt2Ms, doublePoint(pdt2Ms, 66)),
				sumMetric(c1, k1v1k2v2, pdt1Ms, doublePoint(pdt2Ms, 66)),
				cumulativeDistMetric(cd1, k1v1k2v2, pdt1Ms, distPoint(pdt2Ms, bounds0, []uint64{6, 3, 4, 8})),
				summaryMetric(s1, k1v1k2v2, pdt1Ms, summaryPoint(pdt2Ms, 15, 70, percent0, []float64{7, 44, 9})),
			),
			0,
		}, {
			"MultiMetrics: round 3 - combined round 3 of individual metrics",
			metricSlice(
				gaugeMetric(g1, k1v1k2v2, pdt3Ms, doublePoint(pdt3Ms, 55)),
				sumMetric(c1, k1v1k2v2, pdt3Ms, doublePoint(pdt3Ms, 55)),
				cumulativeDistMetric(cd1, k1v1k2v2, pdt3Ms, distPoint(pdt3Ms, bounds0, []uint64{5, 3, 2, 7})),
				summaryMetric(s1, k1v1k2v2, pdt3Ms, summaryPoint(pdt3Ms, 12, 66, percent0, []float64{3, 22, 5})),
			),
			metricSlice(
				gaugeMetric(g1, k1v1k2v2, pdt3Ms, doublePoint(pdt3Ms, 55)),
				sumMetric(c1, k1v1k2v2, pdt3Ms, doublePoint(pdt3Ms, 55)),
				cumulativeDistMetric(cd1, k1v1k2v2, pdt3Ms, distPoint(pdt3Ms, bounds0, []uint64{5, 3, 2, 7})),
				summaryMetric(s1, k1v1k2v2, pdt3Ms, summaryPoint(pdt3Ms, 12, 66, percent0, []float64{3, 22, 5})),
			),
			3,
		}, {
			"MultiMetrics: round 4 - combined round 4 of individual metrics",
			metricSlice(
				sumMetric(c1, k1v1k2v2, pdt4Ms, doublePoint(pdt4Ms, 72)),
				cumulativeDistMetric(cd1, k1v1k2v2, pdt4Ms, distPoint(pdt4Ms, bounds0, []uint64{7, 4, 2, 12})),
				summaryMetric(s1, k1v1k2v2, pdt4Ms, summaryPoint(pdt4Ms, 14, 96, percent0, []float64{9, 47, 8})),
			),
			metricSlice(
				sumMetric(c1, k1v1k2v2, pdt3Ms, doublePoint(pdt4Ms, 72)),
				cumulativeDistMetric(cd1, k1v1k2v2, pdt3Ms, distPoint(pdt4Ms, bounds0, []uint64{7, 4, 2, 12})),
				summaryMetric(s1, k1v1k2v2, pdt3Ms, summaryPoint(pdt4Ms, 14, 96, percent0, []float64{9, 47, 8})),
			),
			0,
		},
	}
	runScript(t, NewJobsMap(time.Minute).get("job", "0"), script)
}

func Test_multiTimeseries_pdata(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			"MultiTimeseries: round 1 - initial first instance, start time is established",
			metricSlice(sumMetric(c1, k1v1k2v2, pdt1Ms, doublePoint(pdt1Ms, 44))),
			metricSlice(sumMetric(c1, k1v1k2v2, pdt1Ms, doublePoint(pdt1Ms, 44))),
			1,
		}, {
			"MultiTimeseries: round 2 - first instance adjusted based on round 1, initial second instance",
			metricSlice(
				sumMetric(c1, k1v1k2v2, pdt2Ms, doublePoint(pdt2Ms, 66)),
				sumMetric(c1, k1v10k2v20, pdt2Ms, doublePoint(pdt2Ms, 20.0)),
			),
			metricSlice(
				sumMetric(c1, k1v1k2v2, pdt1Ms, doublePoint(pdt2Ms, 66)),
				sumMetric(c1, k1v10k2v20, pdt2Ms, doublePoint(pdt2Ms, 20.0)),
			),
			1,
		}, {
			"MultiTimeseries: round 3 - first instance adjusted based on round 1, second based on round 2",
			metricSlice(
				sumMetric(c1, k1v1k2v2, pdt3Ms, doublePoint(pdt3Ms, 88.0)),
				sumMetric(c1, k1v10k2v20, pdt3Ms, doublePoint(pdt3Ms, 49.0)),
			),
			metricSlice(
				sumMetric(c1, k1v1k2v2, pdt1Ms, doublePoint(pdt3Ms, 88.0)),
				sumMetric(c1, k1v10k2v20, pdt2Ms, doublePoint(pdt3Ms, 49.0)),
			),
			0,
		}, {
			"MultiTimeseries: round 4 - first instance reset, second instance adjusted based on round 2, initial third instance",
			metricSlice(
				sumMetric(c1, k1v1k2v2, pdt4Ms, doublePoint(pdt4Ms, 87.0)),
				sumMetric(c1, k1v10k2v20, pdt4Ms, doublePoint(pdt4Ms, 57.0)),
				sumMetric(c1, k1v100k2v200, pdt4Ms, doublePoint(pdt4Ms, 10.0)),
			),
			metricSlice(
				sumMetric(c1, k1v1k2v2, pdt4Ms, doublePoint(pdt4Ms, 87.0)),
				sumMetric(c1, k1v10k2v20, pdt2Ms, doublePoint(pdt4Ms, 57.0)),
				sumMetric(c1, k1v100k2v200, pdt4Ms, doublePoint(pdt4Ms, 10.0)),
			),
			2,
		}, {
			"MultiTimeseries: round 5 - first instance adjusted based on round 4, second on round 2, third on round 4",
			metricSlice(
				sumMetric(c1, k1v1k2v2, pdt5Ms, doublePoint(pdt5Ms, 90.0)),
				sumMetric(c1, k1v10k2v20, pdt5Ms, doublePoint(pdt5Ms, 65.0)),
				sumMetric(c1, k1v100k2v200, pdt5Ms, doublePoint(pdt5Ms, 22.0)),
			),
			metricSlice(
				sumMetric(c1, k1v1k2v2, pdt4Ms, doublePoint(pdt5Ms, 90.0)),
				sumMetric(c1, k1v10k2v20, pdt2Ms, doublePoint(pdt5Ms, 65.0)),
				sumMetric(c1, k1v100k2v200, pdt4Ms, doublePoint(pdt5Ms, 22.0)),
			),
			0,
		},
	}
	runScript(t, NewJobsMap(time.Minute).get("job", "0"), script)
}

var (
	emptyLabels              = []*kv{}
	k1vEmpty                 = []*kv{{"k1", ""}}
	k1vEmptyk2vEmptyk3vEmpty = []*kv{{"k1", ""}, {"k2", ""}, {"k3", ""}}
)

func Test_emptyLabels_pdata(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			"EmptyLabels: round 1 - initial instance, implicitly empty labels, start time is established",
			metricSlice(sumMetric(c1, emptyLabels, pdt1Ms, doublePoint(pdt1Ms, 44))),
			metricSlice(sumMetric(c1, emptyLabels, pdt1Ms, doublePoint(pdt1Ms, 44))),
			1,
		}, {
			"EmptyLabels: round 2 - instance adjusted based on round 1",
			metricSlice(sumMetric(c1, emptyLabels, pdt2Ms, doublePoint(pdt2Ms, 66))),
			metricSlice(sumMetric(c1, emptyLabels, pdt1Ms, doublePoint(pdt2Ms, 66))),
			0,
		}, {
			"EmptyLabels: round 3 - one explicitly empty label, instance adjusted based on round 1",
			metricSlice(sumMetric(c1, k1vEmpty, pdt3Ms, doublePoint(pdt3Ms, 77))),
			metricSlice(sumMetric(c1, k1vEmpty, pdt1Ms, doublePoint(pdt3Ms, 77))),
			0,
		}, {
			"EmptyLabels: round 4 - three explicitly empty labels, instance adjusted based on round 1",
			metricSlice(sumMetric(c1, k1vEmptyk2vEmptyk3vEmpty, pdt3Ms, doublePoint(pdt3Ms, 88))),
			metricSlice(sumMetric(c1, k1vEmptyk2vEmptyk3vEmpty, pdt1Ms, doublePoint(pdt3Ms, 88))),
			0,
		},
	}
	runScript(t, NewJobsMap(time.Minute).get("job", "0"), script)
}

func Test_tsGC_pdata(t *testing.T) {
	script1 := []*metricsAdjusterTest{
		{
			"TsGC: round 1 - initial instances, start time is established",
			metricSlice(
				sumMetric(c1, k1v1k2v2, pdt1Ms, doublePoint(pdt1Ms, 44)),
				sumMetric(c1, k1v10k2v20, pdt1Ms, doublePoint(pdt1Ms, 20)),
				cumulativeDistMetric(cd1, k1v1k2v2, pdt1Ms, distPoint(pdt1Ms, bounds0, []uint64{4, 2, 3, 7})),
				cumulativeDistMetric(cd1, k1v10k2v20, pdt1Ms, distPoint(pdt1Ms, bounds0, []uint64{40, 20, 30, 70})),
			),
			metricSlice(
				sumMetric(c1, k1v1k2v2, pdt1Ms, doublePoint(pdt1Ms, 44)),
				sumMetric(c1, k1v10k2v20, pdt1Ms, doublePoint(pdt1Ms, 20)),
				cumulativeDistMetric(cd1, k1v1k2v2, pdt1Ms, distPoint(pdt1Ms, bounds0, []uint64{4, 2, 3, 7})),
				cumulativeDistMetric(cd1, k1v10k2v20, pdt1Ms, distPoint(pdt1Ms, bounds0, []uint64{40, 20, 30, 70})),
			),
			4,
		},
	}

	script2 := []*metricsAdjusterTest{
		{
			"TsGC: round 2 - metrics first timeseries adjusted based on round 2, second timeseries not updated",
			metricSlice(
				sumMetric(c1, k1v1k2v2, pdt2Ms, doublePoint(pdt2Ms, 88)),
				cumulativeDistMetric(cd1, k1v1k2v2, pdt2Ms, distPoint(pdt2Ms, bounds0, []uint64{8, 7, 9, 14})),
			),
			metricSlice(
				sumMetric(c1, k1v1k2v2, pdt1Ms, doublePoint(pdt2Ms, 88)),
				cumulativeDistMetric(cd1, k1v1k2v2, pdt1Ms, distPoint(pdt2Ms, bounds0, []uint64{8, 7, 9, 14})),
			),
			0,
		},
	}

	script3 := []*metricsAdjusterTest{
		{
			"TsGC: round 3 - metrics first timeseries adjusted based on round 2, second timeseries empty due to timeseries gc()",
			metricSlice(
				sumMetric(c1, k1v1k2v2, pdt3Ms, doublePoint(pdt3Ms, 99)),
				sumMetric(c1, k1v10k2v20, pdt3Ms, doublePoint(pdt3Ms, 80)),
				cumulativeDistMetric(cd1, k1v1k2v2, pdt3Ms, distPoint(pdt3Ms, bounds0, []uint64{9, 8, 10, 15})),
				cumulativeDistMetric(cd1, k1v10k2v20, pdt3Ms, distPoint(pdt3Ms, bounds0, []uint64{55, 66, 33, 77})),
			),
			metricSlice(
				sumMetric(c1, k1v1k2v2, pdt1Ms, doublePoint(pdt3Ms, 99)),
				sumMetric(c1, k1v10k2v20, pdt3Ms, doublePoint(pdt3Ms, 80)),
				cumulativeDistMetric(cd1, k1v1k2v2, pdt1Ms, distPoint(pdt3Ms, bounds0, []uint64{9, 8, 10, 15})),
				cumulativeDistMetric(cd1, k1v10k2v20, pdt3Ms, distPoint(pdt3Ms, bounds0, []uint64{55, 66, 33, 77})),
			),
			2,
		},
	}

	jobsMap := NewJobsMap(time.Minute)

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

func Test_jobGC_pdata(t *testing.T) {
	job1Script1 := []*metricsAdjusterTest{
		{
			"JobGC: job 1, round 1 - initial instances, adjusted should be empty",
			metricSlice(
				sumMetric(c1, k1v1k2v2, pdt1Ms, doublePoint(pdt1Ms, 44)),
				sumMetric(c1, k1v10k2v20, pdt1Ms, doublePoint(pdt1Ms, 20)),
				cumulativeDistMetric(cd1, k1v1k2v2, pdt1Ms, distPoint(pdt1Ms, bounds0, []uint64{4, 2, 3, 7})),
				cumulativeDistMetric(cd1, k1v10k2v20, pdt1Ms, distPoint(pdt1Ms, bounds0, []uint64{40, 20, 30, 70})),
			),
			metricSlice(
				sumMetric(c1, k1v1k2v2, pdt1Ms, doublePoint(pdt1Ms, 44)),
				sumMetric(c1, k1v10k2v20, pdt1Ms, doublePoint(pdt1Ms, 20)),
				cumulativeDistMetric(cd1, k1v1k2v2, pdt1Ms, distPoint(pdt1Ms, bounds0, []uint64{4, 2, 3, 7})),
				cumulativeDistMetric(cd1, k1v10k2v20, pdt1Ms, distPoint(pdt1Ms, bounds0, []uint64{40, 20, 30, 70})),
			),
			4,
		},
	}

	emptyMetricSlice := func() *pmetric.MetricSlice { ms := pmetric.NewMetricSlice(); return &ms }
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
				sumMetric(c1, k1v1k2v2, pdt4Ms, doublePoint(pdt4Ms, 99)),
				sumMetric(c1, k1v10k2v20, pdt4Ms, doublePoint(pdt4Ms, 80)),
				cumulativeDistMetric(cd1, k1v1k2v2, pdt4Ms, distPoint(pdt4Ms, bounds0, []uint64{9, 8, 10, 15})),
				cumulativeDistMetric(cd1, k1v10k2v20, pdt4Ms, distPoint(pdt4Ms, bounds0, []uint64{55, 66, 33, 77})),
			),
			metricSlice(
				sumMetric(c1, k1v1k2v2, pdt4Ms, doublePoint(pdt4Ms, 99)),
				sumMetric(c1, k1v10k2v20, pdt4Ms, doublePoint(pdt4Ms, 80)),
				cumulativeDistMetric(cd1, k1v1k2v2, pdt4Ms, distPoint(pdt4Ms, bounds0, []uint64{9, 8, 10, 15})),
				cumulativeDistMetric(cd1, k1v10k2v20, pdt4Ms, distPoint(pdt4Ms, bounds0, []uint64{55, 66, 33, 77})),
			),
			4,
		},
	}

	gcInterval := 10 * time.Millisecond
	jobsMap := NewJobsMap(gcInterval)

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
}

type metricsAdjusterTest struct {
	description string
	metrics     *pmetric.MetricSlice
	adjusted    *pmetric.MetricSlice
	resets      int
}

func runScript(t *testing.T, tsm *timeseriesMap, script []*metricsAdjusterTest) {
	l := zap.NewNop()
	t.Cleanup(func() { require.NoError(t, l.Sync()) }) // flushes buffer, if any
	ma := NewMetricsAdjuster(tsm, l)

	for _, test := range script {
		expectedResets := test.resets
		resets := ma.AdjustMetricSlice(test.metrics)
		adjusted := test.metrics
		assert.EqualValuesf(t, test.adjusted, adjusted, "Test: %v - expected: %v, actual: %v", test.description, test.adjusted, adjusted)
		assert.Equalf(t, expectedResets, resets, "Test: %v", test.description)
	}
}
