// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gitlabreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/confmap"
	"go.uber.org/multierr"
)

const (
	defaultReadTimeout  = 500 * time.Millisecond
	defaultWriteTimeout = 500 * time.Millisecond

	defaultEndpoint = "localhost:8080"

	defaultPath       = "/events"
	defaultHealthPath = "/health"

	// GitLab default headers: https://docs.gitlab.com/ee/user/project/integrations/webhooks.html#delivery-headers
	defaultUserAgentHeader         = "User-Agent"
	defaultGitLabInstanceHeader    = "X-Gitlab-Instance"
	defaultGitLabWebhookUUIDHeader = "X-Gitlab-Webhook-UUID"
	defaultGitLabEventHeader       = "X-Gitlab-Event"
	defaultGitLabEventUUIDHeader   = "X-Gitlab-Event-UUID"
	defaultIdempotencyKeyHeader    = "Idempotency-Key"
	// #nosec G101 - Not an actual secret, just the name of a header: https://docs.gitlab.com/user/project/integrations/webhooks/#create-a-webhook
	defaultGitLabSecretTokenHeader = "X-Gitlab-Token"
)

var (
	errReadTimeoutExceedsMaxValue  = errors.New("the duration specified for read_timeout exceeds the maximum allowed value of 10s")
	errWriteTimeoutExceedsMaxValue = errors.New("the duration specified for write_timeout exceeds the maximum allowed value of 10s")
	errRequiredHeader              = errors.New("both key and value are required to assign a required_header")
	errGitlabHeader                = errors.New("gitlab default headers [X-Gitlab-Webhook-UUID, X-Gitlab-Event, X-Gitlab-Event-UUID, Idempotency-Key] cannot be configured")
	errConfigNotValid              = errors.New("configuration is not valid for the gitlab receiver")
)

// Config that is exposed to this gitlab receiver through the OTEL config.yaml
type Config struct {
	WebHook WebHook `mapstructure:"webhook"`
}

type WebHook struct {
	confighttp.ServerConfig `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct

	Path       string `mapstructure:"path"`        // path for data collection. default is /events
	HealthPath string `mapstructure:"health_path"` // path for health check api. default is /health_check

	RequiredHeaders map[string]configopaque.String `mapstructure:"required_headers"` // optional setting to set one or more required headers for all requests to have (except the health check)
	GitlabHeaders   GitlabHeaders                  `mapstructure:",squash"`          // GitLab headers set by default

	Secret string `mapstructure:"secret"` // secret for webhook
}

type GitlabHeaders struct {
	Customizable map[string]string `mapstructure:","` // can be overwritten via required_headers
	Fixed        map[string]string `mapstructure:","` // are not allowed to be overwritten
}

func createDefaultConfig() component.Config {
	return &Config{
		WebHook: WebHook{
			ServerConfig: confighttp.ServerConfig{
				Endpoint:     defaultEndpoint,
				ReadTimeout:  defaultReadTimeout,
				WriteTimeout: defaultWriteTimeout,
			},
			GitlabHeaders: GitlabHeaders{
				Customizable: map[string]string{
					defaultUserAgentHeader:      "",
					defaultGitLabInstanceHeader: "https://gitlab.com",
				},
				Fixed: map[string]string{
					defaultGitLabWebhookUUIDHeader: "",
					defaultGitLabEventHeader:       "Pipeline Hook",
					defaultGitLabEventUUIDHeader:   "",
					defaultIdempotencyKeyHeader:    "",
				},
			},
			Path:       defaultPath,
			HealthPath: defaultHealthPath,
		},
	}
}

func (cfg *Config) Validate() error {
	var errs error

	maxReadWriteTimeout, _ := time.ParseDuration("10s")

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

		if _, exists := cfg.WebHook.GitlabHeaders.Fixed[key]; exists {
			errs = multierr.Append(errs, errGitlabHeader)
		}
	}

	return errs
}

func (cfg *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		return nil
	}

	// load the non-dynamic config normally
	err := componentParser.Unmarshal(cfg, confmap.WithIgnoreUnused())
	if err != nil {
		return err
	}

	// overwrite customizable GitLab default headers if configured within the required_headers
	for key, header := range cfg.WebHook.RequiredHeaders {
		if _, exists := cfg.WebHook.GitlabHeaders.Customizable[key]; exists {
			cfg.WebHook.GitlabHeaders.Customizable[key] = string(header)
		}
	}

	return nil
}
