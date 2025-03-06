// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricstreamsencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awscloudwatchmetricstreamsencodingextension"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

var _ encoding.MetricsUnmarshalerExtension = (*encodingExtension)(nil)

type encodingExtension struct {
	pmetric.Unmarshaler
}

func newExtension(cfg *Config) (*encodingExtension, error) {
	switch cfg.Format {
	case formatJSON:
		return &encodingExtension{Unmarshaler: formatJSONUnmarshaler{}}, nil
	case formatOpenTelemetry10:
		return &encodingExtension{Unmarshaler: formatOpenTelemetry10Unmarshaler{}}, nil
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

type formatJSONUnmarshaler struct{}

func (formatJSONUnmarshaler) UnmarshalMetrics([]byte) (pmetric.Metrics, error) {
	// TODO implement
	return pmetric.Metrics{}, fmt.Errorf("UnmarshalMetrics unimplemented for format %q", formatJSON)
}

type formatOpenTelemetry10Unmarshaler struct{}

func (formatOpenTelemetry10Unmarshaler) UnmarshalMetrics([]byte) (pmetric.Metrics, error) {
	// TODO implement
	return pmetric.Metrics{}, fmt.Errorf("UnmarshalMetrics unimplemented for format %q", formatOpenTelemetry10)
}
