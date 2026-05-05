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
	ReceiverWindowsEventLogBatchSize    MetricConfig `mapstructure:"receiver.windows_event_log.batch_size"`
	ReceiverWindowsEventLogChannelSize  MetricConfig `mapstructure:"receiver.windows_event_log.channel_size"`
	ReceiverWindowsEventLogEventSize    MetricConfig `mapstructure:"receiver.windows_event_log.event_size"`
	ReceiverWindowsEventLogLag          MetricConfig `mapstructure:"receiver.windows_event_log.lag"`
	ReceiverWindowsEventLogMissedEvents MetricConfig `mapstructure:"receiver.windows_event_log.missed_events"`
	_                                   struct{}     // avoids unkeyed_literal_initialization
}

// TelemetryConfig is the top-level config block for receiver telemetry settings.
type TelemetryConfig struct {
	Metrics MetricsConfig `mapstructure:"metrics"`
	_       struct{}      // avoids unkeyed_literal_initialization
}
