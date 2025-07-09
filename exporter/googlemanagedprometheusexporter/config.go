// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlemanagedprometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlemanagedprometheusexporter"

import (
	"fmt"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/googlemanagedprometheus"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

// Config defines configuration for Google Cloud Managed Service for Prometheus exporter.
type Config struct {
	GMPConfig `mapstructure:",squash"`

	// Timeout for all API calls. If not set, defaults to 12 seconds.
	TimeoutSettings exporterhelper.TimeoutConfig    `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	QueueSettings   exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`
}

// GMPConfig is a subset of the collector config applicable to the GMP exporter.
type GMPConfig struct {
	ProjectID    string       `mapstructure:"project"`
	UserAgent    string       `mapstructure:"user_agent"`
	MetricConfig MetricConfig `mapstructure:"metric"`
}

type MetricConfig struct {
	// Prefix configures the prefix of metrics sent to GoogleManagedPrometheus.  Defaults to prometheus.googleapis.com.
	// Changing this prefix is not recommended, as it may cause metrics to not be queryable with promql in the Cloud Monitoring UI.
	Prefix          string                         `mapstructure:"prefix"`
	ClientConfig    collector.ClientConfig         `mapstructure:",squash"`
	Config          googlemanagedprometheus.Config `mapstructure:",squash"`
	ResourceFilters []collector.ResourceFilter     `mapstructure:"resource_filters"`
	// CumulativeNormalization normalizes cumulative metrics without start times or with
	// explicit reset points by subtracting subsequent points from the initial point.
	// It is enabled by default. Since it caches starting points, it may result in
	// increased memory usage.
	CumulativeNormalization bool `mapstructure:"cumulative_normalization"`
}

func (c *GMPConfig) toCollectorConfig() collector.Config {
	// start with whatever the default collector config is.
	cfg := collector.DefaultConfig()
	cfg.MetricConfig.Prefix = c.MetricConfig.Prefix
	cfg.MetricConfig.CumulativeNormalization = c.MetricConfig.CumulativeNormalization
	if c.MetricConfig.Prefix == "" {
		cfg.MetricConfig.Prefix = "prometheus.googleapis.com"
	}
	cfg.MetricConfig.SkipCreateMetricDescriptor = true
	cfg.MetricConfig.InstrumentationLibraryLabels = false
	cfg.MetricConfig.ServiceResourceLabels = false
	// Update metric naming to match GMP conventions
	cfg.MetricConfig.GetMetricName = func(baseName string, metric pmetric.Metric) (string, error) {
		compliantName := prometheus.BuildCompliantName(metric, "", c.MetricConfig.Config.AddMetricSuffixes)
		return googlemanagedprometheus.GetMetricName(baseName, compliantName, metric)
	}
	// Map to the prometheus_target monitored resource
	cfg.MetricConfig.MapMonitoredResource = c.MetricConfig.Config.MapToPrometheusTarget
	cfg.MetricConfig.ExtraMetrics = c.MetricConfig.Config.ExtraMetrics
	cfg.MetricConfig.EnableSumOfSquaredDeviation = true
	// map the GMP config's fields to the collector config
	cfg.ProjectID = c.ProjectID
	cfg.UserAgent = c.UserAgent
	cfg.MetricConfig.ClientConfig = c.MetricConfig.ClientConfig
	cfg.MetricConfig.ResourceFilters = c.MetricConfig.ResourceFilters
	return cfg
}

func (cfg *Config) Validate() error {
	if err := collector.ValidateConfig(cfg.toCollectorConfig()); err != nil {
		return fmt.Errorf("exporter settings are invalid :%w", err)
	}
	if err := cfg.MetricConfig.Config.Validate(); err != nil {
		return fmt.Errorf("exporter settings are invalid :%w", err)
	}
	return nil
}
