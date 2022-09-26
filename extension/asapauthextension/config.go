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

package asapauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/asapauthextension"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/config"
	"go.uber.org/multierr"
)

var (
	errNoKeyIDProvided      = errors.New("no key id provided in asapclient configuration")
	errNoTTLProvided        = errors.New("no valid ttl was provided in asapclient configuration")
	errNoIssuerProvided     = errors.New("no issuer provided in asapclient configuration")
	errNoAudienceProvided   = errors.New("no audience provided in asapclient configuration")
	errNoPrivateKeyProvided = errors.New("no private key provided in asapclient configuration")
)

type Config struct {
	config.ExtensionSettings `mapstructure:",squash"`

	KeyID string `mapstructure:"key_id"`

	TTL time.Duration `mapstructure:"ttl"`

	Issuer string `mapstructure:"issuer"`

	Audience []string `mapstructure:"audience"`

	PrivateKey string `mapstructure:"private_key"`
}

func (c *Config) Validate() error {
	var errs error
	if c.KeyID == "" {
		errs = multierr.Append(errs, errNoKeyIDProvided)
	}
	if c.TTL <= 0 {
		errs = multierr.Append(errs, errNoTTLProvided)
	}
	if len(c.Audience) == 0 {
		errs = multierr.Append(errs, errNoAudienceProvided)
	}

	if c.Issuer == "" {
		errs = multierr.Append(errs, errNoIssuerProvided)
	}
	if c.PrivateKey == "" {
		errs = multierr.Append(errs, errNoPrivateKeyProvided)
	}
	return errs
}
