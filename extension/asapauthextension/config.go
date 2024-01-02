// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package asapauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/asapauthextension"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/config/configopaque"
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
	KeyID string `mapstructure:"key_id"`

	TTL time.Duration `mapstructure:"ttl"`

	Issuer string `mapstructure:"issuer"`

	Audience []string `mapstructure:"audience"`

	PrivateKey configopaque.String `mapstructure:"private_key"`
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
