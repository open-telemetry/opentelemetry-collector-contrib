// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricstarttimeprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor"

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/starttimemetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/subtractinitial"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/truereset"
)

// Config holds configuration of the metric start time processor.
type Config struct {
	Strategy   string        `mapstructure:"strategy"`
	GCInterval time.Duration `mapstructure:"gc_interval"`
	// StartTimeMetricRegex only applies then the start_time_metric strategy is used
	StartTimeMetricRegex string `mapstructure:"start_time_metric_regex"`
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
	case subtractinitial.Type:
	case starttimemetric.Type:
	default:
		return fmt.Errorf("%q is not a valid strategy", cfg.Strategy)
	}
	if cfg.GCInterval <= 0 {
		return errors.New("gc_interval must be positive")
	}
	if cfg.StartTimeMetricRegex != "" {
		if _, err := regexp.Compile(cfg.StartTimeMetricRegex); err != nil {
			return err
		}
		if cfg.Strategy != starttimemetric.Type {
			return errors.New("start_time_metric_regex can only be used with the start_time_metric strategy")
		}
	}
	return nil
}
