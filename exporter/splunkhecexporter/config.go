// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"errors"
	"fmt"
	"net/url"
	"path"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

const (
	// hecPath is the default HEC path on the Splunk instance.
	hecPath                          = "services/collector"
	defaultContentLengthLogsLimit    = 2 * 1024 * 1024
	defaultContentLengthMetricsLimit = 2 * 1024 * 1024
	defaultContentLengthTracesLimit  = 2 * 1024 * 1024
	maxContentLengthLogsLimit        = 800 * 1024 * 1024
	maxContentLengthMetricsLimit     = 800 * 1024 * 1024
	maxContentLengthTracesLimit      = 800 * 1024 * 1024
)

// OtelToHecFields defines the mapping of attributes to HEC fields
type OtelToHecFields struct {
	// SeverityText informs the exporter to map the severity text field to a specific HEC field.
	SeverityText string `mapstructure:"severity_text"`
	// SeverityNumber informs the exporter to map the severity number field to a specific HEC field.
	SeverityNumber string `mapstructure:"severity_number"`
	// Name informs the exporter to map the name field to a specific HEC field.
	Name string `mapstructure:"name"`
}

// Config defines configuration for Splunk exporter.
type Config struct {
	config.ExporterSettings        `mapstructure:",squash"`
	exporterhelper.TimeoutSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`

	// LogDataEnabled can be used to disable sending logs by the exporter.
	LogDataEnabled bool `mapstructure:"log_data_enabled"`

	// ProfilingDataEnabled can be used to disable sending profiling data by the exporter.
	ProfilingDataEnabled bool `mapstructure:"profiling_data_enabled"`

	// HEC Token is the authentication token provided by Splunk: https://docs.splunk.com/Documentation/Splunk/latest/Data/UsetheHTTPEventCollector.
	Token string `mapstructure:"token"`

	// URL is the Splunk HEC endpoint where data is going to be sent to.
	Endpoint string `mapstructure:"endpoint"`

	// Optional Splunk source: https://docs.splunk.com/Splexicon:Source.
	// Sources identify the incoming data.
	Source string `mapstructure:"source"`

	// Optional Splunk source type: https://docs.splunk.com/Splexicon:Sourcetype.
	SourceType string `mapstructure:"sourcetype"`

	// Splunk index, optional name of the Splunk index.
	Index string `mapstructure:"index"`

	// MaxConnections is used to set a limit to the maximum idle HTTP connection the exporter can keep open. Defaults to 100.
	MaxConnections uint `mapstructure:"max_connections"`

	// Disable GZip compression. Defaults to false.
	DisableCompression bool `mapstructure:"disable_compression"`

	// Maximum log payload size in bytes. Default value is 2097152 bytes (2MiB).
	// Maximum allowed value is 838860800 (~ 800 MB).
	MaxContentLengthLogs uint `mapstructure:"max_content_length_logs"`

	// Maximum metric payload size in bytes. Default value is 2097152 bytes (2MiB).
	// Maximum allowed value is 838860800 (~ 800 MB).
	MaxContentLengthMetrics uint `mapstructure:"max_content_length_metrics"`

	// Maximum trace payload size in bytes. Default value is 2097152 bytes (2MiB).
	// Maximum allowed value is 838860800 (~ 800 MB).
	MaxContentLengthTraces uint `mapstructure:"max_content_length_traces"`

	// TLSSetting struct exposes TLS client configuration.
	TLSSetting configtls.TLSClientSetting `mapstructure:"tls,omitempty"`

	// App name is used to track telemetry information for Splunk App's using HEC by App name. Defaults to "OpenTelemetry Collector Contrib".
	SplunkAppName string `mapstructure:"splunk_app_name"`

	// App version is used to track telemetry information for Splunk App's using HEC by App version. Defaults to the current OpenTelemetry Collector Contrib build version.
	SplunkAppVersion string `mapstructure:"splunk_app_version"`
	// HecToOtelAttrs creates a mapping from attributes to HEC specific metadata: source, sourcetype, index and host.
	HecToOtelAttrs splunk.HecToOtelAttrs `mapstructure:"hec_metadata_to_otel_attrs"`
	// HecFields creates a mapping from attributes to HEC fields.
	HecFields OtelToHecFields `mapstructure:"otel_to_hec_fields"`
}

func (cfg *Config) getOptionsFromConfig() (*exporterOptions, error) {
	if err := cfg.validateConfig(); err != nil {
		return nil, err
	}

	url, err := cfg.getURL()
	if err != nil {
		return nil, fmt.Errorf(`invalid "endpoint": %w`, err)
	}

	return &exporterOptions{
		url:   url,
		token: cfg.Token,
	}, nil
}

func (cfg *Config) validateConfig() error {
	if cfg.Endpoint == "" {
		return errors.New(`requires a non-empty "endpoint"`)
	}

	if cfg.Token == "" {
		return errors.New(`requires a non-empty "token"`)
	}

	if cfg.MaxContentLengthLogs > maxContentLengthLogsLimit {
		return fmt.Errorf(`requires "max_content_length_logs" <= %d`, maxContentLengthLogsLimit)
	}

	if cfg.MaxContentLengthMetrics > maxContentLengthMetricsLimit {
		return fmt.Errorf(`requires "max_content_length_metrics" <= %d`, maxContentLengthMetricsLimit)
	}

	if cfg.MaxContentLengthTraces > maxContentLengthTracesLimit {
		return fmt.Errorf(`requires "max_content_length_traces <= #{maxContentLengthTracesLimit}`)
	}

	return nil
}

func (cfg *Config) getURL() (out *url.URL, err error) {

	out, err = url.Parse(cfg.Endpoint)
	if err != nil {
		return out, err
	}
	if out.Path == "" || out.Path == "/" {
		out.Path = path.Join(out.Path, hecPath)
	}

	return
}

// Validate checks if the exporter configuration is valid.
func (cfg *Config) Validate() error {
	if err := cfg.QueueSettings.Validate(); err != nil {
		return fmt.Errorf("sending_queue settings has invalid configuration: %w", err)
	}
	if !cfg.LogDataEnabled && !cfg.ProfilingDataEnabled {
		return errors.New(`either "log_data_enabled" or "profiling_data_enabled" has to be true`)
	}
	return nil
}
