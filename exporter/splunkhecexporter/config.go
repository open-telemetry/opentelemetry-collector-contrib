// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

const (
	// hecPath is the default HEC path on the Splunk instance.
	hecPath                          = "services/collector"
	defaultContentLengthLogsLimit    = 2 * 1024 * 1024
	defaultContentLengthMetricsLimit = 2 * 1024 * 1024
	defaultContentLengthTracesLimit  = 2 * 1024 * 1024
	defaultMaxEventSize              = 5 * 1024 * 1024
	maxContentLengthLogsLimit        = 800 * 1024 * 1024
	maxContentLengthMetricsLimit     = 800 * 1024 * 1024
	maxContentLengthTracesLimit      = 800 * 1024 * 1024
	maxMaxEventSize                  = 800 * 1024 * 1024
)

// OtelToHecFields defines the mapping of attributes to HEC fields
type OtelToHecFields struct {
	// SeverityText informs the exporter to map the severity text field to a specific HEC field.
	SeverityText string `mapstructure:"severity_text"`
	// SeverityNumber informs the exporter to map the severity number field to a specific HEC field.
	SeverityNumber string `mapstructure:"severity_number"`
}

// HecHeartbeat defines the heartbeat information for the exporter
type HecHeartbeat struct {
	// Interval represents the time interval for the heartbeat interval. If nothing or 0 is set,
	// heartbeat is not enabled.
	// A heartbeat is an event sent to _internal index with metadata for the current collector/host.
	Interval time.Duration `mapstructure:"interval"`

	// Startup is used to send heartbeat events on exporter's startup.
	Startup bool `mapstructure:"startup"`
}

// HecTelemetry defines the telemetry configuration for the exporter
type HecTelemetry struct {
	// Enabled is the bool to enable telemetry inside splunk hec exporter
	Enabled bool `mapstructure:"enabled"`

	// OverrideMetricsNames is the map to override metrics for internal metrics in splunk hec exporter
	OverrideMetricsNames map[string]string `mapstructure:"override_metrics_names"`

	// ExtraAttributes is the extra attributes for metrics inside splunk hex exporter
	ExtraAttributes map[string]string `mapstructure:"extra_attributes"`
}

// Config defines configuration for Splunk exporter.
type Config struct {
	confighttp.ClientConfig   `mapstructure:",squash"`
	QueueSettings             exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`

	// Experimental: This configuration is at the early stage of development and may change without backward compatibility
	// until https://github.com/open-telemetry/opentelemetry-collector/issues/8122 is resolved.
	BatcherConfig exporterhelper.BatcherConfig `mapstructure:"batcher"` //nolint:staticcheck

	// LogDataEnabled can be used to disable sending logs by the exporter.
	LogDataEnabled bool `mapstructure:"log_data_enabled"`

	// ProfilingDataEnabled can be used to disable sending profiling data by the exporter.
	ProfilingDataEnabled bool `mapstructure:"profiling_data_enabled"`

	// HEC Token is the authentication token provided by Splunk: https://docs.splunk.com/Documentation/Splunk/latest/Data/UsetheHTTPEventCollector.
	Token configopaque.String `mapstructure:"token"`

	// Optional Splunk source: https://docs.splunk.com/Splexicon:Source.
	// Sources identify the incoming data.
	Source string `mapstructure:"source"`

	// Optional Splunk source type: https://docs.splunk.com/Splexicon:Sourcetype.
	SourceType string `mapstructure:"sourcetype"`

	// Splunk index, optional name of the Splunk index.
	Index string `mapstructure:"index"`

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

	// Maximum payload size, raw uncompressed. Default value is 5242880 bytes (5MiB).
	// Maximum allowed value is 838860800 (~ 800 MB).
	MaxEventSize uint `mapstructure:"max_event_size"`

	// App name is used to track telemetry information for Splunk App's using HEC by App name. Defaults to "OpenTelemetry Collector Contrib".
	SplunkAppName string `mapstructure:"splunk_app_name"`

	// App version is used to track telemetry information for Splunk App's using HEC by App version. Defaults to the current OpenTelemetry Collector Contrib build version.
	SplunkAppVersion string `mapstructure:"splunk_app_version"`

	// OtelAttrsToHec creates a mapping from attributes to HEC specific metadata: source, sourcetype, index and host.
	OtelAttrsToHec splunk.HecToOtelAttrs `mapstructure:"otel_attrs_to_hec_metadata"`

	// HecToOtelAttrs creates a mapping from attributes to HEC specific metadata: source, sourcetype, index and host.
	// Deprecated: [v0.113.0] Use OtelAttrsToHec instead.
	HecToOtelAttrs splunk.HecToOtelAttrs `mapstructure:"hec_metadata_to_otel_attrs"`
	// HecFields creates a mapping from attributes to HEC fields.
	HecFields OtelToHecFields `mapstructure:"otel_to_hec_fields"`

	// HealthPath for health API, default is '/services/collector/health'
	HealthPath string `mapstructure:"health_path"`

	// HecHealthCheckEnabled can be used to verify Splunk HEC health on exporter's startup
	HecHealthCheckEnabled bool `mapstructure:"health_check_enabled"`

	// ExportRaw to send only the log's body, targeting a Splunk HEC raw endpoint.
	ExportRaw bool `mapstructure:"export_raw"`

	// UseMultiMetricFormat combines metric events to save space during ingestion.
	UseMultiMetricFormat bool `mapstructure:"use_multi_metric_format"`

	// Heartbeat is the configuration to enable heartbeat
	Heartbeat HecHeartbeat `mapstructure:"heartbeat"`

	// Telemetry is the configuration for splunk hec exporter telemetry
	Telemetry HecTelemetry `mapstructure:"telemetry"`
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
	if !cfg.LogDataEnabled && !cfg.ProfilingDataEnabled {
		return errors.New(`either "log_data_enabled" or "profiling_data_enabled" has to be true`)
	}
	if cfg.Endpoint == "" {
		return errors.New(`requires a non-empty "endpoint"`)
	}
	_, err := cfg.getURL()
	if err != nil {
		return fmt.Errorf(`invalid "endpoint": %w`, err)
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
		return fmt.Errorf(`requires "max_content_length_traces" <= %d`, maxContentLengthTracesLimit)
	}

	if cfg.MaxEventSize > maxMaxEventSize {
		return fmt.Errorf(`requires "max_event_size" <= %d`, maxMaxEventSize)
	}

	return nil
}
