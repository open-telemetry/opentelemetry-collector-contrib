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
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/xstream"
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

// NewMetricsDecoder fulfills the streaming contract, however, does not implement true streaming.
// todo : streaming has to be implemented by JSON & OTLP formats individually.
func (e *encodingExtension) NewMetricsDecoder(reader io.Reader, _ ...encoding.DecoderOption) (encoding.MetricsDecoder, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	isEOF := false
	metrics, err := e.UnmarshalMetrics(data)
	return xstream.MetricsDecoderFunc(func() (pmetric.Metrics, error) {
		if isEOF {
			return pmetric.Metrics{}, io.EOF
		}

		if err != nil {
			return metrics, err
		}

		isEOF = true
		return metrics, nil
	}), err
}
