// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/traces"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/webhookeventreceiver"
)

var errMissingEndpointFromConfig = errors.New("missing receiver server endpoint from config")

// Config defines configuration for GitHub Actions receiver
type Config struct {
	WebhookReceiver   webhookeventreceiver.Config `mapstructure:",squash"`             // squash ensures fields are correctly decoded in embedded struct
	Secret            string                      `mapstructure:"secret"`              // GitHub traces signature. Default is empty
	CustomServiceName string                      `mapstructure:"custom_service_name"` // custom service name. Default is empty
	ServiceNamePrefix string                      `mapstructure:"service_name_prefix"` // service name prefix. Default is empty
	ServiceNameSuffix string                      `mapstructure:"service_name_suffix"` // service name suffix. Default is empty
}

var _ component.Config = (*Config)(nil)

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	var errs error

	// Ignore tracing configuration when endpoint is not set
	if cfg.WebhookReceiver.ServerConfig.Endpoint != "" {
		if webhookErr := cfg.WebhookReceiver.Validate(); webhookErr != nil {
			errs = multierr.Append(errs, webhookErr)
		}
	}

	return errs
}
