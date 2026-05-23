// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricstreamsencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awscloudwatchmetricstreamsencodingextension"

import (
	"context"
	"fmt"
	"io"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

var (
	_ encoding.MetricsUnmarshalerExtension = (*encodingExtension)(nil)
	_ encoding.MetricsDecoderExtension     = (*encodingExtension)(nil)
)

type encodingExtension struct {
	unmarshaler pmetric.Unmarshaler
	format      string
}

func newExtension(cfg *Config, settings extension.Settings) (*encodingExtension, error) {
	switch cfg.Format {
	case formatJSON:
		return &encodingExtension{
			unmarshaler: &formatJSONUnmarshaler{
				buildInfo: settings.BuildInfo,
			},
			format: formatJSON,
		}, nil
	case formatOpenTelemetry10:
		return &encodingExtension{
			unmarshaler: &formatOpenTelemetry10Unmarshaler{},
			format:      formatOpenTelemetry10,
		}, nil
	default:
		// Format will have been validated by Config.Validate,
		// so we'll only get here if we haven't handled a valid
		// format.
		return nil, fmt.Errorf("unimplemented format %q", cfg.Format)
	}
}

func (*encodingExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (*encodingExtension) Shutdown(_ context.Context) error {
	return nil
}

func (e *encodingExtension) UnmarshalMetrics(record []byte) (pmetric.Metrics, error) {
	metrics, err := e.unmarshaler.UnmarshalMetrics(record)
	if err != nil {
		return pmetric.Metrics{}, fmt.Errorf("failed to unmarshal metrics as '%s' format: %w", e.format, err)
	}
	return metrics, nil
}

// NewMetricsDecoder returns a MetricsDecoder if the underlying unmarshaler supports streaming.
// Caller must perform any decompression before passing the reader to the decoder.
// Implementations must utilize derived buffered readers as is.
func (e *encodingExtension) NewMetricsDecoder(reader io.Reader, options ...encoding.DecoderOption) (encoding.MetricsDecoder, error) {
	if u, ok := e.unmarshaler.(streamUnmarshal); ok {
		return u.NewMetricsDecoder(reader, options...)
	}

	return nil, fmt.Errorf("streaming not supported for format %q", e.format)
}

// metricsUnmarshal is an interface that's expected to be implemented by metrics format implementations.
type streamUnmarshal interface {
	pmetric.Unmarshaler
	NewMetricsDecoder(reader io.Reader, options ...encoding.DecoderOption) (encoding.MetricsDecoder, error)
}
