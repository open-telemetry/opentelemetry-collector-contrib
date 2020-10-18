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

package splunkhecexporter

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/testutil/metricstestutil"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
)

func Test_metricDataToSplunk(t *testing.T) {
	logger := zap.NewNop()

	keys := []string{"k0", "k1"}
	values := []string{"v0", "v1"}

	unixSecs := int64(1574092046)
	unixNSecs := int64(11 * time.Millisecond)
	tsUnix := time.Unix(unixSecs, unixNSecs)
	tsMSecs := timestampToEpochMilliseconds(&timestamppb.Timestamp{Seconds: unixSecs, Nanos: int32(unixNSecs)})

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

	tests := []struct {
		name                     string
		metricsDataFn            func() consumerdata.MetricsData
		wantSplunkMetrics        []*splunk.Event
		wantNumDroppedTimeseries int
	}{
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
			wantSplunkMetrics: []*splunk.Event{
				commonSplunkMetric("gauge_double_with_dims", tsMSecs, []string{}, []string{}, doubleVal),
				commonSplunkMetric("gauge_int_with_dims", tsMSecs, []string{}, []string{}, int64Val),
				commonSplunkMetric("cumulative_double_with_dims", tsMSecs, []string{}, []string{}, doubleVal),
				commonSplunkMetric("cumulative_int_with_dims", tsMSecs, []string{}, []string{}, int64Val),
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
			wantSplunkMetrics: []*splunk.Event{
				commonSplunkMetric("gauge_double_with_dims", tsMSecs, keys, values, doubleVal),
				commonSplunkMetric("gauge_int_with_dims", tsMSecs, keys, values, int64Val),
				commonSplunkMetric("cumulative_double_with_dims", tsMSecs, keys, values, doubleVal),
				commonSplunkMetric("cumulative_int_with_dims", tsMSecs, keys, values, int64Val),
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
					},
					Resource: &resourcepb.Resource{
						Labels: map[string]string{
							"k/r0": "vr0",
							"k/r1": "vr1",
						},
					},
					Metrics: []*metricspb.Metric{
						metricstestutil.Gauge("gauge_double_with_dims", keys, metricstestutil.Timeseries(tsUnix, values, doublePt)),
						metricstestutil.GaugeInt("gauge_int_with_dims", keys, metricstestutil.Timeseries(tsUnix, values, int64Pt)),
					},
				}
			},
			wantSplunkMetrics: []*splunk.Event{
				commonSplunkMetric(
					"gauge_double_with_dims",
					tsMSecs,
					append([]string{"k/n0", "k/n1", "k/r0", "k/r1"}, keys...),
					append([]string{"vn0", "vn1", "vr0", "vr1"}, values...),
					doubleVal),
				commonSplunkMetric(
					"gauge_int_with_dims",
					tsMSecs,
					append([]string{"k/n0", "k/n1", "k/r0", "k/r1"}, keys...),
					append([]string{"vn0", "vn1", "vr0", "vr1"}, values...),
					int64Val),
			},
		},
		{
			name: "distributions",
			metricsDataFn: func() consumerdata.MetricsData {
				return consumerdata.MetricsData{
					Metrics: []*metricspb.Metric{
						// metricstestutil.GaugeDist("gauge_distrib", keys, distributionTimeSeries),
						metricstestutil.CumulativeDist("cumulative_distrib", keys, distributionTimeSeries),
					},
				}
			},
			wantSplunkMetrics:
			// expectedFromDistribution("gauge_distrib", tsMSecs, keys, values, distributionTimeSeries),
			expectedFromDistribution("cumulative_distrib", tsMSecs, keys, values, distributionTimeSeries),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := internaldata.OCToMetrics(tt.metricsDataFn())
			gotMetrics, gotNumDroppedTimeSeries, err := metricDataToSplunk(logger, md, &Config{})
			assert.NoError(t, err)
			assert.Equal(t, tt.wantNumDroppedTimeseries, gotNumDroppedTimeSeries)
			assert.Equal(t, len(tt.wantSplunkMetrics), len(gotMetrics))
			sortMetrics(tt.wantSplunkMetrics)
			sortMetrics(gotMetrics)
			for i, want := range tt.wantSplunkMetrics {
				assert.Equal(t, want, gotMetrics[i])
			}
			assert.Equal(t, tt.wantSplunkMetrics, gotMetrics)
		})
	}
}

func sortMetrics(metrics []*splunk.Event) {
	sort.Slice(metrics, func(p, q int) bool {
		firstField := getFieldValue(metrics[p])
		secondField := getFieldValue(metrics[q])
		return strings.Compare(firstField, secondField) > 0
	})
}

func getFieldValue(metric *splunk.Event) string {
	for k := range metric.Fields {
		if strings.HasPrefix(k, "metric_name:") {
			return k
		}
	}
	return ""
}

func commonSplunkMetric(
	metricName string,
	ts float64,
	keys []string,
	values []string,
	val interface{},
) *splunk.Event {
	fields := map[string]interface{}{fmt.Sprintf("metric_name:%s", metricName): val}

	for i, k := range keys {
		fields[k] = values[i]
	}

	return &splunk.Event{
		Time:   ts,
		Host:   "unknown",
		Event:  "metric",
		Fields: fields,
	}
}

func expectedFromDistribution(
	metricName string,
	ts float64,
	keys []string,
	values []string,
	distributionTimeSeries *metricspb.TimeSeries,
) []*splunk.Event {
	distributionValue := distributionTimeSeries.Points[0].GetDistributionValue()

	// Three additional data points: one for count, one for sum and one for sum of squared deviation.
	const extraDataPoints = 3
	dps := make([]*splunk.Event, 0, len(distributionValue.Buckets)+extraDataPoints)

	dps = append(dps,
		commonSplunkMetric(metricName, ts, keys, values,
			distributionValue.Sum),
		commonSplunkMetric(metricName+".count", ts, keys, values,
			distributionValue.Count),
		commonSplunkMetric(metricName+".sum_of_squared_deviation", ts, keys, values,
			distributionValue.SumOfSquaredDeviation))

	explicitBuckets := distributionValue.BucketOptions.GetExplicit()
	splunkBounds := make([]string, len(explicitBuckets.Bounds)+1)
	for i := 0; i < len(explicitBuckets.Bounds); i++ {
		splunkBounds[i] = float64ToDimValue(explicitBuckets.Bounds[i])
	}
	splunkBounds[len(splunkBounds)-1] = infinityBoundSFxDimValue
	for i := 0; i < len(splunkBounds); i++ {
		dps = append(dps,
			commonSplunkMetric(fmt.Sprintf("%s.bucket.%s", metricName, splunkBounds[i]), ts,
				keys,
				values,
				distributionValue.Buckets[i].Count))
	}
	return dps
}

func TestTimestampFormat(t *testing.T) {
	ts := timestamppb.Timestamp{Seconds: 32, Nanos: 1000345}
	assert.Equal(t, 32.001, timestampToEpochMilliseconds(&ts))
}

func TestTimestampFormatRounding(t *testing.T) {
	ts := timestamppb.Timestamp{Seconds: 32, Nanos: 1999345}
	assert.Equal(t, 32.002, timestampToEpochMilliseconds(&ts))
}
