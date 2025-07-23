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
				pcommon.NewMap(),
				time.Unix(1582230020, 0),
				1,
			),
		},
		{
			line: "tst.dbl 3.14 1582230020",
			want: buildDoubleMetric(
				"tst.dbl",
				nil,
				time.Unix(1582230020, 0),
				3.14,
			),
		},
		{
			line: "tst.int.3tags;k0=v_0;k1=v_1;k2=v_2 128 1582230020",
			want: buildIntMetric(
				GaugeMetricType,
				"tst.int.3tags",
				func() pcommon.Map {
					m := pcommon.NewMap()
					m.PutStr("k0", "v_0")
					m.PutStr("k1", "v_1")
					m.PutStr("k2", "v_2")
					return m
				}(),
				time.Unix(1582230020, 0),
				128,
			),
		},
		{
			line: "tst.int.1tag;k0=v_0 1.23 1582230020",
			want: buildDoubleMetric(
				"tst.int.1tag",
				map[string]any{"k0": "v_0"},
				time.Unix(1582230020, 0),
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

	// tests for floating point timestamps
	fpTests := []struct {
		line string
		want pmetric.Metric
	}{
		{
			line: "tst.floattimestamp 3.14 1582230020.1234",
			want: buildDoubleMetric(
				"tst.floattimestamp",
				nil,
				time.Unix(1582230020, 123400000),
				3.14,
			),
		},
		{
			line: "tst.floattimestampnofractionalpart 3.14 1582230020.",
			want: buildDoubleMetric(
				"tst.floattimestampnofractionalpart",
				nil,
				time.Unix(1582230020, 0),
				3.14,
			),
		},
	}

	for _, tt := range fpTests {
		t.Run(tt.line, func(t *testing.T) {
			got, err := p.Parse(tt.line)
			require.NoError(t, err)

			// allow for rounding difference in float conversion.
			assert.WithinDuration(
				t,
				tt.want.Gauge().DataPoints().At(0).Timestamp().AsTime(),
				got.Gauge().DataPoints().At(0).Timestamp().AsTime(),
				100*time.Nanosecond,
			)

			// if the delta on the timestamp is OK, copy them onto the test
			// object so that we can test for equality on the remaining properties.
			got.Gauge().DataPoints().At(0).SetTimestamp(
				tt.want.Gauge().DataPoints().At(0).Timestamp(),
			)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPlaintextParser_parsePath(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		wantName       string
		wantAttributes pcommon.Map
		wantErr        bool
	}{
		{
			name:    "empty_path",
			path:    "",
			wantErr: true,
		},
		{
			name:           "no_tags_but_delim",
			path:           "no.tags;",
			wantName:       "no.tags",
			wantAttributes: pcommon.NewMap(),
		},
		{
			name:           "void_tags",
			path:           "void.tags;;;",
			wantName:       "void.tags",
			wantAttributes: pcommon.NewMap(),
			wantErr:        true,
		},
		{
			name:     "invalid_tag",
			path:     "invalid.tag;k0=v0;k1_v1",
			wantName: "invalid.tag",
			wantAttributes: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("k0", "v0")
				return m
			}(),
			wantErr: true,
		},
		{
			name:     "empty_tag_value_middle",
			path:     "empty.tag.value.middle;k0=;k1=v1",
			wantName: "empty.tag.value.middle",
			wantAttributes: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("k0", "")
				m.PutStr("k1", "v1")
				return m
			}(),
		},
		{
			name:     "empty_tag_value_end",
			path:     "empty.tag.value.end;k0=v0;k1=",
			wantName: "empty.tag.value.end",
			wantAttributes: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("k0", "v0")
				m.PutStr("k1", "")
				return m
			}(),
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
	attributes pcommon.Map,
	timestamp time.Time,
	value int64,
) pmetric.Metric {
	m := pmetric.NewMetric()
	m.SetName(name)
	var dp pmetric.NumberDataPoint
	if typ == CumulativeMetricType {
		sum := m.SetEmptySum()
		sum.SetIsMonotonic(true)
		dp = sum.DataPoints().AppendEmpty()
	} else {
		dp = m.SetEmptyGauge().DataPoints().AppendEmpty()
	}
	dp.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
	attributes.CopyTo(dp.Attributes())
	dp.SetIntValue(value)
	return m
}

func buildDoubleMetric(
	name string,
	attributes map[string]any,
	timestamp time.Time,
	value float64,
) pmetric.Metric {
	m := pmetric.NewMetric()
	m.SetName(name)
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
	_ = dp.Attributes().FromRaw(attributes)
	dp.SetDoubleValue(value)
	return m
}
