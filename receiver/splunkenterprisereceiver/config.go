// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkenterprisereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver"

import (
	"errors"
	"net/url"
	"strings"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver/internal/metadata"
)

var (
	errUnspecifiedEndpoint  = errors.New("endpoint to an unspecified node type")
	errBadOrMissingEndpoint = errors.New("missing a valid endpoint")
	errBadScheme            = errors.New("endpoint scheme must be either http or https")
	errMissingAuthExtension = errors.New("auth extension missing from config")
)

type Config struct {
	confighttp.ClientConfig                 `mapstructure:",squash"`
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	metadata.MetricsBuilderConfig           `mapstructure:",squash"`
	IdxEndpoint                             string `mapstructure:"idx_endpoint"`
	SHEndpoint                              string `mapstructure:"sh_endpoint"`
	CMEndpoint                              string `mapstructure:"cm_endpoint"`
}

func (cfg *Config) Validate() (errors error) {
	var targetURL *url.URL
	var err error
	endpoints := []string{}

	if cfg.Endpoint != "" {
		errors = multierr.Append(errors, errUnspecifiedEndpoint)
	} else if cfg.IdxEndpoint == "" && cfg.SHEndpoint == "" && cfg.CMEndpoint == "" {
		errors = multierr.Append(errors, errBadOrMissingEndpoint)
	} else {
		if cfg.IdxEndpoint != "" {
			endpoints = append(endpoints, cfg.IdxEndpoint)
		}
		if cfg.SHEndpoint != "" {
			endpoints = append(endpoints, cfg.SHEndpoint)
		}
		if cfg.CMEndpoint != "" {
			endpoints = append(endpoints, cfg.CMEndpoint)
		}

		for _, e := range endpoints {
			targetURL, err = url.Parse(e)
			if err != nil {
				errors = multierr.Append(errors, errBadOrMissingEndpoint)
				continue
			}

			// note passes for both http and https
			if !strings.HasPrefix(targetURL.Scheme, "http") {
				errors = multierr.Append(errors, errBadScheme)
			}
		}
	}

	if cfg.ClientConfig.Auth.AuthenticatorID.Name() == "" {
		errors = multierr.Append(errors, errMissingAuthExtension)
	}

	return errors
}
