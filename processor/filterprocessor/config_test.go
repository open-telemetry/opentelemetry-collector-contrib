// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filterprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filtermetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
	fsregexp "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset/regexp"
)

// TestLoadingConfigRegexp tests loading testdata/config_strict.yaml
func TestLoadingConfigStrict(t *testing.T) {
	// list of filters used repeatedly on testdata/config_strict.yaml
	testDataFilters := []string{
		"hello_world",
		"hello/world",
	}

	testDataMetricProperties := &filtermetric.MatchProperties{
		MatchType:   filtermetric.Strict,
		MetricNames: testDataFilters,
	}
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_strict.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       config.ComponentID
		expected *Config
	}{
		{
			id: config.NewComponentIDWithName("filter", "empty"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Metrics: MetricFilters{
					Include: &filtermetric.MatchProperties{
						MatchType: filtermetric.Strict,
					},
				},
			},
		}, {
			id: config.NewComponentIDWithName("filter", "include"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Metrics: MetricFilters{
					Include: testDataMetricProperties,
				},
			},
		}, {
			id: config.NewComponentIDWithName("filter", "exclude"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Metrics: MetricFilters{
					Exclude: testDataMetricProperties,
				},
			},
		}, {
			id: config.NewComponentIDWithName("filter", "includeexclude"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Metrics: MetricFilters{
					Include: testDataMetricProperties,
					Exclude: &filtermetric.MatchProperties{
						MatchType:   filtermetric.Strict,
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
			require.NoError(t, config.UnmarshalProcessor(sub, cfg))

			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

// TestLoadingConfigStrictLogs tests loading testdata/config_logs_strict.yaml
func TestLoadingConfigStrictLogs(t *testing.T) {

	testDataLogPropertiesInclude := &LogMatchProperties{
		LogMatchType: Strict,
		ResourceAttributes: []filterconfig.Attribute{
			{
				Key:   "should_include",
				Value: "true",
			},
		},
	}

	testDataLogPropertiesExclude := &LogMatchProperties{
		LogMatchType: Strict,
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
		id       config.ComponentID
		expected *Config
	}{
		{
			id: config.NewComponentIDWithName("filter", "empty"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Logs: LogFilters{
					Include: &LogMatchProperties{
						LogMatchType: Strict,
					},
				},
			},
		}, {
			id: config.NewComponentIDWithName("filter", "include"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Logs: LogFilters{
					Include: testDataLogPropertiesInclude,
				},
			},
		}, {
			id: config.NewComponentIDWithName("filter", "exclude"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Logs: LogFilters{
					Exclude: testDataLogPropertiesExclude,
				},
			},
		}, {
			id: config.NewComponentIDWithName("filter", "includeexclude"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
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
			require.NoError(t, config.UnmarshalProcessor(sub, cfg))

			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

// TestLoadingConfigSeverityLogsStrict tests loading testdata/config_logs_severity_strict.yaml
func TestLoadingConfigSeverityLogsStrict(t *testing.T) {

	testDataLogPropertiesInclude := &LogMatchProperties{
		LogMatchType:  Strict,
		SeverityTexts: []string{"INFO"},
	}

	testDataLogPropertiesExclude := &LogMatchProperties{
		LogMatchType:  Strict,
		SeverityTexts: []string{"DEBUG", "DEBUG2", "DEBUG3", "DEBUG4"},
	}

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_logs_severity_strict.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       config.ComponentID
		expected *Config
	}{
		{
			id: config.NewComponentIDWithName("filter", "include"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Logs: LogFilters{
					Include: testDataLogPropertiesInclude,
				},
			},
		}, {
			id: config.NewComponentIDWithName("filter", "exclude"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Logs: LogFilters{
					Exclude: testDataLogPropertiesExclude,
				},
			},
		}, {
			id: config.NewComponentIDWithName("filter", "includeexclude"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
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
			require.NoError(t, config.UnmarshalProcessor(sub, cfg))

			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

// TestLoadingConfigSeverityLogsRegexp tests loading testdata/config_logs_severity_regexp.yaml
func TestLoadingConfigSeverityLogsRegexp(t *testing.T) {
	testDataLogPropertiesInclude := &LogMatchProperties{
		LogMatchType:  Regexp,
		SeverityTexts: []string{"INFO[2-4]?"},
	}

	testDataLogPropertiesExclude := &LogMatchProperties{
		LogMatchType:  Regexp,
		SeverityTexts: []string{"DEBUG[2-4]?"},
	}

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_logs_severity_regexp.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       config.ComponentID
		expected *Config
	}{
		{
			id: config.NewComponentIDWithName("filter", "include"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Logs: LogFilters{
					Include: testDataLogPropertiesInclude,
				},
			},
		}, {
			id: config.NewComponentIDWithName("filter", "exclude"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Logs: LogFilters{
					Exclude: testDataLogPropertiesExclude,
				},
			},
		}, {
			id: config.NewComponentIDWithName("filter", "includeexclude"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
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
			require.NoError(t, config.UnmarshalProcessor(sub, cfg))

			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

// TestLoadingConfigBodyLogsStrict tests loading testdata/config_logs_body_strict.yaml
func TestLoadingConfigBodyLogsStrict(t *testing.T) {

	testDataLogPropertiesInclude := &LogMatchProperties{
		LogMatchType: Strict,
		LogBodies:    []string{"This is an important event"},
	}

	testDataLogPropertiesExclude := &LogMatchProperties{
		LogMatchType: Strict,
		LogBodies:    []string{"This event is not important"},
	}

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_logs_body_strict.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       config.ComponentID
		expected *Config
	}{
		{
			id: config.NewComponentIDWithName("filter", "include"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Logs: LogFilters{
					Include: testDataLogPropertiesInclude,
				},
			},
		}, {
			id: config.NewComponentIDWithName("filter", "exclude"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Logs: LogFilters{
					Exclude: testDataLogPropertiesExclude,
				},
			},
		}, {
			id: config.NewComponentIDWithName("filter", "includeexclude"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
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
			require.NoError(t, config.UnmarshalProcessor(sub, cfg))

			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

// TestLoadingConfigBodyLogsStrict tests loading testdata/config_logs_body_regexp.yaml
func TestLoadingConfigBodyLogsRegexp(t *testing.T) {

	testDataLogPropertiesInclude := &LogMatchProperties{
		LogMatchType: Regexp,
		LogBodies:    []string{"^IMPORTANT:"},
	}

	testDataLogPropertiesExclude := &LogMatchProperties{
		LogMatchType: Regexp,
		LogBodies:    []string{"^MINOR:"},
	}

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_logs_body_regexp.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       config.ComponentID
		expected *Config
	}{
		{
			id: config.NewComponentIDWithName("filter", "include"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Logs: LogFilters{
					Include: testDataLogPropertiesInclude,
				},
			},
		}, {
			id: config.NewComponentIDWithName("filter", "exclude"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Logs: LogFilters{
					Exclude: testDataLogPropertiesExclude,
				},
			},
		}, {
			id: config.NewComponentIDWithName("filter", "includeexclude"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
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
			require.NoError(t, config.UnmarshalProcessor(sub, cfg))

			assert.NoError(t, cfg.Validate())
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
		id       config.ComponentID
		expected *Config
	}{
		{
			id: config.NewComponentIDWithName("filter", "include"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Logs: LogFilters{
					Include: testDataLogPropertiesInclude,
				},
			},
		}, {
			id: config.NewComponentIDWithName("filter", "exclude"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Logs: LogFilters{
					Exclude: testDataLogPropertiesExclude,
				},
			},
		}, {
			id: config.NewComponentIDWithName("filter", "includeexclude"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
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
			require.NoError(t, config.UnmarshalProcessor(sub, cfg))

			assert.NoError(t, cfg.Validate())
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

	testDataMetricProperties := &filtermetric.MatchProperties{
		MatchType:   filtermetric.Regexp,
		MetricNames: testDataFilters,
	}

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_regexp.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       config.ComponentID
		expected config.Processor
	}{
		{
			id: config.NewComponentIDWithName("filter", "include"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Metrics: MetricFilters{
					Include: testDataMetricProperties,
				},
			},
		}, {
			id: config.NewComponentIDWithName("filter", "exclude"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Metrics: MetricFilters{
					Exclude: testDataMetricProperties,
				},
			},
		}, {
			id: config.NewComponentIDWithName("filter", "unlimitedcache"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Metrics: MetricFilters{
					Include: &filtermetric.MatchProperties{
						MatchType: filtermetric.Regexp,
						RegexpConfig: &fsregexp.Config{
							CacheEnabled: true,
						},
						MetricNames: testDataFilters,
					},
				},
			},
		}, {
			id: config.NewComponentIDWithName("filter", "limitedcache"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Metrics: MetricFilters{
					Exclude: &filtermetric.MatchProperties{
						MatchType: filtermetric.Regexp,
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
			require.NoError(t, config.UnmarshalProcessor(sub, cfg))

			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestLoadingSpans(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_traces.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       config.ComponentID
		expected config.Processor
	}{
		{
			id: config.NewComponentIDWithName("filter", "spans"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Spans: SpanFilters{
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
			require.NoError(t, config.UnmarshalProcessor(sub, cfg))

			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestLoadingConfigExpr(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_expr.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       config.ComponentID
		expected config.Processor
	}{
		{
			id: config.NewComponentIDWithName("filter", "empty"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Metrics: MetricFilters{
					Include: &filtermetric.MatchProperties{
						MatchType: filtermetric.Expr,
					},
				},
			},
		},
		{
			id: config.NewComponentIDWithName("filter", "include"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Metrics: MetricFilters{
					Include: &filtermetric.MatchProperties{
						MatchType: filtermetric.Expr,
						Expressions: []string{
							`Label("foo") == "bar"`,
							`HasLabel("baz")`,
						},
					},
				},
			},
		},
		{
			id: config.NewComponentIDWithName("filter", "exclude"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Metrics: MetricFilters{
					Exclude: &filtermetric.MatchProperties{
						MatchType: filtermetric.Expr,
						Expressions: []string{
							`Label("foo") == "bar"`,
							`HasLabel("baz")`,
						},
					},
				},
			},
		},
		{
			id: config.NewComponentIDWithName("filter", "includeexclude"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Metrics: MetricFilters{
					Include: &filtermetric.MatchProperties{
						MatchType: filtermetric.Expr,
						Expressions: []string{
							`HasLabel("foo")`,
						},
					},
					Exclude: &filtermetric.MatchProperties{
						MatchType: filtermetric.Expr,
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
			require.NoError(t, config.UnmarshalProcessor(sub, cfg))

			assert.NoError(t, cfg.Validate())
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
			num:  plog.SeverityNumberUndefined,
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
