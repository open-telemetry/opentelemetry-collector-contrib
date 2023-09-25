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
	errBadOrMissingEndpoint = errors.New("missing a valid endpoint")
	errBadScheme            = errors.New("endpoint scheme must be either http or https")
	errMissingAuthExtension = errors.New("auth extension missing from config")
)

type Config struct {
	confighttp.HTTPClientSettings           `mapstructure:",squash"`
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	metadata.MetricsBuilderConfig           `mapstructure:",squash"`
}

func (cfg *Config) Validate() (errors error) {
	var targetURL *url.URL

	if cfg.Endpoint == "" {
		errors = multierr.Append(errors, errBadOrMissingEndpoint)
	} else {
		// we want to validate that the endpoint url supplied by user is at least
		// a little bit valid
		var err error
		targetURL, err = url.Parse(cfg.Endpoint)
		if err != nil {
			errors = multierr.Append(errors, errBadOrMissingEndpoint)
		}

		if !strings.HasPrefix(targetURL.Scheme, "http") {
			errors = multierr.Append(errors, errBadScheme)
		}
	}

	if cfg.HTTPClientSettings.Auth.AuthenticatorID.Name() == "" {
		errors = multierr.Append(errors, errMissingAuthExtension)
	}

	return errors
}
