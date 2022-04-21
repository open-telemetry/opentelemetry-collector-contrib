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

package recombine

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper/operatortest"
)

func TestConfig(t *testing.T) {
	cases := []operatortest.ConfigUnmarshalTest{
		{
			Name:      "default",
			ExpectErr: false,
			Expect:    defaultCfg(),
		},
		{
			Name:      "custom_id",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.OperatorID = "merge-split-lines"
				return cfg
			}(),
		},
		{
			Name:      "combine_with_custom_string",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.CombineWith = "ABC"
				return cfg
			}(),
		},
		{
			Name:      "combine_with_empty_string",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.CombineWith = ""
				return cfg
			}(),
		},
		{
			Name:      "combine_with_tab",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.CombineWith = "\t"
				return cfg
			}(),
		},
		{
			Name:      "combine_with_backslash_t",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.CombineWith = "\\t"
				return cfg
			}(),
		},
		{
			Name:      "combine_with_multiline_string",
			ExpectErr: false,
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.CombineWith = "line1\nLINE2"
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

func defaultCfg() *Config {
	return NewConfig("recombine")
}
