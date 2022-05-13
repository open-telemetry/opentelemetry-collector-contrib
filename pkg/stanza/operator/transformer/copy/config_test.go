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
package copy

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper/operatortest"
)

// test unmarshalling of values into config struct
func TestGoldenConfig(t *testing.T) {
	cases := []operatortest.ConfigUnmarshalTest{
		{
			Name: "body_to_body",
			Expect: func() *CopyOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewBodyField("key")
				cfg.To = entry.NewBodyField("key2")
				return cfg
			}(),
		},
		{
			Name: "body_to_attribute",
			Expect: func() *CopyOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewBodyField("key")
				cfg.To = entry.NewAttributeField("key2")
				return cfg
			}(),
		},
		{
			Name: "attribute_to_resource",
			Expect: func() *CopyOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewAttributeField("key")
				cfg.To = entry.NewResourceField("key2")
				return cfg
			}(),
		},
		{
			Name: "attribute_to_body",
			Expect: func() *CopyOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewAttributeField("key")
				cfg.To = entry.NewBodyField("key2")
				return cfg
			}(),
		},
		{
			Name: "attribute_to_nested_attribute",
			Expect: func() *CopyOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewAttributeField("key")
				cfg.To = entry.NewAttributeField("one", "two", "three")
				return cfg
			}(),
		},
		{
			Name: "resource_to_nested_resource",
			Expect: func() *CopyOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewResourceField("key")
				cfg.To = entry.NewResourceField("one", "two", "three")
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

func defaultCfg() *CopyOperatorConfig {
	return NewCopyOperatorConfig("copy")
}
