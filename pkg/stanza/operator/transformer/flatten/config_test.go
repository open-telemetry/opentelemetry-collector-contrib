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
				Name: "flatten_one_level",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.BodyField{
						Keys: []string{"nested"},
					}
					return cfg
				}(),
				ExpectErr: false,
			},
			{
				Name: "flatten_second_level",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.BodyField{
						Keys: []string{"nested", "secondlevel"},
					}
					return cfg
				}(),
				ExpectErr: false,
			},
			{
				Name: "flatten_attributes",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.BodyField{
						Keys: []string{"attributes", "errField"},
					}
					return cfg
				}(),
				ExpectErr: true,
			},
		},
	}.Run(t)
}
