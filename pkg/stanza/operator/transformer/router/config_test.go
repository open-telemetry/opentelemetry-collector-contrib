// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
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
