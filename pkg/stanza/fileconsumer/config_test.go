// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/emittest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/regex"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestNewConfig(t *testing.T) {
	cfg := NewConfig()
	assert.Equal(t, 200*time.Millisecond, cfg.PollInterval)
	assert.Equal(t, defaultMaxConcurrentFiles, cfg.MaxConcurrentFiles)
	assert.Equal(t, "end", cfg.StartAt)
	assert.Equal(t, fingerprint.DefaultSize, int(cfg.FingerprintSize))
	assert.Equal(t, defaultEncoding, cfg.Encoding)
	assert.Equal(t, reader.DefaultMaxLogSize, int(cfg.MaxLogSize))
	assert.Equal(t, reader.DefaultFlushPeriod, cfg.FlushPeriod)
	assert.True(t, cfg.IncludeFileName)
	assert.False(t, cfg.IncludeFilePath)
	assert.False(t, cfg.IncludeFileNameResolved)
	assert.False(t, cfg.IncludeFilePathResolved)
}

func TestUnmarshal(t *testing.T) {
	operatortest.ConfigUnmarshalTests{
		DefaultConfig: newMockOperatorConfig(NewConfig()),
		TestsFile:     filepath.Join(".", "testdata", "config.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name: "include_one",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "one.log")
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "include_multi",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "one.log", "two.log", "three.log")
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "include_glob",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "*.log")
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "include_glob_double_asterisk",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "**.log")
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "include_glob_double_asterisk_nested",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "directory/**/*.log")
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "include_glob_double_asterisk_prefix",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "**/directory/**/*.log")
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "include_inline",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "a.log", "b.log")
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "include_string",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "aString")
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "exclude_one",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "*.log")
					cfg.Exclude = append(cfg.Exclude, "one.log")
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "exclude_multi",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "*.log")
					cfg.Exclude = append(cfg.Exclude, "one.log", "two.log", "three.log")
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "exclude_glob",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "*.log")
					cfg.Exclude = append(cfg.Exclude, "not*.log")
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "exclude_glob_double_asterisk",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "*.log")
					cfg.Exclude = append(cfg.Exclude, "not**.log")
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "exclude_glob_double_asterisk_nested",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "*.log")
					cfg.Exclude = append(cfg.Exclude, "directory/**/not*.log")
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "exclude_glob_double_asterisk_prefix",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "*.log")
					cfg.Exclude = append(cfg.Exclude, "**/directory/**/not*.log")
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "exclude_inline",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "*.log")
					cfg.Exclude = append(cfg.Exclude, "a.log", "b.log")
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "exclude_string",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "*.log")
					cfg.Exclude = append(cfg.Exclude, "aString")
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "sort_by_timestamp",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.OrderingCriteria = matcher.OrderingCriteria{
						Regex: `err\.[a-zA-Z]\.\d+\.(?P<rotation_time>\d{10})\.log`,
						SortBy: []matcher.Sort{
							{
								SortType:  "timestamp",
								RegexKey:  "rotation_time",
								Ascending: true,
								Location:  "utc",
								Layout:    `%Y%m%d%H`,
							},
						},
					}
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "sort_by_numeric",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.OrderingCriteria = matcher.OrderingCriteria{
						Regex: `err\.(?P<file_num>[a-zA-Z])\.\d+\.\d{10}\.log`,
						SortBy: []matcher.Sort{
							{
								SortType: "numeric",
								RegexKey: "file_num",
							},
						},
					}
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "poll_interval_no_units",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.PollInterval = time.Second
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "poll_interval_1s",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.PollInterval = time.Second
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "poll_interval_1ms",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.PollInterval = time.Millisecond
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "poll_interval_1000ms",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.PollInterval = time.Second
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "fingerprint_size_no_units",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.FingerprintSize = helper.ByteSize(1000)
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "fingerprint_size_1kb_lower",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.FingerprintSize = helper.ByteSize(1000)
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "fingerprint_size_1KB",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.FingerprintSize = helper.ByteSize(1000)
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "fingerprint_size_1kib_lower",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.FingerprintSize = helper.ByteSize(1024)
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "fingerprint_size_1KiB",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.FingerprintSize = helper.ByteSize(1024)
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "fingerprint_size_float",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.FingerprintSize = helper.ByteSize(1100)
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "multiline_line_start_string",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.SplitConfig.LineStartPattern = "Start"
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "multiline_line_start_special",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.SplitConfig.LineStartPattern = "%"
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "multiline_line_end_string",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.SplitConfig.LineEndPattern = "Start"
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "multiline_line_end_special",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.SplitConfig.LineEndPattern = "%"
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "start_at_string",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.StartAt = "beginning"
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "max_concurrent_large",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.MaxConcurrentFiles = 9223372036854775807
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "max_log_size_mib_lower",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.MaxLogSize = helper.ByteSize(1048576)
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "max_log_size_mib_upper",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.MaxLogSize = helper.ByteSize(1048576)
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "max_log_size_mb_upper",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.MaxLogSize = helper.ByteSize(1048576)
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "max_log_size_mb_lower",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.MaxLogSize = helper.ByteSize(1048576)
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "encoding_lower",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.Encoding = "utf-16le"
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "encoding_upper",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.Encoding = "UTF-16lE"
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "max_batches_1",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.MaxBatches = 1
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "header_config",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					regexCfg := regex.NewConfig()
					cfg.Header = &HeaderConfig{
						Pattern: "^#",
						MetadataOperators: []operator.Config{
							{
								Builder: regexCfg,
							},
						},
					}
					return newMockOperatorConfig(cfg)
				}(),
			},
			{
				Name: "ordering_criteria_top_n",
				Expect: func() *mockOperatorConfig {
					cfg := NewConfig()
					cfg.OrderingCriteria = matcher.OrderingCriteria{
						TopN: 10,
					}
					return newMockOperatorConfig(cfg)
				}(),
			},
		},
	}.Run(t)
}

func TestBuild(t *testing.T) {
	t.Parallel()

	basicConfig := func() *Config {
		cfg := NewConfig()
		cfg.Include = []string{"/var/log/testpath.*"}
		cfg.Exclude = []string{"/var/log/testpath.ex*"}
		cfg.PollInterval = 10 * time.Millisecond
		return cfg
	}

	cases := []struct {
		name             string
		modifyBaseConfig func(*Config)
		errorRequirement require.ErrorAssertionFunc
		validate         func(*testing.T, *Manager)
	}{
		{
			"Basic",
			func(cfg *Config) {},
			require.NoError,
			func(t *testing.T, m *Manager) {
				require.Equal(t, m.pollInterval, 10*time.Millisecond)
			},
		},
		{
			"BadIncludeGlob",
			func(cfg *Config) {
				cfg.Include = []string{"["}
			},
			require.Error,
			nil,
		},
		{
			"BadExcludeGlob",
			func(cfg *Config) {
				cfg.Include = []string{"["}
			},
			require.Error,
			nil,
		},
		{
			"MultilineConfiguredStartAndEndPatterns",
			func(cfg *Config) {
				cfg.SplitConfig.LineEndPattern = "Exists"
				cfg.SplitConfig.LineStartPattern = "Exists"
			},
			require.Error,
			nil,
		},
		{
			"MultilineConfiguredStartPattern",
			func(cfg *Config) {
				cfg.SplitConfig.LineStartPattern = "START.*"
			},
			require.NoError,
			func(t *testing.T, f *Manager) {},
		},
		{
			"MultilineConfiguredEndPattern",
			func(cfg *Config) {
				cfg.SplitConfig.LineEndPattern = "END.*"
			},
			require.NoError,
			func(t *testing.T, f *Manager) {},
		},
		{
			"InvalidEncoding",
			func(cfg *Config) {
				cfg.Encoding = "UTF-3233"
			},
			require.Error,
			nil,
		},
		{
			"LineStartAndEnd",
			func(cfg *Config) {
				cfg.SplitConfig.LineStartPattern = ".*"
				cfg.SplitConfig.LineEndPattern = ".*"
			},
			require.Error,
			nil,
		},
		{
			"NoLineStartOrEnd",
			func(cfg *Config) {},
			require.NoError,
			func(t *testing.T, f *Manager) {},
		},
		{
			"InvalidLineStartRegex",
			func(cfg *Config) {
				cfg.SplitConfig.LineStartPattern = "("
			},
			require.Error,
			nil,
		},
		{
			"InvalidLineEndRegex",
			func(cfg *Config) {
				cfg.SplitConfig.LineEndPattern = "("
			},
			require.Error,
			nil,
		},
		{
			"InvalidStartAtDelete",
			func(cfg *Config) {
				cfg.StartAt = "end"
				cfg.DeleteAfterRead = true
			},
			require.Error,
			nil,
		},
		{
			"InvalidMaxBatches",
			func(cfg *Config) {
				cfg.MaxBatches = -1
			},
			require.Error,
			nil,
		},
		{
			"ValidMaxBatches",
			func(cfg *Config) {
				cfg.MaxBatches = 6
			},
			require.NoError,
			func(t *testing.T, m *Manager) {
				require.Equal(t, 6, m.maxBatches)
			},
		},
		{
			"HeaderConfigNoFlag",
			func(cfg *Config) {
				cfg.Header = &HeaderConfig{}
			},
			require.Error,
			nil,
		},
		{
			"BadOrderingCriteriaRegex",
			func(cfg *Config) {
				cfg.OrderingCriteria = matcher.OrderingCriteria{
					SortBy: []matcher.Sort{
						{
							SortType: "numeric",
							RegexKey: "value",
						},
					},
				}
			},
			require.Error,
			nil,
		},
		{
			"OrderingCriteriaTimestampMissingLayout",
			func(cfg *Config) {
				cfg.OrderingCriteria = matcher.OrderingCriteria{
					Regex: ".*",
					SortBy: []matcher.Sort{
						{
							SortType: "timestamp",
							RegexKey: "value",
						},
					},
				}
			},
			require.Error,
			nil,
		},
		{
			"GoodOrderingCriteriaTimestamp",
			func(cfg *Config) {
				cfg.OrderingCriteria = matcher.OrderingCriteria{
					Regex: ".*",
					SortBy: []matcher.Sort{
						{
							SortType: "timestamp",
							RegexKey: "value",
							Layout:   "%Y%m%d%H",
						},
					},
				}
			},
			require.NoError,
			func(t *testing.T, f *Manager) {},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc := tc
			t.Parallel()
			cfg := basicConfig()
			tc.modifyBaseConfig(cfg)

			input, err := cfg.Build(testutil.Logger(t), emittest.Nop)
			tc.errorRequirement(t, err)
			if err != nil {
				return
			}

			tc.validate(t, input)
		})
	}
}

func TestBuildWithSplitFunc(t *testing.T) {
	t.Parallel()

	basicConfig := func() *Config {
		cfg := NewConfig()
		cfg.Include = []string{"/var/log/testpath.*"}
		cfg.Exclude = []string{"/var/log/testpath.ex*"}
		cfg.PollInterval = 10 * time.Millisecond
		return cfg
	}

	cases := []struct {
		name             string
		modifyBaseConfig func(*Config)
		errorRequirement require.ErrorAssertionFunc
		validate         func(*testing.T, *Manager)
	}{
		{
			"Basic",
			func(cfg *Config) {},
			require.NoError,
			func(t *testing.T, m *Manager) {
				require.Equal(t, m.pollInterval, 10*time.Millisecond)
			},
		},
		{
			"BadIncludeGlob",
			func(cfg *Config) {
				cfg.Include = []string{"["}
			},
			require.Error,
			nil,
		},
		{
			"BadExcludeGlob",
			func(cfg *Config) {
				cfg.Include = []string{"["}
			},
			require.Error,
			nil,
		},
		{
			"InvalidEncoding",
			func(cfg *Config) {
				cfg.Encoding = "UTF-3233"
			},
			require.Error,
			nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc := tc
			t.Parallel()
			cfg := basicConfig()
			tc.modifyBaseConfig(cfg)

			splitNone := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
				if !atEOF {
					return 0, nil, nil
				}
				if len(data) == 0 {
					return 0, nil, nil
				}
				return len(data), data, nil
			}

			input, err := cfg.BuildWithSplitFunc(testutil.Logger(t), emittest.Nop, splitNone)
			tc.errorRequirement(t, err)
			if err != nil {
				return
			}

			tc.validate(t, input)
		})
	}
}

func TestBuildWithHeader(t *testing.T) {
	require.NoError(t, featuregate.GlobalRegistry().Set(AllowHeaderMetadataParsing.ID(), true))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(AllowHeaderMetadataParsing.ID(), false))
	})

	basicConfig := func() *Config {
		cfg := NewConfig()
		cfg.Include = []string{"/var/log/testpath.*"}
		cfg.Exclude = []string{"/var/log/testpath.ex*"}
		cfg.PollInterval = 10 * time.Millisecond
		return cfg
	}

	cases := []struct {
		name             string
		modifyBaseConfig func(*Config)
		errorRequirement require.ErrorAssertionFunc
		validate         func(*testing.T, *Manager)
	}{
		{
			"InvalidHeaderConfig",
			func(cfg *Config) {
				cfg.Header = &HeaderConfig{}
				cfg.StartAt = "beginning"
			},
			require.Error,
			nil,
		},
		{
			"HeaderConfigWithStartAtEnd",
			func(cfg *Config) {
				regexCfg := regex.NewConfig()
				regexCfg.Regex = "^(?P<field>.*)"
				cfg.Header = &HeaderConfig{
					Pattern: "^#",
					MetadataOperators: []operator.Config{
						{
							Builder: regexCfg,
						},
					},
				}
				cfg.StartAt = "end"
			},
			require.Error,
			nil,
		},
		{
			"ValidHeaderConfig",
			func(cfg *Config) {
				regexCfg := regex.NewConfig()
				regexCfg.Regex = "^(?P<field>.*)"
				cfg.Header = &HeaderConfig{
					Pattern: "^#",
					MetadataOperators: []operator.Config{
						{
							Builder: regexCfg,
						},
					},
				}
				cfg.StartAt = "beginning"
			},
			require.NoError,
			func(t *testing.T, m *Manager) {
				require.NotNil(t, m.readerFactory.HeaderConfig.SplitFunc)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc := tc
			t.Parallel()
			cfg := basicConfig()
			tc.modifyBaseConfig(cfg)

			input, err := cfg.Build(testutil.Logger(t), emittest.Nop)
			tc.errorRequirement(t, err)
			if err != nil {
				return
			}
			tc.validate(t, input)
		})
	}
}

// includeDir is a builder-like helper for quickly setting up a test config
func (c *Config) includeDir(dir string) *Config {
	c.Include = append(c.Include, fmt.Sprintf("%s/*", dir))
	return c
}

// withHeader is a builder-like helper for quickly setting up a test config header
func (c *Config) withHeader(headerMatchPattern, extractRegex string) *Config {
	regexOpConfig := regex.NewConfig()
	regexOpConfig.Regex = extractRegex

	c.Header = &HeaderConfig{
		Pattern: headerMatchPattern,
		MetadataOperators: []operator.Config{
			{
				Builder: regexOpConfig,
			},
		},
	}

	return c
}

const mockOperatorType = "mock"

func init() {
	operator.Register(mockOperatorType, func() operator.Builder { return newMockOperatorConfig(NewConfig()) })
}

type mockOperatorConfig struct {
	helper.BasicConfig `mapstructure:",squash"`
	*Config            `mapstructure:",squash"`
}

func newMockOperatorConfig(cfg *Config) *mockOperatorConfig {
	return &mockOperatorConfig{
		BasicConfig: helper.NewBasicConfig(mockOperatorType, mockOperatorType),
		Config:      cfg,
	}
}

// This function is impelmented for compatibility with operatortest
// but is not meant to be used directly
func (h *mockOperatorConfig) Build(*zap.SugaredLogger) (operator.Operator, error) {
	panic("not impelemented")
}
