package awsecscontainermetrics

import (
	"fmt"
	"io/ioutil"
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumerdata"
)

type fakeRestClient struct {
}

func (f fakeRestClient) EndpointResponse() ([]byte, error) {
	// data, _ := ioutil.ReadFile("../testdata/sample.json")
	// fmt.Println(string(data))
	return ioutil.ReadFile("../testdata/task_stats.json")
}

func TestMetricSampleFile(t *testing.T) {
	data, err := ioutil.ReadFile("../testdata/sample.json")
	// fmt.Println(string(data))
	require.NoError(t, err)
	require.NotNil(t, data)
}

func TestMetricAccumulator(t *testing.T) {
	provider := NewStatsProvider(&fakeRestClient{})
	statsMap, _ := provider.TaskStats()
	//fmt.Printf("%v\n", stats)

	fmt.Println("map size", len(statsMap))

	require.NotNil(t, statsMap)
}

func requireMetricsDataOk(t *testing.T, mds []*consumerdata.MetricsData) {
	for _, md := range mds {
		requireResourceOk(t, md.Resource)
		for _, metric := range md.Metrics {
			requireDescriptorOk(t, metric.MetricDescriptor)
			for _, ts := range metric.Timeseries {
				requireTimeSeriesOk(t, ts)
			}
		}
	}
}

func requireResourceOk(t *testing.T, resource *resourcepb.Resource) {
	require.True(t, resource.Type != "")
	require.NotNil(t, resource.Labels)
}

func requireDescriptorOk(t *testing.T, desc *metricspb.MetricDescriptor) {
	require.True(t, desc.Name != "")
	require.True(t, desc.Type != metricspb.MetricDescriptor_UNSPECIFIED)
}

func requireTimeSeriesOk(t *testing.T, ts *metricspb.TimeSeries) {
	require.True(t, ts.StartTimestamp.Seconds > 0)
	for _, point := range ts.Points {
		requirePointOk(t, point, ts)
	}
}

func requirePointOk(t *testing.T, point *metricspb.Point, ts *metricspb.TimeSeries) {
	require.True(t, point.Timestamp.Seconds > ts.StartTimestamp.Seconds)
	require.NotNil(t, point.Value)
}
