// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlemanagedprometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlemanagedprometheusexporter"

import (
	"fmt"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/googlemanagedprometheus"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for Google Cloud Managed Service for Prometheus exporter.
type Config struct {
	GMPConfig `mapstructure:",squash"`

	// Timeout for all API calls. If not set, defaults to 12 seconds.
	exporterhelper.TimeoutSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
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
}

func (c *GMPConfig) toCollectorConfig() collector.Config {
	// start with whatever the default collector config is.
	cfg := collector.DefaultConfig()
	cfg.MetricConfig.Prefix = c.MetricConfig.Prefix
	if c.MetricConfig.Prefix == "" {
		cfg.MetricConfig.Prefix = "prometheus.googleapis.com"
	}
	cfg.MetricConfig.SkipCreateMetricDescriptor = true
	cfg.MetricConfig.InstrumentationLibraryLabels = false
	cfg.MetricConfig.ServiceResourceLabels = false
	// Update metric naming to match GMP conventions
	cfg.MetricConfig.GetMetricName = c.MetricConfig.Config.GetMetricName
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
