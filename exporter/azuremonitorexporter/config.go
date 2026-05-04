// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for Azure Monitor
type Config struct {
	QueueSettings          configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`
	ConnectionString       configopaque.String                                      `mapstructure:"connection_string"`
	InstrumentationKey     configopaque.String                                      `mapstructure:"instrumentation_key"`
	MaxBatchSize           int                                                      `mapstructure:"maxbatchsize"`
	MaxBatchInterval       time.Duration                                            `mapstructure:"maxbatchinterval"`
	SpanEventsEnabled      bool                                                     `mapstructure:"spaneventsenabled"`
	ShutdownTimeout        time.Duration                                            `mapstructure:"shutdown_timeout"`
	CustomEventsEnabled    bool                                                     `mapstructure:"custom_events_enabled"`
	ExceptionEventsEnabled bool                                                     `mapstructure:"exception_events_enabled"`
	// NonErrorHTTPStatusCodes lists HTTP status codes that should be reported as Success=true
	// on Application Insights RequestData (server spans) and RemoteDependencyData (client spans),
	// regardless of whether they fall outside the default 100-399 success range.
	NonErrorHTTPStatusCodes []int `mapstructure:"non_error_http_status_codes"`
	// AlignHTTPServerSpanSuccessWithOTelSpec, when true, marks any 4xx response on an HTTP
	// server span as Success=true on RequestData. Aligns with the OpenTelemetry HTTP semantic
	// conventions, which only flip server span status to Error on 5xx. Client spans are unaffected.
	AlignHTTPServerSpanSuccessWithOTelSpec bool                    `mapstructure:"align_http_server_span_success_with_otel_spec"`
	ClientConfig                           confighttp.ClientConfig `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
}

// Validate checks the configuration for invalid values.
func (c *Config) Validate() error {
	for _, code := range c.NonErrorHTTPStatusCodes {
		if code < 100 || code > 599 {
			return fmt.Errorf("non_error_http_status_codes: %d is not a valid HTTP status code (must be in [100, 599])", code)
		}
	}
	return nil
}
