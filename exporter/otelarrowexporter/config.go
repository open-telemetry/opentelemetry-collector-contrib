// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter"

import (
	"fmt"
	"strings"
	"time"

	"github.com/open-telemetry/otel-arrow/pkg/config"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"google.golang.org/grpc"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter/internal/arrow"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/compression/zstd"
)

// Config defines configuration for OTLP exporter.
type Config struct {
	// Timeout, Retry, Queue, and gRPC client settings are
	// inherited from exporterhelper using field names
	// intentionally identical to the core OTLP exporter.

	TimeoutSettings exporterhelper.TimeoutConfig    `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	QueueSettings   exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`

	RetryConfig configretry.BackOffConfig `mapstructure:"retry_on_failure"`

	configgrpc.ClientConfig `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.

	// Experimental: This configuration is at the early stage of development and may change without backward compatibility
	// until https://github.com/open-telemetry/opentelemetry-collector/issues/8122 is resolved
	BatcherConfig exporterhelper.BatcherConfig `mapstructure:"batcher"` //nolint:staticcheck

	// Arrow includes settings specific to OTel Arrow.
	Arrow ArrowConfig `mapstructure:"arrow"`

	// UserDialOptions cannot be configured via `mapstructure`
	// schemes.  This is useful for custom purposes where the
	// exporter is built and configured via code instead of yaml.
	// Uses include custom dialer, custom user-agent, etc.
	UserDialOptions []grpc.DialOption `mapstructure:"-"`

	// MetadataKeys is a list of client.Metadata keys that will be
	// used to form distinct exporters.  If this setting is empty,
	// a single exporter instance will be used.  When this setting
	// is not empty, one exporter will be used per distinct
	// combination of values for the listed metadata keys.
	//
	// Empty value and unset metadata are treated as distinct cases.
	//
	// Entries are case-insensitive.  Duplicated entries will
	// trigger a validation error.
	MetadataKeys []string `mapstructure:"metadata_keys"`

	// MetadataCardinalityLimit indicates the maximum number of
	// exporter instances that will be created through a distinct
	// combination of MetadataKeys.
	MetadataCardinalityLimit uint32 `mapstructure:"metadata_cardinality_limit"`
}

// ArrowConfig includes whether Arrow is enabled and the number of
// concurrent Arrow streams.
type ArrowConfig struct {
	// NumStreams determines the number of OTel Arrow streams.
	NumStreams int `mapstructure:"num_streams"`

	// MaxStreamLifetime should be set to less than the value of
	// grpc: keepalive: max_connection_age_grace plus the timeout.
	MaxStreamLifetime time.Duration `mapstructure:"max_stream_lifetime"`

	// Zstd settings apply to OTel-Arrow use of gRPC specifically.
	// Note that when multiple Otel-Arrow exporters are configured
	// their settings will be applied in arbitrary order.
	// Identical Zstd settings are recommended when multiple
	// OTel-Arrow exporters are in use.
	Zstd zstd.EncoderConfig `mapstructure:"zstd"`

	// PayloadCompression is applied on the Arrow IPC stream
	// internally and may have different results from using
	// gRPC-level compression.  This is disabled by default, since
	// gRPC-level compression is enabled by default.  This can be
	// set to "zstd" to turn on Arrow-Zstd compression.
	// Note that `Zstd` applies to gRPC, not Arrow compression.
	PayloadCompression configcompression.Type `mapstructure:"payload_compression"`

	// Disabled prevents using OTel-Arrow streams.  The exporter
	// falls back to standard OTLP.
	Disabled bool `mapstructure:"disabled"`

	// DisableDowngrade prevents this exporter from fallback back
	// to standard OTLP.  If the Arrow service is unavailable, it
	// will retry and/or fail.
	DisableDowngrade bool `mapstructure:"disable_downgrade"`

	// Prioritizer is a policy name for how load is distributed
	// across streams.
	Prioritizer arrow.PrioritizerName `mapstructure:"prioritizer"`
}

var _ component.Config = (*Config)(nil)

var _ xconfmap.Validator = (*ArrowConfig)(nil)

func (cfg *Config) Validate() error {
	err := cfg.Arrow.Validate()
	if err != nil {
		return err
	}

	uniq := map[string]bool{}
	for _, k := range cfg.MetadataKeys {
		l := strings.ToLower(k)
		if _, has := uniq[l]; has {
			return fmt.Errorf("duplicate entry in metadata_keys: %q (case-insensitive)", l)
		}
		uniq[l] = true
	}

	return nil
}

// Validate returns an error when the number of streams is less than 1.
func (cfg *ArrowConfig) Validate() error {
	if cfg.NumStreams < 1 {
		return fmt.Errorf("stream count must be > 0: %d", cfg.NumStreams)
	}

	if cfg.MaxStreamLifetime.Seconds() < 1 {
		return fmt.Errorf("max stream life must be >= 1s: %d", cfg.MaxStreamLifetime)
	}

	if err := cfg.Zstd.Validate(); err != nil {
		return fmt.Errorf("zstd encoder: invalid configuration: %w", err)
	}

	if err := cfg.Prioritizer.Validate(); err != nil {
		return fmt.Errorf("invalid prioritizer: %w", err)
	}

	// The cfg.PayloadCompression field is validated by the underlying library,
	// but we only support Zstd or none.
	switch cfg.PayloadCompression {
	case "none", "", configcompression.TypeZstd:
	default:
		return fmt.Errorf("unsupported payload compression: %s", cfg.PayloadCompression)
	}
	return nil
}

func (cfg *ArrowConfig) toArrowProducerOptions() (arrowOpts []config.Option) {
	switch cfg.PayloadCompression {
	case configcompression.TypeZstd:
		arrowOpts = append(arrowOpts, config.WithZstd())
	case "none", "":
		arrowOpts = append(arrowOpts, config.WithNoZstd())
	default:
		// Should have failed in validate, nothing we can do.
	}
	return
}
