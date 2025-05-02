// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apachesparkreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver"

import (
	"errors"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver/internal/metadata"
)

const (
	defaultCollectionInterval = time.Minute
	defaultEndpoint           = "http://localhost:4040"
)

var errInvalidEndpoint = errors.New("'endpoint' must be in the form of <scheme>://<hostname>:<port>")

// Config defines the configuration for the various elements of the receiver agent.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	confighttp.ClientConfig        `mapstructure:",squash"`
	ApplicationNames               []string `mapstructure:"application_names"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// Validate validates missing and invalid configuration fields.
func (cfg *Config) Validate() error {
	_, parseErr := url.Parse(cfg.Endpoint)
	if parseErr != nil {
		return errInvalidEndpoint
	}
	return nil
}
