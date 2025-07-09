// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver"

import (
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/metadata"
)

const (
	scrapersKey = "scrapers"

	// GitHub Delivery Headers: https://docs.github.com/en/webhooks/webhook-events-and-payloads#delivery-headers
	defaultGitHubHookIDHeader       = "X-GitHub-Hook-ID"    // Unique identifier of the webhook.
	defaultGitHubEventHeader        = "X-GitHub-Event"      // The name of the event that triggered the delivery.
	defaultGitHubDeliveryHeader     = "X-GitHub-Delivery"   // A globally unique identifier (GUID) to identify the event.
	defaultGitHubSignature256Header = "X-Hub-Signature-256" // The HMAC hex digest of the request body; generated using the SHA-256 hash function and the secret as the HMAC key.
	defaultUserAgentHeader          = "User-Agent"          // Value always prefixed with "GitHub-Hookshot/"
)

// Config that is exposed to this github receiver through the OTEL config.yaml
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	Scrapers                       map[string]internal.Config `mapstructure:"scrapers"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	WebHook                        WebHook `mapstructure:"webhook"`
}

type WebHook struct {
	confighttp.ServerConfig `mapstructure:",squash"`       // squash ensures fields are correctly decoded in embedded struct
	Path                    string                         `mapstructure:"path"`             // path for data collection. Default is /events
	HealthPath              string                         `mapstructure:"health_path"`      // path for health check api. Default is /health_check
	RequiredHeaders         map[string]configopaque.String `mapstructure:"required_headers"` // optional setting to set one or more required headers for all requests to have (except the health check)
	GitHubHeaders           GitHubHeaders                  `mapstructure:",squash"`          // GitLab headers set by default
	Secret                  string                         `mapstructure:"secret"`           // secret for webhook
	ServiceName             string                         `mapstructure:"service_name"`
}

type GitHubHeaders struct {
	Customizable map[string]string `mapstructure:","` // can be overwritten via required_headers
	Fixed        map[string]string `mapstructure:","` // are not allowed to be overwritten
}

var (
	_ component.Config    = (*Config)(nil)
	_ confmap.Unmarshaler = (*Config)(nil)

	errMissingEndpointFromConfig   = errors.New("missing receiver server endpoint from config")
	errReadTimeoutExceedsMaxValue  = errors.New("the duration specified for read_timeout exceeds the maximum allowed value of 10s")
	errWriteTimeoutExceedsMaxValue = errors.New("the duration specified for write_timeout exceeds the maximum allowed value of 10s")
	errRequiredHeader              = errors.New("both key and value are required to assign a required_header")
	errRequireOneScraper           = errors.New("must specify at least one scraper")
	errGitHubHeader                = errors.New("github default headers [X-GitHub-Event, X-GitHub-Delivery, X-GitHub-Hook-ID, X-Hub-Signature-256] cannot be configured")
)

// Validate the configuration passed through the OTEL config.yaml
func (cfg *Config) Validate() error {
	var errs error

	// For now, scrapers are required to be defined in the config. As tracing
	// and other signals are added, this requirement will change.
	if len(cfg.Scrapers) == 0 {
		errs = multierr.Append(errs, errRequireOneScraper)
	}

	maxReadWriteTimeout, _ := time.ParseDuration("10s")

	if cfg.WebHook.Endpoint == "" {
		errs = multierr.Append(errs, errMissingEndpointFromConfig)
	}

	if cfg.WebHook.ReadTimeout > maxReadWriteTimeout {
		errs = multierr.Append(errs, errReadTimeoutExceedsMaxValue)
	}

	if cfg.WebHook.WriteTimeout > maxReadWriteTimeout {
		errs = multierr.Append(errs, errWriteTimeoutExceedsMaxValue)
	}

	for key, value := range cfg.WebHook.RequiredHeaders {
		if key == "" || value == "" {
			errs = multierr.Append(errs, errRequiredHeader)
		}

		if _, exists := cfg.WebHook.GitHubHeaders.Fixed[key]; exists {
			errs = multierr.Append(errs, errGitHubHeader)
		}
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
