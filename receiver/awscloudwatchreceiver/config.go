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
	Region       string `mapstructure:"region"`
	Profile      string `mapstructure:"profile"`
	IMDSEndpoint string `mapstructure:"imds_endpoint"`

	AwsAccountId string `mapstructure:"aws_account_id"`
	AwsRoleArn   string `mapstructure:"aws_role_arn"`
	ExternalId   string `mapstructure:"external_id"`
	AwsAccessKey string `mapstructure:"aws_access_key"`
	AwsSecretKey string `mapstructure:"aws_secret_key"`

	Logs *LogsConfig `mapstructure:"logs"`
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
	Names    []string  `mapstructure:"names"`
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

	if c.AwsRoleArn == "" && c.AwsAccessKey == "" && c.AwsSecretKey == "" {
		return errors.New("no AWS credentials were provided")
	}

	if c.AwsRoleArn != "" && c.AwsAccessKey != "" {
		return errors.New("both role ARN and access keys were provided, only one or the other is permitted")
	}

	if c.AwsRoleArn != "" && c.AwsSecretKey != "" {
		return errors.New("both role ARN and secret key were provided, only one or the other is permitted")
	}

	if (c.AwsAccessKey != "" && c.AwsSecretKey == "") || (c.AwsAccessKey == "" && c.AwsSecretKey != "") {
		return errors.New("only one of access key and secret key was provided, both are required")
	}

	if c.ExternalId == "" && c.AwsRoleArn != "" {
		return errors.New("ExternalId is missing")
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
	err := componentParser.Unmarshal(c)
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
