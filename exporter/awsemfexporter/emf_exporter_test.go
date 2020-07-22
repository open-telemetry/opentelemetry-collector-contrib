package awsemfexporter

import (
	"context"
	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"
	"go.uber.org/zap"
	"testing"
	"time"
)

func TestGetPusher(t *testing.T) {
	logs := &emfExporter{
		groupStreamToPusherMap: map[string]map[string]Pusher{},
		ForceFlushInterval:     time.Second,
	}
	pusher := logs.getPusher("test_log_group", "test_log_stream", nil)
	assert.NotEqual(t, nil, pusher)
}

func TestPushMetricsData(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	factory := new(Factory)
	expCfg := factory.CreateDefaultConfig().(*Config)
	expCfg.Region = "us-west-2"
	exp, err := New(expCfg, component.ExporterCreateParams{Logger: zap.NewNop()})
	assert.Nil(t, err)
	assert.NotNil(t, exp)

	mdPrev := consumerdata.MetricsData{
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
	mdCur := consumerdata.MetricsData{
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
									Seconds: 110,
								},
								Value: &metricspb.Point_Int64Value{
									Int64Value: 12,
								},
							},
						},
					},
				},
			},
		},
	}
	md := pdatautil.MetricsFromMetricsData([]consumerdata.MetricsData{mdPrev})
	require.NoError(t, exp.ConsumeMetrics(ctx, md))
	md = pdatautil.MetricsFromMetricsData([]consumerdata.MetricsData{mdCur})
	time.Sleep(10 * time.Second)
	require.NoError(t, exp.ConsumeMetrics(ctx, md))
	require.NoError(t, exp.Shutdown(ctx))
}