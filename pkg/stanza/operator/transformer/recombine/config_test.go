// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package recombine

import (
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"
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
				Name:               "custom_id",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.OperatorID = "merge-split-lines"
					return cfg
				}(),
			},
			{
				Name:               "combine_with_custom_string",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.CombineWith = "ABC"
					return cfg
				}(),
			},
			{
				Name:               "combine_with_empty_string",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.CombineWith = ""
					return cfg
				}(),
			},
			{
				Name:               "combine_with_tab",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.CombineWith = "\t"
					return cfg
				}(),
			},
			{
				Name:               "combine_with_backslash_t",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.CombineWith = "\\t"
					return cfg
				}(),
			},
			{
				Name:               "combine_with_multiline_string",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.CombineWith = "line1\nLINE2"
					return cfg
				}(),
			},
			{
				Name:               "custom_max_log_size",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.MaxLogSize = helper.ByteSize(256000)
					return cfg
				}(),
			},
			{
				Name:               "custom_max_unmatched_batch_size",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.MaxUnmatchedBatchSize = 50
					return cfg
				}(),
			},
			{
				Name:               "on_error_drop",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.CombineWith = "\\t"
					cfg.OnError = "drop"
					return cfg
				}(),
			},
		},
	}.Run(t)
}
