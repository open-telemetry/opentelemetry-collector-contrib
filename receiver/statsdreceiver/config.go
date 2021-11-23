// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package statsdreceiver

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/protocol"
)

// Config defines configuration for StatsD receiver.
type Config struct {
	config.ReceiverSettings `mapstructure:",squash"`
	NetAddr                 confignet.NetAddr                `mapstructure:",squash"`
	AggregationInterval     time.Duration                    `mapstructure:"aggregation_interval"`
	EnableMetricType        bool                             `mapstructure:"enable_metric_type"`
	IsMonotonicCounter      bool                             `mapstructure:"is_monotonic_counter"`
	TimerHistogramMapping   []protocol.TimerHistogramMapping `mapstructure:"timer_histogram_mapping"`
}

func (c *Config) validate() error {

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
		case protocol.TimingTypeName, protocol.TimingAltTypeName, protocol.HistogramTypeName:
		default:
			errs = multierr.Append(errs, fmt.Errorf("statsd_type is not a supported mapping: %s", eachMap.StatsdType))
		}

		if eachMap.ObserverType == "" {
			TimerHistogramMappingMissingObjectName = true
			break
		}

		switch eachMap.ObserverType {
		case protocol.GaugeObserver, protocol.SummaryObserver:
		default:
			errs = multierr.Append(errs, fmt.Errorf("observer_type is not supported: %s", eachMap.ObserverType))
		}
	}

	if TimerHistogramMappingMissingObjectName {
		errs = multierr.Append(errs, fmt.Errorf("must specify object id for all TimerHistogramMappings"))
	}

	return errs
}
