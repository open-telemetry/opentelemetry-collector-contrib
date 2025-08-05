// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apachereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachereceiver"

import (
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachereceiver/internal/metadata"
)

type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	confighttp.ClientConfig        `mapstructure:",squash"`
	MetricsBuilderConfig           metadata.MetricsBuilderConfig `mapstructure:",squash"`

	// prevent unkeyed literal initialization
	_ struct{}
}

var (
	defaultProtocol = "http://"
	defaultHost     = "localhost"
	defaultPort     = "8080"
	defaultPath     = "server-status"
	defaultEndpoint = fmt.Sprintf("%s%s:%s/%s?auto", defaultProtocol, defaultHost, defaultPort, defaultPath)
)

func (cfg *Config) Validate() error {
	u, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint: '%s': %w", cfg.Endpoint, err)
	}

	if u.Hostname() == "" {
		return fmt.Errorf("missing hostname: '%s'", cfg.Endpoint)
	}

	if u.RawQuery != "auto" {
		return fmt.Errorf("query must be 'auto': '%s'", cfg.Endpoint)
	}

	return nil
}
