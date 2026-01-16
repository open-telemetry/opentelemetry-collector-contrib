// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statsdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver"

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/lightstep/go-expohisto/structure"
	"go.opentelemetry.io/collector/config/confignet"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/protocol"
)

// Config defines configuration for StatsD receiver.
type Config struct {
	NetAddr                 confignet.AddrConfig             `mapstructure:",squash"`
	AggregationInterval     time.Duration                    `mapstructure:"aggregation_interval"`
	EnableIPOnlyAggregation bool                             `mapstructure:"enable_ip_only_aggregation"`
	EnableMetricType        bool                             `mapstructure:"enable_metric_type"`
	EnableSimpleTags        bool                             `mapstructure:"enable_simple_tags"`
	IsMonotonicCounter      bool                             `mapstructure:"is_monotonic_counter"`
	TimerHistogramMapping   []protocol.TimerHistogramMapping `mapstructure:"timer_histogram_mapping"`
	// Will only be used when transport set to 'unixgram'.
	SocketPermissions os.FileMode `mapstructure:"socket_permissions"`
}

func (c *Config) Validate() error {
	var errs error

	if c.AggregationInterval <= 0 {
		errs = multierr.Append(errs, errors.New("aggregation_interval must be a positive duration"))
	}

	var TimerHistogramMappingMissingObjectName bool
	for _, eachMap := range c.TimerHistogramMapping {
		if eachMap.StatsdType == "" {
			TimerHistogramMappingMissingObjectName = true
			break
		}

		switch eachMap.StatsdType {
		case protocol.TimingTypeName, protocol.TimingAltTypeName, protocol.HistogramTypeName, protocol.DistributionTypeName:
			// do nothing
		default:
			errs = multierr.Append(errs, fmt.Errorf("statsd_type is not a supported mapping for histogram and timing metrics: %s", eachMap.StatsdType))
		}

		if eachMap.ObserverType == "" {
			TimerHistogramMappingMissingObjectName = true
			break
		}

		switch eachMap.ObserverType {
		case protocol.GaugeObserver, protocol.SummaryObserver, protocol.HistogramObserver:
			// do nothing
		default:
			errs = multierr.Append(errs, fmt.Errorf("observer_type is not supported for histogram and timing metrics: %s", eachMap.ObserverType))
		}

		if eachMap.ObserverType == protocol.HistogramObserver {
			if eachMap.Histogram.MaxSize != 0 && (eachMap.Histogram.MaxSize < structure.MinSize || eachMap.Histogram.MaxSize > structure.MaximumMaxSize) {
				errs = multierr.Append(errs, fmt.Errorf("histogram max_size out of range: %v", eachMap.Histogram.MaxSize))
			}

			if eachMap.Histogram.ExplicitBuckets != nil {
				if err := c.validateExplicitBuckets(eachMap.Histogram.ExplicitBuckets); err != nil {
					errs = multierr.Append(errs, err)
				}
			}
		} else if eachMap.ObserverType != protocol.HistogramObserver {
			// Non-histogram observer w/ histogram config
			if eachMap.Histogram.MaxSize != 0 || eachMap.Histogram.ExplicitBuckets != nil {
				errs = multierr.Append(errs, errors.New("histogram configuration requires observer_type: histogram"))
			}
		}
		if len(eachMap.Summary.Percentiles) != 0 {
			for _, percentile := range eachMap.Summary.Percentiles {
				if percentile > 100 || percentile < 0 {
					errs = multierr.Append(errs, fmt.Errorf("summary percentiles out of [0, 100] range: %v", percentile))
				}
			}
			if eachMap.ObserverType != protocol.SummaryObserver {
				errs = multierr.Append(errs, errors.New("summary configuration requires observer_type: summary"))
			}
		}
	}

	if TimerHistogramMappingMissingObjectName {
		errs = multierr.Append(errs, errors.New("must specify object id for all TimerHistogramMappings"))
	}

	return errs
}

func (*Config) validateExplicitBuckets(explicitBuckets []protocol.ExplicitBucket) error {
	var errs error
	for i, eb := range explicitBuckets {
		if eb.MatcherPattern == "" {
			errs = multierr.Append(errs, fmt.Errorf("explicit bucket [%d] matcher_pattern must not be empty", i))
		}
		if _, err := regexp.Compile(eb.MatcherPattern); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("explicit bucket [%d] matcher_pattern is not a valid regular expression: %w", i, err))
		}
		if len(eb.Buckets) == 0 {
			return multierr.Append(errs, fmt.Errorf("explicit bucket [%d] buckets must not be empty", i))
		}
		for j := 0; j < len(eb.Buckets)-1; j++ {
			if eb.Buckets[j] > eb.Buckets[j+1] {
				errs = multierr.Append(errs, fmt.Errorf("explicit bucket [%d] buckets are not unique or not ascendingly sorted %+v", i, eb.Buckets))
				break
			}
		}
	}
	return errs
}
