// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowexporter // import "github.com/open-telemetry/otel-arrow/collector/exporter/otelarrowexporter"

import (
	"fmt"
	"time"

	"github.com/open-telemetry/otel-arrow/pkg/config"
	"google.golang.org/grpc"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for OTLP exporter.
type Config struct {
	exporterhelper.TimeoutSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`

	configgrpc.GRPCClientSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.

	// Arrow includes settings specific to OTel Arrow.
	Arrow ArrowSettings `mapstructure:"arrow"`

	// UserDialOptions cannot be configured via `mapstructure`
	// schemes.  This is useful for custom purposes where the
	// exporter is built and configured via code instead of yaml.
	// Uses include custom dialer, custom user-agent, etc.
	UserDialOptions []grpc.DialOption `mapstructure:"-"`
}

// ArrowSettings includes whether Arrow is enabled and the number of
// concurrent Arrow streams.
type ArrowSettings struct {
	// Disabled prevents registering the OTel Arrow service.
	Disabled bool `mapstructure:"disabled"`

	// NumStreams determines the number of OTel Arrow streams.
	NumStreams int `mapstructure:"num_streams"`

	// DisableDowngrade prevents this exporter from fallback back to
	// standard OTLP.
	DisableDowngrade bool `mapstructure:"disable_downgrade"`

	// EnableMixedSignals allows the use of multi-signal streams.
	EnableMixedSignals bool `mapstructure:"enable_mixed_signals"`

	// MaxStreamLifetime should be set to less than the value of
	// grpc: keepalive: max_connection_age_grace plus the timeout.
	MaxStreamLifetime time.Duration `mapstructure:"max_stream_lifetime"`

	// PayloadCompression is applied on the Arrow IPC stream
	// internally and may have different results from using
	// gRPC-level compression.  This is disabled by default, since
	// gRPC-level compression is enabled by default.  This can be
	// set to "zstd" to turn on Arrow-Zstd compression.
	PayloadCompression configcompression.CompressionType `mapstructure:"payload_compression"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	if err := cfg.QueueSettings.Validate(); err != nil {
		return fmt.Errorf("queue settings has invalid configuration: %w", err)
	}
	if err := cfg.Arrow.Validate(); err != nil {
		return fmt.Errorf("arrow settings has invalid configuration: %w", err)
	}

	return nil
}

// Validate returns an error when the number of streams is less than 1.
func (cfg *ArrowSettings) Validate() error {
	if cfg.NumStreams < 1 {
		return fmt.Errorf("stream count must be > 0: %d", cfg.NumStreams)
	}

	if cfg.MaxStreamLifetime.Seconds() < float64(1) {
		return fmt.Errorf("max stream life must be > 0: %d", cfg.MaxStreamLifetime)
	}

	// The cfg.PayloadCompression field is validated by the underlying library,
	// but we only support Zstd or none.
	switch cfg.PayloadCompression {
	case "none", "", configcompression.Zstd:
	default:
		return fmt.Errorf("unsupported payload compression: %s", cfg.PayloadCompression)
	}
	return nil
}

func (cfg *ArrowSettings) ToArrowProducerOptions() (arrowOpts []config.Option) {
	switch cfg.PayloadCompression {
	case configcompression.Zstd:
		arrowOpts = append(arrowOpts, config.WithZstd())
	case "none", "":
		arrowOpts = append(arrowOpts, config.WithNoZstd())
	default:
		// Should have failed in validate, nothing we can do.
	}
	return
}
