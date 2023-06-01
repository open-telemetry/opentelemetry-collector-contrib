// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package wavefrontreceiver

import (
	"math"
	"testing"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func Test_buildLabels(t *testing.T) {
	tests := []struct {
		name       string
		tags       string
		wantKeys   []*metricspb.LabelKey
		wantValues []*metricspb.LabelValue
		wantErr    bool
	}{
		{
			name: "empty_tags",
		},
		{
			name:     "only_source",
			tags:     "source=test",
			wantKeys: []*metricspb.LabelKey{{Key: "source"}},
			wantValues: []*metricspb.LabelValue{
				{Value: "test", HasValue: true},
			},
		},
		{
			name: "no_quotes",
			tags: "source=tst k0=v0 k1=v1",
			wantKeys: []*metricspb.LabelKey{
				{Key: "source"},
				{Key: "k0"},
				{Key: "k1"},
			},
			wantValues: []*metricspb.LabelValue{
				{Value: "tst", HasValue: true},
				{Value: "v0", HasValue: true},
				{Value: "v1", HasValue: true},
			},
		},
		{
			name: "end_with_quotes",
			tags: "source=\"tst escape\\\" tst\" x=\"tst spc\"",
			wantKeys: []*metricspb.LabelKey{
				{Key: "source"},
				{Key: "x"},
			},
			wantValues: []*metricspb.LabelValue{
				{Value: "tst escape\" tst", HasValue: true},
				{Value: "tst spc", HasValue: true},
			},
		},
		{
			name: "multiple_escapes",
			tags: "source=\"tst\\\"\\ntst\\\"\" bgn=\"\nb\" mid=\"tst\\nspc\" end=\"e\n\"",
			wantKeys: []*metricspb.LabelKey{
				{Key: "source"},
				{Key: "bgn"},
				{Key: "mid"},
				{Key: "end"},
			},
			wantValues: []*metricspb.LabelValue{
				{Value: "tst\"\ntst\"", HasValue: true},
				{Value: "\nb", HasValue: true},
				{Value: "tst\nspc", HasValue: true},
				{Value: "e\n", HasValue: true},
			},
		},
		{
			name: "missing_tagValue",
			tags: "k0=0 k1= k2=2",
			wantKeys: []*metricspb.LabelKey{
				{Key: "k0"},
				{Key: "k1"},
				{Key: "k2"},
			},
			wantValues: []*metricspb.LabelValue{
				{Value: "0", HasValue: true},
				{Value: "", HasValue: true},
				{Value: "2", HasValue: true},
			},
		},
		{
			name: "empty_tagValue",
			tags: "k0=0 k1=\"\"",
			wantKeys: []*metricspb.LabelKey{
				{Key: "k0"},
				{Key: "k1"},
			},
			wantValues: []*metricspb.LabelValue{
				{Value: "0", HasValue: true},
				{Value: "", HasValue: true},
			},
		},
		{
			name:    "no_tag",
			tags:    "k0=0 k1",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotKeys, gotValues, err := buildLabels(tt.tags)
			assert.Equal(t, tt.wantKeys, gotKeys)
			assert.Equal(t, tt.wantValues, gotValues)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func Test_wavefrontParser_Parse(t *testing.T) {
	tests := []struct {
		line                string
		extractCollectDTags bool
		missingTimestamp    bool
		want                *metricspb.Metric
		wantErr             bool
	}{
		{
			line: "no.tags 1 1582230020",
			want: buildMetric(
				metricspb.MetricDescriptor_GAUGE_INT64,
				"no.tags",
				nil,
				nil,
				&metricspb.Point{
					Timestamp: &timestamppb.Timestamp{Seconds: 1582230020},
					Value:     &metricspb.Point_Int64Value{Int64Value: 1},
				},
			),
		},
		{
			line: "\"/and,\" 1 1582230020 source=tst",
			want: buildMetric(
				metricspb.MetricDescriptor_GAUGE_INT64,
				"/and,",
				[]string{"source"},
				[]string{"tst"},
				&metricspb.Point{
					Timestamp: &timestamppb.Timestamp{Seconds: 1582230020},
					Value:     &metricspb.Point_Int64Value{Int64Value: 1},
				},
			),
		},
		{
			line: "tst.int 1 1582230020 source=tst",
			want: buildMetric(
				metricspb.MetricDescriptor_GAUGE_INT64,
				"tst.int",
				[]string{"source"},
				[]string{"tst"},
				&metricspb.Point{
					Timestamp: &timestamppb.Timestamp{Seconds: 1582230020},
					Value:     &metricspb.Point_Int64Value{Int64Value: 1},
				},
			),
		},
		{
			line:             "tst.dbl 3.14 source=tst k0=v0",
			missingTimestamp: true,
			want: buildMetric(
				metricspb.MetricDescriptor_GAUGE_DOUBLE,
				"tst.dbl",
				[]string{"source", "k0"},
				[]string{"tst", "v0"},
				&metricspb.Point{
					Value: &metricspb.Point_DoubleValue{DoubleValue: 3.14},
				},
			),
		},
		{
			line: "tst.int.3tags 128 1582230020 k0=v_0 k1=v_1 k2=v_2",
			want: buildMetric(
				metricspb.MetricDescriptor_GAUGE_INT64,
				"tst.int.3tags",
				[]string{"k0", "k1", "k2"},
				[]string{"v_0", "v_1", "v_2"},
				&metricspb.Point{
					Timestamp: &timestamppb.Timestamp{Seconds: 1582230020},
					Value:     &metricspb.Point_Int64Value{Int64Value: 128},
				},
			),
		},
		{
			line: "tst.int.1tag 1.23 1582230020 k0=v_0",
			want: buildMetric(
				metricspb.MetricDescriptor_GAUGE_DOUBLE,
				"tst.int.1tag",
				[]string{"k0"},
				[]string{"v_0"},
				&metricspb.Point{
					Timestamp: &timestamppb.Timestamp{Seconds: 1582230020},
					Value:     &metricspb.Point_DoubleValue{DoubleValue: 1.23},
				},
			),
		},
		{
			line:                "collectd.[cdk=cdv].tags 1 source=tst k0=v0",
			missingTimestamp:    true,
			extractCollectDTags: true,
			want: buildMetric(
				metricspb.MetricDescriptor_GAUGE_INT64,
				"collectd.tags",
				[]string{"source", "k0", "cdk"},
				[]string{"tst", "v0", "cdv"},
				&metricspb.Point{
					Value: &metricspb.Point_Int64Value{Int64Value: 1},
				},
			),
		},
		{
			line:                "mult.[cdk0=cdv0].collectd.[cdk1=cdv1].groups 1 1582230020 source=tst",
			extractCollectDTags: true,
			want: buildMetric(
				metricspb.MetricDescriptor_GAUGE_INT64,
				"mult.collectd.groups",
				[]string{"source", "cdk0", "cdk1"},
				[]string{"tst", "cdv0", "cdv1"},
				&metricspb.Point{
					Timestamp: &timestamppb.Timestamp{Seconds: 1582230020},
					Value:     &metricspb.Point_Int64Value{Int64Value: 1},
				},
			),
		},
		{
			line:                "collectd.last[cdk0=cdv0] 1 1582230020 source=tst",
			extractCollectDTags: true,
			want: buildMetric(
				metricspb.MetricDescriptor_GAUGE_INT64,
				"collectd.last",
				[]string{"source", "cdk0"},
				[]string{"tst", "cdv0"},
				&metricspb.Point{
					Timestamp: &timestamppb.Timestamp{Seconds: 1582230020},
					Value:     &metricspb.Point_Int64Value{Int64Value: 1},
				},
			),
		},
		{
			line:    "incorrect.tags 1.23 1582230000 1582230020",
			wantErr: true,
		},
		{
			line:    "nan.value xyz 1582230000 source=tst",
			wantErr: true,
		},
		{
			line:    " 1.23 1582230000",
			wantErr: true,
		},
		{
			line:    "invalid.timestamppb.not.tag 1.23 xyz source=tst",
			wantErr: true,
		},
		{
			line:    "missing.parts 3",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			p := WavefrontParser{ExtractCollectdTags: tt.extractCollectDTags}
			got, err := p.Parse(tt.line)
			if tt.missingTimestamp {
				// The timestamp was actually generated by the parser.
				// Assert that it is within a certain range around now.
				unixNow := time.Now().Unix()
				ts := got.Timeseries[0].Points[0].Timestamp
				assert.LessOrEqual(t, ts.GetSeconds(), time.Now().Unix())
				assert.LessOrEqual(t, math.Abs(float64(ts.GetSeconds()-unixNow)), 2.0)
				// Copy returned timestamp so asserts below can succeed.
				tt.want.Timeseries[0].Points[0].Timestamp = ts
			}
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func buildMetric(
	typ metricspb.MetricDescriptor_Type,
	name string,
	keys []string,
	values []string,
	point *metricspb.Point,
) *metricspb.Metric {
	var labelKeys []*metricspb.LabelKey
	if len(keys) > 0 {
		labelKeys = make([]*metricspb.LabelKey, 0, len(keys))
		for _, key := range keys {
			labelKeys = append(labelKeys, &metricspb.LabelKey{Key: key})
		}
	}
	var labelValues []*metricspb.LabelValue
	if len(values) > 0 {
		labelValues = make([]*metricspb.LabelValue, 0, len(values))
		for _, value := range values {
			labelValues = append(labelValues, &metricspb.LabelValue{
				Value:    value,
				HasValue: true,
			})
		}
	}
	return &metricspb.Metric{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name:      name,
			Type:      typ,
			LabelKeys: labelKeys,
		},
		Timeseries: []*metricspb.TimeSeries{
			{
				LabelValues: labelValues,
				Points:      []*metricspb.Point{point},
			},
		},
	}
}
