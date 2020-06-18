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
	"fmt"
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"
	etest "go.opentelemetry.io/collector/exporter/exportertest"
)

type metricsTransformTest struct {
	name       string
	action     ConfigAction
	newName    string
	operations []Operation
	inMN       []string // input Metric names
	outMN      []string // output Metric names
}

var (
	inMetricNames = []string{
		"metric1",
		"metric5",
	}
	outMetricNamesUpdate = []string{
		"metric1/new",
		"metric5",
	}

	outMetricNamesInsert = []string{
		"metric1",
		"metric5",
		"metric1/new",
	}

	standardTests = []metricsTransformTest{
		{
			name:    "metric1",
			action:  "update",
			newName: "metric1/new",
			inMN:    inMetricNames,
			outMN:   outMetricNamesUpdate,
		},
		{
			name:    "metric1",
			action:  "insert",
			newName: "metric1/new",
			inMN:    inMetricNames,
			outMN:   outMetricNamesInsert,
		},
		{
			name:    "metric1",
			action:  "update",
			newName: "metric5",
			inMN:    inMetricNames,
			outMN:   inMetricNames,
		},
	}
)

func TestMetricsTransformProcessorRenameMetrics(t *testing.T) {
	for _, test := range standardTests {
		t.Run(test.name, func(t *testing.T) {
			// next stores the results of the aggregation metric processor
			next := &etest.SinkMetricsExporter{}
			cfg := &Config{
				ProcessorSettings: configmodels.ProcessorSettings{
					TypeVal: typeStr,
					NameVal: typeStr,
				},
				MetricName: test.name,
				Action:     test.action,
				NewName:    test.newName,
			}
			amp, err := newMetricsTransformProcessor(next, cfg)
			assert.NotNil(t, amp)
			assert.Nil(t, err)

			caps := amp.GetCapabilities()
			assert.Equal(t, false, caps.MutatesConsumedData)
			ctx := context.Background()
			assert.NoError(t, amp.Start(ctx, nil))

			md := consumerdata.MetricsData{
				Metrics: make([]*metricspb.Metric, len(test.inMN)),
			}

			for idx, in := range test.inMN {
				md.Metrics[idx] = &metricspb.Metric{
					MetricDescriptor: &metricspb.MetricDescriptor{
						Name: in,
					},
				}
			}

			cErr := amp.ConsumeMetrics(
				context.Background(),
				pdatautil.MetricsFromMetricsData([]consumerdata.MetricsData{
					{},
					md,
					{
						Metrics: []*metricspb.Metric{},
					},
				}),
			)
			assert.Nil(t, cErr)

			got := next.AllMetrics()
			fmt.Println(got)
			require.Equal(t, 1, len(got))
			gotMD := pdatautil.MetricsToMetricsData(got[0])
			require.Equal(t, 3, len(gotMD))
			assert.EqualValues(t, consumerdata.MetricsData{}, gotMD[0])
			require.Equal(t, len(test.outMN), len(gotMD[1].Metrics))
			for idx, out := range gotMD[1].Metrics {
				assert.Equal(t, test.outMN[idx], out.MetricDescriptor.Name)
			}

			assert.EqualValues(t, consumerdata.MetricsData{Metrics: []*metricspb.Metric{}}, gotMD[2])
			assert.NoError(t, amp.Shutdown(ctx))
		})
	}
}

func BenchmarkMetricsTransformProcessorRenameMetrics(b *testing.B) {
	// runs 1000 metrics through a filterprocessor with both include and exclude filters.
	stressTest := metricsTransformTest{
		name:    "rename1000Metrics",
		action:  "insert",
		newName: "newname",
	}

	for len(stressTest.inMN) < 1000 {
		stressTest.inMN = append(stressTest.inMN, inMetricNames...)
	}

	benchmarkTests := append(standardTests, stressTest)

	for _, test := range benchmarkTests {
		// next stores the results of the filter metric processor
		next := &etest.SinkMetricsExporter{}
		cfg := &Config{
			ProcessorSettings: configmodels.ProcessorSettings{
				TypeVal: typeStr,
				NameVal: typeStr,
			},
			MetricName: test.name,
			Action:     test.action,
			NewName:    test.newName,
		}

		amp, err := newMetricsTransformProcessor(next, cfg)
		assert.NotNil(b, amp)
		assert.Nil(b, err)

		md := consumerdata.MetricsData{
			Metrics: make([]*metricspb.Metric, len(test.inMN)),
		}

		for idx, in := range test.inMN {
			md.Metrics[idx] = &metricspb.Metric{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name: in,
				},
			}
		}

		b.Run(test.name, func(b *testing.B) {
			assert.NoError(b, amp.ConsumeMetrics(
				context.Background(),
				pdatautil.MetricsFromMetricsData([]consumerdata.MetricsData{
					{},
					md,
					{
						Metrics: []*metricspb.Metric{},
					},
				}),
			))
		})
	}
}
