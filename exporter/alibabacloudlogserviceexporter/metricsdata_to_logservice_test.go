// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package alibabacloudlogserviceexporter

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/testutil/metricstestutil"
	"go.uber.org/zap"
)

func TestMetricDataToLogService(t *testing.T) {
	logger := zap.NewNop()

	keys := []string{"k0", "k1"}
	values := []string{"v0", "v1"}

	unixSecs := int64(1574092046)
	unixNSecs := int64(11 * time.Millisecond)
	tsUnix := time.Unix(unixSecs, unixNSecs)

	doubleVal := 1234.5678
	doublePt := metricstestutil.Double(tsUnix, doubleVal)
	int64Val := int64(123)
	int64Pt := &metricspb.Point{
		Timestamp: metricstestutil.Timestamp(tsUnix),
		Value:     &metricspb.Point_Int64Value{Int64Value: int64Val},
	}

	distributionBounds := []float64{1, 2, 4}
	distributionCounts := []int64{4, 2, 3, 7}
	distributionTimeSeries := metricstestutil.Timeseries(
		tsUnix,
		values,
		metricstestutil.DistPt(tsUnix, distributionBounds, distributionCounts))

	summaryTimeSeries := metricstestutil.Timeseries(
		tsUnix,
		values,
		metricstestutil.SummPt(
			tsUnix,
			11,
			111,
			[]float64{90, 95, 99, 99.9},
			[]float64{100, 6, 4, 1}))

	tests := []struct {
		name                     string
		metricsDataFn            func() consumerdata.MetricsData
		wantNumDroppedTimeseries int
	}{
		{
			name: "nil_node_nil_resources_nil_metric",
			metricsDataFn: func() consumerdata.MetricsData {
				return consumerdata.MetricsData{
					Metrics: []*metricspb.Metric{
						nil,
					},
				}
			},
		},
		{
			name: "nil_node_nil_resources_no_dims",
			metricsDataFn: func() consumerdata.MetricsData {
				return consumerdata.MetricsData{
					Metrics: []*metricspb.Metric{
						metricstestutil.Gauge("gauge_double_with_dims", nil, metricstestutil.Timeseries(tsUnix, nil, doublePt)),
						metricstestutil.GaugeInt("gauge_int_with_dims", nil, metricstestutil.Timeseries(tsUnix, nil, int64Pt)),
						metricstestutil.Cumulative("cumulative_double_with_dims", nil, metricstestutil.Timeseries(tsUnix, nil, doublePt)),
						metricstestutil.CumulativeInt("cumulative_int_with_dims", nil, metricstestutil.Timeseries(tsUnix, nil, int64Pt)),
					},
				}
			},
		},
		{
			name: "nil_node_and_resources_with_dims",
			metricsDataFn: func() consumerdata.MetricsData {
				return consumerdata.MetricsData{
					Metrics: []*metricspb.Metric{
						metricstestutil.Gauge("gauge_double_with_dims", keys, metricstestutil.Timeseries(tsUnix, values, doublePt)),
						metricstestutil.GaugeInt("gauge_int_with_dims", keys, metricstestutil.Timeseries(tsUnix, values, int64Pt)),
						metricstestutil.Cumulative("cumulative_double_with_dims", keys, metricstestutil.Timeseries(tsUnix, values, doublePt)),
						metricstestutil.CumulativeInt("cumulative_int_with_dims", keys, metricstestutil.Timeseries(tsUnix, values, int64Pt)),
					},
				}
			},
		},
		{
			name: "with_node_resources_dims",
			metricsDataFn: func() consumerdata.MetricsData {
				return consumerdata.MetricsData{
					Node: &commonpb.Node{
						Attributes: map[string]string{
							"k/n0": "vn0",
							"k/n1": "vn1",
						},
						Identifier: &commonpb.ProcessIdentifier{
							HostName: "host",
							Pid:      123,
						},
					},
					Resource: &resourcepb.Resource{
						Labels: map[string]string{
							"k/r0": "vr0",
							"k/r1": "vr1",
						},
						Type: "service",
					},
					Metrics: []*metricspb.Metric{
						metricstestutil.Gauge("gauge_double_with_dims", keys, metricstestutil.Timeseries(tsUnix, values, doublePt)),
						metricstestutil.GaugeInt("gauge_int_with_dims", keys, metricstestutil.Timeseries(tsUnix, values, int64Pt)),
					},
				}
			},
		},
		{
			name: "distributions",
			metricsDataFn: func() consumerdata.MetricsData {
				return consumerdata.MetricsData{
					Metrics: []*metricspb.Metric{
						metricstestutil.GaugeDist("gauge_distrib", keys, distributionTimeSeries),
						metricstestutil.CumulativeDist("cumulative_distrib", keys, distributionTimeSeries),
					},
				}
			},
		},
		{
			name: "summary",
			metricsDataFn: func() consumerdata.MetricsData {
				return consumerdata.MetricsData{
					Metrics: []*metricspb.Metric{
						metricstestutil.Summary("summary", keys, summaryTimeSeries),
					},
				}
			},
		},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLogs, gotNumDroppedTimeSeries := metricsDataToLogServiceData(logger, tt.metricsDataFn())
			if i == 0 {
				assert.Equal(t, len(gotLogs), 0)
				return
			}
			if gotNumDroppedTimeSeries != tt.wantNumDroppedTimeseries {
				t.Errorf("Converting metrics data to LogSerive data failed, (got : %d, want : %d)\n", gotNumDroppedTimeSeries, tt.wantNumDroppedTimeseries)
				return
			}
			gotLogPairs := make([][]logKeyValuePair, 0, len(gotLogs))

			for _, log := range gotLogs {
				pairs := make([]logKeyValuePair, 0, len(log.Contents))
				//fmt.Println(log.GetTime())
				for _, content := range log.Contents {
					pairs = append(pairs, logKeyValuePair{
						Key:   content.GetKey(),
						Value: content.GetValue(),
					})
					//fmt.Printf("%s : %s\n", content.GetKey(), content.GetValue())
				}
				gotLogPairs = append(gotLogPairs, pairs)

				//fmt.Println("#################")
			}
			//str, _ := json.Marshal(gotLogPairs)
			//fmt.Println(string(str))

			resultLogFile := fmt.Sprintf("./testdata/logservice_metric_data_%02d.json", i)

			wantLogs := make([][]logKeyValuePair, 0, gotNumDroppedTimeSeries)

			if err := loadFromJSON(resultLogFile, &wantLogs); err != nil {
				t.Errorf("Failed load log key value pairs from %q: %v", resultLogFile, err)
				return
			}
			for j := 0; j < gotNumDroppedTimeSeries; j++ {
				sort.Sort(logKeyValuePairs(gotLogPairs[j]))
				sort.Sort(logKeyValuePairs(wantLogs[j]))
				if !reflect.DeepEqual(gotLogPairs[j], wantLogs[j]) {
					t.Errorf("Unsuccessful conversion %d \nGot:\n\t%v\nWant:\n\t%v", i, gotLogPairs, wantLogs)
				}
			}
		})
	}
}

func TestInvalidMetric(t *testing.T) {
	logger := zap.NewNop()

	unixSecs := int64(1574092046)
	unixNSecs := int64(11 * time.Millisecond)
	tsUnix := time.Unix(unixSecs, unixNSecs)

	doubleVal := 1234.5678
	doublePt := metricstestutil.Double(tsUnix, doubleVal)
	metricData := &consumerdata.MetricsData{
		Metrics: []*metricspb.Metric{
			metricstestutil.Gauge("gauge_double_with_dims", nil, metricstestutil.Timeseries(tsUnix, nil, doublePt)),
		},
	}
	// invalid timestamp
	rawTS := metricData.Metrics[0].Timeseries[0].Points[0].Timestamp
	metricData.Metrics[0].Timeseries[0].Points[0].Timestamp = nil
	gotLogs, gotNumDroppedTimeSeries := metricsDataToLogServiceData(logger, *metricData)
	metricData.Metrics[0].Timeseries[0].Points[0].Timestamp = rawTS
	assert.Equal(t, gotNumDroppedTimeSeries, 1)
	assert.Equal(t, len(gotLogs), 0)

	// invalid distribution
	rawValue := metricData.Metrics[0].Timeseries[0].Points[0].Value
	metricData.Metrics[0].Timeseries[0].Points[0].Value = &metricspb.Point_DistributionValue{
		DistributionValue: &metricspb.DistributionValue{},
	}
	gotLogs, gotNumDroppedTimeSeries = metricsDataToLogServiceData(logger, *metricData)
	metricData.Metrics[0].Timeseries[0].Points[0].Value = rawValue
	assert.Equal(t, gotNumDroppedTimeSeries, 1)
	assert.Equal(t, len(gotLogs), 2)

	// invalid summary
	rawValue = metricData.Metrics[0].Timeseries[0].Points[0].Value
	metricData.Metrics[0].Timeseries[0].Points[0].Value = &metricspb.Point_SummaryValue{
		SummaryValue: &metricspb.SummaryValue{},
	}
	gotLogs, gotNumDroppedTimeSeries = metricsDataToLogServiceData(logger, *metricData)
	metricData.Metrics[0].Timeseries[0].Points[0].Value = rawValue
	assert.Equal(t, gotNumDroppedTimeSeries, 1)
	assert.Equal(t, len(gotLogs), 2)

	// invalid value type
	rawValue = metricData.Metrics[0].Timeseries[0].Points[0].Value
	metricData.Metrics[0].Timeseries[0].Points[0].Value = nil
	gotLogs, gotNumDroppedTimeSeries = metricsDataToLogServiceData(logger, *metricData)
	metricData.Metrics[0].Timeseries[0].Points[0].Value = rawValue
	assert.Equal(t, gotNumDroppedTimeSeries, 1)
	assert.Equal(t, len(gotLogs), 0)
}

func TestMetricCornerCases(t *testing.T) {
	assert.Equal(t, min(1, 2), 1)
	assert.Equal(t, min(2, 1), 1)
	assert.Equal(t, min(1, 1), 1)

	nameContent := &sls.LogContent{
		Key:   proto.String(metricNameKey),
		Value: proto.String("test_name"),
	}
	labelsContent := &sls.LogContent{
		Key:   proto.String(labelsKey),
		Value: proto.String("test_labels"),
	}
	timeNanoContent := &sls.LogContent{
		Key:   proto.String(timeNanoKey),
		Value: proto.String("123"),
	}
	distributionValue := &metricspb.DistributionValue{}
	logs, err := appendDistributionValues(nil, KeyValues{}, 1, nameContent, labelsContent, timeNanoContent, distributionValue)
	assert.Error(t, err)
	assert.Equal(t, len(logs), 2)

	summaryValue := &metricspb.SummaryValue{}
	logs, err = appendSummaryValues(nil, KeyValues{}, 1, nameContent, labelsContent, timeNanoContent, summaryValue)
	assert.Error(t, err)
	assert.Equal(t, len(logs), 2)

}
