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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"
	etest "go.opentelemetry.io/collector/exporter/exportertest"
)

func TestMetricsTransformProcessor(t *testing.T) {
	for _, test := range standardTests {
		t.Run(test.name, func(t *testing.T) {
			// next stores the results of the aggregation metric processor.
			next := &etest.SinkMetricsExporter{}
			cfg := &Config{
				ProcessorSettings: configmodels.ProcessorSettings{
					TypeVal: typeStr,
					NameVal: typeStr,
				},
				Transforms: test.transforms,
			}

			mtp := newMetricsTransformProcessor(next, cfg)
			assert.NotNil(t, mtp)

			caps := mtp.GetCapabilities()
			assert.Equal(t, true, caps.MutatesConsumedData)
			ctx := context.Background()
			assert.NoError(t, mtp.Start(ctx, nil))

			// construct metrics data to feed into the processor
			md := constructTestInputMetricsData(test)

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
				// actualDescriptor := actualOut[idx].MetricDescriptor
				assert.Equal(t, *out, *actualOut[idx])
				// // name
				// assert.Equal(t, out.name, actualDescriptor.Name)
				// // data type
				// assert.Equal(t, out.dataType, actualDescriptor.Type)
				// // label keys
				// assert.Equal(t, len(out.labelKeys), len(actualDescriptor.LabelKeys))
				// for lidx, l := range out.labelKeys {
				// 	assert.Equal(t, l, actualDescriptor.LabelKeys[lidx].Key)
				// }
				// // timeseries
				// assert.Equal(t, len(out.timeseries), len(actualOut[idx].Timeseries))
				// for tidx, ts := range out.timeseries {
				// 	actualTimeseries := actualOut[idx].Timeseries[tidx]
				// 	// start timestamp
				// 	assert.Equal(t, ts.startTimestamp, actualTimeseries.StartTimestamp.Seconds)
				// 	// label values
				// 	assert.Equal(t, len(ts.labelValues), len(actualTimeseries.LabelValues))
				// 	for vidx, v := range ts.labelValues {
				// 		assert.Equal(t, v, actualTimeseries.LabelValues[vidx].Value)
				// 	}
				// 	// points
				// 	assert.Equal(t, len(ts.points), len(actualTimeseries.Points))
				// 	for pidx, p := range ts.points {
				// 		actualPoint := actualTimeseries.Points[pidx]
				// 		assert.Equal(t, p.timestamp, actualPoint.Timestamp.Seconds)
				// 		switch out.dataType {
				// 		case metricspb.MetricDescriptor_CUMULATIVE_INT64, metricspb.MetricDescriptor_GAUGE_INT64:
				// 			assert.Equal(t, int64(p.value), actualPoint.GetInt64Value())
				// 		case metricspb.MetricDescriptor_CUMULATIVE_DOUBLE, metricspb.MetricDescriptor_GAUGE_DOUBLE:
				// 			assert.Equal(t, float64(p.value), actualPoint.GetDoubleValue())
				// 		case metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION, metricspb.MetricDescriptor_GAUGE_DISTRIBUTION:
				// 			assert.Equal(t, p.sum, actualPoint.GetDistributionValue().GetSum())
				// 			assert.Equal(t, p.count, actualPoint.GetDistributionValue().GetCount())
				// 			assert.Equal(t, p.sumOfSquaredDeviation, actualPoint.GetDistributionValue().GetSumOfSquaredDeviation())
				// 			for boIdx, bound := range p.bounds {
				// 				assert.Equal(t, bound, actualPoint.GetDistributionValue().GetBucketOptions().GetExplicit().Bounds[boIdx])
				// 			}
				// 			for buIdx, bucket := range p.buckets {
				// 				assert.Equal(t, bucket, actualPoint.GetDistributionValue().GetBuckets()[buIdx].Count)
				// 			}

				// 		}
				// 	}
				// }
			}

			assert.NoError(t, mtp.Shutdown(ctx))
		})
	}
}

// func BenchmarkMetricsTransformProcessorRenameMetrics(b *testing.B) {
// 	// runs 1000 metrics through a filterprocessor with both include and exclude filters.
// 	stressTest := metricsTransformTest{
// 		name: "1000Metrics",
// 		transforms: []Transform{
// 			{
// 				MetricName: metric1,
// 				Action:     Insert,
// 				NewName:    newMetric1,
// 			},
// 		},
// 	}

// 	for len(stressTest.in) < 1000 {
// 		stressTest.in = append(stressTest.in, initialMetricRename1)
// 	}

// 	benchmarkTests := []metricsTransformTest{stressTest}

// 	for _, test := range benchmarkTests {
// 		// next stores the results of the filter metric processor.
// 		next := &etest.SinkMetricsExporter{}
// 		cfg := &Config{
// 			ProcessorSettings: configmodels.ProcessorSettings{
// 				TypeVal: typeStr,
// 				NameVal: typeStr,
// 			},
// 			Transforms: test.transforms,
// 		}

// 		mtp := newMetricsTransformProcessor(next, cfg)
// 		assert.NotNil(b, mtp)

// 		md := constructTestInputMetricsData(test)

// 		b.Run(test.name, func(b *testing.B) {
// 			for i := 0; i < b.N; i++ {
// 				assert.NoError(b, mtp.ConsumeMetrics(
// 					context.Background(),
// 					pdatautil.MetricsFromMetricsData([]consumerdata.MetricsData{
// 						md,
// 					}),
// 				))
// 			}
// 		})
// 	}
// }
