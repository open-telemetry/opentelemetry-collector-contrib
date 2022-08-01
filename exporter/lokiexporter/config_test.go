// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lokiexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/confighttp"
)

func TestIsLegacy(t *testing.T) {
	testCases := []struct {
		desc    string
		cfg     *Config
		outcome bool
	}{
		{
			// the default mode for an empty config is the new logic
			desc: "not legacy",
			cfg: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "https://loki.example.com",
				},
			},
			outcome: false,
		},
		{
			desc: "format is set to body",
			cfg: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "https://loki.example.com",
				},
				Format: stringp("body"),
			},
			outcome: true,
		},
		{
			desc: "a label is specified",
			cfg: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "https://loki.example.com",
				},
				Labels: &LabelsConfig{
					Attributes: map[string]string{"some_attribute": "some_value"},
				},
			},
			outcome: true,
		},
		{
			desc: "a tenant is specified",
			cfg: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "https://loki.example.com",
				},
				Tenant: &Tenant{
					Source: "static",
					Value:  "acme",
				},
			},
			outcome: true,
		},
		{
			desc: "a tenant ID is specified",
			cfg: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "https://loki.example.com",
				},
				TenantID: stringp("acme"),
			},
			outcome: true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			assert.Equal(t, tC.outcome, tC.cfg.isLegacy())

			// all configs from this table test are valid:
			assert.NoError(t, tC.cfg.Validate())
		})
	}
}

func stringp(str string) *string {
	return &str
}
