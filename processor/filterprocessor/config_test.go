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
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/featuregate"
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

func TestLoadingConfigLogsInvalidSeverity(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_logs_invalid_severity.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	for k := range cm.ToStringMap() {
		t.Run(k, func(t *testing.T) {
			cfg := factory.CreateDefaultConfig()
			sub, err := cm.Sub(k)
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			err = cfg.(*Config).Validate()
			require.Error(t, err)
			require.Contains(t, err.Error(), "not a valid severity")
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

func TestValidateDeprecatedConfig(t *testing.T) {
	gate := metadata.ProcessorFilterprocessorRemoveDeprecatedConfigFeatureGate

	tests := []struct {
		name               string
		featureGateEnabled bool
		cfgFunc            func(*Config)
		expectedError      string
	}{
		// Gate disabled: all deprecated fields are still permitted.
		{
			name:               "gate_disabled_spans_include",
			featureGateEnabled: false,
			cfgFunc: func(cfg *Config) {
				cfg.Spans.Include = &filterconfig.MatchProperties{}
			},
		},
		{
			name:               "gate_disabled_spans_exclude",
			featureGateEnabled: false,
			cfgFunc: func(cfg *Config) {
				cfg.Spans.Exclude = &filterconfig.MatchProperties{}
			},
		},
		{
			name:               "gate_disabled_traces_resource",
			featureGateEnabled: false,
			cfgFunc: func(cfg *Config) {
				cfg.Traces.ResourceConditions = []string{`attributes["test"] == "pass"`}
			},
		},
		{
			name:               "gate_disabled_traces_span",
			featureGateEnabled: false,
			cfgFunc: func(cfg *Config) {
				cfg.Traces.SpanConditions = []string{`attributes["test"] == "pass"`}
			},
		},
		{
			name:               "gate_disabled_traces_spanevent",
			featureGateEnabled: false,
			cfgFunc: func(cfg *Config) {
				cfg.Traces.SpanEventConditions = []string{`attributes["test"] == "pass"`}
			},
		},
		{
			name:               "gate_disabled_metrics_include",
			featureGateEnabled: false,
			cfgFunc: func(cfg *Config) {
				cfg.Metrics.Include = &filterconfig.MetricMatchProperties{MatchType: filterconfig.MetricStrict}
			},
		},
		{
			name:               "gate_disabled_metrics_exclude",
			featureGateEnabled: false,
			cfgFunc: func(cfg *Config) {
				cfg.Metrics.Exclude = &filterconfig.MetricMatchProperties{MatchType: filterconfig.MetricStrict}
			},
		},
		{
			name:               "gate_disabled_metrics_resource",
			featureGateEnabled: false,
			cfgFunc: func(cfg *Config) {
				cfg.Metrics.ResourceConditions = []string{`attributes["test"] == "pass"`}
			},
		},
		{
			name:               "gate_disabled_metrics_metric",
			featureGateEnabled: false,
			cfgFunc: func(cfg *Config) {
				cfg.Metrics.MetricConditions = []string{`name == "pass"`}
			},
		},
		{
			name:               "gate_disabled_metrics_datapoint",
			featureGateEnabled: false,
			cfgFunc: func(cfg *Config) {
				cfg.Metrics.DataPointConditions = []string{`attributes["test"] == "pass"`}
			},
		},
		{
			name:               "gate_disabled_logs_include",
			featureGateEnabled: false,
			cfgFunc: func(cfg *Config) {
				cfg.Logs.Include = &LogMatchProperties{}
			},
		},
		{
			name:               "gate_disabled_logs_exclude",
			featureGateEnabled: false,
			cfgFunc: func(cfg *Config) {
				cfg.Logs.Exclude = &LogMatchProperties{}
			},
		},
		{
			name:               "gate_disabled_logs_resource",
			featureGateEnabled: false,
			cfgFunc: func(cfg *Config) {
				cfg.Logs.ResourceConditions = []string{`attributes["test"] == "pass"`}
			},
		},
		{
			name:               "gate_disabled_logs_log",
			featureGateEnabled: false,
			cfgFunc: func(cfg *Config) {
				cfg.Logs.LogConditions = []string{`attributes["test"] == "pass"`}
			},
		},
		{
			name:               "gate_disabled_profiles_resource",
			featureGateEnabled: false,
			cfgFunc: func(cfg *Config) {
				cfg.Profiles.ResourceConditions = []string{`attributes["test"] == "pass"`}
			},
		},
		{
			name:               "gate_disabled_profiles_profile",
			featureGateEnabled: false,
			cfgFunc: func(cfg *Config) {
				cfg.Profiles.ProfileConditions = []string{`attributes["test"] == "pass"`}
			},
		},
		// Gate enabled: each deprecated field on each signal must be rejected.
		{
			name:               "gate_enabled_spans_include",
			featureGateEnabled: true,
			cfgFunc: func(cfg *Config) {
				cfg.Spans.Include = &filterconfig.MatchProperties{}
			},
			expectedError: "`spans` and `traces` configuration keys are not allowed",
		},
		{
			name:               "gate_enabled_spans_exclude",
			featureGateEnabled: true,
			cfgFunc: func(cfg *Config) {
				cfg.Spans.Exclude = &filterconfig.MatchProperties{}
			},
			expectedError: "`spans` and `traces` configuration keys are not allowed",
		},
		{
			name:               "gate_enabled_traces_resource",
			featureGateEnabled: true,
			cfgFunc: func(cfg *Config) {
				cfg.Traces.ResourceConditions = []string{`attributes["test"] == "pass"`}
			},
			expectedError: "`spans` and `traces` configuration keys are not allowed",
		},
		{
			name:               "gate_enabled_traces_span",
			featureGateEnabled: true,
			cfgFunc: func(cfg *Config) {
				cfg.Traces.SpanConditions = []string{`attributes["test"] == "pass"`}
			},
			expectedError: "`spans` and `traces` configuration keys are not allowed",
		},
		{
			name:               "gate_enabled_traces_spanevent",
			featureGateEnabled: true,
			cfgFunc: func(cfg *Config) {
				cfg.Traces.SpanEventConditions = []string{`attributes["test"] == "pass"`}
			},
			expectedError: "`spans` and `traces` configuration keys are not allowed",
		},
		{
			name:               "gate_enabled_metrics_include",
			featureGateEnabled: true,
			cfgFunc: func(cfg *Config) {
				cfg.Metrics.Include = &filterconfig.MetricMatchProperties{MatchType: filterconfig.MetricStrict}
			},
			expectedError: "`metrics` configuration key is not allowed",
		},
		{
			name:               "gate_enabled_metrics_exclude",
			featureGateEnabled: true,
			cfgFunc: func(cfg *Config) {
				cfg.Metrics.Exclude = &filterconfig.MetricMatchProperties{MatchType: filterconfig.MetricStrict}
			},
			expectedError: "`metrics` configuration key is not allowed",
		},
		{
			name:               "gate_enabled_metrics_resource",
			featureGateEnabled: true,
			cfgFunc: func(cfg *Config) {
				cfg.Metrics.ResourceConditions = []string{`attributes["test"] == "pass"`}
			},
			expectedError: "`metrics` configuration key is not allowed",
		},
		{
			name:               "gate_enabled_metrics_metric",
			featureGateEnabled: true,
			cfgFunc: func(cfg *Config) {
				cfg.Metrics.MetricConditions = []string{`name == "pass"`}
			},
			expectedError: "`metrics` configuration key is not allowed",
		},
		{
			name:               "gate_enabled_metrics_datapoint",
			featureGateEnabled: true,
			cfgFunc: func(cfg *Config) {
				cfg.Metrics.DataPointConditions = []string{`attributes["test"] == "pass"`}
			},
			expectedError: "`metrics` configuration key is not allowed",
		},
		{
			name:               "gate_enabled_logs_include",
			featureGateEnabled: true,
			cfgFunc: func(cfg *Config) {
				cfg.Logs.Include = &LogMatchProperties{}
			},
			expectedError: "`logs` configuration key is not allowed",
		},
		{
			name:               "gate_enabled_logs_exclude",
			featureGateEnabled: true,
			cfgFunc: func(cfg *Config) {
				cfg.Logs.Exclude = &LogMatchProperties{}
			},
			expectedError: "`logs` configuration key is not allowed",
		},
		{
			name:               "gate_enabled_logs_resource",
			featureGateEnabled: true,
			cfgFunc: func(cfg *Config) {
				cfg.Logs.ResourceConditions = []string{`attributes["test"] == "pass"`}
			},
			expectedError: "`logs` configuration key is not allowed",
		},
		{
			name:               "gate_enabled_logs_log",
			featureGateEnabled: true,
			cfgFunc: func(cfg *Config) {
				cfg.Logs.LogConditions = []string{`attributes["test"] == "pass"`}
			},
			expectedError: "`logs` configuration key is not allowed",
		},
		{
			name:               "gate_enabled_trace_deprecated_and_new_style",
			featureGateEnabled: true,
			cfgFunc: func(cfg *Config) {
				cfg.Spans.Include = &filterconfig.MatchProperties{}
				cfg.TraceConditions = []condition.ContextConditions{{Conditions: []string{`attributes["test"] == "pass"`}}}
			},
			expectedError: "`spans` and `traces` configuration keys are not allowed",
		},
		{
			name:               "gate_enabled_metric_deprecated_and_new_style",
			featureGateEnabled: true,
			cfgFunc: func(cfg *Config) {
				cfg.Metrics.Include = &filterconfig.MetricMatchProperties{MatchType: filterconfig.MetricStrict}
				cfg.MetricConditions = []condition.ContextConditions{{Conditions: []string{`name == "pass"`}}}
			},
			expectedError: "`metrics` configuration key is not allowed",
		},
		{
			name:               "gate_enabled_log_deprecated_and_new_style",
			featureGateEnabled: true,
			cfgFunc: func(cfg *Config) {
				cfg.Logs.Include = &LogMatchProperties{}
				cfg.LogConditions = []condition.ContextConditions{{Conditions: []string{`attributes["test"] == "pass"`}}}
			},
			expectedError: "`logs` configuration key is not allowed",
		},
		{
			name:               "gate_enabled_profiles_resource",
			featureGateEnabled: true,
			cfgFunc: func(cfg *Config) {
				cfg.Profiles.ResourceConditions = []string{`attributes["test"] == "pass"`}
			},
			expectedError: "`profiles` configuration key is not allowed",
		},
		{
			name:               "gate_enabled_profiles_profile",
			featureGateEnabled: true,
			cfgFunc: func(cfg *Config) {
				cfg.Profiles.ProfileConditions = []string{`attributes["test"] == "pass"`}
			},
			expectedError: "`profiles` configuration key is not allowed",
		},
		{
			name:               "gate_enabled_profile_deprecated_and_new_style",
			featureGateEnabled: true,
			cfgFunc: func(cfg *Config) {
				cfg.Profiles.ProfileConditions = []string{`attributes["test"] == "pass"`}
				cfg.ProfileConditions = []condition.ContextConditions{{Conditions: []string{`profile.attributes["test"] == "pass"`}}}
			},
			expectedError: "`profiles` configuration key is not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prevValue := gate.IsEnabled()
			require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), tt.featureGateEnabled))
			t.Cleanup(func() {
				require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), prevValue))
			})

			cfg := NewFactory().CreateDefaultConfig().(*Config)
			tt.cfgFunc(cfg)

			err := cfg.Validate()
			if tt.expectedError != "" {
				require.ErrorContains(t, err, tt.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestFeatureGateWithTestdataConfigs validates config entries from the testdata
// files already used in config_test.go with the RemoveDeprecatedConfig feature
// gate enabled. Only entries that are valid without the gate are included;
// entries that are already invalid for other reasons are omitted.
func TestFeatureGateWithTestdataConfigs(t *testing.T) {
	gate := metadata.ProcessorFilterprocessorRemoveDeprecatedConfigFeatureGate

	require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), true))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), false))
	})

	tests := []struct {
		file        string
		id          string
		expectError bool
	}{
		// Deprecated metrics.include / metrics.exclude — rejected by gate.
		{"config_strict.yaml", "filter/empty", true},
		{"config_strict.yaml", "filter/include", true},
		{"config_strict.yaml", "filter/exclude", true},
		{"config_strict.yaml", "filter/includeexclude", true},

		// Deprecated logs.include / logs.exclude — rejected by gate.
		{"config_logs_strict.yaml", "filter/empty", true},
		{"config_logs_strict.yaml", "filter/include", true},
		{"config_logs_strict.yaml", "filter/exclude", true},
		{"config_logs_strict.yaml", "filter/includeexclude", true},
		{"config_logs_severity_strict.yaml", "filter/include", true},
		{"config_logs_severity_strict.yaml", "filter/exclude", true},
		{"config_logs_severity_strict.yaml", "filter/includeexclude", true},
		{"config_logs_severity_regexp.yaml", "filter/include", true},
		{"config_logs_severity_regexp.yaml", "filter/exclude", true},
		{"config_logs_severity_regexp.yaml", "filter/includeexclude", true},
		{"config_logs_body_strict.yaml", "filter/include", true},
		{"config_logs_body_strict.yaml", "filter/exclude", true},
		{"config_logs_body_strict.yaml", "filter/includeexclude", true},
		{"config_logs_body_regexp.yaml", "filter/include", true},
		{"config_logs_body_regexp.yaml", "filter/exclude", true},
		{"config_logs_body_regexp.yaml", "filter/includeexclude", true},
		{"config_logs_min_severity.yaml", "filter/include", true},
		{"config_logs_min_severity.yaml", "filter/exclude", true},
		{"config_logs_min_severity.yaml", "filter/includeexclude", true},

		// Deprecated metrics.include / metrics.exclude (regexp) — rejected by gate.
		// "filter" has no fields set so it passes; the others are rejected.
		{"config_regexp.yaml", "filter", false},
		{"config_regexp.yaml", "filter/include", true},
		{"config_regexp.yaml", "filter/exclude", true},
		{"config_regexp.yaml", "filter/unlimitedcache", true},
		{"config_regexp.yaml", "filter/limitedcache", true},

		// Deprecated spans.include / spans.exclude — rejected by gate.
		{"config_traces.yaml", "filter/spans", true},

		// Deprecated metrics.include / metrics.exclude (expr) — rejected by gate.
		{"config_expr.yaml", "filter/empty", true},
		{"config_expr.yaml", "filter/include", true},
		{"config_expr.yaml", "filter/exclude", true},
		{"config_expr.yaml", "filter/includeexclude", true},

		// Deprecated profiles sub-keys — rejected by gate.
		{"config_profiles.yaml", "filter/profiles", true},

		// Deprecated traces/metrics/logs sub-keys (OTTL style) — rejected by gate.
		{"config_ottl.yaml", "filter/ottl", true},
		{"config_ottl.yaml", "filter/multiline", true},

		// New-style *_conditions — gate has no effect, all pass.
		{"config_ottl.yaml", "filter/context_inferred_trace", false},
		{"config_ottl.yaml", "filter/context_inferred_metric", false},
		{"config_ottl.yaml", "filter/context_inferred_log", false},
		{"config_ottl.yaml", "filter/context_inferred_profile", false},
		{"config_ottl.yaml", "filter/context_inferred_with_error_mode", false},
		{"config_ottl.yaml", "filter/context_inferred_multiple_conditions", false},
		{"config_ottl.yaml", "filter/flat_style", false},
		{"config_ottl.yaml", "filter/advance_style", false},
		{"config_ottl.yaml", "filter/context_conditions_error_mode", false},
	}

	loadedConfs := map[string]*confmap.Conf{}
	for _, tt := range tests {
		t.Run(tt.file+"/"+tt.id, func(t *testing.T) {
			if _, ok := loadedConfs[tt.file]; !ok {
				cm, err := confmaptest.LoadConf(filepath.Join("testdata", tt.file))
				require.NoError(t, err)
				loadedConfs[tt.file] = cm
			}

			cfg := NewFactory().CreateDefaultConfig()
			sub, err := loadedConfs[tt.file].Sub(tt.id)
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			err = cfg.(*Config).Validate()
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
