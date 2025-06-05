// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite

import (
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
)

func BenchmarkFromMetrics(b *testing.B) {
	for _, resourceAttributeCount := range []int{0, 5, 50} {
		b.Run(fmt.Sprintf("resource attribute count: %v", resourceAttributeCount), func(b *testing.B) {
			for _, histogramCount := range []int{0, 1000} {
				b.Run(fmt.Sprintf("histogram count: %v", histogramCount), func(b *testing.B) {
					nonHistogramCounts := []int{0, 1000}

					if resourceAttributeCount == 0 && histogramCount == 0 {
						// Don't bother running a scenario where we'll generate no series.
						nonHistogramCounts = []int{1000}
					}

					for _, nonHistogramCount := range nonHistogramCounts {
						b.Run(fmt.Sprintf("non-histogram count: %v", nonHistogramCount), func(b *testing.B) {
							for _, labelsPerMetric := range []int{2, 20} {
								b.Run(fmt.Sprintf("labels per metric: %v", labelsPerMetric), func(b *testing.B) {
									for _, exemplarsPerSeries := range []int{0, 5, 10} {
										b.Run(fmt.Sprintf("exemplars per series: %v", exemplarsPerSeries), func(b *testing.B) {
											payload := createExportRequest(resourceAttributeCount, histogramCount, nonHistogramCount, labelsPerMetric, exemplarsPerSeries, pcommon.Timestamp(uint64(time.Now().UnixNano())))

											for i := 0; i < b.N; i++ {
												tsMap, err := FromMetrics(payload.Metrics(), Settings{})
												require.NoError(b, err)
												require.NotNil(b, tsMap)
											}
										})
									}
								})
							}
						})
					}
				})
			}
		})
	}
}

func BenchmarkPrometheusConverter_FromMetrics(b *testing.B) {
	for _, resourceAttributeCount := range []int{0, 5, 50} {
		b.Run(fmt.Sprintf("resource attribute count: %v", resourceAttributeCount), func(b *testing.B) {
			for _, histogramCount := range []int{0, 1000} {
				b.Run(fmt.Sprintf("histogram count: %v", histogramCount), func(b *testing.B) {
					nonHistogramCounts := []int{0, 1000}

					if resourceAttributeCount == 0 && histogramCount == 0 {
						// Don't bother running a scenario where we'll generate no series.
						nonHistogramCounts = []int{1000}
					}

					for _, nonHistogramCount := range nonHistogramCounts {
						b.Run(fmt.Sprintf("non-histogram count: %v", nonHistogramCount), func(b *testing.B) {
							for _, labelsPerMetric := range []int{2, 20} {
								b.Run(fmt.Sprintf("labels per metric: %v", labelsPerMetric), func(b *testing.B) {
									for _, exemplarsPerSeries := range []int{0, 5, 10} {
										b.Run(fmt.Sprintf("exemplars per series: %v", exemplarsPerSeries), func(b *testing.B) {
											payload := createExportRequest(resourceAttributeCount, histogramCount, nonHistogramCount, labelsPerMetric, exemplarsPerSeries, pcommon.Timestamp(uint64(time.Now().UnixNano())))

											for i := 0; i < b.N; i++ {
												converter := newPrometheusConverter()
												require.NoError(b, converter.fromMetrics(payload.Metrics(), Settings{}))
												require.NotNil(b, converter.timeSeries())
											}
										})
									}
								})
							}
						})
					}
				})
			}
		})
	}
}

func createExportRequest(resourceAttributeCount int, histogramCount int, nonHistogramCount int, labelsPerMetric int, exemplarsPerSeries int, timestamp pcommon.Timestamp) pmetricotlp.ExportRequest {
	request := pmetricotlp.NewExportRequest()

	rm := request.Metrics().ResourceMetrics().AppendEmpty()
	generateAttributes(rm.Resource().Attributes(), "resource", resourceAttributeCount)

	metrics := rm.ScopeMetrics().AppendEmpty().Metrics()

	for i := 1; i <= histogramCount; i++ {
		m := metrics.AppendEmpty()
		m.SetEmptyHistogram()
		m.SetName(fmt.Sprintf("histogram-%v", i))
		m.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		h := m.Histogram().DataPoints().AppendEmpty()
		h.SetTimestamp(timestamp)

		// Set 50 samples, 10 each with values 0.5, 1, 2, 4, and 8
		h.SetCount(50)
		h.SetSum(155)
		h.BucketCounts().FromRaw([]uint64{10, 10, 10, 10, 10, 0})
		h.ExplicitBounds().FromRaw([]float64{.5, 1, 2, 4, 8, 16}) // Bucket boundaries include the upper limit (ie. each sample is on the upper limit of its bucket)

		generateAttributes(h.Attributes(), "series", labelsPerMetric)
		generateExemplars(h.Exemplars(), exemplarsPerSeries, timestamp)
	}

	for i := 1; i <= nonHistogramCount; i++ {
		m := metrics.AppendEmpty()
		m.SetEmptySum()
		m.SetDescription("test sum description")
		m.SetUnit("By")
		m.SetName(fmt.Sprintf("sum-%v", i))
		m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		point := m.Sum().DataPoints().AppendEmpty()
		point.SetTimestamp(timestamp)
		point.SetDoubleValue(1.23)
		generateAttributes(point.Attributes(), "series", labelsPerMetric)
		generateExemplars(point.Exemplars(), exemplarsPerSeries, timestamp)
	}

	for i := 1; i <= nonHistogramCount; i++ {
		m := metrics.AppendEmpty()
		m.SetEmptyGauge()
		m.SetDescription("test gauge description")
		m.SetUnit("By")
		m.SetName(fmt.Sprintf("gauge-%v", i))
		point := m.Gauge().DataPoints().AppendEmpty()
		point.SetTimestamp(timestamp)
		point.SetDoubleValue(1.23)
		generateAttributes(point.Attributes(), "series", labelsPerMetric)
		generateExemplars(point.Exemplars(), exemplarsPerSeries, timestamp)
	}

	return request
}

func generateAttributes(m pcommon.Map, prefix string, count int) {
	for i := 1; i <= count; i++ {
		m.PutStr(fmt.Sprintf("%v-name-%v", prefix, i), fmt.Sprintf("value-%v", i))
	}
}

func generateExemplars(exemplars pmetric.ExemplarSlice, count int, ts pcommon.Timestamp) {
	for i := 1; i <= count; i++ {
		e := exemplars.AppendEmpty()
		e.SetTimestamp(ts)
		e.SetDoubleValue(2.22)
		e.SetSpanID(pcommon.SpanID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08})
		e.SetTraceID(pcommon.TraceID{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f})
	}
}

func TestIsSameMetric(t *testing.T) {
	tests := []struct {
		name   string
		labels func() []prompb.Label
		ts     func() *prompb.TimeSeries
		same   bool
	}{
		{
			name: "same",
			same: true,
			labels: func() []prompb.Label {
				return getPromLabels(label11, value11, label12, value12)
			},
			ts: func() *prompb.TimeSeries {
				return getTimeSeries(
					getPromLabels(label11, value11, label12, value12),
					getSample(float64(intVal1), msTime1),
				)
			},
		},
		{
			name: "not_same",
			same: false,
			labels: func() []prompb.Label {
				return getPromLabels(label11, value11, label12, value12)
			},
			ts: func() *prompb.TimeSeries {
				return getTimeSeries(
					getPromLabels(label11, value11, label21, value21),
					getSample(float64(intVal1), msTime1),
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			same := isSameMetric(tt.ts(), tt.labels())
			assert.Equal(t, tt.same, same)
		})
	}
}
