package metricstransformprocessor

import (
	"context"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/collector/translator/internaldata"
	"google.golang.org/protobuf/testing/protocmp"

	"go.uber.org/zap"
	"testing"
)

type metricsGroupingTest struct {
	name       string // test name
	transforms []internalTransform
	in         consumerdata.MetricsData
	out        []consumerdata.MetricsData
}

var (
	groupingTests = []metricsGroupingTest{
		{
			name: "metric_group",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "foometric"},
					Action:              Group,
					GroupResourceLabels: map[string]string{"resource.type": "foo"},
				},
			},
			in:  consumerdata.MetricsData{
				Resource: &resourcepb.Resource{
					Labels: map[string]string{
						"original": "label",
					},
				},
				Metrics:  []*metricspb.Metric{
					metricBuilder().setName("foometric").setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
					metricBuilder().setName("barmetric").setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
				},
			},
			out: []consumerdata.MetricsData{
				{
					Resource: &resourcepb.Resource{
						Labels: map[string]string{
							"original": "label",
						},
					},
					Metrics:  []*metricspb.Metric{
						metricBuilder().setName("barmetric").setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
					},
				},
				{
					Resource: &resourcepb.Resource{
						Labels: map[string]string{
							"original": "label",
							"resource.type": "foo",
						},
					},
					Metrics:  []*metricspb.Metric{
						metricBuilder().setName("foometric").setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
					},
				},
			},
		},
	}
)

func TestMetricsGrouping(t *testing.T) {
	for _, test := range groupingTests {
		t.Run(test.name, func(t *testing.T) {
			next := new(consumertest.MetricsSink)
			p := newMetricsTransformProcessor(zap.NewExample(), test.transforms)

			mtp, err := processorhelper.NewMetricsProcessor(&Config{
				ProcessorSettings: configmodels.ProcessorSettings{
					TypeVal: typeStr,
					NameVal: typeStr,
				},
			}, next, p, processorhelper.WithCapabilities(processorCapabilities))

			require.NoError(t, err)

			caps := mtp.GetCapabilities()
			assert.Equal(t, true, caps.MutatesConsumedData)

			// process
			cErr := mtp.ConsumeMetrics(context.Background(), internaldata.OCToMetrics(test.in))
			assert.NoError(t, cErr)

			// get and check results

			got := next.AllMetrics()
			require.Equal(t, 1, len(got))

			gotMD := internaldata.MetricsToOC(got[0])
			require.Equal(t, len(test.out), len(gotMD))

			for idx, out := range gotMD {
				if diff := cmp.Diff(test.out[idx].Resource, out.Resource, protocmp.Transform()); diff != "" {
					t.Errorf("Unexpected difference:\n%v", diff)
				}

				if diff := cmp.Diff(test.out[idx].Metrics, out.Metrics, protocmp.Transform()); diff != "" {
					t.Errorf("Unexpected difference:\n%v", diff)
				}
 			}

			ctx := context.Background()
			assert.NoError(t, mtp.Shutdown(ctx))

		})
	}
}
