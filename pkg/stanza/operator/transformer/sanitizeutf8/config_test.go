// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sanitizeutf8

import (
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"
)

func TestUnmarshal(t *testing.T) {
	operatortest.ConfigUnmarshalTests{
		DefaultConfig: NewConfig(),
		TestsFile:     filepath.Join(".", "testdata", "config.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name:               "default",
				ExpectUnmarshalErr: false,
				Expect:             NewConfig(),
			},
			{
				Name:               "sanitize_utf8_with_field",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewBodyField()
					return cfg
				}(),
			},
			{
				Name:               "on_error_drop",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewBodyField()
					cfg.OnError = "drop"
					return cfg
				}(),
			},
			{
				Name:               "sanitize_utf8_with_invalid_field",
				ExpectUnmarshalErr: true,
			},
		},
	}.Run(t)
}
