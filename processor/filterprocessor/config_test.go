// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	fsregexp "github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset/regexp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/metadata"
)

// TestLoadingConfigRegexp tests loading testdata/config_strict.yaml
func TestLoadingConfigStrict(t *testing.T) {
	// list of filters used repeatedly on testdata/config_strict.yaml
	testDataFilters := []string{
		"hello_world",
		"hello/world",
	}

	testDataMetricProperties := &filterconfig.MetricMatchProperties{
		MatchType:   filterconfig.MetricStrict,
		MetricNames: testDataFilters,
	}
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_strict.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected *Config
	}{
		{
			id: component.MustNewIDWithName("filter", "empty"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Metrics: MetricFilters{
					Include: &filterconfig.MetricMatchProperties{
						MatchType: filterconfig.MetricStrict,
					},
				},
			},
		}, {
			id: component.MustNewIDWithName("filter", "include"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Metrics: MetricFilters{
					Include: testDataMetricProperties,
				},
			},
		}, {
			id: component.MustNewIDWithName("filter", "exclude"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Metrics: MetricFilters{
					Exclude: testDataMetricProperties,
				},
			},
		}, {
			id: component.MustNewIDWithName("filter", "includeexclude"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Metrics: MetricFilters{
					Include: testDataMetricProperties,
					Exclude: &filterconfig.MetricMatchProperties{
						MatchType:   filterconfig.MetricStrict,
						MetricNames: []string{"hello_world"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

// TestLoadingConfigStrictLogs tests loading testdata/config_logs_strict.yaml
func TestLoadingConfigStrictLogs(t *testing.T) {
	testDataLogPropertiesInclude := &LogMatchProperties{
		LogMatchType: strictType,
		ResourceAttributes: []filterconfig.Attribute{
			{
				Key:   "should_include",
				Value: "true",
			},
		},
	}

	testDataLogPropertiesExclude := &LogMatchProperties{
		LogMatchType: strictType,
		ResourceAttributes: []filterconfig.Attribute{
			{
				Key:   "should_exclude",
				Value: "true",
			},
		},
	}

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_logs_strict.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected *Config
	}{
		{
			id: component.MustNewIDWithName("filter", "empty"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Logs: LogFilters{
					Include: &LogMatchProperties{
						LogMatchType: strictType,
					},
				},
			},
		}, {
			id: component.MustNewIDWithName("filter", "include"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Logs: LogFilters{
					Include: testDataLogPropertiesInclude,
				},
			},
		}, {
			id: component.MustNewIDWithName("filter", "exclude"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Logs: LogFilters{
					Exclude: testDataLogPropertiesExclude,
				},
			},
		}, {
			id: component.MustNewIDWithName("filter", "includeexclude"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Logs: LogFilters{
					Include: testDataLogPropertiesInclude,
					Exclude: testDataLogPropertiesExclude,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

// TestLoadingConfigSeverityLogsStrict tests loading testdata/config_logs_severity_strict.yaml
func TestLoadingConfigSeverityLogsStrict(t *testing.T) {
	testDataLogPropertiesInclude := &LogMatchProperties{
		LogMatchType:  strictType,
		SeverityTexts: []string{"INFO"},
	}

	testDataLogPropertiesExclude := &LogMatchProperties{
		LogMatchType:  strictType,
		SeverityTexts: []string{"DEBUG", "DEBUG2", "DEBUG3", "DEBUG4"},
	}

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_logs_severity_strict.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected *Config
	}{
		{
			id: component.MustNewIDWithName("filter", "include"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Logs: LogFilters{
					Include: testDataLogPropertiesInclude,
				},
			},
		}, {
			id: component.MustNewIDWithName("filter", "exclude"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Logs: LogFilters{
					Exclude: testDataLogPropertiesExclude,
				},
			},
		}, {
			id: component.MustNewIDWithName("filter", "includeexclude"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Logs: LogFilters{
					Include: testDataLogPropertiesInclude,
					Exclude: testDataLogPropertiesExclude,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

// TestLoadingConfigSeverityLogsRegexp tests loading testdata/config_logs_severity_regexp.yaml
func TestLoadingConfigSeverityLogsRegexp(t *testing.T) {
	testDataLogPropertiesInclude := &LogMatchProperties{
		LogMatchType:  regexpType,
		SeverityTexts: []string{"INFO[2-4]?"},
	}

	testDataLogPropertiesExclude := &LogMatchProperties{
		LogMatchType:  regexpType,
		SeverityTexts: []string{"DEBUG[2-4]?"},
	}

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_logs_severity_regexp.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected *Config
	}{
		{
			id: component.MustNewIDWithName("filter", "include"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Logs: LogFilters{
					Include: testDataLogPropertiesInclude,
				},
			},
		}, {
			id: component.MustNewIDWithName("filter", "exclude"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Logs: LogFilters{
					Exclude: testDataLogPropertiesExclude,
				},
			},
		}, {
			id: component.MustNewIDWithName("filter", "includeexclude"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Logs: LogFilters{
					Include: testDataLogPropertiesInclude,
					Exclude: testDataLogPropertiesExclude,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

// TestLoadingConfigBodyLogsStrict tests loading testdata/config_logs_body_strict.yaml
func TestLoadingConfigBodyLogsStrict(t *testing.T) {
	testDataLogPropertiesInclude := &LogMatchProperties{
		LogMatchType: strictType,
		LogBodies:    []string{"This is an important event"},
	}

	testDataLogPropertiesExclude := &LogMatchProperties{
		LogMatchType: strictType,
		LogBodies:    []string{"This event is not important"},
	}

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_logs_body_strict.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected *Config
	}{
		{
			id: component.MustNewIDWithName("filter", "include"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Logs: LogFilters{
					Include: testDataLogPropertiesInclude,
				},
			},
		}, {
			id: component.MustNewIDWithName("filter", "exclude"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Logs: LogFilters{
					Exclude: testDataLogPropertiesExclude,
				},
			},
		}, {
			id: component.MustNewIDWithName("filter", "includeexclude"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Logs: LogFilters{
					Include: testDataLogPropertiesInclude,
					Exclude: testDataLogPropertiesExclude,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

// TestLoadingConfigBodyLogsStrict tests loading testdata/config_logs_body_regexp.yaml
func TestLoadingConfigBodyLogsRegexp(t *testing.T) {
	testDataLogPropertiesInclude := &LogMatchProperties{
		LogMatchType: regexpType,
		LogBodies:    []string{"^IMPORTANT:"},
	}

	testDataLogPropertiesExclude := &LogMatchProperties{
		LogMatchType: regexpType,
		LogBodies:    []string{"^MINOR:"},
	}

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_logs_body_regexp.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected *Config
	}{
		{
			id: component.MustNewIDWithName("filter", "include"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Logs: LogFilters{
					Include: testDataLogPropertiesInclude,
				},
			},
		}, {
			id: component.MustNewIDWithName("filter", "exclude"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Logs: LogFilters{
					Exclude: testDataLogPropertiesExclude,
				},
			},
		}, {
			id: component.MustNewIDWithName("filter", "includeexclude"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Logs: LogFilters{
					Include: testDataLogPropertiesInclude,
					Exclude: testDataLogPropertiesExclude,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

// TestLoadingConfigMinSeverityNumberLogs tests loading testdata/config_logs_min_severity.yaml
func TestLoadingConfigMinSeverityNumberLogs(t *testing.T) {
	testDataLogPropertiesInclude := &LogMatchProperties{
		SeverityNumberProperties: &LogSeverityNumberMatchProperties{
			Min:            logSeverity("INFO"),
			MatchUndefined: true,
		},
	}

	testDataLogPropertiesExclude := &LogMatchProperties{
		SeverityNumberProperties: &LogSeverityNumberMatchProperties{
			Min: logSeverity("ERROR"),
		},
	}

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_logs_min_severity.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected *Config
	}{
		{
			id: component.MustNewIDWithName("filter", "include"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Logs: LogFilters{
					Include: testDataLogPropertiesInclude,
				},
			},
		}, {
			id: component.MustNewIDWithName("filter", "exclude"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Logs: LogFilters{
					Exclude: testDataLogPropertiesExclude,
				},
			},
		}, {
			id: component.MustNewIDWithName("filter", "includeexclude"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Logs: LogFilters{
					Include: testDataLogPropertiesInclude,
					Exclude: testDataLogPropertiesExclude,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

// TestLoadingConfigRegexp tests loading testdata/config_regexp.yaml
func TestLoadingConfigRegexp(t *testing.T) {
	// list of filters used repeatedly on testdata/config.yaml
	testDataFilters := []string{
		"prefix/.*",
		"prefix_.*",
		".*/suffix",
		".*_suffix",
		".*/contains/.*",
		".*_contains_.*",
		"full/name/match",
		"full_name_match",
	}

	testDataMetricProperties := &filterconfig.MetricMatchProperties{
		MatchType:   filterconfig.MetricRegexp,
		MetricNames: testDataFilters,
	}

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_regexp.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.MustNewIDWithName("filter", "include"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Metrics: MetricFilters{
					Include: testDataMetricProperties,
				},
			},
		}, {
			id: component.MustNewIDWithName("filter", "exclude"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Metrics: MetricFilters{
					Exclude: testDataMetricProperties,
				},
			},
		}, {
			id: component.MustNewIDWithName("filter", "unlimitedcache"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Metrics: MetricFilters{
					Include: &filterconfig.MetricMatchProperties{
						MatchType: filterconfig.MetricRegexp,
						RegexpConfig: &fsregexp.Config{
							CacheEnabled: true,
						},
						MetricNames: testDataFilters,
					},
				},
			},
		}, {
			id: component.MustNewIDWithName("filter", "limitedcache"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Metrics: MetricFilters{
					Exclude: &filterconfig.MetricMatchProperties{
						MatchType: filterconfig.MetricRegexp,
						RegexpConfig: &fsregexp.Config{
							CacheEnabled:       true,
							CacheMaxNumEntries: 10,
						},
						MetricNames: testDataFilters,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestLoadingSpans(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_traces.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.MustNewIDWithName("filter", "spans"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Spans: filterconfig.MatchConfig{
					Include: &filterconfig.MatchProperties{
						Config: filterset.Config{
							MatchType: filterset.Strict,
						},
						Services: []string{"test", "test2"},
						Attributes: []filterconfig.Attribute{
							{Key: "should_include", Value: "(true|probably_true)"},
						},
					},
					Exclude: &filterconfig.MatchProperties{
						Config: filterset.Config{
							MatchType: filterset.Regexp,
						},
						Attributes: []filterconfig.Attribute{
							{Key: "should_exclude", Value: "(probably_false|false)"},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestLoadingConfigExpr(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_expr.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.MustNewIDWithName("filter", "empty"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Metrics: MetricFilters{
					Include: &filterconfig.MetricMatchProperties{
						MatchType: filterconfig.MetricExpr,
					},
				},
			},
		},
		{
			id: component.MustNewIDWithName("filter", "include"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Metrics: MetricFilters{
					Include: &filterconfig.MetricMatchProperties{
						MatchType: filterconfig.MetricExpr,
						Expressions: []string{
							`Label("foo") == "bar"`,
							`HasLabel("baz")`,
						},
					},
				},
			},
		},
		{
			id: component.MustNewIDWithName("filter", "exclude"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Metrics: MetricFilters{
					Exclude: &filterconfig.MetricMatchProperties{
						MatchType: filterconfig.MetricExpr,
						Expressions: []string{
							`Label("foo") == "bar"`,
							`HasLabel("baz")`,
						},
					},
				},
			},
		},
		{
			id: component.MustNewIDWithName("filter", "includeexclude"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Metrics: MetricFilters{
					Include: &filterconfig.MetricMatchProperties{
						MatchType: filterconfig.MetricExpr,
						Expressions: []string{
							`HasLabel("foo")`,
						},
					},
					Exclude: &filterconfig.MetricMatchProperties{
						MatchType: filterconfig.MetricExpr,
						Expressions: []string{
							`HasLabel("bar")`,
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestLogSeverity_severityNumber(t *testing.T) {
	testCases := []struct {
		name string
		sev  logSeverity
		num  plog.SeverityNumber
	}{
		{
			name: "INFO severity",
			sev:  logSeverity("INFO"),
			num:  plog.SeverityNumberInfo,
		},
		{
			name: "info severity",
			sev:  logSeverity("info"),
			num:  plog.SeverityNumberInfo,
		},
		{
			name: "info3 severity",
			sev:  logSeverity("info3"),
			num:  plog.SeverityNumberInfo3,
		},
		{
			name: "DEBUG severity",
			sev:  logSeverity("DEBUG"),
			num:  plog.SeverityNumberDebug,
		},
		{
			name: "ERROR severity",
			sev:  logSeverity("ERROR"),
			num:  plog.SeverityNumberError,
		},
		{
			name: "WARN severity",
			sev:  logSeverity("WARN"),
			num:  plog.SeverityNumberWarn,
		},
		{
			name: "unknown severity",
			sev:  logSeverity("unknown"),
			num:  plog.SeverityNumberUnspecified,
		},
		{
			name: "Numeric Severity",
			sev:  logSeverity("9"),
			num:  plog.SeverityNumberInfo,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			num := tc.sev.severityNumber()
			require.Equal(t, tc.num, num)
		})
	}
}

func TestLogSeverity_severityValidate(t *testing.T) {
	testCases := []struct {
		name        string
		sev         logSeverity
		expectedErr error
	}{
		{
			name: "INFO severity",
			sev:  logSeverity("INFO"),
		},
		{
			name: "info severity",
			sev:  logSeverity("info"),
		},
		{
			name: "info3 severity",
			sev:  logSeverity("info3"),
		},
		{
			name: "DEBUG severity",
			sev:  logSeverity("DEBUG"),
		},
		{
			name: "ERROR severity",
			sev:  logSeverity("ERROR"),
		},
		{
			name: "WARN severity",
			sev:  logSeverity("WARN"),
		},
		{
			name: "FATAL severity",
			sev:  logSeverity("FATAL"),
		},
		{
			name:        "unknown severity",
			sev:         logSeverity("unknown"),
			expectedErr: errInvalidSeverity,
		},
		{
			name: "empty severity is valid",
			sev:  logSeverity(""),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.sev.validate()
			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLoadingConfigOTTL(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_ottl.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id           component.ID
		expected     *Config
		errorMessage string
	}{
		{
			id: component.MustNewIDWithName("filter", "ottl"),
			expected: &Config{
				ErrorMode: ottl.IgnoreError,
				Traces: TraceFilters{
					SpanConditions: []string{
						`attributes["test"] == "pass"`,
					},
					SpanEventConditions: []string{
						`attributes["test"] == "pass"`,
					},
				},
				Metrics: MetricFilters{
					MetricConditions: []string{
						`name == "pass"`,
					},
					DataPointConditions: []string{
						`attributes["test"] == "pass"`,
					},
				},
				Logs: LogFilters{
					LogConditions: []string{
						`attributes["test"] == "pass"`,
					},
				},
			},
		},
		{
			id: component.MustNewIDWithName("filter", "multiline"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				Traces: TraceFilters{
					SpanConditions: []string{
						`attributes["test"] == "pass"`,
						`attributes["test"] == "also pass"`,
					},
				},
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "spans_mix_config"),
			errorMessage: "cannot use ottl conditions and include/exclude for spans at the same time",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "metrics_mix_config"),
			errorMessage: "cannot use ottl conditions and include/exclude for metrics at the same time",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "logs_mix_config"),
			errorMessage: "cannot use ottl conditions and include/exclude for logs at the same time",
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_syntax_span"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_syntax_spanevent"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_syntax_metric"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_syntax_datapoint"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_syntax_log"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.expected == nil {
				if tt.errorMessage != "" {
					assert.EqualError(t, xconfmap.Validate(cfg), tt.errorMessage)
				} else {
					assert.Error(t, xconfmap.Validate(cfg))
				}
			} else {
				assert.NoError(t, xconfmap.Validate(cfg))
				assert.Equal(t, tt.expected, cfg)
			}
		})
	}
}
