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
package metadata

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper/operatortest"
)

func TestMetaDataGoldenConfig(t *testing.T) {
	cases := []operatortest.ConfigUnmarshalTest{
		{
			Name:   "default",
			Expect: defaultCfg(),
		},
		{
			Name: "id",
			Expect: func() *MetadataOperatorConfig {
				cfg := defaultCfg()
				cfg.OperatorID = "newName"
				return cfg
			}(),
		},
		{
			Name: "output",
			Expect: func() *MetadataOperatorConfig {
				cfg := defaultCfg()
				cfg.OutputIDs = append(cfg.OutputIDs, "nextOperator")
				return cfg
			}(),
		},
		{
			Name: "attributes_single",
			Expect: func() *MetadataOperatorConfig {
				cfg := defaultCfg()
				cfg.Attributes = map[string]helper.ExprStringConfig{
					"key1": `val1`,
				}
				return cfg
			}(),
		},
		{
			Name: "attributes_multi",
			Expect: func() *MetadataOperatorConfig {
				cfg := defaultCfg()
				cfg.Attributes = map[string]helper.ExprStringConfig{
					"key1": `val1`,
					"key2": `val2`,
					"key3": `val3`,
				}
				return cfg
			}(),
		},
		{
			Name: "resource_single",
			Expect: func() *MetadataOperatorConfig {
				cfg := defaultCfg()
				cfg.Resource = map[string]helper.ExprStringConfig{
					"key1": `val1`,
				}
				return cfg
			}(),
		},
		{
			Name: "resource_multi",
			Expect: func() *MetadataOperatorConfig {
				cfg := defaultCfg()
				cfg.Resource = map[string]helper.ExprStringConfig{
					"key1": `val1`,
					"key2": `val2`,
					"key3": `val3`,
				}
				return cfg
			}(),
		},
		{
			Name: "on_error",
			Expect: func() *MetadataOperatorConfig {
				cfg := defaultCfg()
				cfg.OnError = "drop"
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

func defaultCfg() *MetadataOperatorConfig {
	return NewMetadataOperatorConfig("metadata")
}
