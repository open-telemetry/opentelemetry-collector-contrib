// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricstarttimeprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor"

import (
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/truereset"
	"go.opentelemetry.io/collector/component"
)

// Config holds configuration of the metric start time processor.
type Config struct {
	Strategy   string        `mapstructure:"strategy"`
	GCInterval time.Duration `mapstructure:"gc_interval"`
}

var _ component.Config = (*Config)(nil)

func createDefaultConfig() component.Config {
	return &Config{
		Strategy:   truereset.Type,
		GCInterval: 10 * time.Minute,
	}
}

// Validate checks the configuration is valid
func (cfg *Config) Validate() error {
	switch cfg.Strategy {
	case truereset.Type:
	default:
		return fmt.Errorf("%v is not a valid strategy", cfg.Strategy)
	}
	if cfg.GCInterval <= 0 {
		return fmt.Errorf("gc_interval must be positive")
	}
	return nil
}
