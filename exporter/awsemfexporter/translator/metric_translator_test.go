package translator

import (
	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"
	"testing"
)

func TestTranslateOtToCWMetric(t *testing.T) {
	md := consumerdata.MetricsData{
		Node: &commonpb.Node{
			ServiceInfo: &commonpb.ServiceInfo{Name: "test-emf"},
			LibraryInfo: &commonpb.LibraryInfo{ExporterVersion: "SomeVersion"},
		},
		Resource: &resourcepb.Resource{
			Labels: map[string]string{
				"resource": "R1",
			},
		},
		Metrics: []*metricspb.Metric{
			{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name:        "spanCounter",
					Description: "Counting all the spans",
					Unit:        "Count",
					Type:        metricspb.MetricDescriptor_CUMULATIVE_INT64,
					LabelKeys: []*metricspb.LabelKey{
						{Key: "spanName"},
						{Key: "isItAnError"},
					},
				},
				Timeseries: []*metricspb.TimeSeries{
					{
						LabelValues: []*metricspb.LabelValue{
							{Value: "testSpan"},
							{Value: "false"},
						},
						Points: []*metricspb.Point{
							{
								Timestamp: &timestamp.Timestamp{
									Seconds: 100,
								},
								Value: &metricspb.Point_Int64Value{
									Int64Value: 1,
								},
							},
						},
					},
				},
			},
		},
	}
	metric := pdatautil.MetricsFromMetricsData([]consumerdata.MetricsData{md})
	imd := pdatautil.MetricsToInternalMetrics(metric)
	rms := imd.ResourceMetrics()
	assert.NotNil(t, rms)

	rm := rms.At(0)
	assert.NotNil(t, rm)

	cwm, err := TranslateOtToCWMetric(&rm)
	assert.Nil(t, err)
	assert.NotNil(t, cwm)
	assert.Equal(t, len(cwm), 1)
	assert.Equal(t, len(cwm[0].Measurements), 1)

	met := cwm[0]
	assert.Equal(t, met.Fields[OtlibDimensionKey], noInstrumentationLibraryName)
	assert.Equal(t, met.Fields["spanCounter"], 0)

	assert.Equal(t, met.Measurements[0].Namespace, "test-emf")
	assert.Equal(t, len(met.Measurements[0].Dimensions), 1)
	assert.Equal(t, met.Measurements[0].Dimensions[0], []string{OtlibDimensionKey})
	assert.Equal(t, len(met.Measurements[0].Metrics), 1)
	assert.Equal(t, met.Measurements[0].Metrics[0]["Name"], "spanCounter")
	assert.Equal(t, met.Measurements[0].Metrics[0]["Unit"], "Count")
}
