// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatorateprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor/processortest"
)

type testMetric struct {
	metricNames     []string
	metricValues    [][]float64
	metricIntValues [][]int64
	isDelta         []bool
	deltaSecond     int
}

type deltaToRateTest struct {
	name       string
	metrics    []string
	inMetrics  pmetric.Metrics
	outMetrics pmetric.Metrics
}

var (
	testCases = []deltaToRateTest{
		{
			name:    "delta_to_rate_expect_same",
			metrics: nil,
			inMetrics: generateSumMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100}, {4}},
				isDelta:      []bool{true, true},
				deltaSecond:  120,
			}),
			outMetrics: generateSumMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100}, {4}},
				isDelta:      []bool{true, true},
				deltaSecond:  120,
			}),
		},
		{
			name:    "delta_to_rate_one_positive",
			metrics: []string{"metric_1", "metric_2"},
			inMetrics: generateSumMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{120, 240, 360}, {360}},
				isDelta:      []bool{true, true},
				deltaSecond:  120,
			}),
			outMetrics: generateGaugeMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{1, 2, 3}, {3}},
			}),
		},
		{
			name:    "delta_to_rate_with_cumulative",
			metrics: []string{"metric_1", "metric_2"},
			inMetrics: generateSumMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100}, {4}},
				isDelta:      []bool{false, false},
				deltaSecond:  120,
			}),
			outMetrics: generateSumMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100}, {4}},
				isDelta:      []bool{false, false},
				deltaSecond:  120,
			}),
		},
		{
			name:    "delta_to_rate_expect_zero",
			metrics: []string{"metric_1", "metric_2"},
			inMetrics: generateSumMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{120, 240, 360}, {360}},
				isDelta:      []bool{true, true},
				deltaSecond:  0,
			}),
			outMetrics: generateGaugeMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{0, 0, 0}, {0}},
			}),
		},
		{
			name:    "int64-delta_to_rate_one_positive",
			metrics: []string{"metric_1", "metric_2"},
			inMetrics: generateSumMetrics(testMetric{
				metricNames:     []string{"metric_1", "metric_2"},
				metricIntValues: [][]int64{{120, 240, 360}, {360}},
				isDelta:         []bool{true, true},
				deltaSecond:     120,
			}),
			outMetrics: generateGaugeMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{1, 2, 3}, {3}},
			}),
		},
	}
)

func TestCumulativeToDeltaProcessor(t *testing.T) {
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			// next stores the results of the filter metric processor
			next := new(consumertest.MetricsSink)
			cfg := &Config{
				Metrics: test.metrics,
			}
			factory := NewFactory()
			mgp, err := factory.CreateMetricsProcessor(
				context.Background(),
				processortest.NewNopCreateSettings(),
				cfg,
				next,
			)
			assert.NotNil(t, mgp)
			assert.Nil(t, err)

			caps := mgp.Capabilities()
			assert.True(t, caps.MutatesData)
			ctx := context.Background()
			require.NoError(t, mgp.Start(ctx, nil))

			cErr := mgp.ConsumeMetrics(context.Background(), test.inMetrics)
			assert.Nil(t, cErr)
			got := next.AllMetrics()

			require.Equal(t, 1, len(got))
			require.Equal(t, test.outMetrics.ResourceMetrics().Len(), got[0].ResourceMetrics().Len())

			expectedMetrics := test.outMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
			actualMetrics := got[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()

			require.Equal(t, expectedMetrics.Len(), actualMetrics.Len())

			for i := 0; i < expectedMetrics.Len(); i++ {
				eM := expectedMetrics.At(i)
				aM := actualMetrics.At(i)

				require.Equal(t, eM.Name(), aM.Name())

				if eM.Type() == pmetric.MetricTypeGauge {
					eDataPoints := eM.Gauge().DataPoints()
					aDataPoints := aM.Gauge().DataPoints()
					require.Equal(t, eDataPoints.Len(), aDataPoints.Len())

					for j := 0; j < eDataPoints.Len(); j++ {
						require.Equal(t, eDataPoints.At(j).DoubleValue(), aDataPoints.At(j).DoubleValue())
					}
				}

				if eM.Type() == pmetric.MetricTypeSum {
					eDataPoints := eM.Sum().DataPoints()
					aDataPoints := aM.Sum().DataPoints()

					require.Equal(t, eDataPoints.Len(), aDataPoints.Len())
					require.Equal(t, eM.Sum().AggregationTemporality(), aM.Sum().AggregationTemporality())

					for j := 0; j < eDataPoints.Len(); j++ {
						require.Equal(t, eDataPoints.At(j).DoubleValue(), aDataPoints.At(j).DoubleValue())
					}
				}

			}

			require.NoError(t, mgp.Shutdown(ctx))
		})
	}
}

func generateSumMetrics(tm testMetric) pmetric.Metrics {
	md := pmetric.NewMetrics()
	now := time.Now()
	delta := time.Duration(tm.deltaSecond)

	rm := md.ResourceMetrics().AppendEmpty()
	ms := rm.ScopeMetrics().AppendEmpty().Metrics()
	for i, name := range tm.metricNames {
		m := ms.AppendEmpty()
		m.SetName(name)
		sum := m.SetEmptySum()
		sum.SetIsMonotonic(true)

		if tm.isDelta[i] {
			sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		} else {
			sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		}

		if i < len(tm.metricValues) {
			for _, value := range tm.metricValues[i] {
				dp := m.Sum().DataPoints().AppendEmpty()
				dp.SetStartTimestamp(pcommon.NewTimestampFromTime(now))
				dp.SetTimestamp(pcommon.NewTimestampFromTime(now.Add(delta * time.Second)))
				dp.SetDoubleValue(value)
			}
		}
		if i < len(tm.metricIntValues) {
			for _, value := range tm.metricIntValues[i] {
				dp := m.Sum().DataPoints().AppendEmpty()
				dp.SetStartTimestamp(pcommon.NewTimestampFromTime(now))
				dp.SetTimestamp(pcommon.NewTimestampFromTime(now.Add(delta * time.Second)))
				dp.SetIntValue(value)
			}
		}
	}

	return md
}

func generateGaugeMetrics(tm testMetric) pmetric.Metrics {
	md := pmetric.NewMetrics()
	now := time.Now()

	rm := md.ResourceMetrics().AppendEmpty()
	ms := rm.ScopeMetrics().AppendEmpty().Metrics()
	for i, name := range tm.metricNames {
		m := ms.AppendEmpty()
		m.SetName(name)
		dps := m.SetEmptyGauge().DataPoints()
		if i < len(tm.metricValues) {
			for _, value := range tm.metricValues[i] {
				dp := dps.AppendEmpty()
				dp.SetTimestamp(pcommon.NewTimestampFromTime(now.Add(120 * time.Second)))
				dp.SetDoubleValue(value)
			}
		}
		if i < len(tm.metricIntValues) {
			for _, value := range tm.metricIntValues[i] {
				dp := dps.AppendEmpty()
				dp.SetTimestamp(pcommon.NewTimestampFromTime(now.Add(120 * time.Second)))
				dp.SetIntValue(value)
			}
		}
	}

	return md
}
