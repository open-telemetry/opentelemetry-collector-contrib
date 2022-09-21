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
package remove

import (
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"
)

// test unmarshalling of values into config struct
func TestUnmarshal(t *testing.T) {
	operatortest.ConfigUnmarshalTests{
		DefaultConfig: NewConfig(),
		TestsFile:     filepath.Join(".", "testdata", "config.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name: "remove_body",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = newBodyField("nested")
					return cfg
				}(),
			},
			{
				Name: "remove_single_attribute",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = newAttributeField("key")
					return cfg
				}(),
			},
			{
				Name: "remove_single_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = newResourceField("key")
					return cfg
				}(),
			},
			{
				Name: "remove_entire_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field.allResource = true
					return cfg
				}(),
			},
			{
				Name: "remove_entire_body",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field.Field = entry.NewBodyField()
					return cfg
				}(),
			},
			{
				Name: "remove_entire_attributes",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field.allAttributes = true
					return cfg
				}(),
			},
			{
				Name: "remove_nested_body",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = newBodyField("one", "two")
					return cfg
				}(),
			},
			{
				Name: "remove_nested_attribute",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = newAttributeField("one", "two")
					return cfg
				}(),
			},
			{
				Name: "remove_nested_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = newResourceField("one", "two")
					return cfg
				}(),
			},
		},
	}.Run(t)
}

func newBodyField(keys ...string) rootableField {
	field := entry.NewBodyField(keys...)
	return rootableField{Field: field}
}

func newResourceField(keys ...string) rootableField {
	field := entry.NewResourceField(keys...)
	return rootableField{Field: field}
}

func newAttributeField(keys ...string) rootableField {
	field := entry.NewAttributeField(keys...)
	return rootableField{Field: field}
}
