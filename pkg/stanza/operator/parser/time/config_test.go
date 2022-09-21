// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package time

import (
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"
)

func TestUnmarshal(t *testing.T) {
	operatortest.ConfigUnmarshalTests{
		DefaultConfig: NewConfig(),
		TestsFile:     filepath.Join(".", "testdata", "config.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name:   "default",
				Expect: NewConfig(),
			},
			{
				Name: "on_error_drop",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.OnError = "drop"
					return cfg
				}(),
			},
			{
				Name: "parse_strptime",
				Expect: func() *Config {
					cfg := NewConfig()
					from := entry.NewBodyField("from")
					cfg.ParseFrom = &from
					cfg.Layout = "%Y-%m-%d"
					return cfg
				}(),
			},
			{
				Name: "parse_gotime",
				Expect: func() *Config {
					cfg := NewConfig()
					from := entry.NewBodyField("from")
					cfg.ParseFrom = &from
					cfg.LayoutType = "gotime"
					cfg.Layout = "2006-01-02"
					return cfg
				}(),
			},
			{
				Name:      "no_nested",
				ExpectErr: true,
			},
		},
	}.Run(t)
}
