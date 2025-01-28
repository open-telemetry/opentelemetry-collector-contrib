// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkenterprisereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver"

import (
	"errors"
	"net/url"
	"strings"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver/internal/metadata"
)

var (
	errBadOrMissingEndpoint = errors.New("missing a valid endpoint")
	errBadScheme            = errors.New("endpoint scheme must be either http or https")
	errMissingAuthExtension = errors.New("auth extension missing from config")
)

type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	IdxEndpoint                    confighttp.ClientConfig `mapstructure:"indexer"`
	SHEndpoint                     confighttp.ClientConfig `mapstructure:"search_head"`
	CMEndpoint                     confighttp.ClientConfig `mapstructure:"cluster_master"`
	VersionInfo                    bool                    `mapstructure:"build_version_info"`
}

func (cfg *Config) Validate() (errors error) {
	var targetURL *url.URL
	var err error
	endpoints := []string{}

	// if no endpoint is set we do not start the receiver. For each set endpoint we go through and Validate
	// that it contains an auth setting and a valid endpoint, if its missing either of these the receiver will
	// fail to start.
	if cfg.IdxEndpoint.Endpoint == "" && cfg.SHEndpoint.Endpoint == "" && cfg.CMEndpoint.Endpoint == "" {
		errors = multierr.Append(errors, errBadOrMissingEndpoint)
	} else {
		if cfg.IdxEndpoint.Endpoint != "" {
			if cfg.IdxEndpoint.Auth == nil {
				errors = multierr.Append(errors, errMissingAuthExtension)
			}
			endpoints = append(endpoints, cfg.IdxEndpoint.Endpoint)
		}
		if cfg.SHEndpoint.Endpoint != "" {
			if cfg.SHEndpoint.Auth == nil {
				errors = multierr.Append(errors, errMissingAuthExtension)
			}
			endpoints = append(endpoints, cfg.SHEndpoint.Endpoint)
		}
		if cfg.CMEndpoint.Endpoint != "" {
			if cfg.CMEndpoint.Auth == nil {
				errors = multierr.Append(errors, errMissingAuthExtension)
			}
			endpoints = append(endpoints, cfg.CMEndpoint.Endpoint)
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

	return errors
}
