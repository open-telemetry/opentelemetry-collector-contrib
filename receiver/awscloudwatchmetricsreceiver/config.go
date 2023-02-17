// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awscloudwatchmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchmetricsreceiver"

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"go.uber.org/multierr"
)

var (
	defaultPollInterval = time.Minute
)

// Config is the overall config structure for the awscloudwatchmetricsreceiver
type Config struct {
	Region       string         `mapstructure:"region"`
	Profile      string         `mapstructure:"profile"`
	IMDSEndpoint string         `mapstructure:"imds_endpoint"`
	PollInterval time.Duration  `mapstructure:"poll_interval"`
	Metrics      *MetricsConfig `mapstructure:"metrics"`
}

// MetricsConfig is the configuration for the metrics part of the receiver
type MetricsConfig struct {
	Names []NamesConfig `mapstructure:"named"`
}

// NamesConfig is the configuration for the metric namespace and metric names
// https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/cloudwatch_concepts.html
type NamesConfig struct {
	Namespace   string    `mapstructure:"namespace"`
	MetricNames []*string `mapstructure:"metric_names"`
}

var (
	errNoMetricsConfigured = errors.New("no metrics configured")
	errNoRegion            = errors.New("no region was specified")
	errInvalidPollInterval = errors.New("poll interval is incorrect, it must be a duration greater than one second")
)

func (cfg *Config) Validate() error {
	if cfg.Region == "" {
		return errNoRegion
	}

	if cfg.IMDSEndpoint != "" {
		_, err := url.ParseRequestURI(cfg.IMDSEndpoint)
		if err != nil {
			return fmt.Errorf("unable to parse URI for imds_endpoint: %w", err)
		}
	}

	if cfg.PollInterval < time.Second {
		return errInvalidPollInterval
	}
	var errs error
	errs = multierr.Append(errs, cfg.validateMetricsConfig())
	return errs
}

func (cfg *Config) validateMetricsConfig() error {
	if cfg.Metrics == nil {
		return errNoMetricsConfigured
	}
	return validate(cfg.Metrics.Names)
}

func validate(g []NamesConfig) error {
	for _, metric := range g {
		if metric.Namespace == "" {
			return errNoMetricsConfigured
		}

		if len(metric.MetricNames) <= 0 {
			return errNoMetricsConfigured
		}
	}
	return nil
}
