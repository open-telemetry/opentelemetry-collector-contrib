// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package assignkeys

import (
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"
)

// Test unmarshalling of values into config struct
func TestUnmarshal(t *testing.T) {
	// Manually adding operator to the Registry as its behind a feature gate
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })

	operatortest.ConfigUnmarshalTests{
		DefaultConfig: NewConfig(),
		TestsFile:     filepath.Join(".", "testdata", "config.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name: "assign_keys_body",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewBodyField()
					cfg.Keys = []string{"foo", "bar"}
					return cfg
				}(),
				ExpectErr: false,
			},
			{
				Name: "assign_keys_body_nested",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewBodyField("nested")
					cfg.Keys = []string{"foo", "bar"}
					return cfg
				}(),
				ExpectErr: false,
			},
			{
				Name: "assign_keys_attributes",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewAttributeField("input")
					cfg.Keys = []string{"foo", "bar"}
					return cfg
				}(),
				ExpectErr: false,
			},
		},
	}.Run(t)
}
