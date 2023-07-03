// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statsdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver"

import (
	"fmt"
	"time"

	"github.com/lightstep/go-expohisto/structure"
	"go.opentelemetry.io/collector/config/confignet"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/protocol"
)

// Config defines configuration for StatsD receiver.
type Config struct {
	NetAddr               confignet.NetAddr                `mapstructure:",squash"`
	AggregationInterval   time.Duration                    `mapstructure:"aggregation_interval"`
	EnableMetricType      bool                             `mapstructure:"enable_metric_type"`
	IsMonotonicCounter    bool                             `mapstructure:"is_monotonic_counter"`
	TimerHistogramMapping []protocol.TimerHistogramMapping `mapstructure:"timer_histogram_mapping"`
}

func (c *Config) Validate() error {
	var errs error

	if c.AggregationInterval <= 0 {
		errs = multierr.Append(errs, fmt.Errorf("aggregation_interval must be a positive duration"))
	}

	var TimerHistogramMappingMissingObjectName bool
	for _, eachMap := range c.TimerHistogramMapping {

		if eachMap.StatsdType == "" {
			TimerHistogramMappingMissingObjectName = true
			break
		}

		switch eachMap.StatsdType {
		case protocol.TimingTypeName, protocol.TimingAltTypeName, protocol.HistogramTypeName, protocol.CounterTypeName, protocol.GaugeTypeName:
		default:
			errs = multierr.Append(errs, fmt.Errorf("statsd_type is not a supported mapping: %s", eachMap.StatsdType))
		}

		if eachMap.ObserverType == "" {
			TimerHistogramMappingMissingObjectName = true
			break
		}

		switch eachMap.ObserverType {
		case protocol.GaugeObserver, protocol.SummaryObserver, protocol.HistogramObserver, protocol.DefaultObserverType:
		default:
			errs = multierr.Append(errs, fmt.Errorf("observer_type is not supported: %s", eachMap.ObserverType))
		}

		if eachMap.ObserverType == protocol.HistogramObserver {
			if eachMap.Histogram.MaxSize != 0 && (eachMap.Histogram.MaxSize < structure.MinSize || eachMap.Histogram.MaxSize > structure.MaximumMaxSize) {
				errs = multierr.Append(errs, fmt.Errorf("histogram max_size out of range: %v", eachMap.Histogram.MaxSize))
			}
		} else {
			// Non-histogram observer w/ histogram config
			var empty protocol.HistogramConfig
			if eachMap.Histogram != empty {
				errs = multierr.Append(errs, fmt.Errorf("histogram configuration requires observer_type: histogram"))
			}
		}
	}

	if TimerHistogramMappingMissingObjectName {
		errs = multierr.Append(errs, fmt.Errorf("must specify object id for all TimerHistogramMappings"))
	}

	return errs
}
