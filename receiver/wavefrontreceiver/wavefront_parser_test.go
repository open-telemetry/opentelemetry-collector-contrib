// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package wavefrontreceiver

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func Test_buildLabels(t *testing.T) {
	tests := []struct {
		name           string
		tags           string
		wantAttributes pcommon.Map
		wantErr        bool
	}{
		{
			name:           "empty_tags",
			wantAttributes: pcommon.NewMap(),
		},
		{
			name: "only_source",
			tags: "source=test",
			wantAttributes: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("source", "test")
				return m
			}(),
		},
		{
			name: "no_quotes",
			tags: "source=tst k0=v0 k1=v1",
			wantAttributes: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("source", "tst")
				m.PutStr("k0", "v0")
				m.PutStr("k1", "v1")
				return m
			}(),
		},
		{
			name: "end_with_quotes",
			tags: "source=\"test escaped\\\" text\" x=\"tst spc\"",
			wantAttributes: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("source", "test escaped\" text")
				m.PutStr("x", "tst spc")
				return m
			}(),
		},
		{
			name: "multiple_escapes",
			tags: "source=\"tst\\\"\\ntst\\\"\" bgn=\"\nb\" mid=\"tst\\nspc\" end=\"e\n\"",
			wantAttributes: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("source", "tst\"\ntst\"")
				m.PutStr("bgn", "\nb")
				m.PutStr("mid", "tst\nspc")
				m.PutStr("end", "e\n")
				return m
			}(),
		},
		{
			name: "missing_tagValue",
			tags: "k0=0 k1= k2=2",
			wantAttributes: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("k0", "0")
				m.PutStr("k1", "")
				m.PutStr("k2", "2")
				return m
			}(),
		},
		{
			name: "empty_tagValue",
			tags: "k0=0 k1=" +
				"",
			wantAttributes: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("k0", "0")
				m.PutStr("k1", "")
				return m
			}(),
		},
		{
			name: "empty_quoted_tagValue",
			tags: "k0=0 k1=\"\"",
			wantAttributes: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("k0", "0")
				m.PutStr("k1", "")
				return m
			}(),
		},
		{
			name:    "no_tag",
			tags:    "k0=0 k1",
			wantErr: true,
		},
		{
			name:    "partially_quoted",
			tags:    "k0=0 k1=\"test",
			wantErr: true,
		},
		{
			name:    "end_with_escaped_",
			tags:    "k0=0 k1=\"test\\\"",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAttributes := pcommon.NewMap()
			err := buildLabels(gotAttributes, tt.tags)
			if !tt.wantErr {
				assert.Equal(t, tt.wantAttributes, gotAttributes)
			}
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func Test_wavefrontParser_Parse(t *testing.T) {
	tests := []struct {
		line                string
		extractCollectDTags bool
		missingTimestamp    bool
		want                pmetric.Metric
		wantErr             bool
	}{
		{
			line: "no.tags 1 1582230020",
			want: buildIntMetric(
				"no.tags",
				pcommon.NewMap(),
				1582230020,
				1,
			),
		},
		{
			line: "\"/and,\" 1 1582230020 source=tst",
			want: buildIntMetric(
				"/and,",
				func() pcommon.Map {
					m := pcommon.NewMap()
					m.PutStr("source", "tst")
					return m
				}(),
				1582230020,
				1,
			),
		},
		{
			line: "tst.int 1 1582230020 source=tst",
			want: buildIntMetric(
				"tst.int",
				func() pcommon.Map {
					m := pcommon.NewMap()
					m.PutStr("source", "tst")
					return m
				}(),
				1582230020,
				1,
			),
		},
		{
			line:             "tst.dbl 3.14 source=tst k0=v0",
			missingTimestamp: true,
			want: buildDoubleMetric(
				"tst.dbl",
				func() pcommon.Map {
					m := pcommon.NewMap()
					m.PutStr("source", "tst")
					m.PutStr("k0", "v0")
					return m
				}(),
				0,
				3.14,
			),
		},
		{
			line: "tst.int.3tags 128 1582230020 k0=v_0 k1=v_1 k2=v_2",
			want: buildIntMetric(
				"tst.int.3tags",
				func() pcommon.Map {
					m := pcommon.NewMap()
					m.PutStr("k0", "v_0")
					m.PutStr("k1", "v_1")
					m.PutStr("k2", "v_2")
					return m
				}(),
				1582230020,
				128,
			),
		},
		{
			line: "tst.int.1tag 1.23 1582230020 k0=v_0",
			want: buildDoubleMetric(
				"tst.int.1tag",
				func() pcommon.Map {
					m := pcommon.NewMap()
					m.PutStr("k0", "v_0")
					return m
				}(),
				1582230020,
				1.23,
			),
		},
		{
			line:                "collectd.[cdk=cdv].tags 1 source=tst k0=v0",
			missingTimestamp:    true,
			extractCollectDTags: true,
			want: buildIntMetric(
				"collectd.tags",
				func() pcommon.Map {
					m := pcommon.NewMap()
					m.PutStr("source", "tst")
					m.PutStr("k0", "v0")
					m.PutStr("cdk", "cdv")
					return m
				}(),
				0,
				1,
			),
		},
		{
			line:                "mult.[cdk0=cdv0].collectd.[cdk1=cdv1].groups 1 1582230020 source=tst",
			extractCollectDTags: true,
			want: buildIntMetric(
				"mult.collectd.groups",
				func() pcommon.Map {
					m := pcommon.NewMap()
					m.PutStr("source", "tst")
					m.PutStr("cdk0", "cdv0")
					m.PutStr("cdk1", "cdv1")
					return m
				}(),
				1582230020,
				1,
			),
		},
		{
			line:                "collectd.last[cdk0=cdv0] 1 1582230020 source=tst",
			extractCollectDTags: true,
			want: buildIntMetric(
				"collectd.last",
				func() pcommon.Map {
					m := pcommon.NewMap()
					m.PutStr("source", "tst")
					m.PutStr("cdk0", "cdv0")
					return m
				}(),
				1582230020,
				1,
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
			p := wavefrontParser{ExtractCollectdTags: tt.extractCollectDTags}
			got, err := p.Parse(tt.line)
			if tt.missingTimestamp {
				// The timestamp was actually generated by the parser.
				// Assert that it is within a certain range around now.
				unixNow := time.Now().Unix()
				ts := got.Gauge().DataPoints().At(0).Timestamp().AsTime()
				assert.LessOrEqual(t, ts, time.Now())
				assert.LessOrEqual(t, math.Abs(float64(ts.Unix()-unixNow)), 2.0)
				// Copy returned timestamp so asserts below can succeed.
				tt.want.Gauge().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(ts))
			}
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func buildDoubleMetric(
	name string,
	attributes pcommon.Map,
	ts int64,
	dblVal float64,
) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	g := metric.SetEmptyGauge()
	dp := g.DataPoints().AppendEmpty()
	dp.SetDoubleValue(dblVal)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(ts, 0)))
	attributes.CopyTo(dp.Attributes())
	return metric
}

func buildIntMetric(
	name string,
	attributes pcommon.Map,
	ts int64,
	intVal int64,
) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	g := metric.SetEmptyGauge()
	dp := g.DataPoints().AppendEmpty()
	dp.SetIntValue(intVal)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(ts, 0)))
	attributes.CopyTo(dp.Attributes())
	return metric
}
