// Copyright  The OpenTelemetry Authors
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

package asapclientauthextension

import "go.opentelemetry.io/collector/config"

type Config struct {
	config.ExtensionSettings `mapstructure:",squash"`


	KeyId string `mapstructure:"key_id"`

	Ttl int `mapstructure:"ttl_seconds"`

	Issuer string `mapstructure:"issuer"`

	Audience []string `mapstructure:"audience"`

	PrivateKey string `mapstructure:"private_key"`

	SigningMethod string `mapstrucutre:"signing_method"`
}

func (c *Config) Validate() error {
	// todo
	return nil
}
