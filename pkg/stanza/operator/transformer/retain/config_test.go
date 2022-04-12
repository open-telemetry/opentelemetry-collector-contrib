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
package retain

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper/operatortest"
)

// test unmarshalling of values into config struct
func TestGoldenConfigs(t *testing.T) {
	cases := []operatortest.ConfigUnmarshalTest{
		{
			Name: "retain_single",
			Expect: func() *OperatorConfig {
				cfg := defaultCfg()
				cfg.Fields = append(cfg.Fields, entry.NewBodyField("key"))
				return cfg
			}(),
		},
		{
			Name: "retain_multi",
			Expect: func() *OperatorConfig {
				cfg := defaultCfg()
				cfg.Fields = append(cfg.Fields, entry.NewBodyField("key"))
				cfg.Fields = append(cfg.Fields, entry.NewBodyField("nested2"))
				return cfg
			}(),
		},
		{
			Name: "retain_multilevel",
			Expect: func() *OperatorConfig {
				cfg := defaultCfg()
				cfg.Fields = append(cfg.Fields, entry.NewBodyField("foo"))
				cfg.Fields = append(cfg.Fields, entry.NewBodyField("one", "two"))
				cfg.Fields = append(cfg.Fields, entry.NewAttributeField("foo"))
				cfg.Fields = append(cfg.Fields, entry.NewAttributeField("one", "two"))
				cfg.Fields = append(cfg.Fields, entry.NewResourceField("foo"))
				cfg.Fields = append(cfg.Fields, entry.NewResourceField("one", "two"))
				return cfg
			}(),
		},
		{
			Name: "retain_single_attribute",
			Expect: func() *OperatorConfig {
				cfg := defaultCfg()
				cfg.Fields = append(cfg.Fields, entry.NewAttributeField("key"))
				return cfg
			}(),
		},
		{
			Name: "retain_multi_attribute",
			Expect: func() *OperatorConfig {
				cfg := defaultCfg()
				cfg.Fields = append(cfg.Fields, entry.NewAttributeField("key1"))
				cfg.Fields = append(cfg.Fields, entry.NewAttributeField("key2"))
				return cfg
			}(),
		},
		{
			Name: "retain_single_resource",
			Expect: func() *OperatorConfig {
				cfg := defaultCfg()
				cfg.Fields = append(cfg.Fields, entry.NewResourceField("key"))
				return cfg
			}(),
		},
		{
			Name: "retain_multi_resource",
			Expect: func() *OperatorConfig {
				cfg := defaultCfg()
				cfg.Fields = append(cfg.Fields, entry.NewResourceField("key1"))
				cfg.Fields = append(cfg.Fields, entry.NewResourceField("key2"))
				return cfg
			}(),
		},
		{
			Name: "retain_one_of_each",
			Expect: func() *OperatorConfig {
				cfg := defaultCfg()
				cfg.Fields = append(cfg.Fields, entry.NewResourceField("key1"))
				cfg.Fields = append(cfg.Fields, entry.NewAttributeField("key3"))
				cfg.Fields = append(cfg.Fields, entry.NewBodyField("key"))
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

func defaultCfg() *OperatorConfig {
	return NewOperatorConfig("retain")
}
