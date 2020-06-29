// Copyright 2019 OpenTelemetry Authors
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

package signalfxexporter

import (
	"math"
	"sort"
	"testing"
	"time"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/testutil/metricstestutil"
	"go.uber.org/zap"
)

func Test_metricDataToSignalFxV2(t *testing.T) {
	logger := zap.NewNop()

	keys := []string{"k0", "k1"}
	values := []string{"v0", "v1"}

	unixSecs := int64(1574092046)
	unixNSecs := int64(11 * time.Millisecond)
	tsUnix := time.Unix(unixSecs, unixNSecs)
	tsMSecs := unixSecs*1e3 + unixNSecs/1e6

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
		wantSfxDataPoints        []*sfxpb.DataPoint
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
			wantSfxDataPoints: []*sfxpb.DataPoint{
				doubleSFxDataPoint("gauge_double_with_dims", tsMSecs, &sfxMetricTypeGauge, []string{}, []string{}, doubleVal),
				int64SFxDataPoint("gauge_int_with_dims", tsMSecs, &sfxMetricTypeGauge, []string{}, []string{}, int64Val),
				doubleSFxDataPoint("cumulative_double_with_dims", tsMSecs, &sfxMetricTypeCumulativeCounter, []string{}, []string{}, doubleVal),
				int64SFxDataPoint("cumulative_int_with_dims", tsMSecs, &sfxMetricTypeCumulativeCounter, []string{}, []string{}, int64Val),
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
			wantSfxDataPoints: []*sfxpb.DataPoint{
				doubleSFxDataPoint("gauge_double_with_dims", tsMSecs, &sfxMetricTypeGauge, keys, values, doubleVal),
				int64SFxDataPoint("gauge_int_with_dims", tsMSecs, &sfxMetricTypeGauge, keys, values, int64Val),
				doubleSFxDataPoint("cumulative_double_with_dims", tsMSecs, &sfxMetricTypeCumulativeCounter, keys, values, doubleVal),
				int64SFxDataPoint("cumulative_int_with_dims", tsMSecs, &sfxMetricTypeCumulativeCounter, keys, values, int64Val),
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
			wantSfxDataPoints: []*sfxpb.DataPoint{
				doubleSFxDataPoint(
					"gauge_double_with_dims",
					tsMSecs,
					&sfxMetricTypeGauge,
					append([]string{"k_n0", "k_n1", "k_r0", "k_r1"}, keys...),
					append([]string{"vn0", "vn1", "vr0", "vr1"}, values...),
					doubleVal),
				int64SFxDataPoint(
					"gauge_int_with_dims",
					tsMSecs,
					&sfxMetricTypeGauge,
					append([]string{"k_n0", "k_n1", "k_r0", "k_r1"}, keys...),
					append([]string{"vn0", "vn1", "vr0", "vr1"}, values...),
					int64Val),
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
			wantSfxDataPoints: append(
				expectedFromDistribution("gauge_distrib", tsMSecs, keys, values, distributionTimeSeries),
				expectedFromDistribution("cumulative_distrib", tsMSecs, keys, values, distributionTimeSeries)...),
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
			wantSfxDataPoints: expectedFromSummary("summary", tsMSecs, keys, values, summaryTimeSeries),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSfxDataPoints, gotNumDroppedTimeSeries, err := metricDataToSignalFxV2(logger, tt.metricsDataFn())
			assert.NoError(t, err)
			assert.Equal(t, tt.wantNumDroppedTimeseries, gotNumDroppedTimeSeries)
			// Sort SFx dimensions since they are built from maps and the order
			// of those is not deterministic.
			sortDimensions(tt.wantSfxDataPoints)
			sortDimensions(gotSfxDataPoints)
			assert.Equal(t, tt.wantSfxDataPoints, gotSfxDataPoints)
		})
	}
}

func sortDimensions(points []*sfxpb.DataPoint) {
	for _, point := range points {
		if point.Dimensions == nil {
			continue
		}
		sort.Slice(point.Dimensions, func(i, j int) bool {
			return *point.Dimensions[i].Key < *point.Dimensions[j].Key
		})
	}
}

func doubleSFxDataPoint(
	metric string,
	ts int64,
	metricType *sfxpb.MetricType,
	keys []string,
	values []string,
	val float64,
) *sfxpb.DataPoint {
	pt := commonSFxDataPoint(
		metric,
		ts,
		metricType,
		keys,
		values)
	pt.Value = &sfxpb.Datum{DoubleValue: &val}
	return pt
}

func int64SFxDataPoint(
	metric string,
	ts int64,
	metricType *sfxpb.MetricType,
	keys []string,
	values []string,
	val int64,
) *sfxpb.DataPoint {
	pt := commonSFxDataPoint(
		metric,
		ts,
		metricType,
		keys,
		values)
	pt.Value = &sfxpb.Datum{IntValue: &val}
	return pt
}

func commonSFxDataPoint(
	metric string,
	ts int64,
	metricType *sfxpb.MetricType,
	keys []string,
	values []string,
) *sfxpb.DataPoint {
	return &sfxpb.DataPoint{
		Metric:     &metric,
		Timestamp:  &ts,
		MetricType: metricType,
		Dimensions: sfxDimensions(keys, values),
	}
}

func sfxDimensions(keys, values []string) []*sfxpb.Dimension {
	if len(keys) != len(values) {
		panic("keys and values do not match")
	}

	if keys == nil && values == nil {
		return nil
	}

	sfxDims := make([]*sfxpb.Dimension, len(keys))
	for i := 0; i < len(keys); i++ {
		sfxDims[i] = &sfxpb.Dimension{
			Key:   &keys[i],
			Value: &values[i],
		}
	}

	return sfxDims
}

func expectedFromDistribution(
	metricName string,
	ts int64,
	keys []string,
	values []string,
	distributionTimeSeries *metricspb.TimeSeries,
) []*sfxpb.DataPoint {
	distributionValue := distributionTimeSeries.Points[0].GetDistributionValue()

	// Two additional data points: one for count and one for sum.
	const extraDataPoints = 2
	dps := make([]*sfxpb.DataPoint, 0, len(distributionValue.Buckets)+extraDataPoints)

	dps = append(dps,
		int64SFxDataPoint(metricName+"_count", ts, &sfxMetricTypeCumulativeCounter, keys, values,
			distributionValue.Count),
		doubleSFxDataPoint(metricName, ts, &sfxMetricTypeCumulativeCounter, keys, values,
			distributionValue.Sum))

	explicitBuckets := distributionValue.BucketOptions.GetExplicit()
	for i := 0; i < len(explicitBuckets.Bounds); i++ {
		dps = append(dps,
			int64SFxDataPoint(metricName+"_bucket", ts, &sfxMetricTypeCumulativeCounter,
				append(keys, upperBoundDimensionKey),
				append(values, *float64ToDimValue(explicitBuckets.Bounds[i])),
				distributionValue.Buckets[i].Count))
	}
	dps = append(dps,
		int64SFxDataPoint(metricName+"_bucket", ts, &sfxMetricTypeCumulativeCounter,
			append(keys, upperBoundDimensionKey),
			append(values, *float64ToDimValue(math.Inf(1))),
			distributionValue.Buckets[len(distributionValue.Buckets)-1].Count))
	return dps
}

func expectedFromSummary(
	metricName string,
	ts int64,
	keys []string,
	values []string,
	summaryTimeSeries *metricspb.TimeSeries,
) []*sfxpb.DataPoint {
	summaryValue := summaryTimeSeries.Points[0].GetSummaryValue()

	// Two additional data points: one for count and one for sum.
	const extraDataPoints = 2
	dps := make([]*sfxpb.DataPoint, 0, len(summaryValue.Snapshot.PercentileValues)+extraDataPoints)

	dps = append(dps,
		int64SFxDataPoint(metricName+"_count", ts, &sfxMetricTypeCumulativeCounter, keys, values,
			summaryValue.Count.Value),
		doubleSFxDataPoint(metricName, ts, &sfxMetricTypeCumulativeCounter, keys, values,
			summaryValue.Sum.Value))

	percentiles := summaryValue.Snapshot.GetPercentileValues()
	for _, percentile := range percentiles {
		dps = append(dps,
			doubleSFxDataPoint(metricName+"_quantile", ts, &sfxMetricTypeGauge,
				append(keys, quantileDimensionKey),
				append(values, *float64ToDimValue(percentile.Percentile)),
				percentile.Value))
	}

	return dps
}

func Test_InvalidDistribution_NoExplicitBuckets(t *testing.T) {
	logger := zap.NewNop()
	unixSecs := int64(1574092046)
	unixNSecs := int64(11 * time.Millisecond)
	tsUnix := time.Unix(unixSecs, unixNSecs)
	keys := []string{"k0", "k1"}
	values := []string{"v0", "v1"}
	buckets := make([]*metricspb.DistributionValue_Bucket, 2)

	distrValue := &metricspb.DistributionValue{
		BucketOptions: &metricspb.DistributionValue_BucketOptions{
			Type: &metricspb.DistributionValue_BucketOptions_Explicit_{
				Explicit: nil,
			},
		},
		Count:   42,
		Sum:     42,
		Buckets: buckets,
	}
	point := &metricspb.Point{Timestamp: metricstestutil.Timestamp(tsUnix), Value: &metricspb.Point_DistributionValue{DistributionValue: distrValue}}
	point.GetDistributionValue().BucketOptions.GetExplicit()
	metricData := consumerdata.MetricsData{
		Metrics: []*metricspb.Metric{
			metricstestutil.GaugeDist("gauge_distrib", keys, metricstestutil.Timeseries(
				tsUnix,
				values,
				point)),
		},
	}
	_, gotNumDroppedTimeSeries, err := metricDataToSignalFxV2(logger, metricData)
	assert.Equal(t, 1, gotNumDroppedTimeSeries)
	assert.NoError(t, err)
}
