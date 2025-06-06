// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opsrampmetricsexporter

import (
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

// Config defines configuration for the OpsRamp Metrics exporter.
type Config struct {
	// QueueSettings is a wrapper around QueueConfig for queue settings
	QueueSettings exporterhelper.QueueConfig `mapstructure:"sending_queue"`

	// RetrySettings defines configuration for retrying batches in case of export failure.
	RetrySettings configretry.BackOffConfig `mapstructure:"retry_on_failure"`

	// TimeoutSettings defines configuration for timeout when exporting.
	TimeoutSettings exporterhelper.TimeoutConfig `mapstructure:",squash"`

	// AddMetricSuffixes controls whether metric suffixes are added for metric types like histograms
	// and summaries (e.g. "_count", "_sum", "_bucket"). Default is true.
	AddMetricSuffixes bool `mapstructure:"add_metric_suffixes"`

	// ResourceToTelemetrySettings configures the option to convert resource attributes to metric labels.
	ResourceToTelemetrySettings resourcetotelemetry.Settings `mapstructure:"resource_to_telemetry_conversion"`
}
