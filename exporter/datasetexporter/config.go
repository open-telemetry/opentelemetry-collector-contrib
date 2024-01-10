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
	"github.com/scalyr/dataset-go/pkg/server_host_config"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const exportSeparatorDefault = "."
const exportDistinguishingSuffix = "_"

// exportSettings configures separator and distinguishing suffixes for all exported fields
type exportSettings struct {
	// ExportSeparator is separator used when flattening exported attributes
	// Default value: .
	ExportSeparator string `mapstructure:"export_separator"`

	// ExportDistinguishingSuffix is suffix used to be appended to the end of attribute name in case of collision
	// Default value: _
	ExportDistinguishingSuffix string `mapstructure:"export_distinguishing_suffix"`
}

// newDefaultExportSettings returns the default settings for exportSettings.
func newDefaultExportSettings() exportSettings {
	return exportSettings{
		ExportSeparator:            exportSeparatorDefault,
		ExportDistinguishingSuffix: exportDistinguishingSuffix,
	}
}

type TracesSettings struct {
	// exportSettings configures separator and distinguishing suffixes for all exported fields
	exportSettings `mapstructure:",squash"`
}

// newDefaultTracesSettings returns the default settings for TracesSettings.
func newDefaultTracesSettings() TracesSettings {
	return TracesSettings{
		exportSettings: newDefaultExportSettings(),
	}
}

const logsExportResourceInfoDefault = false
const logsExportResourcePrefixDefault = "resource.attributes."
const logsExportScopeInfoDefault = true
const logsExportScopePrefixDefault = "scope.attributes."
const logsDecomposeComplexMessageFieldDefault = false
const logsDecomposedComplexMessageFieldPrefixDefault = "body.map."

type LogsSettings struct {
	// ExportResourceInfo is optional flag to signal that the resource info is being exported to DataSet while exporting Logs.
	// This is especially useful when reducing DataSet billable log volume.
	// Default value: false
	ExportResourceInfo bool `mapstructure:"export_resource_info_on_event"`

	// ExportResourcePrefix is prefix for the resource attributes when they are exported (see ExportResourceInfo).
	// Default value: resource.attributes.
	ExportResourcePrefix string `mapstructure:"export_resource_prefix"`

	// ExportScopeInfo is an optional flag that signals if scope info should be exported (when available) with each event. If scope
	// information is not utilized, it makes sense to disable exporting it since it will result in increased billable log volume.
	// Default value: true
	ExportScopeInfo bool `mapstructure:"export_scope_info_on_event"`

	// ExportScopePrefix is prefix for the scope attributes when they are exported (see ExportScopeInfo).
	// Default value: scope.attributes.
	ExportScopePrefix string `mapstructure:"export_scope_prefix"`

	// DecomposeComplexMessageField is an optional flag to signal that message / body of complex types (e.g. a map) should be
	// decomposed / deconstructed into multiple fields. This is usually done outside of the main DataSet integration on the
	// client side (e.g. as part of the attribute processor or similar) or on the server side (DataSet server side JSON parser
	// for message field) and that's why this functionality is disabled by default.
	DecomposeComplexMessageField bool `mapstructure:"decompose_complex_message_field"`

	// DecomposedComplexMessagePrefix is prefix for the decomposed complex message (see DecomposeComplexMessageField).
	// Default value: body.map.
	DecomposedComplexMessagePrefix string `mapstructure:"decomposed_complex_message_prefix"`

	// exportSettings configures separator and distinguishing suffixes for all exported fields
	exportSettings `mapstructure:",squash"`
}

// newDefaultLogsSettings returns the default settings for LogsSettings.
func newDefaultLogsSettings() LogsSettings {
	return LogsSettings{
		ExportResourceInfo:             logsExportResourceInfoDefault,
		ExportResourcePrefix:           logsExportResourcePrefixDefault,
		ExportScopeInfo:                logsExportScopeInfoDefault,
		ExportScopePrefix:              logsExportScopePrefixDefault,
		DecomposeComplexMessageField:   logsDecomposeComplexMessageFieldDefault,
		DecomposedComplexMessagePrefix: logsDecomposedComplexMessageFieldPrefixDefault,
		exportSettings:                 newDefaultExportSettings(),
	}
}

const bufferMaxLifetime = 5 * time.Second
const bufferRetryInitialInterval = 5 * time.Second
const bufferRetryMaxInterval = 30 * time.Second
const bufferRetryMaxElapsedTime = 300 * time.Second
const bufferRetryShutdownTimeout = 30 * time.Second

type BufferSettings struct {
	MaxLifetime          time.Duration `mapstructure:"max_lifetime"`
	GroupBy              []string      `mapstructure:"group_by"`
	RetryInitialInterval time.Duration `mapstructure:"retry_initial_interval"`
	RetryMaxInterval     time.Duration `mapstructure:"retry_max_interval"`
	RetryMaxElapsedTime  time.Duration `mapstructure:"retry_max_elapsed_time"`
	RetryShutdownTimeout time.Duration `mapstructure:"retry_shutdown_timeout"`
}

// newDefaultBufferSettings returns the default settings for BufferSettings.
func newDefaultBufferSettings() BufferSettings {
	return BufferSettings{
		MaxLifetime:          bufferMaxLifetime,
		GroupBy:              []string{},
		RetryInitialInterval: bufferRetryInitialInterval,
		RetryMaxInterval:     bufferRetryMaxInterval,
		RetryMaxElapsedTime:  bufferRetryMaxElapsedTime,
		RetryShutdownTimeout: bufferRetryShutdownTimeout,
	}
}

type ServerHostSettings struct {
	UseHostName bool   `mapstructure:"use_hostname"`
	ServerHost  string `mapstructure:"server_host"`
}

// newDefaultBufferSettings returns the default settings for BufferSettings.
func newDefaultServerHostSettings() ServerHostSettings {
	return ServerHostSettings{
		UseHostName: true,
		ServerHost:  "",
	}
}

const debugDefault = false

type Config struct {
	DatasetURL                     string              `mapstructure:"dataset_url"`
	APIKey                         configopaque.String `mapstructure:"api_key"`
	Debug                          bool                `mapstructure:"debug"`
	BufferSettings                 `mapstructure:"buffer"`
	TracesSettings                 `mapstructure:"traces"`
	LogsSettings                   `mapstructure:"logs"`
	ServerHostSettings             `mapstructure:"server_host"`
	configretry.BackOffConfig      `mapstructure:"retry_on_failure"`
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
	apiKey, _ := c.APIKey.MarshalText()
	s := ""
	s += fmt.Sprintf("%s: %s; ", "DatasetURL", c.DatasetURL)
	s += fmt.Sprintf("%s: %s (%d); ", "APIKey", apiKey, len(c.APIKey))
	s += fmt.Sprintf("%s: %t; ", "Debug", c.Debug)
	s += fmt.Sprintf("%s: %+v; ", "BufferSettings", c.BufferSettings)
	s += fmt.Sprintf("%s: %+v; ", "LogsSettings", c.LogsSettings)
	s += fmt.Sprintf("%s: %+v; ", "TracesSettings", c.TracesSettings)
	s += fmt.Sprintf("%s: %+v; ", "ServerHostSettings", c.ServerHostSettings)
	s += fmt.Sprintf("%s: %+v; ", "BackOffConfig", c.BackOffConfig)
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
					RetryShutdownTimeout:     c.BufferSettings.RetryShutdownTimeout,
				},
				ServerHostSettings: server_host_config.DataSetServerHostSettings{
					UseHostName: c.ServerHostSettings.UseHostName,
					ServerHost:  c.ServerHostSettings.ServerHost,
				},
				Debug: c.Debug,
			},
			tracesSettings:     c.TracesSettings,
			logsSettings:       c.LogsSettings,
			serverHostSettings: c.ServerHostSettings,
		},
		nil
}

type ExporterConfig struct {
	datasetConfig      *datasetConfig.DataSetConfig
	tracesSettings     TracesSettings
	logsSettings       LogsSettings
	serverHostSettings ServerHostSettings
}
