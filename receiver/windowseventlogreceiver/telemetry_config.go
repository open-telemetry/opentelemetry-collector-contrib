// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windowseventlogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver"

// MetricConfig controls whether a single telemetry metric is emitted.
type MetricConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	_       struct{} // avoids unkeyed_literal_initialization
}

// MetricsConfig holds the per-metric enable/disable flags for the
// receiver's internal telemetry metrics.
type MetricsConfig struct {
	ReceiverWindowsEventLogBatchSize    MetricConfig `mapstructure:"receiver_windows_event_log_batch_size"`
	ReceiverWindowsEventLogChannelSize  MetricConfig `mapstructure:"receiver_windows_event_log_channel_size"`
	ReceiverWindowsEventLogEventSize    MetricConfig `mapstructure:"receiver_windows_event_log_event_size"`
	ReceiverWindowsEventLogLag          MetricConfig `mapstructure:"receiver_windows_event_log_lag"`
	ReceiverWindowsEventLogMissedEvents MetricConfig `mapstructure:"receiver_windows_event_log_missed_events"`
	_                                   struct{}     // avoids unkeyed_literal_initialization
}

// TelemetryConfig is the top-level config block for receiver telemetry settings.
type TelemetryConfig struct {
	Metrics MetricsConfig `mapstructure:"metrics"`
	_       struct{}      // avoids unkeyed_literal_initialization
}
