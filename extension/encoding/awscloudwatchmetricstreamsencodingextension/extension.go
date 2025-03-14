// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricstreamsencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awscloudwatchmetricstreamsencodingextension"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

var errEmptyRecord = errors.New("0 metrics were extracted from the record")

var _ encoding.MetricsUnmarshalerExtension = (*encodingExtension)(nil)

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
				logger:    settings.Logger,
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
