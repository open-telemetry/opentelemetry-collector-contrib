// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gitlabreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.uber.org/multierr"
)

const (
	defaultReadTimeout  = 500 * time.Millisecond
	defaultWriteTimeout = 500 * time.Millisecond

	defaultEndpoint = "localhost:8080"

	defaultPath       = "/events"
	defaultHealthPath = "/health"
)

const (
	// GitLab default headers: https://docs.gitlab.com/ee/user/project/integrations/webhooks.html#delivery-headers
	defaultUserAgentHeader         = "User-Agent"
	defaultGitlabInstanceHeader    = "X-Gitlab-Instance"
	defaultGitlabWebhookUUIDHeader = "X-Gitlab-Webhook-UUID"
	defaultGitlabEventHeader       = "X-Gitlab-Event"
	defaultGitlabEventUUIDHeader   = "X-Gitlab-Event-UUID"
	defaultIdempotencyKeyHeader    = "Idempotency-Key"
)

var (
	errReadTimeoutExceedsMaxValue  = errors.New("the duration specified for read_timeout exceeds the maximum allowed value of 10s")
	errWriteTimeoutExceedsMaxValue = errors.New("the duration specified for write_timeout exceeds the maximum allowed value of 10s")
	errRequiredHeader              = errors.New("both key and value are required to assign a required_header")
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
	GitlabHeaders   []string                       `mapstructure:"gitlab_headers"`   // optional setting to overwrite the by default required GitLab headers for all requests to have (except the health check)

	Secret string `mapstructure:"secret"` // secret for webhook
}

func createDefaultConfig() component.Config {
	return &Config{
		WebHook: WebHook{
			ServerConfig: confighttp.ServerConfig{
				Endpoint:     defaultEndpoint,
				ReadTimeout:  defaultReadTimeout,
				WriteTimeout: defaultWriteTimeout,
			},
			GitlabHeaders: []string{
				defaultUserAgentHeader,
				defaultGitlabInstanceHeader,
				defaultGitlabWebhookUUIDHeader,
				defaultGitlabEventHeader,
				defaultGitlabEventUUIDHeader,
				defaultIdempotencyKeyHeader,
			},
			Path:       defaultPath,
			HealthPath: defaultHealthPath,
		},
	}
}

func (cfg *Config) Validate() error {
	var errs error

	maxReadWriteTimeout, _ := time.ParseDuration("10s")

	if cfg.WebHook.ServerConfig.ReadTimeout > maxReadWriteTimeout {
		errs = multierr.Append(errs, errReadTimeoutExceedsMaxValue)
	}

	if cfg.WebHook.ServerConfig.WriteTimeout > maxReadWriteTimeout {
		errs = multierr.Append(errs, errWriteTimeoutExceedsMaxValue)
	}

	for key, value := range cfg.WebHook.RequiredHeaders {
		if key == "" || value == "" {
			errs = multierr.Append(errs, errRequiredHeader)
		}
	}

	return errs
}
