// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package solacereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigValidateMissingAuth(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Queue = "someQueue"
	err := cfg.Validate()
	assert.Equal(t, errMissingAuthDetails, err)
}

func TestConfigValidateMissingQueue(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Auth.PlainText = &SaslPlainTextConfig{"Username", "Password"}
	err := cfg.Validate()
	assert.Equal(t, errMissingQueueName, err)
}

func TestConfigValidateSuccess(t *testing.T) {
	successCases := map[string]func(*Config){
		"With Plaintext Auth": func(c *Config) {
			c.Auth.PlainText = &SaslPlainTextConfig{"Username", "Password"}
		},
		"With XAuth2 Auth": func(c *Config) {
			c.Auth.XAuth2 = &SaslXAuth2Config{
				Username: "Username",
				Bearer:   "Bearer",
			}
		},
		"With External Auth": func(c *Config) {
			c.Auth.External = &SaslExternalConfig{}
		},
	}

	for caseName, configure := range successCases {
		t.Run(caseName, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Queue = "someQueue"
			configure(cfg)
			err := cfg.Validate()
			assert.NoError(t, err)
		})
	}
}
