// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver"

import (
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/metadata"
)

const (
	scrapersKey = "scrapers"
)

// Config that is exposed to this github receiver through the OTEL config.yaml
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	Scrapers                       map[string]internal.Config `mapstructure:"scrapers"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	WebHook                        WebHook `mapstructure:"webhook"`
}

type WebHook struct {
	confighttp.ServerConfig `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
	Path                    string                   `mapstructure:"path"`            // path for data collection. Default is /events
	HealthPath              string                   `mapstructure:"health_path"`     // path for health check api. Default is /health_check
	RequiredHeader          RequiredHeader           `mapstructure:"required_header"` // optional setting to set a required header for all requests to have
	Secret                  string                   `mapstructure:"secret"`          // secret for webhook
	ServiceName             string                   `mapstructure:"service_name"`    // optional name of service to use for spans
}

type RequiredHeader struct {
	Key   string `mapstructure:"key"`
	Value string `mapstructure:"value"`
}

var (
	_ component.Config    = (*Config)(nil)
	_ confmap.Unmarshaler = (*Config)(nil)

	errMissingEndpointFromConfig   = errors.New("missing receiver server endpoint from config")
	errReadTimeoutExceedsMaxValue  = errors.New("the duration specified for read_timeout exceeds the maximum allowed value of 10s")
	errWriteTimeoutExceedsMaxValue = errors.New("the duration specified for write_timeout exceeds the maximum allowed value of 10s")
	errRequiredHeader              = errors.New("both key and value are required to assign a required_header")
	errRequireOneScraper           = errors.New("must specify at least one scraper")
)

// Validate the configuration passed through the OTEL config.yaml
func (cfg *Config) Validate() error {
	var errs error

	maxReadWriteTimeout, _ := time.ParseDuration("10s")

	if cfg.WebHook.ServerConfig.Endpoint == "" {
		errs = multierr.Append(errs, errMissingEndpointFromConfig)
	}

	if cfg.WebHook.ServerConfig.ReadTimeout > maxReadWriteTimeout {
		errs = multierr.Append(errs, errReadTimeoutExceedsMaxValue)
	}

	if cfg.WebHook.ServerConfig.WriteTimeout > maxReadWriteTimeout {
		errs = multierr.Append(errs, errWriteTimeoutExceedsMaxValue)
	}

	if (cfg.WebHook.RequiredHeader.Key != "" && cfg.WebHook.RequiredHeader.Value == "") || (cfg.WebHook.RequiredHeader.Value != "" && cfg.WebHook.RequiredHeader.Key == "") {
		errs = multierr.Append(errs, errRequiredHeader)
	}

	return errs
}

// Unmarshal a config.Parser into the config struct.
func (cfg *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		return nil
	}

	// load the non-dynamic config normally
	err := componentParser.Unmarshal(cfg, confmap.WithIgnoreUnused())
	if err != nil {
		return err
	}

	// dynamically load the individual collector configs based on the key name

	cfg.Scrapers = map[string]internal.Config{}

	scrapersSection, err := componentParser.Sub(scrapersKey)
	if err != nil {
		return err
	}

	for key := range scrapersSection.ToStringMap() {
		factory, ok := getScraperFactory(key)
		if !ok {
			return fmt.Errorf("invalid scraper key: %q", key)
		}

		collectorCfg := factory.CreateDefaultConfig()
		collectorSection, err := scrapersSection.Sub(key)
		if err != nil {
			return err
		}

		err = collectorSection.Unmarshal(collectorCfg)
		if err != nil {
			return fmt.Errorf("error reading settings for scraper type %q: %w", key, err)
		}

		cfg.Scrapers[key] = collectorCfg
	}

	return nil
}
