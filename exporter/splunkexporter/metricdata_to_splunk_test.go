package splunkexporter


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
	"go.opentelemetry.io/collector/testutils/metricstestutils"
	"go.uber.org/zap"
)

func Test_metricDataToSplunk(t *testing.T) {
	logger := zap.NewNop()

	keys := []string{"k0", "k1"}
	values := []string{"v0", "v1"}

	unixSecs := int64(1574092046)
	unixNSecs := int64(11 * time.Millisecond)
	tsUnix := time.Unix(unixSecs, unixNSecs)
	tsMSecs := unixSecs*1e3 + unixNSecs/1e6

	doubleVal := 1234.5678
	doublePt := metricstestutils.Double(tsUnix, doubleVal)
	int64Val := int64(123)
	int64Pt := &metricspb.Point{
		Timestamp: metricstestutils.Timestamp(tsUnix),
		Value:     &metricspb.Point_Int64Value{Int64Value: int64Val},
	}

	distributionBounds := []float64{1, 2, 4}
	distributionCounts := []int64{4, 2, 3, 7}
	distributionTimeSeries := metricstestutils.Timeseries(
		tsUnix,
		values,
		metricstestutils.DistPt(tsUnix, distributionBounds, distributionCounts))

	summaryTimeSeries := metricstestutils.Timeseries(
		tsUnix,
		values,
		metricstestutils.SummPt(
			tsUnix,
			11,
			111,
			[]float64{90, 95, 99, 99.9},
			[]float64{100, 6, 4, 1}))

	tests := []struct {
		name                     string
		metricsDataFn            func() consumerdata.MetricsData
		wantSplunkMetrics        []*splunkMetric
		wantNumDroppedTimeseries int
	}{
		{
			name: "nil_node_nil_resources_no_dims",
			metricsDataFn: func() consumerdata.MetricsData {
				return consumerdata.MetricsData{
					Metrics: []*metricspb.Metric{
						metricstestutils.Gauge("gauge_double_with_dims", nil, metricstestutils.Timeseries(tsUnix, nil, doublePt)),
						metricstestutils.GaugeInt("gauge_int_with_dims", nil, metricstestutils.Timeseries(tsUnix, nil, int64Pt)),
						metricstestutils.Cumulative("cumulative_double_with_dims", nil, metricstestutils.Timeseries(tsUnix, nil, doublePt)),
						metricstestutils.CumulativeInt("cumulative_int_with_dims", nil, metricstestutils.Timeseries(tsUnix, nil, int64Pt)),
					},
				}
			},
			wantSplunkMetrics: []*splunkMetric{
				commonSplunkMetric("gauge_double_with_dims", tsMSecs, []string{}, []string{}, doubleVal),
				commonSplunkMetric("gauge_int_with_dims", tsMSecs,[]string{}, []string{}, int64Val),
				commonSplunkMetric("cumulative_double_with_dims", tsMSecs, []string{}, []string{}, doubleVal),
				commonSplunkMetric("cumulative_int_with_dims", tsMSecs, []string{}, []string{}, int64Val),
			},
		},
		{
			name: "nil_node_and_resources_with_dims",
			metricsDataFn: func() consumerdata.MetricsData {
				return consumerdata.MetricsData{
					Metrics: []*metricspb.Metric{
						metricstestutils.Gauge("gauge_double_with_dims", keys, metricstestutils.Timeseries(tsUnix, values, doublePt)),
						metricstestutils.GaugeInt("gauge_int_with_dims", keys, metricstestutils.Timeseries(tsUnix, values, int64Pt)),
						metricstestutils.Cumulative("cumulative_double_with_dims", keys, metricstestutils.Timeseries(tsUnix, values, doublePt)),
						metricstestutils.CumulativeInt("cumulative_int_with_dims", keys, metricstestutils.Timeseries(tsUnix, values, int64Pt)),
					},
				}
			},
			wantSplunkMetrics: []*splunkMetric{
				commonSplunkMetric("gauge_double_with_dims", tsMSecs,  keys, values, doubleVal),
				commonSplunkMetric("gauge_int_with_dims", tsMSecs,  keys,values, int64Val),
				commonSplunkMetric("cumulative_double_with_dims", tsMSecs, keys, values, doubleVal),
				commonSplunkMetric("cumulative_int_with_dims", tsMSecs,keys, values, int64Val),
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
						metricstestutils.Gauge("gauge_double_with_dims", keys, metricstestutils.Timeseries(tsUnix, values, doublePt)),
						metricstestutils.GaugeInt("gauge_int_with_dims", keys, metricstestutils.Timeseries(tsUnix, values, int64Pt)),
					},
				}
			},
			wantSplunkMetrics: []*splunkMetric{
				commonSplunkMetric(
					"gauge_double_with_dims",
					tsMSecs,
					append([]string{"k_n0", "k_n1", "k_r0", "k_r1"}, keys...),
					append([]string{"vn0", "vn1", "vr0", "vr1"}, values...),
					doubleVal),
				commonSplunkMetric(
					"gauge_int_with_dims",
					tsMSecs,
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
						metricstestutils.GaugeDist("gauge_distrib", keys, distributionTimeSeries),
						metricstestutils.CumulativeDist("cumulative_distrib", keys, distributionTimeSeries),
					},
				}
			},
			wantSplunkMetrics: append(
				expectedFromDistribution("gauge_distrib", tsMSecs, keys, values, distributionTimeSeries),
				expectedFromDistribution("cumulative_distrib", tsMSecs, keys, values, distributionTimeSeries)...),
		},
		{
			name: "summary",
			metricsDataFn: func() consumerdata.MetricsData {
				return consumerdata.MetricsData{
					Metrics: []*metricspb.Metric{
						metricstestutils.Summary("summary", keys, summaryTimeSeries),
					},
				}
			},
			wantSplunkMetrics: expectedFromSummary("summary", tsMSecs, keys, values, summaryTimeSeries),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMetrics, gotNumDroppedTimeSeries, err := metricDataToSplunk(logger, tt.metricsDataFn(), Config{
				Source: "source", SourceType: "sourceType", Index : "myIndex",
			})
			assert.NoError(t, err)
			assert.Equal(t, tt.wantNumDroppedTimeseries, gotNumDroppedTimeSeries)
			// Sort SFx dimensions since they are built from maps and the order
			// of those is not deterministic.
			sortDimensions(tt.wantSplunkMetrics)
			sortDimensions(gotMetrics)
			assert.Equal(t, tt.wantSplunkMetrics, gotMetrics)
		})
	}
}

func sortDimensions(points []*splunkMetric) {
	for _, point := range points {
		if point.Dimensions == nil {
			continue
		}
		sort.Slice(point.Dimensions, func(i, j int) bool {
			return *point.Dimensions[i].Key < *point.Dimensions[j].Key
		})
	}
}

func commonSplunkMetric(
	metricName string,
	ts int64,
	keys []string,
	values []string,
	val interface{},
) *splunkMetric {
	return &splunkMetric{
		Time: ts,
		// TODO map over rest
	}
}

func expectedFromDistribution(
	metricName string,
	ts int64,
	keys []string,
	values []string,
	distributionTimeSeries *metricspb.TimeSeries,
) []*splunkMetric{
	distributionValue := distributionTimeSeries.Points[0].GetDistributionValue()

	// Two additional data points: one for count and one for sum.
	const extraDataPoints = 2
	dps := make([]*sfxpb.DataPoint, 0, len(distributionValue.Buckets)+extraDataPoints)

	dps = append(dps,
		commonSplunkMetric(metricName+"_count", ts, keys, values,
			distributionValue.Count),
		commonSplunkMetric(metricName, ts, keys, values,
			distributionValue.Sum))

	explicitBuckets := distributionValue.BucketOptions.GetExplicit()
	for i := 0; i < len(explicitBuckets.Bounds); i++ {
		dps = append(dps,
			commonSplunkMetric(metricName+"_bucket", ts,
				append(keys, upperBoundDimensionKey),
				append(values, *float64ToDimValue(explicitBuckets.Bounds[i])),
				distributionValue.Buckets[i].Count))
	}
	dps = append(dps,
		commonSplunkMetric(metricName+"_bucket", ts,
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
) []*splunkMetric {
	summaryValue := summaryTimeSeries.Points[0].GetSummaryValue()

	// Two additional data points: one for count and one for sum.
	const extraDataPoints = 2
	dps := make([]*sfxpb.DataPoint, 0, len(summaryValue.Snapshot.PercentileValues)+extraDataPoints)

	dps = append(dps,
		commonSplunkMetric(metricName+"_count", ts, keys, values,
			summaryValue.Count.Value),
		commonSplunkMetric(metricName, ts, keys, values,
			summaryValue.Sum.Value))

	percentiles := summaryValue.Snapshot.GetPercentileValues()
	for _, percentile := range percentiles {
		dps = append(dps,
			commonSplunkMetric(metricName+"_quantile", ts,
				append(keys, quantileDimensionKey),
				append(values, *float64ToDimValue(percentile.Percentile)),
				percentile.Value))
	}

	return dps
}
