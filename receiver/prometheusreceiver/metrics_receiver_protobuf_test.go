// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"math"
	"testing"

	dto "github.com/prometheus/prometheus/prompb/io/prometheus/client"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestScrapeViaProtobuf(t *testing.T) {
	mf := &dto.MetricFamily{
		Name: "test_counter",
		Type: dto.MetricType_COUNTER,
		Metric: []dto.Metric{
			{
				Label: []dto.LabelPair{
					{
						Name:  "foo",
						Value: "bar",
					},
				},
				Counter: &dto.Counter{
					Value: 1234,
				},
			},
		},
	}
	buffer := prometheusMetricFamilyToProtoBuf(t, nil, mf)

	mf = &dto.MetricFamily{
		Name: "test_gauge",
		Type: dto.MetricType_GAUGE,
		Metric: []dto.Metric{
			{
				Gauge: &dto.Gauge{
					Value: 400.8,
				},
			},
		},
	}
	prometheusMetricFamilyToProtoBuf(t, buffer, mf)

	mf = &dto.MetricFamily{
		Name: "test_summary",
		Type: dto.MetricType_SUMMARY,
		Metric: []dto.Metric{
			{
				Summary: &dto.Summary{
					SampleCount: 1213,
					SampleSum:   456,
					Quantile: []dto.Quantile{
						{
							Quantile: 0.5,
							Value:    789,
						},
						{
							Quantile: 0.9,
							Value:    1011,
						},
					},
				},
			},
		},
	}
	prometheusMetricFamilyToProtoBuf(t, buffer, mf)

	mf = &dto.MetricFamily{
		Name: "test_histogram",
		Type: dto.MetricType_HISTOGRAM,
		Metric: []dto.Metric{
			{
				Histogram: &dto.Histogram{
					SampleCount: 1213,
					SampleSum:   456,
					Bucket: []dto.Bucket{
						{
							UpperBound:      0.5,
							CumulativeCount: 789,
						},
						{
							UpperBound:      10,
							CumulativeCount: 1011,
						},
						{
							UpperBound:      math.Inf(1),
							CumulativeCount: 1213,
						},
					},
				},
			},
		},
	}
	prometheusMetricFamilyToProtoBuf(t, buffer, mf)

	expectations := []testExpectation{
		assertMetricPresent(
			"test_counter",
			compareMetricType(pmetric.MetricTypeSum),
			compareMetricUnit(""),
			[]dataPointExpectation{{
				numberPointComparator: []numberPointComparator{
					compareDoubleValue(1234),
				},
			}},
		),
		assertMetricPresent(
			"test_gauge",
			compareMetricType(pmetric.MetricTypeGauge),
			compareMetricUnit(""),
			[]dataPointExpectation{{
				numberPointComparator: []numberPointComparator{
					compareDoubleValue(400.8),
				},
			}},
		),
		assertMetricPresent(
			"test_summary",
			compareMetricType(pmetric.MetricTypeSummary),
			compareMetricUnit(""),
			[]dataPointExpectation{{
				summaryPointComparator: []summaryPointComparator{
					compareSummary(1213, 456, [][]float64{{0.5, 789}, {0.9, 1011}}),
				},
			}},
		),
		assertMetricPresent(
			"test_histogram",
			compareMetricType(pmetric.MetricTypeHistogram),
			compareMetricUnit(""),
			[]dataPointExpectation{{
				histogramPointComparator: []histogramPointComparator{
					compareHistogram(1213, 456, []float64{0.5, 10}, []uint64{789, 222, 202}),
				},
			}},
		),
	}

	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, useProtoBuf: true, buf: buffer.Bytes()},
			},
			validateFunc: func(t *testing.T, td *testData, result []pmetric.ResourceMetrics) {
				verifyNumValidScrapeResults(t, td, result)
				doCompare(t, "target1", td.attributes, result[0], expectations)
			},
		},
	}

	testComponent(t, targets, func(c *Config) {
		c.EnableProtobufNegotiation = true
	})
}
