// Copyright 2020 OpenTelemetry Authors
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

package metricstransformprocessor

import (
	"context"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"testing"
)

// Toggle Data Type
// {
// 	name: "metric_toggle_scalar_data_type_int64_to_double",
// 	transforms: []Transform{
// 		{
// 			MetricName: "metric1",
// 			Action:     Update,
// 			Operations: []Operation{{Action: ToggleScalarDataType}},
// 		},
// 		{
// 			MetricName: "metric2",
// 			Action:     Update,
// 			Operations: []Operation{{Action: ToggleScalarDataType}},
// 		},
// 	},
// 	inMetrics: []*metricspb.MetricDescriptor{
// 		{Name: "metric1", Type: metricspb.MetricDescriptor_CUMULATIVE_INT64},
// 		{Name: "metric2", Type: metricspb.MetricDescriptor_GAUGE_INT64},
// 	},
// 	outMetrics: []*metricspb.MetricDescriptor{
// 		{Name: "metric1", Type: metricspb.MetricDescriptor_CUMULATIVE_DOUBLE},
// 		{Name: "metric2", Type: metricspb.MetricDescriptor_GAUGE_DOUBLE},
// 	},
// },
// {
// 	name: "metric_toggle_scalar_data_type_double_to_int64",
// 	transforms: []Transform{
// 		{
// 			MetricName: "metric1",
// 			Action:     Update,
// 			Operations: []Operation{{Action: ToggleScalarDataType}},
// 		},
// 		{
// 			MetricName: "metric2",
// 			Action:     Update,
// 			Operations: []Operation{{Action: ToggleScalarDataType}},
// 		},
// 	},
// 	inMetrics: []*metricspb.MetricDescriptor{
// 		{Name: "metric1", Type: metricspb.MetricDescriptor_CUMULATIVE_DOUBLE},
// 		{Name: "metric2", Type: metricspb.MetricDescriptor_GAUGE_DOUBLE},
// 	},
// 	outMetrics: []*metricspb.MetricDescriptor{
// 		{Name: "metric1", Type: metricspb.MetricDescriptor_CUMULATIVE_INT64},
// 		{Name: "metric2", Type: metricspb.MetricDescriptor_GAUGE_INT64},
// 	},
// },
// {
// 	name: "metric_toggle_scalar_data_type_no_effect",
// 	transforms: []Transform{
// 		{
// 			MetricName: "metric1",
// 			Action:     Update,
// 			Operations: []Operation{{Action: ToggleScalarDataType}},
// 		},
// 	},
// 	inMetrics:  []*metricspb.MetricDescriptor{{Name: "metric1", Type: metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION}},
// 	outMetrics: []*metricspb.MetricDescriptor{{Name: "metric1", Type: metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION}},
// },
// // Add Label to a metric
// {
// 	name: "update existing metric by adding a new label when there are no labels",
// 	transforms: []Transform{
// 		{
// 			MetricName: "metric1",
// 			Action:     Update,
// 			Operations: []Operation{
// 				{
// 					Action:   AddLabel,
// 					NewLabel: "foo",
// 					NewValue: "bar",
// 				},
// 			},
// 		},
// 	},
// 	inMetrics:  initialMetricNames,
// 	outMetrics: initialMetricNames,
// 	outLabels:  []labelKeyValue{{Key: "foo", Value: "bar"}},
// },
// {
// 	name: "update existing metric by adding a new label when there are labels",
// 	transforms: []Transform{
// 		{
// 			MetricName: "metric1",
// 			Action:     Update,
// 			Operations: []Operation{
// 				{
// 					Action:   AddLabel,
// 					NewLabel: "foo",
// 					NewValue: "bar",
// 				},
// 			},
// 		},
// 	},
// 	inMetrics:  initialMetricNames,
// 	outMetrics: initialMetricNames,
// 	inLabels:   initialLabels,
// 	outLabels: []labelKeyValue{
// 		{
// 			Key:   "label1",
// 			Value: "value1",
// 		},
// 		{
// 			Key:   "label2",
// 			Value: "value2",
// 		},
// 		{
// 			Key:   "foo",
// 			Value: "bar",
// 		},
// 	},
// },
// {
// 	name: "update existing metric by adding a label that is duplicated in the list",
// 	transforms: []Transform{
// 		{
// 			MetricName: "metric1",
// 			Action:     Update,
// 			Operations: []Operation{
// 				{
// 					Action:   AddLabel,
// 					NewLabel: "label1",
// 					NewValue: "value1",
// 				},
// 			},
// 		},
// 	},
// 	inMetrics:  initialMetricNames,
// 	outMetrics: initialMetricNames,
// 	inLabels:   initialLabels,
// 	outLabels: []labelKeyValue{
// 		{
// 			Key:   "label1",
// 			Value: "value1",
// 		},
// 		{
// 			Key:   "label2",
// 			Value: "value2",
// 		},
// 		{
// 			Key:   "label1",
// 			Value: "value1",
// 		},
// 	},
// },
// {
// 	name: "update does not happen because target metric doesn't exist",
// 	transforms: []Transform{
// 		{
// 			MetricName: "mymetric",
// 			Action:     Update,
// 			Operations: []Operation{
// 				{
// 					Action:   AddLabel,
// 					NewLabel: "foo",
// 					NewValue: "bar",
// 				},
// 			},
// 		},
// 	},
// 	inMetrics:  initialMetricNames,
// 	outMetrics: initialMetricNames,
// 	inLabels:   initialLabels,
// 	outLabels:  initialLabels,
// },

func TestMetricsTransformProcessor(t *testing.T) {
	for _, test := range standardTests {
		t.Run(test.name, func(t *testing.T) {
			next := &exportertest.SinkMetricsExporter{}
			// next stores the results of the aggregation metric processor.
			// next := &etest.SinkMetricsExporter{}

			mtp := newMetricsTransformProcessor(next, nil, test.transforms)
			assert.NotNil(t, mtp)

			assert.True(t, mtp.GetCapabilities().MutatesConsumedData)
			assert.NoError(t, mtp.Start(context.Background(), componenttest.NewNopHost()))
			defer func() { assert.NoError(t, mtp.Shutdown(context.Background())) }()

			inputMetrics := createTestMetrics(test.inMetrics, test.inLabels)
			assert.NoError(t, mtp.ConsumeMetrics(context.Background(), inputMetrics))

			// construct metrics data to feed into the processor
			md := consumerdata.MetricsData{Metrics: test.in}

			// process
			cErr := mtp.ConsumeMetrics(
				context.Background(),
				pdatautil.MetricsFromMetricsData([]consumerdata.MetricsData{
					md,
				}),
			)
			assert.NoError(t, cErr)

			// get and check results
			got := next.AllMetrics()
			require.Equal(t, 1, len(got))
			gotMD := pdatautil.MetricsToMetricsData(got[0])
			require.Equal(t, 1, len(gotMD))
			actualOut := gotMD[0].Metrics
			require.Equal(t, len(test.out), len(actualOut))

			for idx, out := range test.out {
				assert.Equal(t, *out, *actualOut[idx])
			}
		})
	}
}

// func validateLabels(t *testing.T, expectLabels []labelKeyValue, inMetric *metricspb.Metric) {
// 	assert.Equal(t, len(expectLabels), len(inMetric.MetricDescriptor.LabelKeys))
// 	for lidx, l := range inMetric.MetricDescriptor.LabelKeys {
// 		assert.Equal(t, expectLabels[lidx].Key, l.Key)
// 	}
// 	for _, ts := range inMetric.Timeseries {
// 		assert.Equal(t, len(expectLabels), len(ts.LabelValues))
// 		for lidx, l := range ts.LabelValues {
// 			assert.Equal(t, expectLabels[lidx].Value, l.Value)
// 			assert.True(t, l.HasValue)
// 		}
// 	}
// }

// func createTestMetrics(inMetrics []*metricspb.MetricDescriptor, labelKV []labelKeyValue) pdata.Metrics {
// 	md := consumerdata.MetricsData{
// 		Metrics: make([]*metricspb.Metric, len(inMetrics)),
// 	}

// 	for i, inMetric := range inMetrics {
// 		descriptor := proto.Clone(inMetric).(*metricspb.MetricDescriptor)

// 		labelKeys := make([]*metricspb.LabelKey, len(labelKV))
// 		labelValues := make([]*metricspb.LabelValue, len(labelKV))
// 		for j, label := range labelKV {
// 			labelKeys[j] = &metricspb.LabelKey{Key: label.Key}
// 			labelValues[j] = &metricspb.LabelValue{Value: label.Value, HasValue: true}
// 		}
// 		descriptor.LabelKeys = labelKeys

// 		md.Metrics[i] = &metricspb.Metric{
// 			MetricDescriptor: descriptor,
// 			Timeseries: []*metricspb.TimeSeries{
// 				{
// 					Points: []*metricspb.Point{
// 						{
// 							Value: &metricspb.Point_Int64Value{Int64Value: 1},
// 						}, {
// 							Value: &metricspb.Point_DoubleValue{DoubleValue: 2},
// 						},
// 					},
// 					LabelValues: labelValues,
// 				},
// 			},
// 		}
// 	}

// 	return pdatautil.MetricsFromMetricsData([]consumerdata.MetricsData{md})
// }

func TestComputeDistVals(t *testing.T) {
	// next stores the results of the aggregation metric processor.
	next := &etest.SinkMetricsExporter{}

	mtp := newMetricsTransformProcessor(next, nil, nil)
	assert.NotNil(t, mtp)

	caps := mtp.GetCapabilities()
	assert.Equal(t, true, caps.MutatesConsumedData)
	ctx := context.Background()
	assert.NoError(t, mtp.Start(ctx, nil))

	pointGroup1 := []float64{1, 2, 3, 7, 4}
	pointGroup2 := []float64{1, 2, 3, 3, 1}
	sum1, sumOfSquaredDeviation1 := calculateSumOfSquaredDeviation(pointGroup1)
	sum2, sumOfSquaredDeviation2 := calculateSumOfSquaredDeviation(pointGroup2)
	sum, sumOfSquaredDeviation := calculateSumOfSquaredDeviation(append(pointGroup1, pointGroup2...))

	val1 := &metricspb.DistributionValue{
		Count:                 int64(len(pointGroup1)),
		Sum:                   sum1,
		SumOfSquaredDeviation: sumOfSquaredDeviation1,
		BucketOptions: &metricspb.DistributionValue_BucketOptions{
			Type: &metricspb.DistributionValue_BucketOptions_Explicit_{
				Explicit: &metricspb.DistributionValue_BucketOptions_Explicit{
					Bounds: []float64{},
				},
			},
		},
	}

	val2 := &metricspb.DistributionValue{
		Count:                 int64(len(pointGroup2)),
		Sum:                   sum2,
		SumOfSquaredDeviation: sumOfSquaredDeviation2,
		BucketOptions: &metricspb.DistributionValue_BucketOptions{
			Type: &metricspb.DistributionValue_BucketOptions_Explicit_{
				Explicit: &metricspb.DistributionValue_BucketOptions_Explicit{
					Bounds: []float64{},
				},
			},
		},
	}

	outVal := mtp.computeDistVals(val1, val2)

	assert.Equal(t, int64(len(pointGroup1)+len(pointGroup2)), outVal.Count)
	assert.Equal(t, sum, outVal.Sum)
	assert.Equal(t, sumOfSquaredDeviation, outVal.SumOfSquaredDeviation)
}

func TestExemplers(t *testing.T) {
	t.Run("distribution value calculation test", func(t *testing.T) {
		// next stores the results of the aggregation metric processor.
		next := &etest.SinkMetricsExporter{}

		mtp := newMetricsTransformProcessor(next, nil, nil)
		assert.NotNil(t, mtp)

		caps := mtp.GetCapabilities()
		assert.Equal(t, true, caps.MutatesConsumedData)
		ctx := context.Background()
		assert.NoError(t, mtp.Start(ctx, nil))

		evenVal := &metricspb.DistributionValue{
			BucketOptions: &metricspb.DistributionValue_BucketOptions{
				Type: &metricspb.DistributionValue_BucketOptions_Explicit_{
					Explicit: &metricspb.DistributionValue_BucketOptions_Explicit{
						Bounds: []float64{},
					},
				},
			},
			Buckets: []*metricspb.DistributionValue_Bucket{
				{
					Exemplar: &metricspb.DistributionValue_Exemplar{
						Value: 2,
					},
				},
				{
					Exemplar: &metricspb.DistributionValue_Exemplar{
						Value: 4,
					},
				},
				{
					Exemplar: &metricspb.DistributionValue_Exemplar{
						Value: 6,
					},
				},
				{
					Exemplar: &metricspb.DistributionValue_Exemplar{
						Value: 8,
					},
				},
				{
					Exemplar: &metricspb.DistributionValue_Exemplar{
						Value: 0,
					},
				},
			},
		}

		oddVal := &metricspb.DistributionValue{
			BucketOptions: &metricspb.DistributionValue_BucketOptions{
				Type: &metricspb.DistributionValue_BucketOptions_Explicit_{
					Explicit: &metricspb.DistributionValue_BucketOptions_Explicit{
						Bounds: []float64{},
					},
				},
			},
			Buckets: []*metricspb.DistributionValue_Bucket{
				{
					Exemplar: &metricspb.DistributionValue_Exemplar{
						Value: 1,
					},
				},
				{
					Exemplar: &metricspb.DistributionValue_Exemplar{
						Value: 3,
					},
				},
				{
					Exemplar: &metricspb.DistributionValue_Exemplar{
						Value: 5,
					},
				},
				{
					Exemplar: &metricspb.DistributionValue_Exemplar{
						Value: 7,
					},
				},
				{
					Exemplar: &metricspb.DistributionValue_Exemplar{
						Value: 9,
					},
				},
			},
		}

		retry := 3
		foundOdd := false
		foundEven := false
		for i := 0; i < retry; i++ {
			outVal := mtp.computeDistVals(evenVal, oddVal)
			for _, b := range outVal.Buckets {
				if int(b.Exemplar.Value)%2 == 0 {
					foundEven = true
				} else {
					foundOdd = true
				}
			}
		}

		assert.True(t, foundEven && foundOdd)
	})
}

func BenchmarkMetricsTransformProcessorRenameMetrics(b *testing.B) {
	// runs 1000 metrics through a filterprocessor with both include and exclude filters.
	stressTest := metricsTransformTest{
		name: "1000Metrics",
		transforms: []mtpTransform{
			{
				MetricName: "metric1",
				Action:     Insert,
				NewName:    "new/metric1",
			},
		},
	}

	for len(stressTest.in) < 1000 {
		stressTest.in = append(stressTest.in, testcaseBuilder().setName("metric1").build())
	}

	benchmarkTests := []metricsTransformTest{stressTest}

	for _, test := range benchmarkTests {
		// next stores the results of the filter metric processor.
		next := &etest.SinkMetricsExporter{}

		mtp := newMetricsTransformProcessor(next, nil, test.transforms)
		assert.NotNil(b, mtp)

		md := consumerdata.MetricsData{Metrics: test.in}

		b.Run(test.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				assert.NoError(b, mtp.ConsumeMetrics(
					context.Background(),
					pdatautil.MetricsFromMetricsData([]consumerdata.MetricsData{
						md,
					}),
				))
			}
		})
	}
}
