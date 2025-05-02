// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudfoundryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudfoundryreceiver"

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
)

type RLPGatewayConfig struct {
	confighttp.ClientConfig `mapstructure:",squash"`
	ShardID                 string `mapstructure:"shard_id"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// LimitedTLSClientSetting is a subset of TLSClientSetting, see LimitedClientConfig for more details
type LimitedTLSClientSetting struct {
	InsecureSkipVerify bool `mapstructure:"insecure_skip_verify"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// LimitedClientConfig is a subset of ClientConfig, implemented as a separate type due to the library this
// configuration is used with not taking a preconfigured http.Client as input, but only taking these specific options
type LimitedClientConfig struct {
	Endpoint   string                  `mapstructure:"endpoint"`
	TLSSetting LimitedTLSClientSetting `mapstructure:"tls"`

	// prevent unkeyed literal initialization
	_ struct{}
}

type UAAConfig struct {
	LimitedClientConfig `mapstructure:",squash"`
	Username            string              `mapstructure:"username"`
	Password            configopaque.String `mapstructure:"password"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// Config defines configuration for Collectd receiver.
type Config struct {
	RLPGateway RLPGatewayConfig `mapstructure:"rlp_gateway"`
	UAA        UAAConfig        `mapstructure:"uaa"`

	// prevent unkeyed literal initialization
	_ struct{}
}

func (c *Config) Validate() error {
	err := validateURLOption("rlp_gateway.endpoint", c.RLPGateway.Endpoint)
	if err != nil {
		return err
	}

	if strings.TrimSpace(c.RLPGateway.ShardID) == "" {
		return errors.New("shardID cannot be empty")
	}

	err = validateURLOption("uaa.endpoint", c.UAA.Endpoint)
	if err != nil {
		return err
	}

	if c.UAA.Username == "" {
		return errors.New("UAA username not specified")
	}

	if c.UAA.Password == "" {
		return errors.New("UAA password not specified")
	}

	return nil
}

func validateURLOption(name string, value string) error {
	if value == "" {
		return fmt.Errorf("%s not specified", name)
	}

	_, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("failed to parse %s as url: %w", name, err)
	}

	return nil
}
