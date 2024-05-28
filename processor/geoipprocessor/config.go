// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package geoipprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor"

import (
	"errors"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider"
)

// Config holds the configuration for the GeoIP processor.
type Config struct {
	// Providers specifies the sources to extract geographical information about a given IP.
	Providers map[string]provider.Config `mapstructure:"-"`
}

var _ component.Config = (*Config)(nil)

func (cfg *Config) Validate() error {
	if len(cfg.Providers) == 0 {
		return errors.New("must specify at least one geo IP data provider when using the geoip processor")
	}
	return nil
}
