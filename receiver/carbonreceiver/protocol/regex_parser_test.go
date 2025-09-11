// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestRegexParserConfigBuildParser(t *testing.T) {
	tests := []struct {
		name    string
		config  ParserConfig
		wantErr bool
	}{
		{
			name:    "nil_method_receiver",
			config:  (*RegexParserConfig)(nil),
			wantErr: true,
		},
		{
			name:    "no_rules",
			config:  &RegexParserConfig{},
			wantErr: true,
		},
		{
			name: "invalid_regexp",
			config: &RegexParserConfig{
				Rules: []*RegexRule{
					{Regexp: "(?P<key_good>test).env(?P<key_env>[^.]*).(?P<key_host>[^.]*)"},
					{Regexp: "(?<bad>test)"},
				},
			},
			wantErr: true,
		},
		{
			name: "no_prefix_regexp_submatch",
			config: &RegexParserConfig{
				Rules: []*RegexRule{
					{Regexp: "(?P<key_good>test).env(?P<env>[^.]*).(?P<key_host>[^.]*)"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid_metric_type",
			config: &RegexParserConfig{
				Rules: []*RegexRule{
					{
						Regexp:     "(?P<key_good>test).env(?P<key_env>[^.]*).(?P<key_host>[^.]*)",
						MetricType: "unknown",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid_rules",
			config: &RegexParserConfig{
				Rules: []*RegexRule{
					{Regexp: "(?P<key_good>test).env(?P<key_env>[^.]*).(?P<key_host>[^.]*)"},
					{Regexp: "(?P<key_another>good).env(?P<key_env>[^.]*).(?P<key_host>[^.]*)"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.config.BuildParser()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
				return
			}

			assert.NoError(t, err)
			require.NotNil(t, got)
		})
	}
}

func Test_regexParser_parsePath(t *testing.T) {
	config := RegexParserConfig{
		Rules: []*RegexRule{
			{
				Regexp:     `(?P<key_svc>[^.]+)\.(?P<key_host>[^.]+)\.cpu\.seconds`,
				NamePrefix: "cpu_seconds",
				Labels:     map[string]string{"k": "v"},
			},
			{
				Regexp:     `(?P<key_svc>[^.]+)\.(?P<key_host>[^.]+)\.rpc\.count`,
				NamePrefix: "rpc",
				MetricType: string(CumulativeMetricType),
			},
			{
				Regexp:     `^(?P<key_svc>[^.]+)\.(?P<key_host>[^.]+)\.(?P<name_0>[^.]+).(?P<name_1>[^.]+)$`,
				MetricType: string(GaugeMetricType),
			},
		},
	}

	require.NoError(t, compileRegexRules(config.Rules))
	rp := &regexPathParser{
		rules: config.Rules,
	}

	tests := []struct {
		name           string
		path           string
		wantName       string
		wantAttributes pcommon.Map
		wantMetricType TargetMetricType
		wantErr        bool
	}{
		{
			name:           "no_rule_match",
			path:           "service_name.host01.rpc.duration.seconds",
			wantName:       "service_name.host01.rpc.duration.seconds",
			wantAttributes: pcommon.NewMap(),
		},
		{
			name:     "match_rule0",
			path:     "service_name.host00.cpu.seconds",
			wantName: "cpu_seconds",
			wantAttributes: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("svc", "service_name")
				m.PutStr("host", "host00")
				m.PutStr("k", "v")
				return m
			}(),
		},
		{
			name:     "match_rule1",
			path:     "service_name.host01.rpc.count",
			wantName: "rpc",
			wantAttributes: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("svc", "service_name")
				m.PutStr("host", "host01")
				return m
			}(),
			wantMetricType: CumulativeMetricType,
		},
		{
			name:     "match_rule2",
			path:     "svc_02.host02.avg.duration",
			wantName: "avgduration",
			wantAttributes: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("svc", "svc_02")
				m.PutStr("host", "host02")
				return m
			}(),
			wantMetricType: GaugeMetricType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParsedPath{}
			err := rp.ParsePath(tt.path, &got)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.Equal(t, tt.wantName, got.MetricName)
			assert.Equal(t, tt.wantAttributes, got.Attributes)
			assert.Equal(t, tt.wantMetricType, got.MetricType)
		})
	}
}

func Test_regexParser_parsePath_simple_unnamed_group(t *testing.T) {
	config := RegexParserConfig{
		Rules: []*RegexRule{
			{
				Regexp:     `(prefix\.)?(?P<key_svc>[^.]+)\.(?P<key_host>[^.]+)\.cpu\.seconds`,
				NamePrefix: "cpu_seconds",
			},
		},
	}

	require.NoError(t, compileRegexRules(config.Rules))
	rp := &regexPathParser{
		rules: config.Rules,
	}

	tests := []struct {
		name           string
		path           string
		wantName       string
		wantAttributes pcommon.Map
		wantMetricType TargetMetricType
		wantErr        bool
	}{
		{
			name:           "no_rule_match",
			path:           "service_name.host01.rpc.duration.seconds",
			wantName:       "service_name.host01.rpc.duration.seconds",
			wantAttributes: pcommon.NewMap(),
		},
		{
			name:     "match_no_prefix",
			path:     "service_name.host00.cpu.seconds",
			wantName: "cpu_seconds",
			wantAttributes: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("svc", "service_name")
				m.PutStr("host", "host00")
				return m
			}(),
		},
		{
			name:     "match_optional_prefix",
			path:     "prefix.service_name.host00.cpu.seconds",
			wantName: "cpu_seconds",
			wantAttributes: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("svc", "service_name")
				m.PutStr("host", "host00")
				return m
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParsedPath{}
			err := rp.ParsePath(tt.path, &got)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.Equal(t, tt.wantName, got.MetricName)
			assert.Equal(t, tt.wantAttributes, got.Attributes)
			assert.Equal(t, tt.wantMetricType, got.MetricType)
		})
	}
}

func Test_regexParser_parsePath_key_inside_unnamed_group(t *testing.T) {
	config := RegexParserConfig{
		Rules: []*RegexRule{
			{
				Regexp:     `(job=(?P<key_job>[^.]+).)?(?P<key_svc>[^.]+)\.(?P<key_host>[^.]+)\.cpu\.seconds`,
				NamePrefix: "cpu_seconds",
				MetricType: string(GaugeMetricType),
			},
		},
	}

	require.NoError(t, compileRegexRules(config.Rules))
	rp := &regexPathParser{
		rules: config.Rules,
	}

	tests := []struct {
		name           string
		path           string
		wantName       string
		wantAttributes pcommon.Map
		wantMetricType TargetMetricType
		wantErr        bool
	}{
		{
			name:           "no_rule_match",
			path:           "service_name.host01.rpc.duration.seconds",
			wantName:       "service_name.host01.rpc.duration.seconds",
			wantAttributes: pcommon.NewMap(),
		},
		{
			name:     "match_missing_optional_key",
			path:     "service_name.host00.cpu.seconds",
			wantName: "cpu_seconds",
			wantAttributes: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("svc", "service_name")
				m.PutStr("host", "host00")
				return m
			}(),
			wantMetricType: GaugeMetricType,
		},
		{
			name:     "match_present_optional_key",
			path:     "job=71972c09-de94-4a4e-a8a7-ad3de050a141.service_name.host00.cpu.seconds",
			wantName: "cpu_seconds",
			wantAttributes: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("job", "71972c09-de94-4a4e-a8a7-ad3de050a141")
				m.PutStr("svc", "service_name")
				m.PutStr("host", "host00")
				return m
			}(),
			wantMetricType: GaugeMetricType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParsedPath{}
			err := rp.ParsePath(tt.path, &got)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.Equal(t, tt.wantName, got.MetricName)
			assert.Equal(t, tt.wantAttributes, got.Attributes)
			assert.Equal(t, tt.wantMetricType, got.MetricType)
		})
	}
}

var res struct {
	name       string
	attributes pcommon.Map
	metricType TargetMetricType
	err        error
}

func Benchmark_regexPathParser_ParsePath(b *testing.B) {
	config := RegexParserConfig{
		Rules: []*RegexRule{
			{
				Regexp:     `(?P<key_svc>[^.]+)\.(?P<key_host>[^.]+)\.cpu\.seconds`,
				NamePrefix: "cpu_seconds",
				Labels:     map[string]string{"k": "v"},
			},
			{
				Regexp:     `(?P<key_svc>[^.]+)\.(?P<key_host>[^.]+)\.rpc\.count`,
				NamePrefix: "rpc",
				MetricType: string(CumulativeMetricType),
			},
			{
				Regexp: `^(?P<key_svc>[^.]+)\.(?P<key_host>[^.]+)\.(?P<name_0>[^.]+).(?P<name_1>[^.]+)$`,
			},
		},
	}

	require.NoError(b, compileRegexRules(config.Rules))
	rp := &regexPathParser{
		rules: config.Rules,
	}

	tests := []string{
		"service_name.host01.rpc.duration.seconds",
		"service_name.host00.cpu.seconds",
		"service_name.host01.rpc.count",
		"svc_02.host02.avg.duration",
	}

	got := ParsedPath{}
	err := rp.ParsePath(tests[0], &got)
	res.name = got.MetricName
	res.attributes = got.Attributes
	res.metricType = got.MetricType
	res.err = err

	for n := 0; n < b.N; n++ {
		for i := 0; i < len(tests); i++ {
			err = rp.ParsePath(tests[i], &got)
		}
	}

	res.name = got.MetricName
	res.attributes = got.Attributes
	res.metricType = got.MetricType
	res.err = err
}
