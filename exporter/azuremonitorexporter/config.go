// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"

import (
	"time"

	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for Azure Monitor
type Config struct {
	QueueSettings          exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`
	Endpoint               string                          `mapstructure:"endpoint"`
	ConnectionString       configopaque.String             `mapstructure:"connection_string"`
	InstrumentationKey     configopaque.String             `mapstructure:"instrumentation_key"`
	MaxBatchSize           int                             `mapstructure:"maxbatchsize"`
	MaxBatchInterval       time.Duration                   `mapstructure:"maxbatchinterval"`
	SpanEventsEnabled      bool                            `mapstructure:"spaneventsenabled"`
	ShutdownTimeout        time.Duration                   `mapstructure:"shutdown_timeout"`
	CustomEventsEnabled    bool                            `mapstructure:"custom_events_enabled"`
	ExceptionEventsEnabled bool                            `mapstructure:"exception_events_enabled"`
}
