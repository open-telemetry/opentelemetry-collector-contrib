// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package retain

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
				Name: "retain_single",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Fields = append(cfg.Fields, entry.NewBodyField("key"))
					return cfg
				}(),
			},
			{
				Name: "retain_multi",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Fields = append(cfg.Fields, entry.NewBodyField("key"))
					cfg.Fields = append(cfg.Fields, entry.NewBodyField("nested2"))
					return cfg
				}(),
			},
			{
				Name: "retain_multilevel",
				Expect: func() *Config {
					cfg := NewConfig()
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
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Fields = append(cfg.Fields, entry.NewAttributeField("key"))
					return cfg
				}(),
			},
			{
				Name: "retain_multi_attribute",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Fields = append(cfg.Fields, entry.NewAttributeField("key1"))
					cfg.Fields = append(cfg.Fields, entry.NewAttributeField("key2"))
					return cfg
				}(),
			},
			{
				Name: "retain_single_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Fields = append(cfg.Fields, entry.NewResourceField("key"))
					return cfg
				}(),
			},
			{
				Name: "retain_multi_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Fields = append(cfg.Fields, entry.NewResourceField("key1"))
					cfg.Fields = append(cfg.Fields, entry.NewResourceField("key2"))
					return cfg
				}(),
			},
			{
				Name: "retain_one_of_each",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Fields = append(cfg.Fields, entry.NewResourceField("key1"))
					cfg.Fields = append(cfg.Fields, entry.NewAttributeField("key3"))
					cfg.Fields = append(cfg.Fields, entry.NewBodyField("key"))
					return cfg
				}(),
			},
		},
	}.Run(t)

}
