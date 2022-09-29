package add

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
				Name: "add_value",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewBodyField("new")
					cfg.Value = "randomMessage"
					return cfg
				}(),
			},
			{
				Name: "add_expr",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewBodyField("new")
					cfg.Value = `EXPR(body.key + "_suffix")`
					return cfg
				}(),
			},
			{
				Name: "add_nest",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewBodyField("new")
					cfg.Value = map[string]interface{}{
						"nest": map[string]interface{}{"key": "val"},
					}
					return cfg
				}(),
			},
			{
				Name: "add_attribute",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewAttributeField("new")
					cfg.Value = "newVal"
					return cfg
				}(),
			},
			{
				Name: "add_nested_attribute",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewAttributeField("one", "two")
					cfg.Value = "newVal"
					return cfg
				}(),
			},
			{
				Name: "add_nested_obj_attribute",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewAttributeField("one", "two")
					cfg.Value = map[string]interface{}{
						"nest": map[string]interface{}{"key": "val"},
					}
					return cfg
				}(),
			},
			{
				Name: "add_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewResourceField("new")
					cfg.Value = "newVal"
					return cfg
				}(),
			},
			{
				Name: "add_nested_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewResourceField("one", "two")
					cfg.Value = "newVal"
					return cfg
				}(),
			},
			{
				Name: "add_nested_obj_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewResourceField("one", "two")
					cfg.Value = map[string]interface{}{
						"nest": map[string]interface{}{"key": "val"},
					}
					return cfg
				}(),
			},
			{
				Name: "add_resource_expr",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewResourceField("new")
					cfg.Value = `EXPR(body.key + "_suffix")`
					return cfg
				}(),
			},
			{
				Name: "add_array_to_body",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewBodyField("new")
					cfg.Value = []interface{}{1, 2, 3, 4}
					return cfg
				}(),
			},
			{
				Name: "add_array_to_attributes",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewAttributeField("new")
					cfg.Value = []interface{}{1, 2, 3, 4}
					return cfg
				}(),
			},

			{
				Name: "add_array_to_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewResourceField("new")
					cfg.Value = []interface{}{1, 2, 3, 4}
					return cfg
				}(),
			},
		},
	}.Run(t)
}
