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
	"math"
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestMetricsTransformProcessor(t *testing.T) {
	for _, test := range standardTests {
		t.Run(test.name, func(t *testing.T) {
			next := new(consumertest.MetricsSink)

			p := newMetricsTransformProcessor(zap.NewExample(), test.transforms)

			mtp, err := processorhelper.NewMetricsProcessor(
				&Config{
					ProcessorSettings: configmodels.ProcessorSettings{
						TypeVal: typeStr,
						NameVal: typeStr,
					},
				},
				next,
				p,
				processorhelper.WithCapabilities(processorCapabilities))
			require.NoError(t, err)

			caps := mtp.GetCapabilities()
			assert.Equal(t, true, caps.MutatesConsumedData)
			ctx := context.Background()

			// construct metrics data to feed into the processor
			md := consumerdata.MetricsData{Metrics: test.in}

			// process
			cErr := mtp.ConsumeMetrics(context.Background(), internaldata.OCToMetrics(md))
			assert.NoError(t, cErr)

			// get and check results
			got := next.AllMetrics()
			require.Equal(t, 1, len(got))
			gotMD := internaldata.MetricsToOC(got[0])
			require.Equal(t, 1, len(gotMD))
			actualOutMetrics := gotMD[0].Metrics
			require.Equal(t, len(test.out), len(actualOutMetrics))

			for idx, out := range test.out {
				actualOut := actualOutMetrics[idx]
				if diff := cmp.Diff(actualOut, out, protocmp.Transform()); diff != "" {
					t.Errorf("Unexpected difference:\n%v", diff)
				}
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
			p := newMetricsTransformProcessor(nil, nil)

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

			outVal := p.computeSumOfSquaredDeviation(val1, val2)

			assert.Equal(t, sumOfSquaredDeviation, outVal)
		})
	}
}

// calculateSumOfSquaredDeviation returns the sum and the sumOfSquaredDeviation for this slice
func calculateSumOfSquaredDeviation(slice []float64) (sum float64, sumOfSquaredDeviation float64) {
	sum = 0
	for _, e := range slice {
		sum += e
	}
	ave := sum / float64(len(slice))
	sumOfSquaredDeviation = 0
	for _, e := range slice {
		sumOfSquaredDeviation += math.Pow((e - ave), 2)
	}
	return
}

func TestExemplars(t *testing.T) {
	p := newMetricsTransformProcessor(nil, nil)
	exe1 := &metricspb.DistributionValue_Exemplar{Value: 1}
	exe2 := &metricspb.DistributionValue_Exemplar{Value: 2}
	picked := p.pickExemplar(exe1, exe2)
	assert.True(t, picked == exe1 || picked == exe2)
}

func BenchmarkMetricsTransformProcessorRenameMetrics(b *testing.B) {
	const metricCount = 1000

	transforms := []internalTransform{
		{
			MetricName: "metric1",
			Action:     Insert,
			NewName:    "new/metric1",
		},
	}

	in := make([]*metricspb.Metric, metricCount)
	for i := 0; i < metricCount; i++ {
		in[i] = metricBuilder().setName("metric1").build()
	}
	md := consumerdata.MetricsData{Metrics: in}

	p := newMetricsTransformProcessor(nil, transforms)
	mtp, _ := processorhelper.NewMetricsProcessor(&Config{}, consumertest.NewMetricsNop(), p)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mtp.ConsumeMetrics(context.Background(), internaldata.OCToMetrics(md))
	}
}
