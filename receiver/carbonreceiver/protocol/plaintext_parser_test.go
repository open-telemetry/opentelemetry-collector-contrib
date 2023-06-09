// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package protocol

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func Test_plaintextParser_Parse(t *testing.T) {
	p, err := (&PlaintextConfig{}).BuildParser()
	require.NoError(t, err)
	tests := []struct {
		line    string
		want    pmetric.Metric
		wantErr bool
	}{
		{
			line: "tst.int 1 1582230020",
			want: buildIntMetric(
				GaugeMetricType,
				"tst.int",
				nil,
				1582230020,
				1,
			),
		},
		{
			line: "tst.dbl 3.14 1582230020",
			want: buildDoubleMetric(
				GaugeMetricType,
				"tst.dbl",
				nil,
				1582230020,
				3.14,
			),
		},
		{
			line: "tst.int.3tags;k0=v_0;k1=v_1;k2=v_2 128 1582230020",
			want: buildIntMetric(
				GaugeMetricType,
				"tst.int.3tags",
				map[string]any{"k0": "v_0", "k1": "v_1", "k2": "v_2"},
				1582230020,
				128,
			),
		},
		{
			line: "tst.int.1tag;k0=v_0 1.23 1582230020",
			want: buildDoubleMetric(
				GaugeMetricType,
				"tst.int.1tag",
				map[string]any{"k0": "v_0"},
				1582230020,
				1.23,
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
		name           string
		path           string
		wantName       string
		wantAttributes map[string]any
		wantErr        bool
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
			name:           "void_tags",
			path:           "void.tags;;;",
			wantName:       "void.tags",
			wantAttributes: map[string]any{},
			wantErr:        true,
		},
		{
			name:           "invalid_tag",
			path:           "invalid.tag;k0=v0;k1_v1",
			wantName:       "invalid.tag",
			wantAttributes: map[string]any{"k0": "v0"},
			wantErr:        true,
		},
		{
			name:           "empty_tag_value_middle",
			path:           "empty.tag.value.middle;k0=;k1=v1",
			wantName:       "empty.tag.value.middle",
			wantAttributes: map[string]any{"k0": "", "k1": "v1"},
		},
		{
			name:           "empty_tag_value_end",
			path:           "empty.tag.value.end;k0=v0;k1=",
			wantName:       "empty.tag.value.end",
			wantAttributes: map[string]any{"k0": "v0", "k1": ""},
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
				assert.Equal(t, tt.wantAttributes, got.Attributes)
				assert.Equal(t, DefaultMetricType, got.MetricType)
			}
		})
	}
}

func buildIntMetric(
	typ TargetMetricType,
	name string,
	attributes map[string]any,
	timestamp int64,
	value int64,
) pmetric.Metric {
	m := pmetric.NewMetric()
	m.SetName(name)
	if typ == CumulativeMetricType {
		sum := m.SetEmptySum()
		sum.SetIsMonotonic(true)
		dp := sum.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(timestamp, 0)))
		_ = dp.Attributes().FromRaw(attributes)
		dp.SetIntValue(value)
	} else {
		g := m.SetEmptyGauge()
		dp := g.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(timestamp, 0)))
		_ = dp.Attributes().FromRaw(attributes)
		dp.SetIntValue(value)
	}
	return m
}

func buildDoubleMetric(
	typ TargetMetricType,
	name string,
	attributes map[string]any,
	timestamp int64,
	value float64,
) pmetric.Metric {
	m := pmetric.NewMetric()
	m.SetName(name)
	if typ == CumulativeMetricType {
		sum := m.SetEmptySum()
		sum.SetIsMonotonic(true)
		dp := sum.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(timestamp, 0)))
		_ = dp.Attributes().FromRaw(attributes)
		dp.SetDoubleValue(value)
	} else {
		g := m.SetEmptyGauge()
		dp := g.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(timestamp, 0)))
		_ = dp.Attributes().FromRaw(attributes)
		dp.SetDoubleValue(value)
	}
	return m
}
