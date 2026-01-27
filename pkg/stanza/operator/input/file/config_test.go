// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package file

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestUnmarshal(t *testing.T) {
	operatortest.ConfigUnmarshalTests{
		DefaultConfig: NewConfig(),
		TestsFile:     filepath.Join(".", "testdata", "config.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name:               "default",
				ExpectUnmarshalErr: false,
				Expect:             NewConfig(),
			},
			{
				Name:               "id_custom",
				ExpectUnmarshalErr: false,
				Expect:             NewConfigWithID("test_id"),
			},
			{
				Name:               "include_one",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "one.log")
					return cfg
				}(),
			},
			{
				Name:               "include_multi",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "one.log", "two.log", "three.log")
					return cfg
				}(),
			},
			{
				Name:               "include_glob",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "*.log")
					return cfg
				}(),
			},
			{
				Name:               "include_glob_double_asterisk",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "**.log")
					return cfg
				}(),
			},
			{
				Name:               "include_glob_double_asterisk_nested",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "directory/**/*.log")
					return cfg
				}(),
			},
			{
				Name:               "include_glob_double_asterisk_prefix",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "**/directory/**/*.log")
					return cfg
				}(),
			},
			{
				Name:               "include_inline",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "a.log", "b.log")
					return cfg
				}(),
			},
			{
				Name:               "include_invalid",
				ExpectUnmarshalErr: true,
				Expect:             nil,
			},
			{
				Name:               "exclude_one",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "*.log")
					cfg.Exclude = append(cfg.Exclude, "one.log")
					return cfg
				}(),
			},
			{
				Name:               "exclude_multi",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "*.log")
					cfg.Exclude = append(cfg.Exclude, "one.log", "two.log", "three.log")
					return cfg
				}(),
			},
			{
				Name:               "exclude_glob",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "*.log")
					cfg.Exclude = append(cfg.Exclude, "not*.log")
					return cfg
				}(),
			},
			{
				Name:               "exclude_glob_double_asterisk",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "*.log")
					cfg.Exclude = append(cfg.Exclude, "not**.log")
					return cfg
				}(),
			},
			{
				Name:               "exclude_glob_double_asterisk_nested",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "*.log")
					cfg.Exclude = append(cfg.Exclude, "directory/**/not*.log")
					return cfg
				}(),
			},
			{
				Name:               "exclude_glob_double_asterisk_prefix",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "*.log")
					cfg.Exclude = append(cfg.Exclude, "**/directory/**/not*.log")
					return cfg
				}(),
			},
			{
				Name:               "exclude_inline",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "*.log")
					cfg.Exclude = append(cfg.Exclude, "a.log", "b.log")
					return cfg
				}(),
			},
			{
				Name:               "exclude_invalid",
				ExpectUnmarshalErr: true,
				Expect:             nil,
			},
			{
				Name:               "poll_interval_no_units",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.PollInterval = time.Nanosecond
					return cfg
				}(),
			},
			{
				Name:               "poll_interval_1s",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.PollInterval = time.Second
					return cfg
				}(),
			},
			{
				Name:               "poll_interval_1ms",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.PollInterval = time.Millisecond
					return cfg
				}(),
			},
			{
				Name:               "poll_interval_1000ms",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.PollInterval = time.Second
					return cfg
				}(),
			},
			{
				Name:               "fingerprint_size_no_units",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.FingerprintSize = helper.ByteSize(1000)
					return cfg
				}(),
			},
			{
				Name:               "fingerprint_size_1kb_lower",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.FingerprintSize = helper.ByteSize(1000)
					return cfg
				}(),
			},
			{
				Name:               "fingerprint_size_1KB",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.FingerprintSize = helper.ByteSize(1000)
					return cfg
				}(),
			},
			{
				Name:               "fingerprint_size_1kib_lower",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.FingerprintSize = helper.ByteSize(1024)
					return cfg
				}(),
			},
			{
				Name:               "fingerprint_size_1KiB",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.FingerprintSize = helper.ByteSize(1024)
					return cfg
				}(),
			},
			{
				Name:               "fingerprint_size_float",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.FingerprintSize = helper.ByteSize(1100)
					return cfg
				}(),
			},
			{
				Name:               "include_file_name_lower",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "one.log")
					cfg.IncludeFileName = true
					return cfg
				}(),
			},
			{
				Name:               "include_file_name_upper",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "one.log")
					cfg.IncludeFileName = true
					return cfg
				}(),
			},
			{
				Name:               "include_file_path_lower",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "one.log")
					cfg.IncludeFilePath = true
					return cfg
				}(),
			},
			{
				Name:               "include_file_path_upper",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "one.log")
					cfg.IncludeFilePath = true
					return cfg
				}(),
			},
			{
				Name:               "include_file_path_nonbool",
				ExpectUnmarshalErr: true,
				Expect:             nil,
			},
			{
				Name:               "multiline_line_start_string",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.SplitConfig.LineStartPattern = "Start"
					return cfg
				}(),
			},
			{
				Name:               "multiline_line_start_special",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.SplitConfig.LineStartPattern = "%"
					return cfg
				}(),
			},
			{
				Name:               "multiline_line_end_string",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.SplitConfig.LineEndPattern = "Start"
					return cfg
				}(),
			},
			{
				Name:               "multiline_line_end_special",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.SplitConfig.LineEndPattern = "%"
					return cfg
				}(),
			},
			{
				Name:               "start_at_string",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.StartAt = "beginning"
					return cfg
				}(),
			},
			{
				Name:               "max_concurrent_large",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.MaxConcurrentFiles = 9223372036854775807
					return cfg
				}(),
			},
			{
				Name:               "max_log_size_mib_lower",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.MaxLogSize = helper.ByteSize(1048576)
					return cfg
				}(),
			},
			{
				Name:               "max_log_size_mib_upper",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.MaxLogSize = helper.ByteSize(1048576)
					return cfg
				}(),
			},
			{
				Name:               "max_log_size_mb_upper",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.MaxLogSize = helper.ByteSize(1048576)
					return cfg
				}(),
			},
			{
				Name:               "max_log_size_mb_lower",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.MaxLogSize = helper.ByteSize(1048576)
					return cfg
				}(),
			},
			{
				Name:               "encoding_lower",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Encoding = "utf-16le"
					return cfg
				}(),
			},
			{
				Name:               "encoding_upper",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Encoding = "UTF-16lE"
					return cfg
				}(),
			},
		},
	}.Run(t)
}

func TestBuild(t *testing.T) {
	t.Parallel()
	fakeOutput := testutil.NewMockOperator("fake")

	basicConfig := func() *Config {
		cfg := NewConfigWithID("testfile")
		cfg.OutputIDs = []string{"fake"}
		cfg.Include = []string{"/var/log/testpath.*"}
		cfg.Exclude = []string{"/var/log/testpath.ex*"}
		cfg.PollInterval = 10 * time.Millisecond
		return cfg
	}

	cases := []struct {
		name             string
		modifyBaseConfig func(*Config)
		errorRequirement require.ErrorAssertionFunc
		validate         func(*testing.T, *Input)
	}{
		{
			"Default",
			func(_ *Config) {},
			require.NoError,
			func(t *testing.T, f *Input) {
				require.Equal(t, f.OutputOperators[0], fakeOutput)
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
			func(_ *testing.T, _ *Input) {},
		},
		{
			"MultilineConfiguredEndPattern",
			func(cfg *Config) {
				cfg.SplitConfig.LineEndPattern = "END.*"
			},
			require.NoError,
			func(_ *testing.T, _ *Input) {},
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
			func(_ *Config) {},
			require.NoError,
			func(_ *testing.T, _ *Input) {},
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
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := basicConfig()
			tc.modifyBaseConfig(cfg)

			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
			tc.errorRequirement(t, err)
			if err != nil {
				return
			}

			err = op.SetOutputs([]operator.Operator{fakeOutput})
			require.NoError(t, err)

			fileInput := op.(*Input)
			tc.validate(t, fileInput)
		})
	}
}
