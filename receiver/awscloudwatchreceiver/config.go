// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/confmap"
)

var (
	defaultPollInterval  = time.Minute
	defaultEventLimit    = 1000
	defaultLogGroupLimit = 50
)

// Config is the overall config structure for the awscloudwatchreceiver
type Config struct {
	Region       string      `mapstructure:"region"`
	Profile      string      `mapstructure:"profile"`
	IMDSEndpoint string      `mapstructure:"imds_endpoint"`
	Logs         *LogsConfig `mapstructure:"logs"`
}

// LogsConfig is the configuration for the logs portion of this receiver
type LogsConfig struct {
	PollInterval        time.Duration `mapstructure:"poll_interval"`
	MaxEventsPerRequest int           `mapstructure:"max_events_per_request"`
	Groups              GroupConfig   `mapstructure:"groups"`
}

// GroupConfig is the configuration for log group collection
type GroupConfig struct {
	AutodiscoverConfig *AutodiscoverConfig     `mapstructure:"autodiscover,omitempty"`
	NamedConfigs       map[string]StreamConfig `mapstructure:"named"`
}

// AutodiscoverConfig is the configuration for the autodiscovery functionality of log groups
type AutodiscoverConfig struct {
	Prefix  string       `mapstructure:"prefix"`
	Limit   int          `mapstructure:"limit"`
	Streams StreamConfig `mapstructure:"streams"`
}

// StreamConfig represents the configuration for the log stream filtering
type StreamConfig struct {
	Prefixes []*string `mapstructure:"prefixes"`
	Names    []*string `mapstructure:"names"`
}

var (
	errNoRegion                       = errors.New("no region was specified")
	errNoLogsConfigured               = errors.New("no logs configured")
	errInvalidEventLimit              = errors.New("event limit is improperly configured, value must be greater than 0")
	errInvalidPollInterval            = errors.New("poll interval is incorrect, it must be a duration greater than one second")
	errInvalidAutodiscoverLimit       = errors.New("the limit of autodiscovery of log groups is improperly configured, value must be greater than 0")
	errAutodiscoverAndNamedConfigured = errors.New("both autodiscover and named configs are configured, Only one or the other is permitted")
)

// Validate validates all portions of the relevant config
func (c *Config) Validate() error {
	if c.Region == "" {
		return errNoRegion
	}

	if c.IMDSEndpoint != "" {
		_, err := url.ParseRequestURI(c.IMDSEndpoint)
		if err != nil {
			return fmt.Errorf("unable to parse URI for imds_endpoint: %w", err)
		}
	}

	var errs error
	errs = errors.Join(errs, c.validateLogsConfig())
	return errs
}

// Unmarshal is a custom unmarshaller that ensures that autodiscover is nil if
// autodiscover is not specified
func (c *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		return errors.New("")
	}
	err := componentParser.Unmarshal(c, confmap.WithErrorUnused())
	if err != nil {
		return err
	}

	if componentParser.IsSet("logs::groups::named") && !componentParser.IsSet("logs::groups::autodiscover") {
		c.Logs.Groups.AutodiscoverConfig = nil
	}
	return nil
}

func (c *Config) validateLogsConfig() error {
	if c.Logs == nil {
		return errNoLogsConfigured
	}

	if c.Logs.MaxEventsPerRequest <= 0 {
		return errInvalidEventLimit
	}
	if c.Logs.PollInterval < time.Second {
		return errInvalidPollInterval
	}

	return c.Logs.Groups.validate()
}

func (c *GroupConfig) validate() error {
	if c.AutodiscoverConfig != nil && len(c.NamedConfigs) > 0 {
		return errAutodiscoverAndNamedConfigured
	}

	if c.AutodiscoverConfig != nil {
		return validateAutodiscover(*c.AutodiscoverConfig)
	}

	return nil
}

func validateAutodiscover(cfg AutodiscoverConfig) error {
	if cfg.Limit <= 0 {
		return errInvalidAutodiscoverLimit
	}
	return nil
}
