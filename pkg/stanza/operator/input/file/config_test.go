// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package file

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

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
				Name:      "default",
				ExpectErr: false,
				Expect:    NewConfig(),
			},
			{
				Name:      "id_custom",
				ExpectErr: false,
				Expect:    NewConfigWithID("test_id"),
			},
			{
				Name:      "include_one",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "one.log")
					return cfg
				}(),
			},
			{
				Name:      "include_multi",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "one.log", "two.log", "three.log")
					return cfg
				}(),
			},
			{
				Name:      "include_glob",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "*.log")
					return cfg
				}(),
			},
			{
				Name:      "include_glob_double_asterisk",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "**.log")
					return cfg
				}(),
			},
			{
				Name:      "include_glob_double_asterisk_nested",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "directory/**/*.log")
					return cfg
				}(),
			},
			{
				Name:      "include_glob_double_asterisk_prefix",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "**/directory/**/*.log")
					return cfg
				}(),
			},
			{
				Name:      "include_inline",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "a.log", "b.log")
					return cfg
				}(),
			},
			{
				Name:      "include_invalid",
				ExpectErr: true,
				Expect:    nil,
			},
			{
				Name:      "exclude_one",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "*.log")
					cfg.Exclude = append(cfg.Exclude, "one.log")
					return cfg
				}(),
			},
			{
				Name:      "exclude_multi",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "*.log")
					cfg.Exclude = append(cfg.Exclude, "one.log", "two.log", "three.log")
					return cfg
				}(),
			},
			{
				Name:      "exclude_glob",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "*.log")
					cfg.Exclude = append(cfg.Exclude, "not*.log")
					return cfg
				}(),
			},
			{
				Name:      "exclude_glob_double_asterisk",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "*.log")
					cfg.Exclude = append(cfg.Exclude, "not**.log")
					return cfg
				}(),
			},
			{
				Name:      "exclude_glob_double_asterisk_nested",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "*.log")
					cfg.Exclude = append(cfg.Exclude, "directory/**/not*.log")
					return cfg
				}(),
			},
			{
				Name:      "exclude_glob_double_asterisk_prefix",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "*.log")
					cfg.Exclude = append(cfg.Exclude, "**/directory/**/not*.log")
					return cfg
				}(),
			},
			{
				Name:      "exclude_inline",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "*.log")
					cfg.Exclude = append(cfg.Exclude, "a.log", "b.log")
					return cfg
				}(),
			},
			{
				Name:      "exclude_invalid",
				ExpectErr: true,
				Expect:    nil,
			},
			{
				Name:      "poll_interval_no_units",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.PollInterval = time.Nanosecond
					return cfg
				}(),
			},
			{
				Name:      "poll_interval_1s",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.PollInterval = time.Second
					return cfg
				}(),
			},
			{
				Name:      "poll_interval_1ms",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.PollInterval = time.Millisecond
					return cfg
				}(),
			},
			{
				Name:      "poll_interval_1000ms",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.PollInterval = time.Second
					return cfg
				}(),
			},
			{
				Name:      "fingerprint_size_no_units",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.FingerprintSize = helper.ByteSize(1000)
					return cfg
				}(),
			},
			{
				Name:      "fingerprint_size_1kb_lower",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.FingerprintSize = helper.ByteSize(1000)
					return cfg
				}(),
			},
			{
				Name:      "fingerprint_size_1KB",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.FingerprintSize = helper.ByteSize(1000)
					return cfg
				}(),
			},
			{
				Name:      "fingerprint_size_1kib_lower",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.FingerprintSize = helper.ByteSize(1024)
					return cfg
				}(),
			},
			{
				Name:      "fingerprint_size_1KiB",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.FingerprintSize = helper.ByteSize(1024)
					return cfg
				}(),
			},
			{
				Name:      "fingerprint_size_float",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.FingerprintSize = helper.ByteSize(1100)
					return cfg
				}(),
			},
			{
				Name:      "include_file_name_lower",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "one.log")
					cfg.IncludeFileName = true
					return cfg
				}(),
			},
			{
				Name:      "include_file_name_upper",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "one.log")
					cfg.IncludeFileName = true
					return cfg
				}(),
			},
			{
				Name:      "include_file_path_lower",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "one.log")
					cfg.IncludeFilePath = true
					return cfg
				}(),
			},
			{
				Name:      "include_file_path_upper",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Include = append(cfg.Include, "one.log")
					cfg.IncludeFilePath = true
					return cfg
				}(),
			},
			{
				Name:      "include_file_path_nonbool",
				ExpectErr: true,
				Expect:    nil,
			},
			{
				Name:      "multiline_line_start_string",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					newSplit := helper.NewSplitterConfig()
					newSplit.Multiline.LineStartPattern = "Start"
					cfg.Splitter = newSplit
					return cfg
				}(),
			},
			{
				Name:      "multiline_line_start_special",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					newSplit := helper.NewSplitterConfig()
					newSplit.Multiline.LineStartPattern = "%"
					cfg.Splitter = newSplit
					return cfg
				}(),
			},
			{
				Name:      "multiline_line_end_string",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					newSplit := helper.NewSplitterConfig()
					newSplit.Multiline.LineEndPattern = "Start"
					cfg.Splitter = newSplit
					return cfg
				}(),
			},
			{
				Name:      "multiline_line_end_special",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					newSplit := helper.NewSplitterConfig()
					newSplit.Multiline.LineEndPattern = "%"
					cfg.Splitter = newSplit
					return cfg
				}(),
			},
			{
				Name:      "start_at_string",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.StartAt = "beginning"
					return cfg
				}(),
			},
			{
				Name:      "max_concurrent_large",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.MaxConcurrentFiles = 9223372036854775807
					return cfg
				}(),
			},
			{
				Name:      "max_log_size_mib_lower",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.MaxLogSize = helper.ByteSize(1048576)
					return cfg
				}(),
			},
			{
				Name:      "max_log_size_mib_upper",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.MaxLogSize = helper.ByteSize(1048576)
					return cfg
				}(),
			},
			{
				Name:      "max_log_size_mb_upper",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.MaxLogSize = helper.ByteSize(1048576)
					return cfg
				}(),
			},
			{
				Name:      "max_log_size_mb_lower",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.MaxLogSize = helper.ByteSize(1048576)
					return cfg
				}(),
			},
			{
				Name:      "encoding_lower",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Splitter.EncodingConfig = helper.EncodingConfig{Encoding: "utf-16le"}
					return cfg
				}(),
			},
			{
				Name:      "encoding_upper",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Splitter.EncodingConfig = helper.EncodingConfig{Encoding: "UTF-16lE"}
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
			func(f *Config) {},
			require.NoError,
			func(t *testing.T, f *Input) {
				require.Equal(t, f.OutputOperators[0], fakeOutput)
				expectOptions := []preEmitOption{setFileName}
				requireSamePreEmitOptions(t, expectOptions, f.preEmitOptions)
			},
		},
		{
			"IncludeFilePath",
			func(f *Config) {
				f.IncludeFilePath = true
			},
			require.NoError,
			func(t *testing.T, f *Input) {
				require.Equal(t, f.OutputOperators[0], fakeOutput)
				expectOptions := []preEmitOption{setFileName, setFilePath}
				requireSamePreEmitOptions(t, expectOptions, f.preEmitOptions)
			},
		},
		{
			"IncludeFileNameResolved",
			func(f *Config) {
				f.IncludeFileNameResolved = true
			},
			require.NoError,
			func(t *testing.T, f *Input) {
				require.Equal(t, f.OutputOperators[0], fakeOutput)
				expectOptions := []preEmitOption{setFileName, setFileNameResolved}
				requireSamePreEmitOptions(t, expectOptions, f.preEmitOptions)
			},
		},
		{
			"IncludeFilePathResolved",
			func(f *Config) {
				f.IncludeFilePathResolved = true
			},
			require.NoError,
			func(t *testing.T, f *Input) {
				require.Equal(t, f.OutputOperators[0], fakeOutput)
				expectOptions := []preEmitOption{setFileName, setFilePathResolved}
				requireSamePreEmitOptions(t, expectOptions, f.preEmitOptions)
			},
		},
		{
			"IncludeResolvedAttrs",
			func(f *Config) {
				f.IncludeFileName = false
				f.IncludeFilePath = false
				f.IncludeFilePathResolved = true
				f.IncludeFileNameResolved = true
			},
			require.NoError,
			func(t *testing.T, f *Input) {
				require.Equal(t, f.OutputOperators[0], fakeOutput)
				expectOptions := []preEmitOption{setFileNameResolved, setFilePathResolved}
				requireSamePreEmitOptions(t, expectOptions, f.preEmitOptions)
			},
		},
		{
			"IncludeAllFileAttrs",
			func(f *Config) {
				f.IncludeFileName = true
				f.IncludeFilePath = true
				f.IncludeFilePathResolved = true
				f.IncludeFileNameResolved = true
			},
			require.NoError,
			func(t *testing.T, f *Input) {
				require.Equal(t, f.OutputOperators[0], fakeOutput)
				expectOptions := []preEmitOption{setFileName, setFilePath, setFileNameResolved, setFilePathResolved}
				requireSamePreEmitOptions(t, expectOptions, f.preEmitOptions)
			},
		},
		{
			"BadIncludeGlob",
			func(f *Config) {
				f.Include = []string{"["}
			},
			require.Error,
			nil,
		},
		{
			"BadExcludeGlob",
			func(f *Config) {
				f.Include = []string{"["}
			},
			require.Error,
			nil,
		},
		{
			"MultilineConfiguredStartAndEndPatterns",
			func(f *Config) {
				f.Splitter = helper.NewSplitterConfig()
				f.Splitter.Multiline = helper.MultilineConfig{
					LineEndPattern:   "Exists",
					LineStartPattern: "Exists",
				}
			},
			require.Error,
			nil,
		},
		{
			"MultilineConfiguredStartPattern",
			func(f *Config) {
				f.Splitter = helper.NewSplitterConfig()
				f.Splitter.Multiline = helper.MultilineConfig{
					LineStartPattern: "START.*",
				}
			},
			require.NoError,
			func(t *testing.T, f *Input) {},
		},
		{
			"MultilineConfiguredEndPattern",
			func(f *Config) {
				f.Splitter = helper.NewSplitterConfig()
				f.Splitter.Multiline = helper.MultilineConfig{
					LineEndPattern: "END.*",
				}
			},
			require.NoError,
			func(t *testing.T, f *Input) {},
		},
		{
			"InvalidEncoding",
			func(f *Config) {
				f.Splitter.EncodingConfig = helper.EncodingConfig{Encoding: "UTF-3233"}
			},
			require.Error,
			nil,
		},
		{
			"LineStartAndEnd",
			func(f *Config) {
				f.Splitter = helper.NewSplitterConfig()
				f.Splitter.Multiline = helper.MultilineConfig{
					LineStartPattern: ".*",
					LineEndPattern:   ".*",
				}
			},
			require.Error,
			nil,
		},
		{
			"NoLineStartOrEnd",
			func(f *Config) {
				f.Splitter = helper.NewSplitterConfig()
				f.Splitter.Multiline = helper.MultilineConfig{}
			},
			require.NoError,
			func(t *testing.T, f *Input) {},
		},
		{
			"InvalidLineStartRegex",
			func(f *Config) {
				f.Splitter = helper.NewSplitterConfig()
				f.Splitter.Multiline = helper.MultilineConfig{
					LineStartPattern: "(",
				}
			},
			require.Error,
			nil,
		},
		{
			"InvalidLineEndRegex",
			func(f *Config) {
				f.Splitter = helper.NewSplitterConfig()
				f.Splitter.Multiline = helper.MultilineConfig{
					LineEndPattern: "(",
				}
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

			op, err := cfg.Build(testutil.Logger(t))
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

func requireSamePreEmitOptions(t *testing.T, expect, actual []preEmitOption) {
	// Comparing functions is not directly possible
	require.Equal(t, len(expect), len(actual))
	for i := range expect {
		// Credit https://github.com/stretchr/testify/issues/182#issuecomment-495359313
		expectFuncName := runtime.FuncForPC(reflect.ValueOf(expect[i]).Pointer()).Name()
		actualFuncName := runtime.FuncForPC(reflect.ValueOf(actual[i]).Pointer()).Name()
		require.Equal(t, expectFuncName, actualFuncName)
	}
}
