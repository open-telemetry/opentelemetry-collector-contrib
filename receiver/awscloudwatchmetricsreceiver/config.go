// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchmetricsreceiver"

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/collector/confmap"
)

var (
	defaultPollInterval = 5 * time.Minute
)

// Config is the overall config structure for the awscloudwatchmetricsreceiver
type Config struct {
	Region          string        `mapstructure:"region"`
	IMDSEndpoint    string        `mapstructure:"imds_endpoint"`
	PollInterval    time.Duration `mapstructure:"poll_interval"`
	PollingApproach string        `mapstructure:"polling_approach"`
	Profile         string        `mapstructure:"profile"`
	AwsAccountId    string        `mapstructure:"aws_account_id"`
	AwsRoleArn      string        `mapstructure:"aws_role_arn"`
	ExternalId      string        `mapstructure:"external_id"`
	AwsAccessKey    string        `mapstructure:"aws_access_key"`
	AwsSecretKey    string        `mapstructure:"aws_secret_key"`

	Metrics *MetricsConfig `mapstructure:"metrics"`
}

// MetricsConfig is the configuration for the metrics part of the receiver
// this is so we could expand to other inputs such as autodiscover
type MetricsConfig struct {
	Group        []GroupConfig       `mapstructure:"group"`
	AutoDiscover *AutoDiscoverConfig `mapstructure:"autodiscover,omitempty"`
}

type GroupConfig struct {
	Namespace  string        `mapstructure:"namespace"`
	Period     time.Duration `mapstructure:"period"`
	MetricName []NamedConfig `mapstructure:"name"`
}

// NamesConfig is the configuration for the metric namespace and metric names
// https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/cloudwatch_concepts.html
type NamedConfig struct {
	MetricName     string                   `mapstructure:"metric_name"`
	AwsAggregation string                   `mapstructure:"aws_aggregation"`
	Dimensions     []MetricDimensionsConfig `mapstructure:"dimensions"`
}

type AutoDiscoverConfig struct {
	Namespace         string                   `mapstructure:"namespace"`
	Limit             int                      `mapstructure:"limit"`
	AwsAggregation    string                   `mapstructure:"aws_aggregation"`
	Period            time.Duration            `mapstructure:"period"`
	DefaultDimensions []MetricDimensionsConfig `mapstructure:"dimensions"`
}

// MetricDimensionConfig is the configuration for the metric dimensions
type MetricDimensionsConfig struct {
	Name  string `mapstructure:"Name"`
	Value string `mapstructure:"Value"`
}

var (
	errNoMetricsConfigured            = errors.New("no named metrics configured")
	errNoMetricNameConfigured         = errors.New("metric name was empty")
	errNoNamespaceConfigured          = errors.New("no metric namespace configured")
	errNoRegion                       = errors.New("no region was specified")
	errInvalidPollInterval            = errors.New("poll interval is incorrect, it must be a duration greater than one second")
	errInvalidAutodiscoverLimit       = errors.New("the limit of autodiscovery of log groups is improperly configured, value must be greater than 0")
	errAutodiscoverAndNamedConfigured = errors.New("both autodiscover and named configs are configured, only one or the other is permitted")

	// https://docs.aws.amazon.com/cli/latest/reference/cloudwatch/get-metric-data.html
	errEmptyDimensions       = errors.New("dimensions name and value is empty")
	errTooManyDimensions     = errors.New("you cannot define more than 30 dimensions for a metric")
	errDimensionColonPrefix  = errors.New("dimension name cannot start with a colon")
	errInvalidAwsAggregation = errors.New("invalid AWS aggregation")
)

func (cfg *Config) Validate() error {
	if cfg.Metrics == nil {
		return errNoMetricsConfigured
	}

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
	return cfg.validateMetricsConfig()
}

// Unmarshal is a custom unmarshaller that ensures that autodiscover is nil if
// autodiscover is not specified
// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/awscloudwatchreceiver/config.go
func (cfg *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		return errors.New("")
	}
	err := componentParser.Unmarshal(cfg, confmap.WithErrorUnused())
	if err != nil {
		return err
	}

	if componentParser.IsSet("metrics::group") && !componentParser.IsSet("metrics::autodiscover") {
		cfg.Metrics.AutoDiscover = nil
		return nil
	}

	if componentParser.IsSet("metrics::autodiscover") && !componentParser.IsSet("metrics::group") {
		cfg.Metrics.Group = nil
		return nil
	}

	return nil
}

func (cfg *Config) validateMetricsConfig() error {
	if len(cfg.Metrics.Group) > 0 && cfg.Metrics.AutoDiscover != nil {
		return errAutodiscoverAndNamedConfigured
	}
	if cfg.Metrics.AutoDiscover != nil {
		return validateAutoDiscoveryConfig(cfg.Metrics.AutoDiscover)
	}
	return cfg.validateMetricsGroupConfig()
}

func validateAutoDiscoveryConfig(autodiscoveryConfig *AutoDiscoverConfig) error {
	if autodiscoveryConfig.Limit <= 0 {
		return errInvalidAutodiscoverLimit
	}
	return nil
}

func (cfg *Config) validateMetricsGroupConfig() error {
	var err, errs error

	metricsNamespaces := cfg.Metrics.Group
	for _, namespace := range metricsNamespaces {
		if namespace.Namespace == "" {
			return errNoNamespaceConfigured
		}
		err = validateNamedMetricConfig(&namespace.MetricName)
	}
	errs = errors.Join(errs, err)
	return errs
}

func validateNamedMetricConfig(metricName *[]NamedConfig) error {
	var errs error

	for _, name := range *metricName {
		err := validateAwsAggregation(name.AwsAggregation)
		if err != nil {
			return err
		}
		if name.MetricName == "" {
			return errNoMetricNameConfigured
		}
		errs = errors.Join(errs, validate(name.Dimensions))
	}
	return errs
}

func validate(mdc []MetricDimensionsConfig) error {
	for _, dimensionConfig := range mdc {
		if dimensionConfig.Name == "" || dimensionConfig.Value == "" {
			return errEmptyDimensions
		}
		if strings.HasPrefix(dimensionConfig.Name, ":") {
			return errDimensionColonPrefix
		}
	}
	if len(mdc) > 30 {
		return errTooManyDimensions
	}
	return nil
}

// https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Statistics-definitions.html
func validateAwsAggregation(agg string) error {
	switch {
	case agg == "SampleCount":
		return nil
	case agg == "Sum":
		return nil
	case agg == "Average":
		return nil
	case agg == "Minimum":
		return nil
	case agg == "Maximum":
		return nil
	case strings.HasPrefix(agg, "p"):
		return nil
	case strings.HasPrefix(agg, "TM"):
		return nil
	case agg == "IQM":
		return nil
	case strings.HasPrefix(agg, "PR"):
		return nil
	case strings.HasPrefix(agg, "TC"):
		return nil
	case strings.HasPrefix(agg, "TS"):
		return nil
	default:
		return errInvalidAwsAggregation
	}
}
