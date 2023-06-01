// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package protocol

import (
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func Test_plaintextParser_Parse(t *testing.T) {
	p, err := (&PlaintextConfig{}).BuildParser()
	require.NoError(t, err)
	tests := []struct {
		line    string
		want    *metricspb.Metric
		wantErr bool
	}{
		{
			line: "tst.int 1 1582230020",
			want: buildMetric(
				metricspb.MetricDescriptor_GAUGE_INT64,
				"tst.int",
				nil,
				nil,
				&metricspb.Point{
					Timestamp: &timestamppb.Timestamp{Seconds: 1582230020},
					Value:     &metricspb.Point_Int64Value{Int64Value: 1},
				},
			),
		},
		{
			line: "tst.dbl 3.14 1582230020",
			want: buildMetric(
				metricspb.MetricDescriptor_GAUGE_DOUBLE,
				"tst.dbl",
				nil,
				nil,
				&metricspb.Point{
					Timestamp: &timestamppb.Timestamp{Seconds: 1582230020},
					Value:     &metricspb.Point_DoubleValue{DoubleValue: 3.14},
				},
			),
		},
		{
			line: "tst.int.3tags;k0=v_0;k1=v_1;k2=v_2 128 1582230020",
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
			line: "tst.int.1tag;k0=v_0 1.23 1582230020",
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
			line:    "more.than.3.parts 1.23 1582230000 1582230020",
			wantErr: true,
		},
		{
			line:    "nan.value xyz 1582230000",
			wantErr: true,
		},
		{
			line:    ";invalid=path 1.23 1582230000",
			wantErr: true,
		},
		{
			line:    "invalid.timestamp 1.23 xyz",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got, err := p.Parse(tt.line)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestPlaintextParser_parsePath(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantName   string
		wantKeys   []*metricspb.LabelKey
		wantValues []*metricspb.LabelValue
		wantErr    bool
	}{
		{
			name:    "empty_path",
			path:    "",
			wantErr: true,
		},
		{
			name:     "no_tags_but_delim",
			path:     "no.tags;",
			wantName: "no.tags",
		},
		{
			name:       "void_tags",
			path:       "void.tags;;;",
			wantName:   "void.tags",
			wantKeys:   []*metricspb.LabelKey{},
			wantValues: []*metricspb.LabelValue{},
			wantErr:    true,
		},
		{
			name:       "invalid_tag",
			path:       "invalid.tag;k0=v0;k1_v1",
			wantName:   "invalid.tag",
			wantKeys:   []*metricspb.LabelKey{{Key: "k0"}},
			wantValues: []*metricspb.LabelValue{{Value: "v0", HasValue: true}},
			wantErr:    true,
		},
		{
			name:     "empty_tag_value_middle",
			path:     "empty.tag.value.middle;k0=;k1=v1",
			wantName: "empty.tag.value.middle",
			wantKeys: []*metricspb.LabelKey{{Key: "k0"}, {Key: "k1"}},
			wantValues: []*metricspb.LabelValue{
				{Value: "", HasValue: true},
				{Value: "v1", HasValue: true},
			},
		},
		{
			name:     "empty_tag_value_end",
			path:     "empty.tag.value.end;k0=v0;k1=",
			wantName: "empty.tag.value.end",
			wantKeys: []*metricspb.LabelKey{{Key: "k0"}, {Key: "k1"}},
			wantValues: []*metricspb.LabelValue{
				{Value: "v0", HasValue: true},
				{Value: "", HasValue: true},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PlaintextPathParser{}
			got := ParsedPath{}
			err := p.ParsePath(tt.path, &got)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantName, got.MetricName)
				assert.Equal(t, tt.wantKeys, got.LabelKeys)
				assert.Equal(t, tt.wantValues, got.LabelValues)
				assert.Equal(t, DefaultMetricType, got.MetricType)
			}
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
	return buildMetricForSinglePoint(
		name,
		typ,
		labelKeys,
		labelValues,
		point)
}
