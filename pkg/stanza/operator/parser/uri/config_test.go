// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package uri

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper/operatortest"
)

func TestParserGoldenConfig(t *testing.T) {
	cases := []operatortest.ConfigUnmarshalTest{
		{
			Name:   "default",
			Expect: defaultCfg(),
		},
		{
			Name: "parse_from_simple",
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.ParseFrom = entry.NewBodyField("from")
				return cfg
			}(),
		},
		{
			Name: "parse_to_simple",
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.ParseTo = entry.NewBodyField("log")
				return cfg
			}(),
		},
		{
			Name: "on_error_drop",
			Expect: func() *Config {
				cfg := defaultCfg()
				cfg.OnError = "drop"
				return cfg
			}(),
		},
		{
			Name: "timestamp",
			Expect: func() *Config {
				cfg := defaultCfg()
				parseField := entry.NewBodyField("timestamp_field")
				newTime := helper.TimeParser{
					LayoutType: "strptime",
					Layout:     "%Y-%m-%d",
					ParseFrom:  &parseField,
				}
				cfg.TimeParser = &newTime
				return cfg
			}(),
		},
		{
			Name: "severity",
			Expect: func() *Config {
				cfg := defaultCfg()
				parseField := entry.NewBodyField("severity_field")
				severityField := helper.NewSeverityConfig()
				severityField.ParseFrom = &parseField
				mapping := map[interface{}]interface{}{
					"critical": "5xx",
					"error":    "4xx",
					"info":     "3xx",
					"debug":    "2xx",
				}
				severityField.Mapping = mapping
				cfg.Config = &severityField
				return cfg
			}(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			tc.Run(t, defaultCfg())
		})
	}
}

func defaultCfg() *Config {
	return NewConfig("uri_parser")
}
