// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

var (
	defaultPollInterval          = time.Minute
	defaultEventLimit            = 1000
	defaultLogGroupLimit         = 50
	defaultMetricsPeriod         = 300 * time.Second
	defaultMetricsStat           = "Average"
	defaultMetricsCollectionInt  = 5 * time.Minute
	defaultMetricsDiscoverLimit  = 100
)

// Config is the overall config structure for the awscloudwatchreceiver
type Config struct {
	Region       string        `mapstructure:"region"`
	Profile      string        `mapstructure:"profile"`
	IMDSEndpoint string        `mapstructure:"imds_endpoint"`
	Logs         LogsConfig    `mapstructure:"logs"`
	Metrics      MetricsConfig `mapstructure:"metrics"`
	StorageID    *component.ID `mapstructure:"storage"`
}

// MetricsConfig is the configuration for the metrics (GetMetricData) portion of this receiver.
// Either Metrics (explicit list) or Discovery (ListMetrics-based) may be set, not both.
// Collection interval and scraper behavior are controlled via the embedded ControllerConfig.
type MetricsConfig struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	Period                         time.Duration             `mapstructure:"period"`
	Metrics                        []MetricQuery             `mapstructure:"metrics"`
	Discovery                      *MetricsDiscoveryConfig  `mapstructure:"discovery,omitempty"`
}

// MetricsDiscoveryConfig configures automatic discovery of metrics via ListMetrics.
// Discovered metrics are then scraped with GetMetricData. Mutually exclusive with metrics (explicit list).
type MetricsDiscoveryConfig struct {
	Namespace  string `mapstructure:"namespace"`   // optional filter, e.g. "AWS/EC2"
	MetricName string `mapstructure:"metric_name"` // optional filter
	Limit      int    `mapstructure:"limit"`       // max metrics to discover and scrape (default 100)
	Stat       string `mapstructure:"stat"`        // e.g. Average, Sum; default Average
}

// MetricQuery defines a single CloudWatch metric to scrape via GetMetricData.
type MetricQuery struct {
	Namespace  string            `mapstructure:"namespace"`
	MetricName string            `mapstructure:"metric_name"`
	Dimensions map[string]string `mapstructure:"dimensions"`
	Stat       string            `mapstructure:"stat"` // e.g. Average, Sum, Maximum, Minimum; default Average
}

// LogsConfig is the configuration for the logs portion of this receiver
type LogsConfig struct {
	StartFrom           string        `mapstructure:"start_from"`
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
	Prefix                string       `mapstructure:"prefix"`
	Pattern               string       `mapstructure:"pattern"`
	Limit                 int          `mapstructure:"limit"`
	Streams               StreamConfig `mapstructure:"streams"`
	AccountIdentifiers    []string     `mapstructure:"account_identifiers"`
	IncludeLinkedAccounts *bool        `mapstructure:"include_linked_accounts"`
}

// StreamConfig represents the configuration for the log stream filtering
type StreamConfig struct {
	Prefixes []*string `mapstructure:"prefixes"`
	Names    []*string `mapstructure:"names"`
}

var (
	errNoRegion                       = errors.New("no region was specified")
	errInvalidEventLimit              = errors.New("event limit is improperly configured, value must be greater than 0")
	errInvalidPollInterval            = errors.New("poll interval is incorrect, it must be a duration greater than one second")
	errInvalidAutodiscoverLimit       = errors.New("the limit of autodiscovery of log groups is improperly configured, value must be greater than 0")
	errAutodiscoverAndNamedConfigured = errors.New("both autodiscover and named configs are configured, Only one or the other is permitted")
	errPrefixAndPatternConfigured     = errors.New("cannot specify both prefix and pattern")
	errInvalidMetricsPeriod           = errors.New("metrics period must be at least 1 second")
	errInvalidMetricsCollectionInterval = errors.New("metrics collection_interval must be at least 1 second")
	errMetricMissingNamespace         = errors.New("metric must have namespace")
	errMetricMissingName              = errors.New("metric must have metric_name")
	errMetricsAndDiscoveryConfigured  = errors.New("metrics and discovery are mutually exclusive; set one or the other")
	errInvalidDiscoveryLimit          = errors.New("metrics discovery limit must be greater than 0")
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
	errs = errors.Join(errs, c.validateMetricsConfig())
	return errs
}

func (c *Config) validateMetricsConfig() error {
	if c.Metrics.Discovery != nil && len(c.Metrics.Metrics) > 0 {
		return errMetricsAndDiscoveryConfigured
	}
	if c.Metrics.Discovery != nil {
		if c.Metrics.Discovery.Limit <= 0 {
			return errInvalidDiscoveryLimit
		}
		if c.Metrics.ControllerConfig.CollectionInterval != 0 && c.Metrics.ControllerConfig.CollectionInterval < time.Second {
			return errInvalidMetricsCollectionInterval
		}
		if c.Metrics.Period != 0 && c.Metrics.Period < time.Second {
			return errInvalidMetricsPeriod
		}
		return nil
	}
	if len(c.Metrics.Metrics) == 0 {
		return nil
	}
	if c.Metrics.ControllerConfig.CollectionInterval != 0 && c.Metrics.ControllerConfig.CollectionInterval < time.Second {
		return errInvalidMetricsCollectionInterval
	}
	if c.Metrics.Period < time.Second {
		return errInvalidMetricsPeriod
	}
	for i, m := range c.Metrics.Metrics {
		if m.Namespace == "" {
			return fmt.Errorf("metrics[%d]: %w", i, errMetricMissingNamespace)
		}
		if m.MetricName == "" {
			return fmt.Errorf("metrics[%d]: %w", i, errMetricMissingName)
		}
	}
	return nil
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
	if c.Logs.StartFrom != "" {
		_, err := time.Parse(time.RFC3339, c.Logs.StartFrom)
		if err != nil {
			return fmt.Errorf("invalid start_from time format: %w", err)
		}
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
	if cfg.Pattern != "" && cfg.Prefix != "" {
		return errPrefixAndPatternConfigured
	}
	return nil
}
