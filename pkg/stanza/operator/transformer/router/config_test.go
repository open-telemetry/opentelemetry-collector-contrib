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
package router

import (
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"
)

func TestRouterGoldenConfig(t *testing.T) {
	operatortest.ConfigUnmarshalTests{
		DefaultConfig: NewConfig(),
		TestsFile:     filepath.Join(".", "testdata", "config.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name:   "default",
				Expect: NewConfig(),
			},
			{
				Name: "routes_one",
				Expect: func() *Config {
					cfg := NewConfig()
					newRoute := &RouteConfig{
						Expression: `body.format == "json"`,
						OutputIDs:  []string{"my_json_parser"},
					}
					cfg.Routes = append(cfg.Routes, newRoute)
					return cfg
				}(),
			},
			{
				Name: "routes_multi",
				Expect: func() *Config {
					cfg := NewConfig()
					newRoute := []*RouteConfig{
						{
							Expression: `body.format == "json"`,
							OutputIDs:  []string{"my_json_parser"},
						},
						{
							Expression: `body.format == "json"2`,
							OutputIDs:  []string{"my_json_parser2"},
						},
						{
							Expression: `body.format == "json"3`,
							OutputIDs:  []string{"my_json_parser3"},
						},
					}
					cfg.Routes = newRoute
					return cfg
				}(),
			},
			{
				Name: "routes_attributes",
				Expect: func() *Config {
					cfg := NewConfig()

					attVal := helper.NewAttributerConfig()
					attVal.Attributes = map[string]helper.ExprStringConfig{
						"key1": "val1",
					}

					cfg.Routes = []*RouteConfig{
						{
							Expression:       `body.format == "json"`,
							OutputIDs:        []string{"my_json_parser"},
							AttributerConfig: attVal,
						},
					}
					return cfg
				}(),
			},
			{
				Name: "routes_default",
				Expect: func() *Config {
					cfg := NewConfig()
					newRoute := &RouteConfig{
						Expression: `body.format == "json"`,
						OutputIDs:  []string{"my_json_parser"},
					}
					cfg.Routes = append(cfg.Routes, newRoute)
					cfg.Default = append(cfg.Default, "catchall")
					return cfg
				}(),
			},
		},
	}.Run(t)
}
