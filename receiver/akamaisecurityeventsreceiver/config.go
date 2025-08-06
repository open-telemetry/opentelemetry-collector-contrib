// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package akamaisecurityeventsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/akamaisecurityeventsreceiver"

import (
	"errors"
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	confighttp.ClientConfig        `mapstructure:",squash"`

	// ConfigIDs is a list of unique Akamai security configuration identifiers. To report on more than one configuration, separate integer identifiers with semicolons.
	ConfigIDs string `mapstructure:"config_ids"`

	// ClientToken for Akamai EdgeGrid authentication
	ClientToken string `mapstructure:"client_token"`

	// ClientSecret for Akamai EdgeGrid authentication
	ClientSecret string `mapstructure:"client_secret"`

	// AccessToken for Akamai EdgeGrid authentication
	AccessToken string `mapstructure:"access_token"`

	// Limit is the maximum number of events to fetch per request (default 10000, max 600000)
	Limit int `mapstructure:"limit"`

	// ParseRuleData enables parsing of rule data from security events
	ParseRuleData bool `mapstructure:"parse_rule_data"`

	// FlattenRuleData flattens nested rule data structures into top-level fields
	FlattenRuleData bool `mapstructure:"flatten_rule_data"`

	// StorageID is the ID of the storage to use for persisting read offset
	StorageID *component.ID `mapstructure:"storage"`
}

var _ component.Config = (*Config)(nil)

func (cfg *Config) Validate() error {
	if _, err := url.Parse(cfg.Endpoint); err != nil {
		return fmt.Errorf("invalid endpoint URL: %w", err)
	}

	if cfg.ClientToken == "" {
		return errors.New("client_token is required")
	}

	if cfg.ClientSecret == "" {
		return errors.New("client_secret is required")
	}

	if cfg.AccessToken == "" {
		return errors.New("access_token is required")
	}

	if cfg.ConfigIDs == "" {
		return errors.New("config_id is required")
	}

	return nil
}
