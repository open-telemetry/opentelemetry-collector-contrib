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

package googlemanagedprometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlemanagedprometheusexporter"

import (
	"fmt"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/googlemanagedprometheus"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for Google Cloud Managed Service for Prometheus exporter.
type Config struct {
	config.ExporterSettings `mapstructure:",squash"`
	GMPConfig               `mapstructure:",squash"`

	// Timeout for all API calls. If not set, defaults to 12 seconds.
	exporterhelper.TimeoutSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`
}

// GMPConfig is a subset of the collector config applicable to the GMP exporter.
type GMPConfig struct {
	ProjectID    string                 `mapstructure:"project"`
	UserAgent    string                 `mapstructure:"user_agent"`
	ClientConfig collector.ClientConfig `mapstructure:",squash"`
}

func (c *GMPConfig) toCollectorConfig() collector.Config {
	// start with whatever the default collector config is.
	cfg := collector.DefaultConfig()
	// hard-code some config options to make it work with GMP
	cfg.MetricConfig.Prefix = "prometheus.googleapis.com"
	cfg.MetricConfig.SkipCreateMetricDescriptor = true
	cfg.MetricConfig.InstrumentationLibraryLabels = false
	cfg.MetricConfig.ServiceResourceLabels = false
	// Update metric naming to match GMP conventions
	cfg.MetricConfig.GetMetricName = googlemanagedprometheus.GetMetricName
	// Map to the prometheus_target monitored resource
	cfg.MetricConfig.MapMonitoredResource = googlemanagedprometheus.MapToPrometheusTarget
	cfg.MetricConfig.EnableSumOfSquaredDeviation = true
	// map the GMP config's fields to the collector config
	cfg.ProjectID = c.ProjectID
	cfg.UserAgent = c.UserAgent
	cfg.MetricConfig.ClientConfig = c.ClientConfig
	return cfg
}

func (cfg *Config) Validate() error {
	if err := cfg.ExporterSettings.Validate(); err != nil {
		return fmt.Errorf("exporter settings are invalid :%w", err)
	}
	if err := collector.ValidateConfig(cfg.toCollectorConfig()); err != nil {
		return fmt.Errorf("exporter settings are invalid :%w", err)
	}
	return nil
}
