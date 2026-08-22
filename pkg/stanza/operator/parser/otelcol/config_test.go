// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelcol

import (
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"
)

func TestConfig(t *testing.T) {
	operatortest.ConfigUnmarshalTests{
		DefaultConfig: NewConfig(),
		TestsFile:     filepath.Join(".", "testdata", "config.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name:   "default",
				Expect: NewConfig(),
			},
			{
				Name: "on_error_drop",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.OnError = "drop"
					return cfg
				}(),
			},
			{
				Name: "format_json",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Format = formatJSON
					return cfg
				}(),
			},
			{
				Name: "format_console",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Format = formatConsole
					return cfg
				}(),
			},
			{
				Name: "format_auto",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Format = formatAuto
					return cfg
				}(),
			},
		},
	}.Run(t)
}
