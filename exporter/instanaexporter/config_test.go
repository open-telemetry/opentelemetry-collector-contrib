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

package instanaexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
)

func TestConfigValidate(t *testing.T) {
	t.Run("Empty configuration", func(t *testing.T) {
		c := &Config{}
		err := c.Validate()
		assert.Error(t, err)
	})

	t.Run("Valid configuration", func(t *testing.T) {
		c := &Config{Endpoint: "https://example.com/", AgentKey: "key1"}
		err := c.Validate()
		assert.NoError(t, err)

		assert.Equal(t, "https://example.com/", c.Endpoint, "no Instana endpoint set")
		assert.Equal(t, "", c.TLSSetting.CAFile, "optional ca_file property not set")
	})

	t.Run("Valid configuration with ca_file", func(t *testing.T) {
		c := &Config{Endpoint: "https://example.com/", AgentKey: "key1", HTTPClientSettings: confighttp.HTTPClientSettings{
			TLSSetting: configtls.TLSClientSetting{
				TLSSetting: configtls.TLSSetting{
					CAFile: "ca.crt",
				},
			},
		}}
		err := c.Validate()
		assert.NoError(t, err)

		assert.Equal(t, "https://example.com/", c.Endpoint, "no Instana endpoint set")
		assert.Equal(t, "ca.crt", c.TLSSetting.CAFile, "optional ca_file property set")
	})

	t.Run("Invalid Endpoint Invalid URL", func(t *testing.T) {
		c := &Config{Endpoint: "https://example.}~", AgentKey: "key1"}
		err := c.Validate()
		assert.Error(t, err)
	})

	t.Run("Invalid Endpoint No Protocol", func(t *testing.T) {
		c := &Config{Endpoint: "example.com", AgentKey: "key1"}
		err := c.Validate()
		assert.Error(t, err)
	})

	t.Run("Invalid Endpoint No https:// Protocol", func(t *testing.T) {
		c := &Config{Endpoint: "http://example.com", AgentKey: "key1"}
		err := c.Validate()
		assert.Error(t, err, "endpoint must start with https://")
	})

	t.Run("No Agent key", func(t *testing.T) {
		c := &Config{Endpoint: "https://example.com/"}
		err := c.Validate()
		assert.Error(t, err)
	})
}
