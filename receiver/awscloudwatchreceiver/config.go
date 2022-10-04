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

package awscloudwatchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/config"
	"go.uber.org/multierr"
)

var (
	defaultPollInterval = 1 * time.Minute
	defaultEventLimit   = int64(1000)
)

// Config is the overall config structure for the awscloudwatchreceiver
type Config struct {
	config.ReceiverSettings `mapstructure:",squash"`
	Region                  string     `mapstructure:"region"`
	Profile                 string     `mapstructure:"profile"`
	Logs                    LogsConfig `mapstructure:"logs"`
}

// LogsConfig is the configuration  for the logs portion of this receiver
type LogsConfig struct {
	PollInterval   time.Duration    `mapstructure:"poll_interval"`
	EventLimit     int64            `mapstructure:"event_limit"`
	LogGroupPrefix string           `mapstructure:"log_group_prefix"`
	LogGroups      []LogGroupConfig `mapstructure:"log_groups"`
}

// LogGroupConfig represents configuration for how to treat a log group
type LogGroupConfig struct {
	Name         string    `mapstructure:"name"`
	StreamPrefix string    `mapstructure:"stream_prefix"`
	LogStreams   []*string `mapstructure:"log_streams"`
}

var (
	errNoRegion            = errors.New("no region was specified")
	errInvalidEventLimit   = errors.New("event limit is improperly configured, value must be greater than 1")
	errInvalidPollInterval = errors.New("poll interval is incorrect, it must be a duration greater than one second")
)

// Validate validates all portions of the relevant config
func (c *Config) Validate() error {
	if c.Region == "" {
		return errNoRegion
	}

	var errs error
	errs = multierr.Append(errs, c.ReceiverSettings.Validate())
	errs = multierr.Append(errs, c.validateLogsConfig())
	return errs
}

func (c *Config) validateLogsConfig() error {
	if c.Logs.EventLimit <= 0 {
		return errInvalidEventLimit
	}
	if c.Logs.PollInterval < time.Second {
		return errInvalidPollInterval
	}
	return nil
}
