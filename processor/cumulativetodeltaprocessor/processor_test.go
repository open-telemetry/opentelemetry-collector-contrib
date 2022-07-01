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

package cumulativetodeltaprocessor

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
)

type testMetric struct {
	metricNames  []string
	metricValues [][]float64
	isCumulative []bool
}

type cumulativeToDeltaTest struct {
	name       string
	metrics    []string
	include    MatchMetrics
	exclude    MatchMetrics
	inMetrics  pmetric.Metrics
	outMetrics pmetric.Metrics
}

var (
	testCases = []cumulativeToDeltaTest{
		{
			name:    "legacy_cumulative_to_delta_one_positive",
			metrics: []string{"metric_1"},
			inMetrics: generateTestMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100, 200, 500}, {4}},
				isCumulative: []bool{true, true},
			}),
			outMetrics: generateTestMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100, 100, 300}, {4}},
				isCumulative: []bool{false, true},
			}),
		},
		{
			name:    "legacy_cumulative_to_delta_nan_value",
			metrics: []string{"metric_1"},
			inMetrics: generateTestMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100, 200, math.NaN()}, {4}},
				isCumulative: []bool{true, true},
			}),
			outMetrics: generateTestMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100, 100, math.NaN()}, {4}},
				isCumulative: []bool{false, true},
			}),
		},
		{
			name:    "cumulative_to_delta_convert_nothing",
			metrics: nil,
			exclude: MatchMetrics{
				Metrics: []string{".*"},
				Config: filterset.Config{
					MatchType:    "regexp",
					RegexpConfig: nil,
				},
			},
			inMetrics: generateTestMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100}, {4}},
				isCumulative: []bool{true, true},
			}),
			outMetrics: generateTestMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100}, {4}},
				isCumulative: []bool{true, true},
			}),
		},
		{
			name: "cumulative_to_delta_one_positive",
			include: MatchMetrics{
				Metrics: []string{"metric_1"},
				Config: filterset.Config{
					MatchType:    "strict",
					RegexpConfig: nil,
				},
			},
			inMetrics: generateTestMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100, 200, 500}, {4}},
				isCumulative: []bool{true, true},
			}),
			outMetrics: generateTestMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100, 100, 300}, {4}},
				isCumulative: []bool{false, true},
			}),
		},
		{
			name: "cumulative_to_delta_nan_value",
			include: MatchMetrics{
				Metrics: []string{"_1"},
				Config: filterset.Config{
					MatchType:    "regexp",
					RegexpConfig: nil,
				},
			},
			inMetrics: generateTestMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100, 200, math.NaN()}, {4}},
				isCumulative: []bool{true, true},
			}),
			outMetrics: generateTestMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100, 100, math.NaN()}, {4}},
				isCumulative: []bool{false, true},
			}),
		},
		{
			name:    "cumulative_to_delta_exclude_precedence",
			metrics: nil,
			include: MatchMetrics{
				Metrics: []string{".*"},
				Config: filterset.Config{
					MatchType:    "regexp",
					RegexpConfig: nil,
				},
			},
			exclude: MatchMetrics{
				Metrics: []string{".*"},
				Config: filterset.Config{
					MatchType:    "regexp",
					RegexpConfig: nil,
				},
			},
			inMetrics: generateTestMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100}, {4}},
				isCumulative: []bool{true, true},
			}),
			outMetrics: generateTestMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100}, {4}},
				isCumulative: []bool{true, true},
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
				Include:           test.include,
				Exclude:           test.exclude,
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

			expectedMetrics := test.outMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
			actualMetrics := got[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()

			require.Equal(t, expectedMetrics.Len(), actualMetrics.Len())

			for i := 0; i < expectedMetrics.Len(); i++ {
				eM := expectedMetrics.At(i)
				aM := actualMetrics.At(i)

				require.Equal(t, eM.Name(), aM.Name())

				if eM.DataType() == pmetric.MetricDataTypeGauge {
					eDataPoints := eM.Gauge().DataPoints()
					aDataPoints := aM.Gauge().DataPoints()
					require.Equal(t, eDataPoints.Len(), aDataPoints.Len())

					for j := 0; j < eDataPoints.Len(); j++ {
						require.Equal(t, eDataPoints.At(j).DoubleVal(), aDataPoints.At(j).DoubleVal())
					}
				}

				if eM.DataType() == pmetric.MetricDataTypeSum {
					eDataPoints := eM.Sum().DataPoints()
					aDataPoints := aM.Sum().DataPoints()

					require.Equal(t, eDataPoints.Len(), aDataPoints.Len())
					require.Equal(t, eM.Sum().AggregationTemporality(), aM.Sum().AggregationTemporality())

					for j := 0; j < eDataPoints.Len(); j++ {
						if math.IsNaN(eDataPoints.At(j).DoubleVal()) {
							assert.True(t, math.IsNaN(aDataPoints.At(j).DoubleVal()))
						} else {
							require.Equal(t, eDataPoints.At(j).DoubleVal(), aDataPoints.At(j).DoubleVal())
						}
					}
				}

			}

			require.NoError(t, mgp.Shutdown(ctx))
		})
	}
}

func generateTestMetrics(tm testMetric) pmetric.Metrics {
	md := pmetric.NewMetrics()
	now := time.Now()

	rm := md.ResourceMetrics().AppendEmpty()
	ms := rm.ScopeMetrics().AppendEmpty().Metrics()
	for i, name := range tm.metricNames {
		m := ms.AppendEmpty()
		m.SetName(name)
		m.SetDataType(pmetric.MetricDataTypeSum)

		sum := m.Sum()
		sum.SetIsMonotonic(true)

		if tm.isCumulative[i] {
			sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
		} else {
			sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
		}

		for _, value := range tm.metricValues[i] {
			dp := m.Sum().DataPoints().AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(now.Add(10 * time.Second)))
			dp.SetDoubleVal(value)
		}
	}

	return md
}

func BenchmarkConsumeMetrics(b *testing.B) {
	c := consumertest.NewNop()
	params := component.ProcessorCreateSettings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
		BuildInfo: component.BuildInfo{},
	}
	cfg := createDefaultConfig().(*Config)
	cfg.Metrics = []string{""}
	p, err := createMetricsProcessor(context.Background(), params, cfg, c)
	if err != nil {
		b.Fatal(err)
	}

	metrics := pmetric.NewMetrics()
	rms := metrics.ResourceMetrics().AppendEmpty()
	r := rms.Resource()
	r.Attributes().Insert("resource", pcommon.NewValueBool(true))
	ilms := rms.ScopeMetrics().AppendEmpty()
	ilms.Scope().SetName("test")
	ilms.Scope().SetVersion("0.1")
	m := ilms.Metrics().AppendEmpty()
	m.SetDataType(pmetric.MetricDataTypeSum)
	m.Sum().SetIsMonotonic(true)
	dp := m.Sum().DataPoints().AppendEmpty()
	dp.Attributes().Insert("tag", pcommon.NewValueString("value"))

	reset := func() {
		m.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
		dp.SetDoubleVal(100.0)
	}

	// Load initial value
	reset()
	assert.NoError(b, p.ConsumeMetrics(context.Background(), metrics))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reset()
		assert.NoError(b, p.ConsumeMetrics(context.Background(), metrics))
	}
}
