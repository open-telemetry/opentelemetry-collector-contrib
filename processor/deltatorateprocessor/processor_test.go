// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package deltatorateprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/model/pdata"
)

type testMetric struct {
	metricNames  []string
	metricValues [][]float64
	isDelta      []bool
	deltaSecond  int
}

type deltaToRateTest struct {
	name       string
	metrics    []string
	inMetrics  pdata.Metrics
	outMetrics pdata.Metrics
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
	}
)

func TestCumulativeToDeltaProcessor(t *testing.T) {
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			// next stores the results of the filter metric processor
			next := new(consumertest.MetricsSink)
			cfg := &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Metrics:           test.metrics,
			}
			factory := NewFactory()
			mgp, err := factory.CreateMetricsProcessor(
				context.Background(),
				componenttest.NewNopProcessorCreateSettings(),
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

			expectedMetrics := test.outMetrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()
			actualMetrics := got[0].ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()

			require.Equal(t, expectedMetrics.Len(), actualMetrics.Len())

			for i := 0; i < expectedMetrics.Len(); i++ {
				eM := expectedMetrics.At(i)
				aM := actualMetrics.At(i)

				require.Equal(t, eM.Name(), aM.Name())

				if eM.DataType() == pdata.MetricDataTypeGauge {
					eDataPoints := eM.Gauge().DataPoints()
					aDataPoints := aM.Gauge().DataPoints()
					require.Equal(t, eDataPoints.Len(), aDataPoints.Len())

					for j := 0; j < eDataPoints.Len(); j++ {
						require.Equal(t, eDataPoints.At(j).DoubleVal(), aDataPoints.At(j).DoubleVal())
					}
				}

				if eM.DataType() == pdata.MetricDataTypeSum {
					eDataPoints := eM.Sum().DataPoints()
					aDataPoints := aM.Sum().DataPoints()

					require.Equal(t, eDataPoints.Len(), aDataPoints.Len())
					require.Equal(t, eM.Sum().AggregationTemporality(), aM.Sum().AggregationTemporality())

					for j := 0; j < eDataPoints.Len(); j++ {
						require.Equal(t, eDataPoints.At(j).DoubleVal(), aDataPoints.At(j).DoubleVal())
					}
				}

			}

			require.NoError(t, mgp.Shutdown(ctx))
		})
	}
}

func generateSumMetrics(tm testMetric) pdata.Metrics {
	md := pdata.NewMetrics()
	now := time.Now()
	delta := time.Duration(tm.deltaSecond)

	rm := md.ResourceMetrics().AppendEmpty()
	ms := rm.InstrumentationLibraryMetrics().AppendEmpty().Metrics()
	for i, name := range tm.metricNames {
		m := ms.AppendEmpty()
		m.SetName(name)
		m.SetDataType(pdata.MetricDataTypeSum)

		sum := m.Sum()
		sum.SetIsMonotonic(true)

		if tm.isDelta[i] {
			sum.SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
		} else {
			sum.SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
		}

		for _, value := range tm.metricValues[i] {
			dp := m.Sum().DataPoints().AppendEmpty()
			dp.SetStartTimestamp(pdata.NewTimestampFromTime(now))
			dp.SetTimestamp(pdata.NewTimestampFromTime(now.Add(delta * time.Second)))
			dp.SetDoubleVal(value)
		}
	}

	return md
}

func generateGaugeMetrics(tm testMetric) pdata.Metrics {
	md := pdata.NewMetrics()
	now := time.Now()

	rm := md.ResourceMetrics().AppendEmpty()
	ms := rm.InstrumentationLibraryMetrics().AppendEmpty().Metrics()
	for i, name := range tm.metricNames {
		m := ms.AppendEmpty()
		m.SetName(name)
		m.SetDataType(pdata.MetricDataTypeGauge)
		for _, value := range tm.metricValues[i] {
			dp := m.Gauge().DataPoints().AppendEmpty()
			dp.SetTimestamp(pdata.NewTimestampFromTime(now.Add(120 * time.Second)))
			dp.SetDoubleVal(value)
		}
	}

	return md
}
