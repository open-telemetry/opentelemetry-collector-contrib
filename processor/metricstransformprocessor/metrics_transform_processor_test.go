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
	metricName string
	action     ConfigAction
	newName    string
	operations []Operation
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

	outMetricNamesUpdate = []string{
		"metric1/new",
		"metric5",
	}

	outMetricNamesInsert = []string{
		"metric1",
		"metric5",
		"metric1/new",
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
			name:       "metric_name_update",
			metricName: "metric1",
			action:     "update",
			newName:    "metric1/new",
			inMN:       initialMetricNames,
			outMN:      outMetricNamesUpdate,
		},
		{
			name:       "metric_name_invalid_update",
			metricName: "metric1",
			action:     "update",
			newName:    "metric5",
			inMN:       initialMetricNames,
			outMN:      initialMetricNames,
		},
		{
			name:       "metric_label_update",
			metricName: "metric1",
			action:     "update",
			operations: []Operation{validUpateLabelOperation},
			inMN:       initialMetricNames,
			outMN:      initialMetricNames,
			inLabels:   initialLabels,
			outLabels:  outLabels,
		},
		{
			name:       "metric_label_invalid_update",
			metricName: "metric1",
			action:     "update",
			operations: []Operation{invalidUpdateLabelOperation},
			inMN:       initialMetricNames,
			outMN:      initialMetricNames,
			inLabels:   initialLabels,
			outLabels:  initialLabels,
		},
		// INSERT
		{
			name:       "metric_name_insert",
			metricName: "metric1",
			action:     "insert",
			newName:    "metric1/new",
			inMN:       initialMetricNames,
			outMN:      outMetricNamesInsert,
		},
		{
			name:       "metric_label_update_with_metric_insert",
			metricName: "metric1",
			action:     "insert",
			newName:    "metric1/new",
			operations: []Operation{validUpateLabelOperation},
			inMN:       initialMetricNames,
			outMN:      outMetricNamesInsert,
			inLabels:   initialLabels,
			outLabels:  outLabels,
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
				MetricName: test.metricName,
				Action:     test.action,
				NewName:    test.newName,
				Operations: test.operations,
			}
			amp, err := newMetricsTransformProcessor(next, cfg)
			assert.NotNil(t, amp)
			assert.Nil(t, err)

			caps := amp.GetCapabilities()
			assert.Equal(t, true, caps.MutatesConsumedData)
			ctx := context.Background()
			assert.NoError(t, amp.Start(ctx, nil))

			md := consumerdata.MetricsData{
				Metrics: make([]*metricspb.Metric, len(test.inMN)),
			}

			// construct metrics data to feed into the processor
			for idx, in := range test.inMN {
				labels := make([]*metricspb.LabelKey, 0)
				if in == test.metricName {
					for _, l := range test.inLabels {
						labels = append(
							labels,
							&metricspb.LabelKey{
								Key: l,
							},
						)
					}
				}
				md.Metrics[idx] = &metricspb.Metric{
					MetricDescriptor: &metricspb.MetricDescriptor{
						Name:      in,
						LabelKeys: labels,
					},
				}
			}

			// process
			cErr := amp.ConsumeMetrics(
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

			targetName := test.metricName
			if test.newName != "" {
				targetName = test.newName
			}
			for idx, out := range gotMD[0].Metrics {
				// check name
				assert.Equal(t, test.outMN[idx], out.MetricDescriptor.Name)
				// check labels
				// check the updated or inserted is correctly updated
				if out.MetricDescriptor.Name == targetName {
					for lidx, l := range out.MetricDescriptor.LabelKeys {
						assert.Equal(t, test.outLabels[lidx], l.Key)
					}
				}
				// check the original is untouched if insert
				if test.action == Insert {
					if out.MetricDescriptor.Name == test.metricName {
						for lidx, l := range out.MetricDescriptor.LabelKeys {
							assert.Equal(t, test.inLabels[lidx], l.Key)
						}
					}
				}
			}

			assert.NoError(t, amp.Shutdown(ctx))
		})
	}
}

func BenchmarkMetricsTransformProcessorRenameMetrics(b *testing.B) {
	// runs 1000 metrics through a filterprocessor with both include and exclude filters.
	stressTest := metricsTransformTest{
		name:       "1000Metrics",
		metricName: "metric1",
		action:     "insert",
		newName:    "newname",
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
			MetricName: test.metricName,
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
