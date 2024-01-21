// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package json_array

import (
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"
)

func TestConfig(t *testing.T) {
	operatortest.ConfigUnmarshalTests{
		DefaultConfig: NewConfig(),
		TestsFile:     filepath.Join(".", "testdata", "config.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name: "basic",
				Expect: func() *Config {
					p := NewConfig()
					p.ParseFrom = entry.NewBodyField("message")
					p.ParseTo = entry.RootableField{Field: entry.NewBodyField("messageParsed")}
					return p
				}(),
			},
			{
				Name: "parse_to_attributes",
				Expect: func() *Config {
					p := NewConfig()
					p.ParseFrom = entry.NewBodyField()
					p.ParseTo = entry.RootableField{Field: entry.NewAttributeField("output")}
					return p
				}(),
			},
			{
				Name: "parse_to_body",
				Expect: func() *Config {
					p := NewConfig()
					p.ParseTo = entry.RootableField{Field: entry.NewBodyField()}
					return p
				}(),
			},
			{
				Name: "parse_to_resource",
				Expect: func() *Config {
					p := NewConfig()
					p.ParseTo = entry.RootableField{Field: entry.NewResourceField("output")}
					return p
				}(),
			},
		},
	}.Run(t)
}
