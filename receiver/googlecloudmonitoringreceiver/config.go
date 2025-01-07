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
	defaultFetchDelay         = 60 * time.Second  // Default value for fetch delay
)

type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`

	ProjectID   string         `mapstructure:"project_id"`
	MetricsList []MetricConfig `mapstructure:"metrics_list"`
}

type MetricConfig struct {
	MetricName string `mapstructure:"metric_name"`
}

func (config *Config) Validate() error {
	if config.CollectionInterval < defaultCollectionInterval {
		return fmt.Errorf("\"collection_interval\" must be not lower than the collection interval: %v, current value is %v", defaultCollectionInterval, config.CollectionInterval)
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
	if metric.MetricName == "" {
		return errors.New("field \"metric_name\" is required and cannot be empty for metric configuration")
	}

	return nil
}
