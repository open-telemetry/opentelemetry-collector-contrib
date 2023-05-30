// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package trace

import (
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
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
				Name: "spanid",
				Expect: func() *Config {
					parseFrom := entry.NewBodyField("app_span_id")
					cfg := helper.SpanIDConfig{}
					cfg.ParseFrom = &parseFrom

					c := NewConfig()
					c.SpanID = &cfg
					return c
				}(),
			},
			{
				Name: "traceid",
				Expect: func() *Config {
					parseFrom := entry.NewBodyField("app_trace_id")
					cfg := helper.TraceIDConfig{}
					cfg.ParseFrom = &parseFrom

					c := NewConfig()
					c.TraceID = &cfg
					return c
				}(),
			},
			{
				Name: "trace_flags",
				Expect: func() *Config {
					parseFrom := entry.NewBodyField("app_trace_flags_id")
					cfg := helper.TraceFlagsConfig{}
					cfg.ParseFrom = &parseFrom

					c := NewConfig()
					c.TraceFlags = &cfg
					return c
				}(),
			},
		},
	}.Run(t)
}
