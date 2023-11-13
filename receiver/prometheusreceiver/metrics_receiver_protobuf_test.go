// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"testing"

	dto "github.com/prometheus/prometheus/prompb/io/prometheus/client"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestScrapeViaProtobuf(t *testing.T) {
	// Create a Prometheus metric families and encode them to protobuf.
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

	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, useProtoBuf: true, buf: buffer.Bytes()},
			},
			validateFunc: func(t *testing.T, td *testData, result []pmetric.ResourceMetrics) {
				verifyNumValidScrapeResults(t, td, result)
				assertMetricPresent(
					"test_counter",
					compareMetricType(pmetric.MetricTypeSum),
					compareMetricUnit(""),
					[]dataPointExpectation{{
						numberPointComparator: []numberPointComparator{
							compareDoubleValue(1234),
						},
					}},
				)
				assertMetricPresent(
					"test_gauge",
					compareMetricType(pmetric.MetricTypeGauge),
					compareMetricUnit(""),
					[]dataPointExpectation{{
						numberPointComparator: []numberPointComparator{
							compareDoubleValue(400.8),
						},
					}},
				)
			},
		},
	}

	testComponent(t, targets, func(c *Config) {
		c.EnableProtobufNegotiation = true
	})
}
