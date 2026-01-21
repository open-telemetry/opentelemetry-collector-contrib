// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterprocessor

import (
	"fmt"
	"path/filepath"
	"strings"
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
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/condition"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/metadata"
)

func assertConfigContainsDefaultFunctions(t *testing.T, config Config) {
	t.Helper()
	for _, f := range DefaultLogFunctionsNew() {
		assert.Contains(t, config.logFunctions, f.Name(), "missing log function %v", f.Name())
	}
	for _, f := range DefaultDataPointFunctionsNew() {
		assert.Contains(t, config.dataPointFunctions, f.Name(), "missing data point function %v", f.Name())
	}
	for _, f := range DefaultMetricFunctionsNew() {
		assert.Contains(t, config.metricFunctions, f.Name(), "missing metric function %v", f.Name())
	}
	for _, f := range DefaultSpanFunctionsNew() {
		assert.Contains(t, config.spanFunctions, f.Name(), "missing span function %v", f.Name())
	}
	for _, f := range DefaultSpanEventFunctionsNew() {
		assert.Contains(t, config.spanEventFunctions, f.Name(), "missing span event function %v", f.Name())
	}
	for _, f := range DefaultProfileFunctions() {
		assert.Contains(t, config.profileFunctions, f.Name(), "missing profile function %v", f.Name())
	}
}

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
			assert.EqualExportedValues(t, tt.expected, cfg)
			assertConfigContainsDefaultFunctions(t, *cfg.(*Config))
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
			assert.EqualExportedValues(t, tt.expected, cfg)
			assertConfigContainsDefaultFunctions(t, *cfg.(*Config))
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
			assert.EqualExportedValues(t, tt.expected, cfg)
			assertConfigContainsDefaultFunctions(t, *cfg.(*Config))
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
			assert.EqualExportedValues(t, tt.expected, cfg)
			assertConfigContainsDefaultFunctions(t, *cfg.(*Config))
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
			assert.EqualExportedValues(t, tt.expected, cfg)
			assertConfigContainsDefaultFunctions(t, *cfg.(*Config))
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
			assert.EqualExportedValues(t, tt.expected, cfg)
			assertConfigContainsDefaultFunctions(t, *cfg.(*Config))
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
			assert.EqualExportedValues(t, tt.expected, cfg)
			assertConfigContainsDefaultFunctions(t, *cfg.(*Config))
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
			assert.EqualExportedValues(t, tt.expected, cfg)
			assertConfigContainsDefaultFunctions(t, *cfg.(*Config))
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
			assert.EqualExportedValues(t, tt.expected, cfg)
			assertConfigContainsDefaultFunctions(t, *cfg.(*Config))
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
			assert.EqualExportedValues(t, tt.expected, cfg)
			assertConfigContainsDefaultFunctions(t, *cfg.(*Config))
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
					ResourceConditions: []string{
						`attributes["test"] == "pass"`,
					},
					SpanConditions: []string{
						`attributes["test"] == "pass"`,
					},
					SpanEventConditions: []string{
						`attributes["test"] == "pass"`,
					},
				},
				Metrics: MetricFilters{
					ResourceConditions: []string{
						`attributes["test"] == "pass"`,
					},
					MetricConditions: []string{
						`name == "pass"`,
					},
					DataPointConditions: []string{
						`attributes["test"] == "pass"`,
					},
				},
				Logs: LogFilters{
					ResourceConditions: []string{
						`attributes["test"] == "pass"`,
					},
					LogConditions: []string{
						`attributes["test"] == "pass"`,
					},
				},
				Profiles: ProfileFilters{
					ResourceConditions: []string{
						`attributes["test"] == "pass"`,
					},
					ProfileConditions: []string{
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
			errorMessage: "cannot use \"traces.resource\", \"traces.span\", \"traces.spanevent\" and the span settings \"spans.include\", \"spans.exclude\" at the same time",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "metrics_mix_config"),
			errorMessage: "cannot use \"metrics.resource\", \"metrics.metric\", \"metrics.datapoint\" and the settings \"metrics.include\", \"metrics.exclude\" at the same time",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "logs_mix_config"),
			errorMessage: "cannot use \"logs.resource\", \"logs.log\" and the settings \"logs.include\", \"logs.exclude\" at the same time",
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
		{
			id: component.NewIDWithName(metadata.Type, "context_inferred_trace"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				TraceConditions: []condition.ContextConditions{
					{
						Conditions: []string{`span.attributes["test"] == "pass"`},
						ErrorMode:  "",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "context_inferred_metric"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				MetricConditions: []condition.ContextConditions{
					{
						Conditions: []string{`metric.name == "pass"`},
						ErrorMode:  "",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "context_inferred_log"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				LogConditions: []condition.ContextConditions{
					{
						Conditions: []string{`log.attributes["test"] == "pass"`},
						ErrorMode:  "",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "context_inferred_profile"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				ProfileConditions: []condition.ContextConditions{
					{
						Conditions: []string{`profile.attributes["test"] == "pass"`},
						ErrorMode:  "",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "context_inferred_with_error_mode"),
			expected: &Config{
				ErrorMode: ottl.IgnoreError,
				TraceConditions: []condition.ContextConditions{
					{
						Conditions: []string{`span.attributes["test"] == "pass"`},
						ErrorMode:  "",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "context_inferred_multiple_conditions"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				TraceConditions: []condition.ContextConditions{
					{
						Conditions: []string{
							`span.attributes["test"] == "pass"`,
							`span.attributes["test2"] == "also pass"`,
						},
						ErrorMode: "",
					},
					{
						Conditions: []string{`span.attributes["event"] == "pass"`},
						ErrorMode:  "",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "context_conditions_error_mode"),
			expected: &Config{
				ErrorMode: ottl.IgnoreError,
				TraceConditions: []condition.ContextConditions{
					{
						Conditions: []string{`span.attributes["test"] == "pass"`},
						ErrorMode:  ottl.SilentError,
					},
				},
				MetricConditions: []condition.ContextConditions{
					{
						Conditions: []string{`metric.name == "pass"`},
						ErrorMode:  ottl.SilentError,
					},
				},
				LogConditions: []condition.ContextConditions{
					{
						Conditions: []string{`log.attributes["test"] == "pass"`},
						ErrorMode:  ottl.PropagateError,
					},
				},
				ProfileConditions: []condition.ContextConditions{
					{
						Conditions: []string{`profile.attributes["test"] == "pass"`},
						ErrorMode:  ottl.SilentError,
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "flat_style"),
			expected: &Config{
				ErrorMode:         ottl.PropagateError,
				TraceConditions:   getInferredContextConditions("span"),
				MetricConditions:  getInferredContextConditions("datapoint"),
				LogConditions:     getInferredContextConditions("log"),
				ProfileConditions: getInferredContextConditions("profile"),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "advance_style"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
				TraceConditions: []condition.ContextConditions{
					getDefinedContextConditions("span"),
					getDefinedContextConditions("spanevent"),
					getDefinedContextConditions("scope"),
					getDefinedContextConditions("resource"),
				},
				MetricConditions: []condition.ContextConditions{
					{
						Conditions: []string{`name == "pass"`},
						Context:    "metric",
					},
					getDefinedContextConditions("datapoint"),
					getDefinedContextConditions("scope"),
					getDefinedContextConditions("resource"),
				},
				LogConditions: []condition.ContextConditions{
					getDefinedContextConditions("log"),
					getDefinedContextConditions("scope"),
					getDefinedContextConditions("resource"),
				},
				ProfileConditions: []condition.ContextConditions{
					getDefinedContextConditions("profile"),
					getDefinedContextConditions("scope"),
					getDefinedContextConditions("resource"),
				},
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "mix_trace_conditions"),
			errorMessage: `cannot use context inferred trace conditions "trace_conditions" and the settings "traces.resource", "traces.span", "traces.spanevent" at the same time`,
		},
		{
			id:           component.NewIDWithName(metadata.Type, "mix_metric_conditions"),
			errorMessage: `cannot use context inferred metric conditions "metric_conditions" and the settings "metrics.resource", "metrics.metric", "metrics.datapoint", "metrics.include", "metrics.exclude" at the same time`,
		},
		{
			id:           component.NewIDWithName(metadata.Type, "mix_log_conditions"),
			errorMessage: `cannot use context inferred log conditions "log_conditions" and the settings "logs.resource", "logs.log", "logs.include", "logs.exclude" at the same time`,
		},
		{
			id:           component.NewIDWithName(metadata.Type, "mix_profile_conditions"),
			errorMessage: `cannot use context inferred profile conditions "profile_conditions" and the settings "profiles.resource", "profiles.profile" at the same time`,
		},
		{
			id:           component.NewIDWithName(metadata.Type, "mix_metric_conditions_with_include"),
			errorMessage: `cannot use context inferred metric conditions "metric_conditions" and the settings "metrics.resource", "metrics.metric", "metrics.datapoint", "metrics.include", "metrics.exclude" at the same time`,
		},
		{
			id:           component.NewIDWithName(metadata.Type, "mix_log_conditions_with_exclude"),
			errorMessage: `cannot use context inferred log conditions "log_conditions" and the settings "logs.resource", "logs.log", "logs.include", "logs.exclude" at the same time`,
		},
		{
			id:           component.NewIDWithName(metadata.Type, "bad_syntax_context_inferred_trace"),
			errorMessage: `condition has invalid syntax: 1:23: unexpected token "<EOF>" (expected Field ("." Field)*)`,
		},
		{
			id:           component.NewIDWithName(metadata.Type, "bad_syntax_context_inferred_metric"),
			errorMessage: `converter names must start with an uppercase letter but got 'invalid_function'`,
		},
		{
			id:           component.NewIDWithName(metadata.Type, "bad_syntax_context_inferred_log"),
			errorMessage: `condition has invalid syntax: 1:11: unexpected token "condition" (expected <opcomparison> Value)`,
		},
		{
			id:           component.NewIDWithName(metadata.Type, "bad_syntax_context_inferred_profile"),
			errorMessage: `condition has invalid syntax: 1:11: unexpected token "condition" (expected <opcomparison> Value)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_ottl.yaml"))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.expected == nil {
				err = xconfmap.Validate(cfg)
				assert.Error(t, err)

				if tt.errorMessage != "" {
					assert.EqualError(t, err, tt.errorMessage)
				}
			} else {
				assert.NoError(t, xconfmap.Validate(cfg))
				assert.EqualExportedValues(t, tt.expected, cfg)
				assertConfigContainsDefaultFunctions(t, *cfg.(*Config))
			}
		})
	}
}

func Test_UnknownErrorMode(t *testing.T) {
	id := component.NewIDWithName(metadata.Type, "unknown_error_mode")

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_ottl.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(id.String())
	require.NoError(t, err)
	assert.ErrorContains(t, sub.Unmarshal(cfg), "unknown error mode test")
}

func Test_UnknownContext(t *testing.T) {
	id := component.NewIDWithName(metadata.Type, "unknown_context")

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_ottl.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(id.String())
	require.NoError(t, err)
	assert.ErrorContains(t, sub.Unmarshal(cfg), "unknown context abc")
}

func Test_EmptyFlatStyle(t *testing.T) {
	id := component.NewIDWithName(metadata.Type, "empty_flat_style")

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_ottl.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(id.String())
	require.NoError(t, err)
	assert.ErrorContains(t, sub.Unmarshal(cfg), "condition cannot be empty")
}

func Test_MixedConfigurationStyles(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_ottl.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "mixed_advance_and_flat_styles").String())
	require.NoError(t, err)
	assert.ErrorContains(t, sub.Unmarshal(cfg), "configuring multiple configuration styles is not supported")
}

func getInferredContextConditions(prefix string) []condition.ContextConditions {
	return []condition.ContextConditions{
		{
			Conditions: getConditionStrings(prefix),
		},
	}
}

func getDefinedContextConditions(prefix string) condition.ContextConditions {
	return condition.ContextConditions{
		Conditions: getConditionStrings(""),
		Context:    condition.ContextID(prefix),
	}
}

func getConditionStrings(prefix string) []string {
	if prefix != "" && !strings.HasSuffix(prefix, ".") {
		prefix += "."
	}
	return []string{
		fmt.Sprintf(`%sattributes["test"] == "pass"`, prefix),
		fmt.Sprintf(`%sattributes["another"] == "pass"`, prefix),
	}
}
