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
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper/operatortest"
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
				Name:      "custom_id",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.OperatorID = "merge-split-lines"
					return cfg
				}(),
			},
			{
				Name:      "combine_with_custom_string",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.CombineWith = "ABC"
					return cfg
				}(),
			},
			{
				Name:      "combine_with_empty_string",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.CombineWith = ""
					return cfg
				}(),
			},
			{
				Name:      "combine_with_tab",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.CombineWith = "\t"
					return cfg
				}(),
			},
			{
				Name:      "combine_with_backslash_t",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.CombineWith = "\\t"
					return cfg
				}(),
			},
			{
				Name:      "combine_with_multiline_string",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.CombineWith = "line1\nLINE2"
					return cfg
				}(),
			},
		},
	}.Run(t)
}
