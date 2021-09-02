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

package carbonexporter

import (
	"strconv"
	"strings"
	"testing"
	"time"

	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/metricstestutil"
)

func Test_sanitizeTagKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{
			name: "no_changes",
			key:  "a valid tag key",
			want: "a valid tag key",
		},
		{
			name: "remove_tag_set",
			key:  "a" + tagKeyValueSeparator + "c",
			want: "a" + string(sanitizedRune) + "c",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeTagKey(tt.key)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_sanitizeTagValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "no_changes",
			value: "a valid tag value",
			want:  "a valid tag value",
		},
		{
			name:  "replace_tilde",
			value: "a~c",
			want:  "a" + string(sanitizedRune) + "c",
		},
		{
			name:  "replace_semicol",
			value: "a;c",
			want:  "a" + string(sanitizedRune) + "c",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeTagValue(tt.value)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_buildPath(t *testing.T) {
	type args struct {
		name        string
		tagKeys     []string
		labelValues []*metricspb.LabelValue
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "happy_path",
			args: args{
				name:    "happy.path",
				tagKeys: []string{"key0"},
				labelValues: []*metricspb.LabelValue{
					{Value: "val0", HasValue: true},
				},
			},
			want: "happy.path;key0=val0",
		},
		{
			name: "emoty_value",
			args: args{
				name:    "t",
				tagKeys: []string{"k0", "k1"},
				labelValues: []*metricspb.LabelValue{
					{Value: "", HasValue: true},
					{Value: "v1", HasValue: true},
				},
			},
			want: "t;k0=" + tagValueEmptyPlaceholder + ";k1=v1",
		},
		{
			name: "not_set_value",
			args: args{
				name:    "t",
				tagKeys: []string{"k0", "k1"},
				labelValues: []*metricspb.LabelValue{
					{Value: "v0", HasValue: true},
					{Value: "", HasValue: false},
				},
			},
			want: "t;k0=v0;k1=" + tagValueNotSetPlaceholder,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPath(tt.args.name, tt.args.tagKeys, tt.args.labelValues)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_metricDataToPlaintext(t *testing.T) {

	keys := []string{"k0", "k1"}
	values := []string{"v0", "v1"}
	expectedTagsStr := ";k0=v0;k1=v1"

	unixSecs := int64(1574092046)
	expectedUnixSecsStr := strconv.FormatInt(unixSecs, 10)
	unixNSecs := int64(11 * time.Millisecond)
	tsUnix := time.Unix(unixSecs, unixNSecs)

	doubleVal := 1234.5678
	expectedDobuleValStr := strconv.FormatFloat(doubleVal, 'g', -1, 64)
	doublePt := metricstestutil.Double(tsUnix, doubleVal)
	int64Val := int64(123)
	expectedInt64ValStr := "123"
	int64Pt := &metricspb.Point{
		Timestamp: timestamppb.New(tsUnix),
		Value:     &metricspb.Point_Int64Value{Int64Value: int64Val},
	}

	distributionBounds := []float64{1.5, 2, 4}
	distributionCounts := []int64{4, 2, 3, 7}
	distributionTimeSeries := metricstestutil.Timeseries(
		tsUnix,
		values,
		metricstestutil.DistPt(tsUnix, distributionBounds, distributionCounts))
	distributionPoints := distributionTimeSeries.GetPoints()
	require.Equal(t, 1, len(distributionPoints))
	distribubionPoint := distributionPoints[0].Value.(*metricspb.Point_DistributionValue)
	distributionValue := distribubionPoint.DistributionValue

	summaryTimeSeries := metricstestutil.Timeseries(
		tsUnix,
		values,
		metricstestutil.SummPt(
			tsUnix,
			11,
			111,
			[]float64{90, 95, 99, 99.9},
			[]float64{100, 6, 4, 1}))
	summaryPoints := summaryTimeSeries.GetPoints()
	require.Equal(t, 1, len(summaryPoints))
	summarySnapshot := summaryPoints[0].GetSummaryValue().GetSnapshot()

	tests := []struct {
		name                       string
		metricsDataFn              func() []*agentmetricspb.ExportMetricsServiceRequest
		wantLines                  []string
		wantNumConvertedTimeseries int
		wantNumDroppedTimeseries   int
	}{
		{
			name: "no_dims",
			metricsDataFn: func() []*agentmetricspb.ExportMetricsServiceRequest {
				return []*agentmetricspb.ExportMetricsServiceRequest{
					{
						Metrics: []*metricspb.Metric{
							metricstestutil.Gauge("gauge_double_no_dims", nil, metricstestutil.Timeseries(tsUnix, nil, doublePt)),
							metricstestutil.GaugeInt("gauge_int_no_dims", nil, metricstestutil.Timeseries(tsUnix, nil, int64Pt)),
						},
					},
					{
						Metrics: []*metricspb.Metric{
							metricstestutil.Cumulative("cumulative_double_no_dims", nil, metricstestutil.Timeseries(tsUnix, nil, doublePt)),
							metricstestutil.CumulativeInt("cumulative_int_no_dims", nil, metricstestutil.Timeseries(tsUnix, nil, int64Pt)),
						},
					},
				}
			},
			wantLines: []string{
				"gauge_double_no_dims " + expectedDobuleValStr + " " + expectedUnixSecsStr,
				"gauge_int_no_dims " + expectedInt64ValStr + " " + expectedUnixSecsStr,
				"cumulative_double_no_dims " + expectedDobuleValStr + " " + expectedUnixSecsStr,
				"cumulative_int_no_dims " + expectedInt64ValStr + " " + expectedUnixSecsStr,
			},
			wantNumConvertedTimeseries: 4,
		},
		{
			name: "with_dims",
			metricsDataFn: func() []*agentmetricspb.ExportMetricsServiceRequest {
				return []*agentmetricspb.ExportMetricsServiceRequest{
					{
						Metrics: []*metricspb.Metric{
							metricstestutil.Gauge("gauge_double_with_dims", keys, metricstestutil.Timeseries(tsUnix, values, doublePt)),
							metricstestutil.GaugeInt("gauge_int_with_dims", keys, metricstestutil.Timeseries(tsUnix, values, int64Pt)),
							metricstestutil.Cumulative("cumulative_double_with_dims", keys, metricstestutil.Timeseries(tsUnix, values, doublePt)),
							metricstestutil.CumulativeInt("cumulative_int_with_dims", keys, metricstestutil.Timeseries(tsUnix, values, int64Pt)),
						},
					},
				}
			},
			wantLines: []string{
				"gauge_double_with_dims" + expectedTagsStr + " " + expectedDobuleValStr + " " + expectedUnixSecsStr,
				"gauge_int_with_dims" + expectedTagsStr + " " + expectedInt64ValStr + " " + expectedUnixSecsStr,
				"cumulative_double_with_dims" + expectedTagsStr + " " + expectedDobuleValStr + " " + expectedUnixSecsStr,
				"cumulative_int_with_dims" + expectedTagsStr + " " + expectedInt64ValStr + " " + expectedUnixSecsStr,
			},
			wantNumConvertedTimeseries: 4,
		},
		{
			name: "distributions",
			metricsDataFn: func() []*agentmetricspb.ExportMetricsServiceRequest {
				return []*agentmetricspb.ExportMetricsServiceRequest{
					{
						Metrics: []*metricspb.Metric{
							metricstestutil.GaugeDist("distrib", keys, distributionTimeSeries),
						},
					},
				}
			},
			wantLines: expectedDistributionLines(
				"distrib", expectedTagsStr, expectedUnixSecsStr,
				distributionValue.Sum,
				distributionValue.Count,
				distributionBounds,
				distributionCounts),
			wantNumConvertedTimeseries: 1,
		},
		{
			name: "summary",
			metricsDataFn: func() []*agentmetricspb.ExportMetricsServiceRequest {
				return []*agentmetricspb.ExportMetricsServiceRequest{
					{
						Metrics: []*metricspb.Metric{
							metricstestutil.Summary("summary", keys, summaryTimeSeries),
						},
					},
				}
			},
			wantLines: expectedSummaryLines(
				"summary", expectedTagsStr, expectedUnixSecsStr,
				summaryPoints[0].GetSummaryValue().GetSum().Value,
				summaryPoints[0].GetSummaryValue().GetCount().Value,
				summarySnapshot.PercentileValues),
			wantNumConvertedTimeseries: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLines, gotNunConvertedTimeseries, gotNumDroppedTimeseries := metricDataToPlaintext(tt.metricsDataFn())
			assert.Equal(t, tt.wantNumConvertedTimeseries, gotNunConvertedTimeseries)
			assert.Equal(t, tt.wantNumDroppedTimeseries, gotNumDroppedTimeseries)
			got := strings.Split(gotLines, "\n")
			got = got[:len(got)-1]
			assert.Equal(t, tt.wantLines, got)
		})
	}
}

func expectedDistributionLines(
	metricName, tags, timestampStr string,
	sum float64,
	count int64,
	bounds []float64,
	counts []int64,
) []string {
	lines := []string{
		metricName + ".count" + tags + " " + formatInt64(count) + " " + timestampStr,
		metricName + tags + " " + formatFloatForLabel(sum) + " " + timestampStr,
	}

	for i, bound := range bounds {
		lines = append(lines,
			metricName+".bucket"+tags+";upper_bound="+formatFloatForLabel(bound)+" "+formatInt64(counts[i])+" "+timestampStr)
	}
	lines = append(lines,
		metricName+".bucket"+tags+";upper_bound=inf "+formatInt64(counts[len(bounds)])+" "+timestampStr)

	return lines
}

func expectedSummaryLines(
	metricName, tags, timestampStr string,
	sum float64,
	count int64,
	percentiles []*metricspb.SummaryValue_Snapshot_ValueAtPercentile,
) []string {
	lines := []string{
		metricName + ".count" + tags + " " + formatInt64(count) + " " + timestampStr,
		metricName + tags + " " + formatFloatForValue(sum) + " " + timestampStr,
	}

	for _, p := range percentiles {
		lines = append(lines,
			metricName+".quantile"+tags+";quantile="+formatFloatForLabel(p.Percentile)+" "+formatFloatForValue(p.Value)+" "+timestampStr)
	}

	return lines
}
