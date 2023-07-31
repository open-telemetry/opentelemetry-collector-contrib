// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkenterprisereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver"

import (
	"errors"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver/internal/metadata"
)

var (
	errBadOrMissingEndpoint = errors.New("Missing a valid endpoint")
	errMissingUsername      = errors.New("Missing valid username")
	errMissingPassword      = errors.New("Missing valid password")
	errBadScheme            = errors.New("Endpoint scheme must be either http or https")
)

type Config struct {
	confighttp.HTTPClientSettings           `mapstructure:",squash"`
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	metadata.MetricsBuilderConfig           `mapstructure:",squash"`
	// Username and password with associated with an account with
	// permission to access the Splunk deployments REST api
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	// default is 60s
	MaxSearchWaitTime time.Duration `mapstructure:"max_search_wait_time"`
}

func (cfg *Config) Validate() error {
	var errors error
	var targetUrl *url.URL

	if cfg.Endpoint == "" {
		errors = multierr.Append(errors, errBadOrMissingEndpoint)
	} else {
		// we want to validate that the endpoint url supplied by user is at least
		// a little bit valid
		var err error
		targetUrl, err = url.Parse(cfg.Endpoint)
		if err != nil {
			errors = multierr.Append(errors, errBadOrMissingEndpoint)
		}

		if targetUrl.Scheme != "http" && targetUrl.Scheme != "https" {
			errors = multierr.Append(errors, errBadScheme)
		}
	}

	if cfg.Username == "" {
		errors = multierr.Append(errors, errMissingUsername)
	}

	if cfg.Password == "" {
		errors = multierr.Append(errors, errMissingPassword)
	}

	return errors
}
