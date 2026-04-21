// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"
)

// test unmarshalling of values into config struct
func TestUnmarshal(t *testing.T) {
	operatortest.ConfigUnmarshalTests{
		DefaultConfig: NewConfig(),
		TestsFile:     filepath.Join(".", "testdata", "config.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name:   "default",
				Expect: NewConfig(),
			},
			{
				Name: "drop_ratio_0",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.DropRatio = 0
					return cfg
				}(),
			},
			{
				Name: "drop_ratio_half",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.DropRatio = 0.5
					return cfg
				}(),
			},
			{
				Name: "drop_ratio_1",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.DropRatio = 1
					return cfg
				}(),
			},
			{
				Name: "expr",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Expression = "body == 'value'"
					return cfg
				}(),
			},
		},
	}.Run(t)
}
