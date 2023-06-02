// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasetexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datasetexporter"

import (
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/scalyr/dataset-go/pkg/buffer"
	"github.com/scalyr/dataset-go/pkg/buffer_config"
	datasetConfig "github.com/scalyr/dataset-go/pkg/config"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const tracesMaxWait = 5 * time.Second
const tracesAggregate = false

type TracesSettings struct {
	Aggregate bool          `mapstructure:"aggregate"`
	MaxWait   time.Duration `mapstructure:"max_wait"`
}

// newDefaultTracesSettings returns the default settings for TracesSettings.
func newDefaultTracesSettings() TracesSettings {
	return TracesSettings{
		Aggregate: tracesAggregate,
		MaxWait:   tracesMaxWait,
	}
}

const bufferMaxLifetime = 5 * time.Second
const bufferRetryInitialInterval = 5 * time.Second
const bufferRetryMaxInterval = 30 * time.Second
const bufferRetryMaxElapsedTime = 300 * time.Second

type BufferSettings struct {
	MaxLifetime          time.Duration `mapstructure:"max_lifetime"`
	GroupBy              []string      `mapstructure:"group_by"`
	RetryInitialInterval time.Duration `mapstructure:"retry_initial_interval"`
	RetryMaxInterval     time.Duration `mapstructure:"retry_max_interval"`
	RetryMaxElapsedTime  time.Duration `mapstructure:"retry_max_elapsed_time"`
}

// newDefaultBufferSettings returns the default settings for BufferSettings.
func newDefaultBufferSettings() BufferSettings {
	return BufferSettings{
		MaxLifetime:          bufferMaxLifetime,
		GroupBy:              []string{},
		RetryInitialInterval: bufferRetryInitialInterval,
		RetryMaxInterval:     bufferRetryMaxInterval,
		RetryMaxElapsedTime:  bufferRetryMaxElapsedTime,
	}
}

type Config struct {
	DatasetURL                     string              `mapstructure:"dataset_url"`
	APIKey                         configopaque.String `mapstructure:"api_key"`
	BufferSettings                 `mapstructure:"buffer"`
	TracesSettings                 `mapstructure:"traces"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
	exporterhelper.TimeoutSettings `mapstructure:"timeout"`
}

func (c *Config) Unmarshal(conf *confmap.Conf) error {
	if err := conf.Unmarshal(c, confmap.WithErrorUnused()); err != nil {
		return fmt.Errorf("cannot unmarshal config: %w", err)
	}

	return nil
}

// Validate checks if all required fields in Config are set and have valid values.
// If any of the required fields are missing or have invalid values, it returns an error.
func (c *Config) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("api_key is required")
	}
	if c.DatasetURL == "" {
		return fmt.Errorf("dataset_url is required")
	}

	return nil
}

// String returns a string representation of the Config object.
// It includes all the fields and their values in the format "field_name: field_value".
func (c *Config) String() string {
	s := ""
	s += fmt.Sprintf("%s: %s; ", "DatasetURL", c.DatasetURL)
	s += fmt.Sprintf("%s: %+v; ", "BufferSettings", c.BufferSettings)
	s += fmt.Sprintf("%s: %+v; ", "TracesSettings", c.TracesSettings)
	s += fmt.Sprintf("%s: %+v; ", "RetrySettings", c.RetrySettings)
	s += fmt.Sprintf("%s: %+v; ", "QueueSettings", c.QueueSettings)
	s += fmt.Sprintf("%s: %+v", "TimeoutSettings", c.TimeoutSettings)

	return s
}

func (c *Config) convert() (*ExporterConfig, error) {
	err := c.Validate()
	if err != nil {
		return nil, fmt.Errorf("config is not valid: %w", err)
	}

	return &ExporterConfig{
			datasetConfig: &datasetConfig.DataSetConfig{
				Endpoint: c.DatasetURL,
				Tokens:   datasetConfig.DataSetTokens{WriteLog: string(c.APIKey)},
				BufferSettings: buffer_config.DataSetBufferSettings{
					MaxLifetime:              c.BufferSettings.MaxLifetime,
					MaxSize:                  buffer.LimitBufferSize,
					GroupBy:                  c.BufferSettings.GroupBy,
					RetryInitialInterval:     c.BufferSettings.RetryInitialInterval,
					RetryMaxInterval:         c.BufferSettings.RetryMaxInterval,
					RetryMaxElapsedTime:      c.BufferSettings.RetryMaxElapsedTime,
					RetryMultiplier:          backoff.DefaultMultiplier,
					RetryRandomizationFactor: backoff.DefaultRandomizationFactor,
				},
			},
			tracesSettings: c.TracesSettings,
		},
		nil
}

type ExporterConfig struct {
	datasetConfig  *datasetConfig.DataSetConfig
	tracesSettings TracesSettings
}
