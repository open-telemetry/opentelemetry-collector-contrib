// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package file

import (
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper/operatortest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestUnmarshal(t *testing.T) {
	cases := []operatortest.ConfigUnmarshalTest{
		{
			Name:      "default",
			ExpectErr: false,
			Expect:    defaultCfg(),
		},
		{

			Name:      "extra_field",
			ExpectErr: false,
			Expect:    defaultCfg(),
		},
		{
			Name:      "id_custom",
			ExpectErr: false,
			Expect:    NewConfig("test_id"),
		},
		{
			Name:      "include_one",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.Include = append(cfg.Include, "one.log")
				return cfg
			}(),
		},
		{
			Name:      "include_multi",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.Include = append(cfg.Include, "one.log", "two.log", "three.log")
				return cfg
			}(),
		},
		{
			Name:      "include_glob",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.Include = append(cfg.Include, "*.log")
				return cfg
			}(),
		},
		{
			Name:      "include_glob_double_asterisk",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.Include = append(cfg.Include, "**.log")
				return cfg
			}(),
		},
		{
			Name:      "include_glob_double_asterisk_nested",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.Include = append(cfg.Include, "directory/**/*.log")
				return cfg
			}(),
		},
		{
			Name:      "include_glob_double_asterisk_prefix",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.Include = append(cfg.Include, "**/directory/**/*.log")
				return cfg
			}(),
		},
		{
			Name:      "include_inline",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
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
				cfg := defaultCfg()
				cfg.Include = append(cfg.Include, "*.log")
				cfg.Exclude = append(cfg.Exclude, "one.log")
				return cfg
			}(),
		},
		{
			Name:      "exclude_multi",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.Include = append(cfg.Include, "*.log")
				cfg.Exclude = append(cfg.Exclude, "one.log", "two.log", "three.log")
				return cfg
			}(),
		},
		{
			Name:      "exclude_glob",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.Include = append(cfg.Include, "*.log")
				cfg.Exclude = append(cfg.Exclude, "not*.log")
				return cfg
			}(),
		},
		{
			Name:      "exclude_glob_double_asterisk",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.Include = append(cfg.Include, "*.log")
				cfg.Exclude = append(cfg.Exclude, "not**.log")
				return cfg
			}(),
		},
		{
			Name:      "exclude_glob_double_asterisk_nested",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.Include = append(cfg.Include, "*.log")
				cfg.Exclude = append(cfg.Exclude, "directory/**/not*.log")
				return cfg
			}(),
		},
		{
			Name:      "exclude_glob_double_asterisk_prefix",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.Include = append(cfg.Include, "*.log")
				cfg.Exclude = append(cfg.Exclude, "**/directory/**/not*.log")
				return cfg
			}(),
		},
		{
			Name:      "exclude_inline",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
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
				cfg := defaultCfg()
				cfg.PollInterval = helper.NewDuration(time.Second)
				return cfg
			}(),
		},
		{
			Name:      "poll_interval_1s",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.PollInterval = helper.NewDuration(time.Second)
				return cfg
			}(),
		},
		{
			Name:      "poll_interval_1ms",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.PollInterval = helper.NewDuration(time.Millisecond)
				return cfg
			}(),
		},
		{
			Name:      "poll_interval_1000ms",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.PollInterval = helper.NewDuration(time.Second)
				return cfg
			}(),
		},
		{
			Name:      "fingerprint_size_no_units",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.FingerprintSize = helper.ByteSize(1000)
				return cfg
			}(),
		},
		{
			Name:      "fingerprint_size_1kb_lower",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.FingerprintSize = helper.ByteSize(1000)
				return cfg
			}(),
		},
		{
			Name:      "fingerprint_size_1KB",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.FingerprintSize = helper.ByteSize(1000)
				return cfg
			}(),
		},
		{
			Name:      "fingerprint_size_1kib_lower",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.FingerprintSize = helper.ByteSize(1024)
				return cfg
			}(),
		},
		{
			Name:      "fingerprint_size_1KiB",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.FingerprintSize = helper.ByteSize(1024)
				return cfg
			}(),
		},
		{
			Name:      "fingerprint_size_float",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.FingerprintSize = helper.ByteSize(1100)
				return cfg
			}(),
		},
		{
			Name:      "include_file_name_lower",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.Include = append(cfg.Include, "one.log")
				cfg.IncludeFileName = true
				return cfg
			}(),
		},
		{
			Name:      "include_file_name_upper",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.Include = append(cfg.Include, "one.log")
				cfg.IncludeFileName = true
				return cfg
			}(),
		},
		{
			Name:      "include_file_name_on",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.Include = append(cfg.Include, "one.log")
				cfg.IncludeFileName = true
				return cfg
			}(),
		},
		{
			Name:      "include_file_name_yes",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.Include = append(cfg.Include, "one.log")
				cfg.IncludeFileName = true
				return cfg
			}(),
		},
		{
			Name:      "include_file_path_lower",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.Include = append(cfg.Include, "one.log")
				cfg.IncludeFilePath = true
				return cfg
			}(),
		},
		{
			Name:      "include_file_path_upper",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.Include = append(cfg.Include, "one.log")
				cfg.IncludeFilePath = true
				return cfg
			}(),
		},
		{
			Name:      "include_file_path_on",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.Include = append(cfg.Include, "one.log")
				cfg.IncludeFilePath = true
				return cfg
			}(),
		},
		{
			Name:      "include_file_path_yes",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.Include = append(cfg.Include, "one.log")
				cfg.IncludeFilePath = true
				return cfg
			}(),
		},
		{
			Name:      "include_file_path_off",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.Include = append(cfg.Include, "one.log")
				cfg.IncludeFilePath = false
				return cfg
			}(),
		},
		{
			Name:      "include_file_path_no",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.Include = append(cfg.Include, "one.log")
				cfg.IncludeFilePath = false
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
				cfg := defaultCfg()
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
				cfg := defaultCfg()
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
				cfg := defaultCfg()
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
				cfg := defaultCfg()
				newSplit := helper.NewSplitterConfig()
				newSplit.Multiline.LineEndPattern = "%"
				cfg.Splitter = newSplit
				return cfg
			}(),
		},
		{
			Name:      "multiline_random",
			ExpectErr: true,
			Expect:    nil,
		},
		{
			Name:      "start_at_string",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.StartAt = "beginning"
				return cfg
			}(),
		},
		{
			Name:      "max_concurrent_large",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.MaxConcurrentFiles = 9223372036854775807
				return cfg
			}(),
		},
		{
			Name:      "max_log_size_mib_lower",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.MaxLogSize = helper.ByteSize(1048576)
				return cfg
			}(),
		},
		{
			Name:      "max_log_size_mib_upper",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.MaxLogSize = helper.ByteSize(1048576)
				return cfg
			}(),
		},
		{
			Name:      "max_log_size_mb_upper",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.MaxLogSize = helper.ByteSize(1048576)
				return cfg
			}(),
		},
		{
			Name:      "max_log_size_mb_lower",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.MaxLogSize = helper.ByteSize(1048576)
				return cfg
			}(),
		},
		{
			Name:      "encoding_lower",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.Splitter.EncodingConfig = helper.EncodingConfig{Encoding: "utf-16le"}
				return cfg
			}(),
		},
		{
			Name:      "encoding_upper",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.Splitter.EncodingConfig = helper.EncodingConfig{Encoding: "UTF-16lE"}
				return cfg
			}(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			tc.Run(t, defaultCfg())
		})
	}
}

func TestBuild(t *testing.T) {
	t.Parallel()
	fakeOutput := testutil.NewMockOperator("fake")

	basicConfig := func() *Config {
		cfg := NewConfig("testfile")
		cfg.OutputIDs = []string{"fake"}
		cfg.Include = []string{"/var/log/testpath.*"}
		cfg.Exclude = []string{"/var/log/testpath.ex*"}
		cfg.PollInterval = helper.Duration{Duration: 10 * time.Millisecond}
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

func defaultCfg() *Config {
	return NewConfig("file_input")
}

func NewTestConfig() *Config {
	cfg := NewConfig("config_test")
	cfg.Include = []string{"i1", "i2"}
	cfg.Exclude = []string{"e1", "e2"}
	cfg.Splitter = helper.NewSplitterConfig()
	cfg.Splitter.Multiline = helper.MultilineConfig{
		LineStartPattern: "start",
		LineEndPattern:   "end",
	}
	cfg.FingerprintSize = 1024
	cfg.Splitter.EncodingConfig = helper.EncodingConfig{Encoding: "utf16"}
	return cfg
}

func TestMapStructureDecodeConfigWithHook(t *testing.T) {
	expect := NewTestConfig()
	input := map[string]interface{}{
		// Config
		"id":            "config_test",
		"type":          "file_input",
		"attributes":    map[string]interface{}{},
		"resource":      map[string]interface{}{},
		"include":       expect.Include,
		"exclude":       expect.Exclude,
		"poll_interval": 0.2,
		"multiline": map[string]interface{}{
			"line_start_pattern": expect.Splitter.Multiline.LineStartPattern,
			"line_end_pattern":   expect.Splitter.Multiline.LineEndPattern,
		},
		"force_flush_period":   0.5,
		"include_file_name":    true,
		"include_file_path":    false,
		"start_at":             "end",
		"fingerprint_size":     "1024",
		"max_log_size":         "1mib",
		"max_concurrent_files": 1024,
		"encoding":             "utf16",
	}

	var actual Config
	dc := &mapstructure.DecoderConfig{Result: &actual, DecodeHook: helper.JSONUnmarshalerHook()}
	ms, err := mapstructure.NewDecoder(dc)
	require.NoError(t, err)
	err = ms.Decode(input)
	require.NoError(t, err)
	require.Equal(t, expect, &actual)
}

func TestMapStructureDecodeConfig(t *testing.T) {
	expect := NewTestConfig()
	input := map[string]interface{}{
		// Config
		"id":         "config_test",
		"type":       "file_input",
		"attributes": map[string]interface{}{},
		"resource":   map[string]interface{}{},
		"include":    expect.Include,
		"exclude":    expect.Exclude,
		"poll_interval": map[string]interface{}{
			"Duration": 200 * 1000 * 1000,
		},
		"multiline": map[string]interface{}{
			"line_start_pattern": expect.Splitter.Multiline.LineStartPattern,
			"line_end_pattern":   expect.Splitter.Multiline.LineEndPattern,
		},
		"include_file_name":    true,
		"include_file_path":    false,
		"start_at":             "end",
		"fingerprint_size":     1024,
		"max_log_size":         1024 * 1024,
		"max_concurrent_files": 1024,
		"encoding":             "utf16",
		"force_flush_period": map[string]interface{}{
			"Duration": 500 * 1000 * 1000,
		},
	}

	var actual Config
	err := mapstructure.Decode(input, &actual)
	require.NoError(t, err)
	require.Equal(t, expect, &actual)
}
