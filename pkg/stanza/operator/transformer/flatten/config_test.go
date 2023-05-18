// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package flatten

import (
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"
)

// Test unmarshalling of values into config struct
func TestUnmarshal(t *testing.T) {
	operatortest.ConfigUnmarshalTests{
		DefaultConfig: NewConfig(),
		TestsFile:     filepath.Join(".", "testdata", "config.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name: "flatten_body_one_level",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewBodyField("nested")
					return cfg
				}(),
				ExpectErr: false,
			},
			{
				Name: "flatten_body_second_level",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewBodyField("nested", "secondlevel")
					return cfg
				}(),
				ExpectErr: false,
			},
			{
				Name: "flatten_resource_one_level",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewResourceField("nested")
					return cfg
				}(),
				ExpectErr: false,
			},
			{
				Name: "flatten_resource_second_level",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewResourceField("nested", "secondlevel")
					return cfg
				}(),
				ExpectErr: false,
			},
			{
				Name: "flatten_attributes_one_level",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewAttributeField("nested")
					return cfg
				}(),
				ExpectErr: false,
			},
			{
				Name: "flatten_attributes_second_level",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewAttributeField("nested", "secondlevel")
					return cfg
				}(),
				ExpectErr: false,
			},
		},
	}.Run(t)
}
