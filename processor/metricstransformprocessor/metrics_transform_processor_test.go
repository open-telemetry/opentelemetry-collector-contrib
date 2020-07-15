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
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"
	"go.opentelemetry.io/collector/exporter/exportertest"
)

type metricsTransformTest struct {
	name       string // test name
	transforms []Transform
	inMetrics  []*metricspb.MetricDescriptor // input Metric names
	outMetrics []*metricspb.MetricDescriptor // output Metric names
	inLabels   []string
	outLabels  []string
}

var (
	initialMetricNames = []*metricspb.MetricDescriptor{
		{Name: "metric1"},
		{Name: "metric5"},
	}

	outMetricNamesUpdateSingle = []*metricspb.MetricDescriptor{
		{Name: "metric1/new"},
		{Name: "metric5"},
	}
	outMetricNamesUpdateMultiple = []*metricspb.MetricDescriptor{
		{Name: "metric1/new"},
		{Name: "metric5/new"},
	}

	outMetricNamesInsertSingle = []*metricspb.MetricDescriptor{
		{Name: "metric1"},
		{Name: "metric5"},
		{Name: "metric1/new"},
	}

	outMetricNamesInsertMultiple = []*metricspb.MetricDescriptor{
		{Name: "metric1"},
		{Name: "metric5"},
		{Name: "metric1/new"},
		{Name: "metric5/new"},
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
			inMetrics:  initialMetricNames,
			outMetrics: outMetricNamesUpdateSingle,
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
			inMetrics:  initialMetricNames,
			outMetrics: outMetricNamesUpdateMultiple,
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
			inMetrics:  initialMetricNames,
			outMetrics: initialMetricNames,
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
			inMetrics:  initialMetricNames,
			outMetrics: initialMetricNames,
			inLabels:   initialLabels,
			outLabels:  outLabels,
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
			inMetrics:  initialMetricNames,
			outMetrics: outMetricNamesInsertSingle,
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
			inMetrics:  initialMetricNames,
			outMetrics: outMetricNamesInsertMultiple,
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
			inMetrics:  initialMetricNames,
			outMetrics: outMetricNamesInsertSingle,
			inLabels:   initialLabels,
			outLabels:  outLabels,
		},
		// Toggle Data Type
		{
			name: "metric_toggle_scalar_data_type_int64_to_double",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []Operation{{Action: ToggleScalarDataType}},
				},
				{
					MetricName: "metric2",
					Action:     Update,
					Operations: []Operation{{Action: ToggleScalarDataType}},
				},
			},
			inMetrics: []*metricspb.MetricDescriptor{
				{Name: "metric1", Type: metricspb.MetricDescriptor_CUMULATIVE_INT64},
				{Name: "metric2", Type: metricspb.MetricDescriptor_GAUGE_INT64},
			},
			outMetrics: []*metricspb.MetricDescriptor{
				{Name: "metric1", Type: metricspb.MetricDescriptor_CUMULATIVE_DOUBLE},
				{Name: "metric2", Type: metricspb.MetricDescriptor_GAUGE_DOUBLE},
			},
		},
		{
			name: "metric_toggle_scalar_data_type_double_to_int64",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []Operation{{Action: ToggleScalarDataType}},
				},
				{
					MetricName: "metric2",
					Action:     Update,
					Operations: []Operation{{Action: ToggleScalarDataType}},
				},
			},
			inMetrics: []*metricspb.MetricDescriptor{
				{Name: "metric1", Type: metricspb.MetricDescriptor_CUMULATIVE_DOUBLE},
				{Name: "metric2", Type: metricspb.MetricDescriptor_GAUGE_DOUBLE},
			},
			outMetrics: []*metricspb.MetricDescriptor{
				{Name: "metric1", Type: metricspb.MetricDescriptor_CUMULATIVE_INT64},
				{Name: "metric2", Type: metricspb.MetricDescriptor_GAUGE_INT64},
			},
		},
		{
			name: "metric_toggle_scalar_data_type_no_effect",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []Operation{{Action: ToggleScalarDataType}},
				},
			},
			inMetrics:  []*metricspb.MetricDescriptor{{Name: "metric1", Type: metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION}},
			outMetrics: []*metricspb.MetricDescriptor{{Name: "metric1", Type: metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION}},
		},
	}
)

func TestMetricsTransformProcessor(t *testing.T) {
	for _, test := range standardTests {
		t.Run(test.name, func(t *testing.T) {
			next := &exportertest.SinkMetricsExporter{}
			cfg := &Config{Transforms: test.transforms}

			mtp := newMetricsTransformProcessor(next, cfg)
			assert.True(t, mtp.GetCapabilities().MutatesConsumedData)
			assert.NoError(t, mtp.Start(context.Background(), componenttest.NewNopHost()))
			defer func() { assert.NoError(t, mtp.Shutdown(context.Background())) }()

			inputMetrics := createTestMetrics(test.inMetrics, test.inLabels)
			assert.NoError(t, mtp.ConsumeMetrics(context.Background(), inputMetrics))

			got := next.AllMetrics()

			// build map of metric names to transforms
			targetNameToTransform := make(map[string]Transform, len(test.transforms))
			for _, transform := range test.transforms {
				targetName := transform.MetricName
				if transform.NewName != "" {
					targetName = transform.NewName
				}
				targetNameToTransform[targetName] = transform
			}

			// validate results
			require.Equal(t, 1, len(got))
			gotMD := pdatautil.MetricsToMetricsData(got[0])
			require.Equal(t, 1, len(gotMD))
			require.Equal(t, len(test.outMetrics), len(gotMD[0].Metrics))

			for idx, out := range gotMD[0].Metrics {
				assert.Equal(t, test.outMetrics[idx].Name, out.MetricDescriptor.Name)
				assert.Equal(t, test.outMetrics[idx].Type, out.MetricDescriptor.Type)

				transform, ok := targetNameToTransform[out.MetricDescriptor.Name]

				// check the original labels are untouched if not transformed, or insert
				if !ok || (transform.Action == Insert && out.MetricDescriptor.Name == transform.MetricName) {
					for lidx, l := range out.MetricDescriptor.LabelKeys {
						assert.Equal(t, test.inLabels[lidx], l.Key)
					}
				}

				if !ok {
					continue
				}

				// check the labels are correctly updated
				for lidx, l := range out.MetricDescriptor.LabelKeys {
					assert.Equal(t, test.outLabels[lidx], l.Key)
				}

				// check the data type was changed correctly
				for _, ts := range out.Timeseries {
					for _, p := range ts.Points {
						switch out.MetricDescriptor.Type {
						case metricspb.MetricDescriptor_CUMULATIVE_INT64, metricspb.MetricDescriptor_GAUGE_INT64:
							assert.Equal(t, float64(0), p.GetDoubleValue())
						case metricspb.MetricDescriptor_CUMULATIVE_DOUBLE, metricspb.MetricDescriptor_GAUGE_DOUBLE:
							assert.Equal(t, int64(0), p.GetInt64Value())
						}
					}
				}
			}
		})
	}
}

func createTestMetrics(inMetrics []*metricspb.MetricDescriptor, inLabels []string) pdata.Metrics {
	md := consumerdata.MetricsData{
		Metrics: make([]*metricspb.Metric, len(inMetrics)),
	}

	for i, inMetric := range inMetrics {
		descriptor := proto.Clone(inMetric).(*metricspb.MetricDescriptor)

		labels := make([]*metricspb.LabelKey, len(inLabels))
		for j, labelKey := range inLabels {
			labels[j] = &metricspb.LabelKey{Key: labelKey}
		}
		descriptor.LabelKeys = labels

		md.Metrics[i] = &metricspb.Metric{
			MetricDescriptor: descriptor,
			Timeseries: []*metricspb.TimeSeries{
				{
					Points: []*metricspb.Point{
						{
							Value: &metricspb.Point_Int64Value{Int64Value: 1},
						}, {
							Value: &metricspb.Point_DoubleValue{DoubleValue: 2},
						},
					},
				},
			},
		}
	}

	return pdatautil.MetricsFromMetricsData([]consumerdata.MetricsData{md})
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

	for len(stressTest.inMetrics) < 1000 {
		stressTest.inMetrics = append(stressTest.inMetrics, initialMetricNames...)
	}

	benchmarkTests := []metricsTransformTest{stressTest}

	for _, test := range benchmarkTests {
		// next stores the results of the filter metric processor.
		next := &exportertest.SinkMetricsExporter{}
		cfg := &Config{
			ProcessorSettings: configmodels.ProcessorSettings{
				TypeVal: typeStr,
				NameVal: typeStr,
			},
			Transforms: test.transforms,
		}

		mtp := newMetricsTransformProcessor(next, cfg)
		assert.NotNil(b, mtp)

		md := consumerdata.MetricsData{
			Metrics: make([]*metricspb.Metric, len(test.inMetrics)),
		}

		for idx, in := range test.inMetrics {
			md.Metrics[idx] = &metricspb.Metric{
				MetricDescriptor: in,
			}
		}

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
