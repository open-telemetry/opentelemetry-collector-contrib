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
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/metadata"
)

func assertConfigContainsDefaultFunctions(t *testing.T, config Config) {
	t.Helper()
	for _, f := range DefaultLogFunctions() {
		assert.Contains(t, config.logFunctions, f.Name(), "missing log function %v", f.Name())
	}
	for _, f := range DefaultDataPointFunctions() {
		assert.Contains(t, config.dataPointFunctions, f.Name(), "missing data point function %v", f.Name())
	}
	for _, f := range DefaultMetricFunctions() {
		assert.Contains(t, config.metricFunctions, f.Name(), "missing metric function %v", f.Name())
	}
	for _, f := range DefaultSpanFunctions() {
		assert.Contains(t, config.spanFunctions, f.Name(), "missing span function %v", f.Name())
	}
	for _, f := range DefaultSpanEventFunctions() {
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
			expected: createConfig(func(cfg *Config) {
				cfg.Metrics.Include = &filterconfig.MetricMatchProperties{
					MatchType: filterconfig.MetricStrict,
				}
			}),
		},
		{
			id: component.MustNewIDWithName("filter", "include"),
			expected: createConfig(func(cfg *Config) {
				cfg.Metrics.Include = testDataMetricProperties
			}),
		},
		{
			id: component.MustNewIDWithName("filter", "exclude"),
			expected: createConfig(func(cfg *Config) {
				cfg.Metrics.Exclude = testDataMetricProperties
			}),
		},
		{
			id: component.MustNewIDWithName("filter", "includeexclude"),
			expected: createConfig(func(cfg *Config) {
				cfg.Metrics.Include = testDataMetricProperties
				cfg.Metrics.Exclude = &filterconfig.MetricMatchProperties{
					MatchType:   filterconfig.MetricStrict,
					MetricNames: []string{"hello_world"},
				}
			}),
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
			expected: createConfig(func(cfg *Config) {
				cfg.Logs.Include = &LogMatchProperties{LogMatchType: strictType}
			}),
		}, {
			id: component.MustNewIDWithName("filter", "include"),
			expected: createConfig(func(cfg *Config) {
				cfg.Logs.Include = testDataLogPropertiesInclude
			}),
		}, {
			id: component.MustNewIDWithName("filter", "exclude"),
			expected: createConfig(func(cfg *Config) {
				cfg.Logs.Exclude = testDataLogPropertiesExclude
			}),
		}, {
			id: component.MustNewIDWithName("filter", "includeexclude"),
			expected: createConfig(func(cfg *Config) {
				cfg.Logs.Include = testDataLogPropertiesInclude
				cfg.Logs.Exclude = testDataLogPropertiesExclude
			}),
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
			expected: createConfig(func(cfg *Config) {
				cfg.Logs.Include = testDataLogPropertiesInclude
			}),
		},
		{
			id: component.MustNewIDWithName("filter", "exclude"),
			expected: createConfig(func(cfg *Config) {
				cfg.Logs.Exclude = testDataLogPropertiesExclude
			}),
		},
		{
			id: component.MustNewIDWithName("filter", "includeexclude"),
			expected: createConfig(func(cfg *Config) {
				cfg.Logs.Include = testDataLogPropertiesInclude
				cfg.Logs.Exclude = testDataLogPropertiesExclude
			}),
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
			expected: createConfig(func(cfg *Config) {
				cfg.Logs.Include = testDataLogPropertiesInclude
			}),
		},
		{
			id: component.MustNewIDWithName("filter", "exclude"),
			expected: createConfig(func(cfg *Config) {
				cfg.Logs.Exclude = testDataLogPropertiesExclude
			}),
		},
		{
			id: component.MustNewIDWithName("filter", "includeexclude"),
			expected: createConfig(func(cfg *Config) {
				cfg.Logs.Include = testDataLogPropertiesInclude
				cfg.Logs.Exclude = testDataLogPropertiesExclude
			}),
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
			expected: createConfig(func(cfg *Config) {
				cfg.Logs.Include = testDataLogPropertiesInclude
			}),
		},
		{
			id: component.MustNewIDWithName("filter", "exclude"),
			expected: createConfig(func(cfg *Config) {
				cfg.Logs.Exclude = testDataLogPropertiesExclude
			}),
		},
		{
			id: component.MustNewIDWithName("filter", "includeexclude"),
			expected: createConfig(func(cfg *Config) {
				cfg.Logs.Include = testDataLogPropertiesInclude
				cfg.Logs.Exclude = testDataLogPropertiesExclude
			}),
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
			expected: createConfig(func(cfg *Config) {
				cfg.Logs.Include = testDataLogPropertiesInclude
			}),
		}, {
			id: component.MustNewIDWithName("filter", "exclude"),
			expected: createConfig(func(cfg *Config) {
				cfg.Logs.Exclude = testDataLogPropertiesExclude
			}),
		}, {
			id: component.MustNewIDWithName("filter", "includeexclude"),
			expected: createConfig(func(cfg *Config) {
				cfg.Logs.Include = testDataLogPropertiesInclude
				cfg.Logs.Exclude = testDataLogPropertiesExclude
			}),
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
			expected: createConfig(func(cfg *Config) {
				cfg.Logs.Include = testDataLogPropertiesInclude
			}),
		}, {
			id: component.MustNewIDWithName("filter", "exclude"),
			expected: createConfig(func(cfg *Config) {
				cfg.Logs.Exclude = testDataLogPropertiesExclude
			}),
		}, {
			id: component.MustNewIDWithName("filter", "includeexclude"),
			expected: createConfig(func(cfg *Config) {
				cfg.Logs.Include = testDataLogPropertiesInclude
				cfg.Logs.Exclude = testDataLogPropertiesExclude
			}),
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
			expected: createConfig(func(cfg *Config) {
				cfg.Metrics.Include = testDataMetricProperties
			}),
		}, {
			id: component.MustNewIDWithName("filter", "exclude"),
			expected: createConfig(func(cfg *Config) {
				cfg.Metrics.Exclude = testDataMetricProperties
			}),
		}, {
			id: component.MustNewIDWithName("filter", "unlimitedcache"),
			expected: createConfig(func(cfg *Config) {
				cfg.Metrics.Include = &filterconfig.MetricMatchProperties{
					MatchType: filterconfig.MetricRegexp,
					RegexpConfig: &fsregexp.Config{
						CacheEnabled: true,
					},
					MetricNames: testDataFilters,
				}
			}),
		}, {
			id: component.MustNewIDWithName("filter", "limitedcache"),
			expected: createConfig(func(cfg *Config) {
				cfg.Metrics.Exclude = &filterconfig.MetricMatchProperties{
					MatchType: filterconfig.MetricRegexp,
					RegexpConfig: &fsregexp.Config{
						CacheEnabled:       true,
						CacheMaxNumEntries: 10,
					},
					MetricNames: testDataFilters,
				}
			}),
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
			expected: createConfig(func(cfg *Config) {
				cfg.Spans.Include = &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Services: []string{"test", "test2"},
					Attributes: []filterconfig.Attribute{
						{Key: "should_include", Value: "(true|probably_true)"},
					},
				}
				cfg.Spans.Exclude = &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Attributes: []filterconfig.Attribute{
						{Key: "should_exclude", Value: "(probably_false|false)"},
					},
				}
			}),
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
			expected: createConfig(func(cfg *Config) {
				cfg.Metrics.Include = &filterconfig.MetricMatchProperties{
					MatchType: filterconfig.MetricExpr,
				}
			}),
		},
		{
			id: component.MustNewIDWithName("filter", "include"),
			expected: createConfig(func(cfg *Config) {
				cfg.Metrics.Include = &filterconfig.MetricMatchProperties{
					MatchType: filterconfig.MetricExpr,
					Expressions: []string{
						`Label("foo") == "bar"`,
						`HasLabel("baz")`,
					},
				}
			}),
		},
		{
			id: component.MustNewIDWithName("filter", "exclude"),
			expected: createConfig(func(cfg *Config) {
				cfg.Metrics.Exclude = &filterconfig.MetricMatchProperties{
					MatchType: filterconfig.MetricExpr,
					Expressions: []string{
						`Label("foo") == "bar"`,
						`HasLabel("baz")`,
					},
				}
			}),
		},
		{
			id: component.MustNewIDWithName("filter", "includeexclude"),
			expected: createConfig(func(cfg *Config) {
				cfg.Metrics.Include = &filterconfig.MetricMatchProperties{
					MatchType: filterconfig.MetricExpr,
					Expressions: []string{
						`HasLabel("foo")`,
					},
				}
				cfg.Metrics.Exclude = &filterconfig.MetricMatchProperties{
					MatchType: filterconfig.MetricExpr,
					Expressions: []string{
						`HasLabel("bar")`,
					},
				}
			}),
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
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_ottl.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id              component.ID
		expected        *Config
		errorMessage    string
		unmarshalErrMsg string
	}{
		{
			id: component.MustNewIDWithName("filter", "ottl"),
			expected: createConfig(func(cfg *Config) {
				cfg.ErrorMode = ottl.IgnoreError
				cfg.Traces.SpanConditions = []string{
					`attributes["test"] == "pass"`,
				}
				cfg.Traces.SpanEventConditions = []string{
					`attributes["test"] == "pass"`,
				}
				cfg.Metrics.MetricConditions = []string{
					`name == "pass"`,
				}
				cfg.Metrics.DataPointConditions = []string{
					`attributes["test"] == "pass"`,
				}
				cfg.Logs.LogConditions = []string{
					`attributes["test"] == "pass"`,
				}
				cfg.Profiles.ProfileConditions = []string{
					`attributes["test"] == "pass"`,
				}
			}),
		},
		{
			id: component.MustNewIDWithName("filter", "multiline"),
			expected: createConfig(func(cfg *Config) {
				cfg.Traces.SpanConditions = []string{
					`attributes["test"] == "pass"`,
					`attributes["test"] == "also pass"`,
				}
			}),
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
		{
			id: component.NewIDWithName(metadata.Type, "bad_action"),
			unmarshalErrMsg: func() string {
				signals := []string{"metrics", "logs", "traces", "profiles"}
				lines := make([]string, len(signals))
				for i, signal := range signals {
					lines[i] = fmt.Sprintf("'%s.action' unknown action \"invalid\": must be \"drop\" or \"keep\"", signal)
				}
				return strings.Join(lines, "\n")
			}(),
		},
		{
			id: component.MustNewIDWithName("filter", "keep_action"),
			expected: createConfig(func(cfg *Config) {
				cfg.Metrics.Action = keepAction
				cfg.Logs.Action = keepAction
				cfg.Traces.Action = keepAction
				cfg.Profiles.Action = keepAction
			}),
		},
		{
			id: component.MustNewIDWithName("filter", "drop_action"),
			expected: createConfig(func(cfg *Config) {
				cfg.Metrics.Action = dropAction
				cfg.Logs.Action = dropAction
				cfg.Traces.Action = dropAction
				cfg.Profiles.Action = dropAction
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)

			if tt.unmarshalErrMsg != "" {
				require.ErrorContains(t, sub.Unmarshal(cfg), tt.unmarshalErrMsg)
				return
			}
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.expected == nil {
				if tt.errorMessage != "" {
					assert.EqualError(t, xconfmap.Validate(cfg), tt.errorMessage)
				} else {
					assert.Error(t, xconfmap.Validate(cfg))
				}
			} else {
				assert.NoError(t, xconfmap.Validate(cfg))
				assert.EqualExportedValues(t, tt.expected, cfg)
				assertConfigContainsDefaultFunctions(t, *cfg.(*Config))
			}
		})
	}
}

func TestAction_UnmarshalText(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expected    Action
		expectedErr bool
	}{
		{
			name:     "valid drop action lowercase",
			input:    "drop",
			expected: dropAction,
		},
		{
			name:     "valid keep action lowercase",
			input:    "keep",
			expected: keepAction,
		},
		{
			name:        "invalid action",
			input:       "invalid",
			expectedErr: true,
		},
		{
			name:        "empty action",
			input:       "",
			expectedErr: true,
		},
		{
			name:        "unknown action",
			input:       "delete",
			expectedErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var action Action
			err := action.UnmarshalText([]byte(tc.input))

			if tc.expectedErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "unknown action")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, action)
			}
		})
	}
}

func createConfig(modify func(cfg *Config)) *Config {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	if modify != nil {
		modify(cfg)
	}

	return cfg
}
