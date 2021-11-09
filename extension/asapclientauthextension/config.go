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

import (
	"errors"

	"go.opentelemetry.io/collector/config"
)

var (
	errNoKeyIDProvided      = errors.New("no key id provided in asapclient configuration")
	errNoTtlProvided        = errors.New("no valid ttl was provided in asapclient configuration")
	errNoIssuerProvided     = errors.New("no issuer provided in asapclient configuration")
	errNoAudienceProvided   = errors.New("no audience provided in asapclient configuration")
	errNoPrivateKeyProvided = errors.New("no private key provided in asapclient configuration")
)

type Config struct {
	config.ExtensionSettings `mapstructure:",squash"`

	KeyId string `mapstructure:"key_id"`

	Ttl int `mapstructure:"ttl_seconds"`

	Issuer string `mapstructure:"issuer"`

	Audience []string `mapstructure:"audience"`

	PrivateKey string `mapstructure:"private_key"`
}

func (c *Config) Validate() error {
	if c.KeyId == "" {
		return errNoKeyIDProvided
	}
	if c.Ttl <= 0 {
		return errNoTtlProvided
	}
	if c.Audience == nil || len(c.Audience) == 0 {
		return errNoAudienceProvided
	}
	if c.Issuer == "" {
		return errNoIssuerProvided
	}
	if c.PrivateKey == "" {
		return errNoPrivateKeyProvided
	}
	return nil
}
