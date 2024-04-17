// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package intervalprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/intervalprocessor"

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestAggregation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		inputs  []testMetric
		next    []testMetric
		outputs []testMetric
	}{
		{
			name: "BasicAggregation",
			inputs: []testMetric{
				{
					Name:        "test_metric_total",
					Type:        pmetric.MetricTypeSum,
					IsMonotonic: true,
					Temporality: pmetric.AggregationTemporalityCumulative,
					DataPoints: []any{
						testNumberDataPoint{
							Timestamp: 50,
							Value:     333,
							Attributes: map[string]any{
								"aaa": "bbb",
							},
						},
						testNumberDataPoint{
							Timestamp: 20,
							Value:     222,
							Attributes: map[string]any{
								"aaa": "bbb",
							},
						},
						testNumberDataPoint{
							Timestamp: 80,
							Value:     444,
							Attributes: map[string]any{
								"aaa": "bbb",
							},
						},
					},
				},
			},
			next: []testMetric{},
			outputs: []testMetric{
				{
					Name:        "test_metric_total",
					Type:        pmetric.MetricTypeSum,
					IsMonotonic: true,
					Temporality: pmetric.AggregationTemporalityCumulative,
					DataPoints: []any{
						testNumberDataPoint{
							Timestamp: 80,
							Value:     444,
							Attributes: map[string]any{
								"aaa": "bbb",
							},
						},
					},
				},
			},
		},
		{
			name: "NonMonotonicSumsArePassedThrough",
			inputs: []testMetric{
				{
					Name:        "test_metric_total",
					Type:        pmetric.MetricTypeSum,
					IsMonotonic: false,
					Temporality: pmetric.AggregationTemporalityCumulative,
					DataPoints: []any{
						testNumberDataPoint{
							Timestamp: 50,
							Value:     333,
							Attributes: map[string]any{
								"aaa": "bbb",
							},
						},
						testNumberDataPoint{
							Timestamp: 20,
							Value:     222,
							Attributes: map[string]any{
								"aaa": "bbb",
							},
						},
						testNumberDataPoint{
							Timestamp: 80,
							Value:     111,
							Attributes: map[string]any{
								"aaa": "bbb",
							},
						},
					},
				},
			},
			next: []testMetric{
				{
					Name:        "test_metric_total",
					Type:        pmetric.MetricTypeSum,
					IsMonotonic: false,
					Temporality: pmetric.AggregationTemporalityCumulative,
					DataPoints: []any{
						testNumberDataPoint{
							Timestamp: 50,
							Value:     333,
							Attributes: map[string]any{
								"aaa": "bbb",
							},
						},
						testNumberDataPoint{
							Timestamp: 20,
							Value:     222,
							Attributes: map[string]any{
								"aaa": "bbb",
							},
						},
						testNumberDataPoint{
							Timestamp: 80,
							Value:     111,
							Attributes: map[string]any{
								"aaa": "bbb",
							},
						},
					},
				},
			},
			outputs: []testMetric{},
		},
		{
			name: "GaugesArePassedThrough",
			inputs: []testMetric{
				{
					Name: "test_metric_value",
					Type: pmetric.MetricTypeGauge,
					DataPoints: []any{
						testNumberDataPoint{
							Timestamp: 50,
							Value:     345,
							Attributes: map[string]any{
								"aaa": "bbb",
							},
						},
						testNumberDataPoint{
							Timestamp: 20,
							Value:     258,
							Attributes: map[string]any{
								"aaa": "bbb",
							},
						},
						testNumberDataPoint{
							Timestamp: 80,
							Value:     178,
							Attributes: map[string]any{
								"aaa": "bbb",
							},
						},
					},
				},
			},
			next: []testMetric{
				{
					Name: "test_metric_value",
					Type: pmetric.MetricTypeGauge,
					DataPoints: []any{
						testNumberDataPoint{
							Timestamp: 50,
							Value:     345,
							Attributes: map[string]any{
								"aaa": "bbb",
							},
						},
						testNumberDataPoint{
							Timestamp: 20,
							Value:     258,
							Attributes: map[string]any{
								"aaa": "bbb",
							},
						},
						testNumberDataPoint{
							Timestamp: 80,
							Value:     178,
							Attributes: map[string]any{
								"aaa": "bbb",
							},
						},
					},
				},
			},
			outputs: []testMetric{},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config := &Config{Interval: time.Second}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// next stores the results of the filter metric processor
			next := &consumertest.MetricsSink{}

			factory := NewFactory()
			mgp, err := factory.CreateMetricsProcessor(
				context.Background(),
				processortest.NewNopCreateSettings(),
				config,
				next,
			)
			require.NoError(t, err)

			md := generateTestSumMetrics(t, testMetrics{
				{
					SchemaURL: "https://test-res-schema.com/schema",
					Attributes: map[string]any{
						"asdf": "foo",
					},
					ScopeMetrics: []testScopeMetrics{
						{
							Name:    "MyTestInstrument",
							Version: "1.2.3",
							Attributes: map[string]any{
								"foo": "bar",
							},
							Metrics: tc.inputs,
						},
					},
				},
			})

			// Test that ConsumeMetrics works
			err = mgp.ConsumeMetrics(ctx, md)
			require.NoError(t, err)

			require.IsType(t, &Processor{}, mgp)
			processor := mgp.(*Processor)

			// Pretend we hit the interval timer and call export
			processor.exportMetrics()

			// Processor should now be empty
			require.Equal(t, 0, len(processor.numbers))
			require.Equal(t, 0, len(processor.histograms))
			require.Equal(t, 0, len(processor.expHistograms))

			// Exporting again should return nothing
			processor.exportMetrics()

			// Next should have gotten three data sets:
			// 1. Anything left over from ConsumeMetrics()
			// 2. Anything exported from exportMetrics()
			// 3. An empty entry for the second call to exportMetrics()
			allMetrics := next.AllMetrics()
			require.Len(t, allMetrics, 3)

			nextData := convertMetricsToTestData(t, allMetrics[0])
			exportData := convertMetricsToTestData(t, allMetrics[1])
			secondExportData := convertMetricsToTestData(t, allMetrics[2])
			require.Empty(t, secondExportData)

			expectedNextData := testMetrics{
				{
					SchemaURL: "https://test-res-schema.com/schema",
					Attributes: map[string]any{
						"asdf": "foo",
					},
					ScopeMetrics: []testScopeMetrics{
						{
							Name:    "MyTestInstrument",
							Version: "1.2.3",
							Attributes: map[string]any{
								"foo": "bar",
							},
							Metrics: tc.next,
						},
					},
				},
			}
			if len(tc.next) == 0 {
				expectedNextData = testMetrics{}
			}

			expectedExportData := testMetrics{
				{
					SchemaURL: "https://test-res-schema.com/schema",
					Attributes: map[string]any{
						"asdf": "foo",
					},
					ScopeMetrics: []testScopeMetrics{
						{
							Name:    "MyTestInstrument",
							Version: "1.2.3",
							Attributes: map[string]any{
								"foo": "bar",
							},
							Metrics: tc.outputs,
						},
					},
				},
			}
			if len(tc.outputs) == 0 {
				expectedExportData = testMetrics{}
			}

			require.Equal(t, expectedNextData, nextData)
			require.Equal(t, expectedExportData, exportData)
		})
	}
}

type testMetrics []testResourceMetrics

type testResourceMetrics struct {
	SchemaURL  string
	Attributes map[string]any

	ScopeMetrics []testScopeMetrics
}

type testScopeMetrics struct {
	Name       string
	Version    string
	SchemaURL  string
	Attributes map[string]any

	Metrics []testMetric
}

type testMetric struct {
	Name        string
	Type        pmetric.MetricType
	IsMonotonic bool
	Temporality pmetric.AggregationTemporality

	DataPoints []any
}

type testNumberDataPoint struct {
	StartTimestamp pcommon.Timestamp
	Timestamp      pcommon.Timestamp

	Value      float64
	Attributes map[string]any
}

type testSummaryDataPoint struct {
	StartTimestamp pcommon.Timestamp
	Timestamp      pcommon.Timestamp

	QuantileValues []testValueAtQuantile
	Attributes     map[string]any
}

type testValueAtQuantile struct {
	Value    float64
	Quantile float64
}

type testHistogramDataPoint struct {
	StartTimestamp pcommon.Timestamp
	Timestamp      pcommon.Timestamp

	ExplicitBounds []float64
	BucketCounts   []uint64
	Attributes     map[string]any
}

func generateTestSumMetrics(t *testing.T, tmd testMetrics) pmetric.Metrics {
	md := pmetric.NewMetrics()

	for _, trm := range tmd {
		rm := md.ResourceMetrics().AppendEmpty()
		err := rm.Resource().Attributes().FromRaw(trm.Attributes)
		require.NoError(t, err)
		rm.SetSchemaUrl(trm.SchemaURL)

		for _, tsm := range trm.ScopeMetrics {
			sm := rm.ScopeMetrics().AppendEmpty()
			scope := pcommon.NewInstrumentationScope()
			scope.SetName(tsm.Name)
			scope.SetVersion(tsm.Version)
			err = scope.Attributes().FromRaw(tsm.Attributes)
			require.NoError(t, err)
			scope.MoveTo(sm.Scope())
			sm.SetSchemaUrl(tsm.SchemaURL)

			for _, tm := range tsm.Metrics {
				m := sm.Metrics().AppendEmpty()
				m.SetName(tm.Name)

				switch tm.Type {
				case pmetric.MetricTypeSum:
					sum := m.SetEmptySum()
					sum.SetIsMonotonic(tm.IsMonotonic)
					sum.SetAggregationTemporality(tm.Temporality)

					for _, tdp := range tm.DataPoints {
						require.IsType(t, testNumberDataPoint{}, tdp)
						tdpp := tdp.(testNumberDataPoint)

						dp := sum.DataPoints().AppendEmpty()

						dp.SetStartTimestamp(tdpp.StartTimestamp)
						dp.SetTimestamp(tdpp.Timestamp)

						dp.SetDoubleValue(tdpp.Value)

						err = dp.Attributes().FromRaw(tdpp.Attributes)
						require.NoError(t, err)
					}
				case pmetric.MetricTypeGauge:
					gauge := m.SetEmptyGauge()

					for _, tdp := range tm.DataPoints {
						require.IsType(t, testNumberDataPoint{}, tdp)
						tdpp := tdp.(testNumberDataPoint)

						dp := gauge.DataPoints().AppendEmpty()

						dp.SetStartTimestamp(tdpp.StartTimestamp)
						dp.SetTimestamp(tdpp.Timestamp)

						dp.SetDoubleValue(tdpp.Value)

						err = dp.Attributes().FromRaw(tdpp.Attributes)
						require.NoError(t, err)
					}
				case pmetric.MetricTypeSummary:
					summary := m.SetEmptySummary()

					for _, tdp := range tm.DataPoints {
						require.IsType(t, testSummaryDataPoint{}, tdp)
						tdpp := tdp.(testSummaryDataPoint)

						dp := summary.DataPoints().AppendEmpty()

						dp.SetStartTimestamp(tdpp.StartTimestamp)
						dp.SetTimestamp(tdpp.Timestamp)

						for _, valueAtQuantile := range tdpp.QuantileValues {
							qv := dp.QuantileValues().AppendEmpty()

							qv.SetQuantile(valueAtQuantile.Quantile)
							qv.SetValue(valueAtQuantile.Value)
						}

						err = dp.Attributes().FromRaw(tdpp.Attributes)
						require.NoError(t, err)
					}
				case pmetric.MetricTypeHistogram:
					histogram := m.SetEmptyHistogram()
					histogram.SetAggregationTemporality(tm.Temporality)

					for _, tdp := range tm.DataPoints {
						require.IsType(t, testHistogramDataPoint{}, tdp)
						tdpp := tdp.(testHistogramDataPoint)

						dp := histogram.DataPoints().AppendEmpty()

						dp.SetStartTimestamp(tdpp.StartTimestamp)
						dp.SetTimestamp(tdpp.Timestamp)

						bucketSum := uint64(0)
						for _, bucketCount := range tdpp.BucketCounts {
							bucketSum += bucketCount
						}
						dp.SetCount(bucketSum)
						dp.BucketCounts().FromRaw(tdpp.BucketCounts)
						dp.ExplicitBounds().FromRaw(tdpp.ExplicitBounds)

						err = dp.Attributes().FromRaw(tdpp.Attributes)
						require.NoError(t, err)
					}
				default:
					t.Fatalf("Unsupported metric type %v", m.Type())
				}
			}
		}
	}

	return md
}

func convertMetricsToTestData(t *testing.T, md pmetric.Metrics) testMetrics {
	tmd := testMetrics{}

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)

		trm := testResourceMetrics{
			SchemaURL:    rm.SchemaUrl(),
			Attributes:   rm.Resource().Attributes().AsRaw(),
			ScopeMetrics: []testScopeMetrics{},
		}

		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)

			tsm := testScopeMetrics{
				Name:       sm.Scope().Name(),
				Version:    sm.Scope().Version(),
				SchemaURL:  sm.SchemaUrl(),
				Attributes: sm.Scope().Attributes().AsRaw(),
				Metrics:    []testMetric{},
			}

			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)

				switch m.Type() {
				case pmetric.MetricTypeSum:
					sum := m.Sum()

					tm := testMetric{
						Name:        m.Name(),
						Type:        pmetric.MetricTypeSum,
						IsMonotonic: sum.IsMonotonic(),
						Temporality: sum.AggregationTemporality(),
						DataPoints:  []any{},
					}

					for r := 0; r < sum.DataPoints().Len(); r++ {
						dp := sum.DataPoints().At(r)

						tdp := testNumberDataPoint{
							StartTimestamp: dp.StartTimestamp(),
							Timestamp:      dp.Timestamp(),
							Value:          dp.DoubleValue(),
							Attributes:     dp.Attributes().AsRaw(),
						}

						tm.DataPoints = append(tm.DataPoints, tdp)
					}

					tsm.Metrics = append(tsm.Metrics, tm)
				case pmetric.MetricTypeGauge:
					gauge := m.Gauge()

					tm := testMetric{
						Name:       m.Name(),
						Type:       pmetric.MetricTypeGauge,
						DataPoints: []any{},
					}

					for r := 0; r < gauge.DataPoints().Len(); r++ {
						dp := gauge.DataPoints().At(r)

						tdp := testNumberDataPoint{
							StartTimestamp: dp.StartTimestamp(),
							Timestamp:      dp.Timestamp(),
							Value:          dp.DoubleValue(),
							Attributes:     dp.Attributes().AsRaw(),
						}

						tm.DataPoints = append(tm.DataPoints, tdp)
					}

					tsm.Metrics = append(tsm.Metrics, tm)
				case pmetric.MetricTypeSummary:
					summary := m.Summary()

					tm := testMetric{
						Name:       m.Name(),
						Type:       pmetric.MetricTypeSummary,
						DataPoints: []any{},
					}

					for r := 0; r < summary.DataPoints().Len(); r++ {
						dp := summary.DataPoints().At(r)

						tdp := testSummaryDataPoint{
							StartTimestamp: dp.StartTimestamp(),
							Timestamp:      dp.Timestamp(),
							QuantileValues: []testValueAtQuantile{},
							Attributes:     dp.Attributes().AsRaw(),
						}

						for s := 0; s < dp.QuantileValues().Len(); s++ {
							qv := dp.QuantileValues().At(s)

							tqv := testValueAtQuantile{
								Value:    qv.Value(),
								Quantile: qv.Quantile(),
							}

							tdp.QuantileValues = append(tdp.QuantileValues, tqv)
						}

						tm.DataPoints = append(tm.DataPoints, tdp)
					}

					tsm.Metrics = append(tsm.Metrics, tm)
				case pmetric.MetricTypeHistogram:
					histogram := m.Histogram()

					tm := testMetric{
						Name:        m.Name(),
						Type:        pmetric.MetricTypeHistogram,
						Temporality: histogram.AggregationTemporality(),
						DataPoints:  []any{},
					}

					for r := 0; r < histogram.DataPoints().Len(); r++ {
						dp := histogram.DataPoints().At(r)

						tdp := testHistogramDataPoint{
							StartTimestamp: dp.StartTimestamp(),
							Timestamp:      dp.Timestamp(),
							ExplicitBounds: dp.ExplicitBounds().AsRaw(),
							BucketCounts:   dp.BucketCounts().AsRaw(),
							Attributes:     dp.Attributes().AsRaw(),
						}

						tm.DataPoints = append(tm.DataPoints, tdp)
					}

					tsm.Metrics = append(tsm.Metrics, tm)
				default:
					t.Fatalf("Invalid metric type %v", m.Type())
				}
			}

			trm.ScopeMetrics = append(trm.ScopeMetrics, tsm)
		}

		tmd = append(tmd, trm)
	}

	return tmd
}
