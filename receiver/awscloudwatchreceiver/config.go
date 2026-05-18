// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

var (
	defaultPollInterval         = time.Minute
	defaultEventLimit           = 1000
	defaultLogGroupLimit        = 50
	defaultMetricsPeriod        = 300 * time.Second
	defaultMetricsCollectionInt = 5 * time.Minute
	defaultMetricsDiscoverLimit = 100
	defaultMetricsDelay         = 10 * time.Minute
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
	// Period is the CloudWatch statistics aggregation window. Defaults to 5m.
	Period time.Duration `mapstructure:"period"`
	// Delay shifts the query window back from now to account for CloudWatch metric publication latency.
	// CloudWatch metrics are typically available within 3–10 minutes of being recorded.
	// Defaults to 10m.
	Delay     time.Duration           `mapstructure:"delay"`
	Queries   []MetricQuery           `mapstructure:"queries"`
	Discovery *MetricsDiscoveryConfig `mapstructure:"discovery"`
}

// MetricsDiscoveryConfig configures automatic discovery of metrics via ListMetrics.
// Discovered metrics are then scraped with GetMetricData. Mutually exclusive with metrics (explicit list).
type MetricsDiscoveryConfig struct {
	Filters configoptional.Optional[MetricsDiscoveryFilters] `mapstructure:"filters"`
	Limit   int                                              `mapstructure:"limit"` // max metrics to discover and scrape (default 100)
	// Stats selects which CloudWatch statistics to fetch for all discovered metrics.
	// Same semantics as MetricQuery.Stats.
	Stats []string `mapstructure:"stats"`
}

// MetricsDiscoveryFilters optionally narrows which metrics are discovered.
// When absent, all metrics in all namespaces are discovered.
type MetricsDiscoveryFilters struct {
	Namespace  string `mapstructure:"namespace"`
	MetricName string `mapstructure:"metric_name"`
}

// MetricQuery defines a single CloudWatch metric to scrape via GetMetricData.
// When Stats is empty all four statistics (Sum, SampleCount, Minimum, Maximum) are fetched and
// combined into an OpenTelemetry Summary metric aligned with the CloudWatch Metric Streams
// OpenTelemetry 1.0.0 format.
// When Stats is set, each named statistic is fetched and emitted as a separate Gauge data point
// on the same metric, with a "stat" attribute identifying the statistic.
// CloudWatch statistic names: Sum, Average, Minimum, Maximum, SampleCount, p99, p95, etc.
type MetricQuery struct {
	Namespace  string            `mapstructure:"namespace"`
	MetricName string            `mapstructure:"metric_name"`
	Dimensions map[string]string `mapstructure:"dimensions"`
	Stats      []string          `mapstructure:"stats"`
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
	errNoRegion                         = errors.New("no region was specified")
	errInvalidEventLimit                = errors.New("event limit is improperly configured, value must be greater than 0")
	errInvalidPollInterval              = errors.New("poll interval is incorrect, it must be a duration greater than one second")
	errInvalidAutodiscoverLimit         = errors.New("the limit of autodiscovery of log groups is improperly configured, value must be greater than 0")
	errAutodiscoverAndNamedConfigured   = errors.New("both autodiscover and named configs are configured, Only one or the other is permitted")
	errPrefixAndPatternConfigured       = errors.New("cannot specify both prefix and pattern")
	errInvalidMetricsPeriod             = errors.New("metrics period must be at least 1 second")
	errInvalidMetricsDelay              = errors.New("metrics delay must be at least 1 second")
	errInvalidMetricsCollectionInterval = errors.New("metrics collection_interval must be at least 1 second")
	errMetricMissingNamespace           = errors.New("metric must have namespace")
	errMetricMissingName                = errors.New("metric must have metric_name")
	errMetricsAndDiscoveryConfigured    = errors.New("metrics and discovery are mutually exclusive; set one or the other")
	errInvalidDiscoveryLimit            = errors.New("metrics discovery limit must be greater than 0")
	errEmptyStatName                    = errors.New("stat name must not be empty")
	errCollectionIntervalLessThanPeriod = errors.New("metrics collection_interval must be greater than or equal to period")
)

// Validate overrides the embedded ControllerConfig.Validate so that a zero CollectionInterval
// does not cause a validation error when metrics collection is not configured. The scraper
// framework requires a positive CollectionInterval only when the metrics controller is started.
func (m *MetricsConfig) Validate() error {
	if len(m.Queries) == 0 && m.Discovery == nil {
		return nil
	}
	return m.ControllerConfig.Validate()
}

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

func (c *Config) validateMetricsDurations() error {
	if c.Metrics.CollectionInterval != 0 && c.Metrics.CollectionInterval < time.Second {
		return errInvalidMetricsCollectionInterval
	}
	if c.Metrics.Period != 0 && c.Metrics.Period < time.Second {
		return errInvalidMetricsPeriod
	}
	if c.Metrics.Delay != 0 && c.Metrics.Delay < time.Second {
		return errInvalidMetricsDelay
	}
	if c.Metrics.CollectionInterval > 0 && c.Metrics.Period > 0 &&
		c.Metrics.CollectionInterval < c.Metrics.Period {
		return errCollectionIntervalLessThanPeriod
	}
	return nil
}

func (c *Config) validateMetricsConfig() error {
	if c.Metrics.Discovery != nil && len(c.Metrics.Queries) > 0 {
		return errMetricsAndDiscoveryConfigured
	}
	if discovery := c.Metrics.Discovery; discovery != nil {
		if discovery.Limit <= 0 {
			return errInvalidDiscoveryLimit
		}
		for j, st := range discovery.Stats {
			if st == "" {
				return fmt.Errorf("metrics.discovery.stats[%d]: %w", j, errEmptyStatName)
			}
		}
		return c.validateMetricsDurations()
	}
	if len(c.Metrics.Queries) == 0 {
		return nil
	}
	if err := c.validateMetricsDurations(); err != nil {
		return err
	}
	for i, m := range c.Metrics.Queries {
		if m.Namespace == "" {
			return fmt.Errorf("metrics[%d]: %w", i, errMetricMissingNamespace)
		}
		if m.MetricName == "" {
			return fmt.Errorf("metrics[%d]: %w", i, errMetricMissingName)
		}
		for j, st := range m.Stats {
			if st == "" {
				return fmt.Errorf("metrics[%d].stats[%d]: %w", i, j, errEmptyStatName)
			}
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
