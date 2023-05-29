// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package protocol

import (
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		wantKeys       []*metricspb.LabelKey
		wantValues     []*metricspb.LabelValue
		wantMetricType TargetMetricType
		wantErr        bool
	}{
		{
			name:     "no_rule_match",
			path:     "service_name.host01.rpc.duration.seconds",
			wantName: "service_name.host01.rpc.duration.seconds",
		},
		{
			name:     "match_rule0",
			path:     "service_name.host00.cpu.seconds",
			wantName: "cpu_seconds",
			wantKeys: []*metricspb.LabelKey{
				{Key: "svc"},
				{Key: "host"},
				{Key: "k"},
			},
			wantValues: []*metricspb.LabelValue{
				{Value: "service_name", HasValue: true},
				{Value: "host00", HasValue: true},
				{Value: "v", HasValue: true},
			},
		},
		{
			name:     "match_rule1",
			path:     "service_name.host01.rpc.count",
			wantName: "rpc",
			wantKeys: []*metricspb.LabelKey{
				{Key: "svc"},
				{Key: "host"},
			},
			wantValues: []*metricspb.LabelValue{
				{Value: "service_name", HasValue: true},
				{Value: "host01", HasValue: true},
			},
			wantMetricType: CumulativeMetricType,
		},
		{
			name:     "match_rule2",
			path:     "svc_02.host02.avg.duration",
			wantName: "avgduration",
			wantKeys: []*metricspb.LabelKey{
				{Key: "svc"},
				{Key: "host"},
			},
			wantValues: []*metricspb.LabelValue{
				{Value: "svc_02", HasValue: true},
				{Value: "host02", HasValue: true},
			},
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
			assert.Equal(t, tt.wantKeys, got.LabelKeys)
			assert.Equal(t, tt.wantValues, got.LabelValues)
			assert.Equal(t, tt.wantMetricType, got.MetricType)
		})
	}
}

var res struct {
	name       string
	keys       []*metricspb.LabelKey
	values     []*metricspb.LabelValue
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
	res.keys = got.LabelKeys
	res.values = got.LabelValues
	res.metricType = got.MetricType
	res.err = err

	for n := 0; n < b.N; n++ {
		for i := 0; i < len(tests); i++ {
			err = rp.ParsePath(tests[i], &got)
		}
	}

	res.name = got.MetricName
	res.keys = got.LabelKeys
	res.values = got.LabelValues
	res.metricType = got.MetricType
	res.err = err
}
