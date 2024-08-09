// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudmonitoringreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudmonitoringreceiver"

import (
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

const minCollectionIntervalSeconds = 60

type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`

	ProjectID   string         `mapstructure:"project_id"`
	MetricsList []MetricConfig `mapstructure:"metrics_list"`
}

type MetricConfig struct {
	MetricName string        `mapstructure:"metric_name"`
	Delay      time.Duration `mapstructure:"delay"`
}

func (config *Config) Validate() error {
	if config.CollectionInterval.Seconds() < minCollectionIntervalSeconds {
		return fmt.Errorf("\"collection_interval\" must be not lower than %v seconds, current value is %v seconds", minCollectionIntervalSeconds, config.CollectionInterval.Seconds())
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

	if metric.Delay < 0 {
		return errors.New("field \"delay\" cannot be negative for metric configuration")
	}

	return nil
}
