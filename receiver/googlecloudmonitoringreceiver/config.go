// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudmonitoringreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudmonitoringreceiver"

import (
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

const (
	defaultCollectionInterval = 300 * time.Second // Default value for collection interval
	minCollectionInterval     = 60 * time.Second  // Minimum value for collection interval
	defaultFetchDelay         = 60 * time.Second  // Default value for fetch delay
	maxIngestDelay            = 24 * time.Hour    // Maximum allowed default_ingest_delay
)

type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`

	ProjectID string `mapstructure:"project_id"`
	// Overrides the default monitoring.googleapis.com:443 endpoint.
	// Use this when targeting non-standard universe domains.
	Endpoint string `mapstructure:"endpoint"`
	// DefaultIngestDelay is used when a metric descriptor does not report
	// Metadata.IngestDelay (typical for user log-based, custom, and external
	// Prometheus metrics). Default 60s preserves the prior hardcoded fallback.
	DefaultIngestDelay time.Duration  `mapstructure:"default_ingest_delay"`
	MetricsList        []MetricConfig `mapstructure:"metrics_list"`
}

type MetricConfig struct {
	MetricName string `mapstructure:"metric_name"`
	// Filter for listing metric descriptors. Only support `project` and `metric.type` as filter objects.
	// See https://cloud.google.com/monitoring/api/v3/filters#metric-descriptor-filter for more details.
	MetricDescriptorFilter string `mapstructure:"metric_descriptor_filter"`
}

func (config *Config) Validate() error {
	if config.CollectionInterval < minCollectionInterval {
		return fmt.Errorf("\"collection_interval\" must be not lower than the collection interval: %v, current value is %v", minCollectionInterval, config.CollectionInterval)
	}

	if config.DefaultIngestDelay < 0 {
		return fmt.Errorf("\"default_ingest_delay\" must be non-negative, current value is %v", config.DefaultIngestDelay)
	}

	if config.DefaultIngestDelay > maxIngestDelay {
		return fmt.Errorf("\"default_ingest_delay\" must not exceed %v, current value is %v", maxIngestDelay, config.DefaultIngestDelay)
	}

	if len(config.MetricsList) == 0 {
		return errors.New("missing required field \"metrics_list\" or its value is empty")
	}

	for _, metric := range config.MetricsList {
		if err := metric.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func (metric MetricConfig) Validate() error {
	if metric.MetricName != "" && metric.MetricDescriptorFilter != "" {
		return errors.New("fields \"metric_name\" and \"metric_descriptor_filter\" cannot both have value")
	}

	if metric.MetricName == "" && metric.MetricDescriptorFilter == "" {
		return errors.New("fields \"metric_name\" and \"metric_descriptor_filter\" cannot both be empty")
	}

	return nil
}
