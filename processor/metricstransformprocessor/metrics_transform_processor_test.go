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

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"
	etest "go.opentelemetry.io/collector/exporter/exportertest"
	"go.uber.org/zap"
)

func TestMetricsTransformProcessor(t *testing.T) {
	for _, test := range standardTests {
		t.Run(test.name, func(t *testing.T) {
			// next stores the results of the aggregation metric processor.
			next := &etest.SinkMetricsExporter{}

			mtp := newMetricsTransformProcessor(next, zap.NewExample(), test.transforms)
			assert.NotNil(t, mtp)

			caps := mtp.GetCapabilities()
			assert.Equal(t, true, caps.MutatesConsumedData)
			ctx := context.Background()
			assert.NoError(t, mtp.Start(ctx, nil))

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

			assert.NoError(t, mtp.Shutdown(ctx))
		})
	}
}

func TestComputeDistVals(t *testing.T) {
	ssdTests := []struct {
		name        string
		pointGroup1 []float64
		pointGroup2 []float64
	}{
		{
			name:        "similar point groups",
			pointGroup1: []float64{1, 2, 3, 7, 4},
			pointGroup2: []float64{1, 2, 3, 3, 1},
		},
		{
			name:        "different size point groups",
			pointGroup1: []float64{1, 2, 3, 7, 4},
			pointGroup2: []float64{1},
		},
		{
			name:        "point groups with an outlier",
			pointGroup1: []float64{1, 2, 3, 7, 1000},
			pointGroup2: []float64{1, 2, 5},
		},
	}

	for _, test := range ssdTests {
		t.Run(test.name, func(t *testing.T) {
			// next stores the results of the aggregation metric processor.
			next := &etest.SinkMetricsExporter{}

			mtp := newMetricsTransformProcessor(next, nil, nil)
			assert.NotNil(t, mtp)

			caps := mtp.GetCapabilities()
			assert.Equal(t, true, caps.MutatesConsumedData)
			ctx := context.Background()
			assert.NoError(t, mtp.Start(ctx, nil))

			pointGroup1 := test.pointGroup1
			pointGroup2 := test.pointGroup2
			sum1, sumOfSquaredDeviation1 := calculateSumOfSquaredDeviation(pointGroup1)
			sum2, sumOfSquaredDeviation2 := calculateSumOfSquaredDeviation(pointGroup2)
			_, sumOfSquaredDeviation := calculateSumOfSquaredDeviation(append(pointGroup1, pointGroup2...))

			val1 := &metricspb.DistributionValue{
				Count:                 int64(len(pointGroup1)),
				Sum:                   sum1,
				SumOfSquaredDeviation: sumOfSquaredDeviation1,
			}

			val2 := &metricspb.DistributionValue{
				Count:                 int64(len(pointGroup2)),
				Sum:                   sum2,
				SumOfSquaredDeviation: sumOfSquaredDeviation2,
			}

			outVal := mtp.computeSumOfSquaredDeviation(val1, val2)

			assert.Equal(t, sumOfSquaredDeviation, outVal)

		})
	}
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

		exe1 := &metricspb.DistributionValue_Exemplar{
			Value: 1,
		}

		exe2 := &metricspb.DistributionValue_Exemplar{
			Value: 2,
		}

		picked := mtp.pickExemplar(exe1, exe2)

		assert.True(t, picked == exe1 || picked == exe2)
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
