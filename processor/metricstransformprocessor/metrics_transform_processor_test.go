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
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"
	etest "go.opentelemetry.io/collector/exporter/exportertest"
)

type metricsTransformTest struct {
	name       string // test name
	transforms []Transform
	inMN       []string // input Metric names
	outMN      []string // output Metric names
	inLabels   []string
	outLabels  []string
}

var (
	initialMetricNames = []string{
		"metric1",
		"metric5",
	}

	outMetricNamesUpdateSingle = []string{
		"metric1/new",
		"metric5",
	}
	outMetricNamesUpdateMultiple = []string{
		"metric1/new",
		"metric5/new",
	}

	outMetricNamesInsertSingle = []string{
		"metric1",
		"metric5",
		"metric1/new",
	}

	outMetricNamesInsertMultiple = []string{
		"metric1",
		"metric5",
		"metric1/new",
		"metric5/new",
	}

	initialLabels = []string{
		"label1",
		"label2",
	}

	outLabels = []string{
		"label1/new",
		"label2",
	}

	validUpateLabelOperation = Operation{
		Action:   UpdateLabel,
		Label:    "label1",
		NewLabel: "label1/new",
	}

	invalidUpdateLabelOperation = Operation{
		Action:   UpdateLabel,
		Label:    "label1",
		NewLabel: "label2",
	}

	standardTests = []metricsTransformTest{
		// UPDATE
		{
			name: "metric_name_update",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Update,
					NewName:    "metric1/new",
				},
			},
			inMN:  initialMetricNames,
			outMN: outMetricNamesUpdateSingle,
		},
		{
			name: "metric_name_update_multiple",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Update,
					NewName:    "metric1/new",
				},
				{
					MetricName: "metric5",
					Action:     Update,
					NewName:    "metric5/new",
				},
			},
			inMN:  initialMetricNames,
			outMN: outMetricNamesUpdateMultiple,
		},
		{
			name: "metric_name_update_nonexist",
			transforms: []Transform{
				{
					MetricName: "metric100",
					Action:     Update,
					NewName:    "metric1/new",
				},
			},
			inMN:  initialMetricNames,
			outMN: initialMetricNames,
		},
		{
			name: "metric_name_update_invalid",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Update,
					NewName:    "metric5",
				},
			},
			inMN:  initialMetricNames,
			outMN: initialMetricNames,
		},
		{
			name: "metric_label_update",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []Operation{validUpateLabelOperation},
				},
			},
			inMN:      initialMetricNames,
			outMN:     initialMetricNames,
			inLabels:  initialLabels,
			outLabels: outLabels,
		},
		{
			name: "metric_label_update_invalid",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []Operation{invalidUpdateLabelOperation},
				},
			},
			inMN:      initialMetricNames,
			outMN:     initialMetricNames,
			inLabels:  initialLabels,
			outLabels: initialLabels,
		},
		// INSERT
		{
			name: "metric_name_insert",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Insert,
					NewName:    "metric1/new",
				},
			},
			inMN:  initialMetricNames,
			outMN: outMetricNamesInsertSingle,
		},
		{
			name: "metric_name_insert_multiple",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Insert,
					NewName:    "metric1/new",
				},
				{
					MetricName: "metric5",
					Action:     Insert,
					NewName:    "metric5/new",
				},
			},
			inMN:  initialMetricNames,
			outMN: outMetricNamesInsertMultiple,
		},
		{
			name: "metric_label_update_with_metric_insert",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Insert,
					NewName:    "metric1/new",
					Operations: []Operation{validUpateLabelOperation},
				},
			},
			inMN:      initialMetricNames,
			outMN:     outMetricNamesInsertSingle,
			inLabels:  initialLabels,
			outLabels: outLabels,
		},
	}
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

			mtp, err := newMetricsTransformProcessor(next, cfg)
			assert.NotNil(t, mtp)
			assert.Nil(t, err)

			caps := mtp.GetCapabilities()
			assert.Equal(t, true, caps.MutatesConsumedData)
			ctx := context.Background()
			assert.NoError(t, mtp.Start(ctx, nil))

			// construct metrics data to feed into the processor
			md := consumerdata.MetricsData{
				Metrics: make([]*metricspb.Metric, len(test.inMN)),
			}
			for idx, in := range test.inMN {
				labels := make([]*metricspb.LabelKey, 0)
				for _, l := range test.inLabels {
					labels = append(
						labels,
						&metricspb.LabelKey{
							Key: l,
						},
					)
				}
				md.Metrics[idx] = &metricspb.Metric{
					MetricDescriptor: &metricspb.MetricDescriptor{
						Name:      in,
						LabelKeys: labels,
					},
				}
			}

			// process
			cErr := mtp.ConsumeMetrics(
				context.Background(),
				pdatautil.MetricsFromMetricsData([]consumerdata.MetricsData{
					md,
				}),
			)
			assert.Nil(t, cErr)

			// get and check results
			got := next.AllMetrics()
			require.Equal(t, 1, len(got))
			gotMD := pdatautil.MetricsToMetricsData(got[0])
			require.Equal(t, 1, len(gotMD))
			require.Equal(t, len(test.outMN), len(gotMD[0].Metrics))

			targetNameToTransform := make(map[string]Transform)
			for _, transform := range test.transforms {
				targetName := transform.MetricName
				if transform.NewName != "" {
					targetName = transform.NewName
				}
				targetNameToTransform[targetName] = transform
			}
			for idx, out := range gotMD[0].Metrics {
				// check name
				assert.Equal(t, test.outMN[idx], out.MetricDescriptor.Name)
				// check labels
				// check the updated or inserted is correctly updated
				transform, ok := targetNameToTransform[out.MetricDescriptor.Name]
				if ok {
					for lidx, l := range out.MetricDescriptor.LabelKeys {
						assert.Equal(t, test.outLabels[lidx], l.Key)
					}
				}
				// check the original is untouched if insert
				if transform.Action == Insert && out.MetricDescriptor.Name == transform.MetricName {
					for lidx, l := range out.MetricDescriptor.LabelKeys {
						assert.Equal(t, test.inLabels[lidx], l.Key)
					}
				}
			}
			assert.NoError(t, mtp.Shutdown(ctx))
		})
	}
}

func BenchmarkMetricsTransformProcessorRenameMetrics(b *testing.B) {
	// runs 1000 metrics through a filterprocessor with both include and exclude filters.
	stressTest := metricsTransformTest{
		name: "1000Metrics",
		transforms: []Transform{
			{
				MetricName: "metric1",
				Action:     "insert",
				NewName:    "newname",
			},
		},
	}

	for len(stressTest.inMN) < 1000 {
		stressTest.inMN = append(stressTest.inMN, initialMetricNames...)
	}

	benchmarkTests := []metricsTransformTest{stressTest}

	for _, test := range benchmarkTests {
		// next stores the results of the filter metric processor.
		next := &etest.SinkMetricsExporter{}
		cfg := &Config{
			ProcessorSettings: configmodels.ProcessorSettings{
				TypeVal: typeStr,
				NameVal: typeStr,
			},
			Transforms: test.transforms,
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
			for i := 0; i < b.N; i++ {
				assert.NoError(b, amp.ConsumeMetrics(
					context.Background(),
					pdatautil.MetricsFromMetricsData([]consumerdata.MetricsData{
						md,
					}),
				))
			}
		})
	}
}
