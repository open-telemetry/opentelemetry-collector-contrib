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

type BreakerSettings struct {
	// Enabled indicates whether to not retry sending batches in case of export failure.
	Enabled           bool          `mapstructure:"enabled"`
	FailureThreshold  uint32        `mapstructure:"threshold"`
	ThresholdDuration time.Duration `mapstructure:"threshold_duration"`

	// Timeout the time to wait after the failure threshold is met before sending a test request
	Timeout time.Duration `mapstructure:"timeout"`

	// Uncomment for exponential backoff
	// // RandomizationFactor is a random factor used to calculate next backoffs
	// // Randomized interval = RetryInterval * (1 ± RandomizationFactor)
	// RandomizationFactor float64 `mapstructure:"randomization_factor"`
	// // Multiplier is the value multiplied by the backoff interval bounds
	// Multiplier float64 `mapstructure:"multiplier"`
	// // MaxInterval is the upper bound on backoff interval. Once this value is reached the delay between
	// // consecutive retries will always be `MaxInterval`.
	// MaxInterval time.Duration `mapstructure:"max_interval"`
	// // MaxElapsedTime is the maximum amount of time (including retries) spent trying to send a request/batch.
	// // Once this value is reached, the data is discarded.
	// MaxElapsedTime time.Duration `mapstructure:"max_elapsed_time"`
}

func NewDefaultBreakerSettings() BreakerSettings {
	return BreakerSettings{
		Enabled:           true,
		FailureThreshold:  5,
		ThresholdDuration: 30 * time.Second,
		Timeout:           5 * time.Second,

		// Uncomment for exponential backoff
		// InitialInterval:     5 * time.Second,
		// RandomizationFactor: backoff.DefaultRandomizationFactor,
		// Multiplier:          backoff.DefaultMultiplier,
		// MaxInterval:         30 * time.Second,
		// MaxElapsedTime:      5 * time.Minute,
	}
}

// Config defines configuration for Splunk exporter.
type Config struct {
	confighttp.HTTPClientSettings `mapstructure:",squash"`
	exporterhelper.QueueSettings  `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings  `mapstructure:"retry_on_failure"`
	BreakerSettings               `mapstructure:"circuit_breaker"`

	FailoverEndpoint string `mapstructure:"failover_endpoint"`

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

	// MaxConnections is used to set a limit to the maximum idle HTTP connection the exporter can keep open. Defaults to 100.
	// Deprecated: use HTTPClientSettings.MaxIdleConns or HTTPClientSettings.MaxIdleConnsPerHost instead.
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

	// Maximum payload size, raw uncompressed. Default value is 5242880 bytes (5MiB).
	// Maximum allowed value is 838860800 (~ 800 MB).
	MaxEventSize uint `mapstructure:"max_event_size"`

	// App name is used to track telemetry information for Splunk App's using HEC by App name. Defaults to "OpenTelemetry Collector Contrib".
	SplunkAppName string `mapstructure:"splunk_app_name"`

	// App version is used to track telemetry information for Splunk App's using HEC by App version. Defaults to the current OpenTelemetry Collector Contrib build version.
	SplunkAppVersion string `mapstructure:"splunk_app_version"`
	// HecToOtelAttrs creates a mapping from attributes to HEC specific metadata: source, sourcetype, index and host.
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

	out, err = url.Parse(cfg.HTTPClientSettings.Endpoint)
	if err != nil {
		return out, err
	}
	if out.Path == "" || out.Path == "/" {
		out.Path = path.Join(out.Path, hecPath)
	}

	return
}

func (cfg *Config) getFailoverURL() (out *url.URL) {

	if cfg.FailoverEndpoint == "" {
		return nil
	}

	out, err := url.Parse(cfg.FailoverEndpoint)
	if err != nil {
		return nil
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
	if cfg.HTTPClientSettings.Endpoint == "" {
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

	if err := cfg.QueueSettings.Validate(); err != nil {
		return fmt.Errorf("sending_queue settings has invalid configuration: %w", err)
	}
	return nil
}
