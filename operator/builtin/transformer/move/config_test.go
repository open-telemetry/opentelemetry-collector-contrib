package move

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
	"testing"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper/operatortest"
)

// test unmarshalling of values into config struct
func TestGoldenConfig(t *testing.T) {
	cases := []operatortest.ConfigUnmarshalTest{
		{
			Name: "MoveBodyToBody",
			Expect: func() *MoveOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewBodyField("key")
				cfg.To = entry.NewBodyField("new")
				return cfg
			}(),
		},
		{
			Name: "MoveBodyToAttribute",
			Expect: func() *MoveOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewBodyField("key")
				cfg.To = entry.NewAttributeField("new")
				return cfg
			}(),
		},
		{
			Name: "MoveAttributeToBody",
			Expect: func() *MoveOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewAttributeField("new")
				cfg.To = entry.NewBodyField("new")
				return cfg
			}(),
		},
		{
			Name: "MoveAttributeToResource",
			Expect: func() *MoveOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewAttributeField("new")
				cfg.To = entry.NewResourceField("new")
				return cfg
			}(),
		},
		{
			Name: "MoveBracketedAttributeToResource",
			Expect: func() *MoveOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewAttributeField("dotted.field.name")
				cfg.To = entry.NewResourceField("new")
				return cfg
			}(),
		},
		{
			Name: "MoveResourceToAttribute",
			Expect: func() *MoveOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewResourceField("new")
				cfg.To = entry.NewAttributeField("new")
				return cfg
			}(),
		},
		{
			Name: "MoveNest",
			Expect: func() *MoveOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewBodyField("nested")
				cfg.To = entry.NewBodyField("NewNested")
				return cfg
			}(),
		},
		{
			Name: "MoveFromNestedObj",
			Expect: func() *MoveOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewBodyField("nested", "nestedkey")
				cfg.To = entry.NewBodyField("unnestedkey")
				return cfg
			}(),
		},
		{
			Name: "MoveToNestedObj",
			Expect: func() *MoveOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewBodyField("newnestedkey")
				cfg.To = entry.NewBodyField("nested", "newnestedkey")
				return cfg
			}(),
		},
		{
			Name: "MoveDoubleNestedObj",
			Expect: func() *MoveOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewBodyField("nested", "nested2")
				cfg.To = entry.NewBodyField("nested2")
				return cfg
			}(),
		},
		{
			Name: "MoveNestToResource",
			Expect: func() *MoveOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewBodyField("nested")
				cfg.To = entry.NewResourceField("NewNested")
				return cfg
			}(),
		},
		{
			Name: "MoveNestToAttribute",
			Expect: func() *MoveOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewBodyField("nested")
				cfg.To = entry.NewAttributeField("NewNested")
				return cfg
			}(),
		},
		{
			Name: "ImplicitBodyFrom",
			Expect: func() *MoveOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewBodyField("implicitkey")
				cfg.To = entry.NewAttributeField("new")
				return cfg
			}(),
		},
		{
			Name: "ImplicitBodyTo",
			Expect: func() *MoveOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewAttributeField("new")
				cfg.To = entry.NewBodyField("implicitkey")
				return cfg
			}(),
		},
		{
			Name: "ImplicitNestedKey",
			Expect: func() *MoveOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewAttributeField("new")
				cfg.To = entry.NewBodyField("key", "key2")
				return cfg
			}(),
		},
		{
			Name: "ReplaceBody",
			Expect: func() *MoveOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewBodyField("nested")
				cfg.To = entry.NewBodyField()
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

func defaultCfg() *MoveOperatorConfig {
	return NewMoveOperatorConfig("move")
}
