// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkenterprisereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver"

import (
	"errors"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver/internal/metadata"
	"go.uber.org/multierr"
)

var (
    errMissingEndpoint = errors.New("Missing a valid endpoint")
    errMissingUsername = errors.New("Missing valid username")
    errMissingPassword = errors.New("Missing valid password")
    errBadScheme       = errors.New("Endpoing scheme must be either http or https")
)

type Config struct {
    confighttp.HTTPClientSettings `mapstructure:",squash"`
    scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
    metadata.MetricsBuilderConfig `mapstructure:",squash"`
    // Username and password with associated with an account with
    // permission to access the Splunk deployments REST api
    Username          string `mapstructure:"username"`
    Password          string `mapstructure:"password"`
    // default is 60s
    MaxSearchWaitTime time.Duration `mapstructure:"max_search_wait_time"`
}

func (cfg *Config) Validate() error {
    var errors error

    if cfg.Endpoint == "" {
        multierr.Append(errors, errMissingEndpoint)
    }

    if cfg.Username == "" {
        multierr.Append(errors, errMissingUsername)
    }

    if cfg.Password == "" {
        multierr.Append(errors, errMissingPassword)
    }

    // we want to validate that the endpoint url supplied by user is at least
    // a little bit valid
    targetUrl, err := url.Parse(cfg.Endpoint)
    if err == nil {
        multierr.Append(errors, err)
    } 

    if (targetUrl.Scheme != "http" || targetUrl.Scheme != "https") {
        multierr.Append(errors, errBadScheme)
    }

    return errors
}
