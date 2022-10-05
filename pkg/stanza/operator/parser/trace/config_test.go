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
