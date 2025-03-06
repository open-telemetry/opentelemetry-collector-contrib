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
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awscloudwatchmetricstreamsencodingextension/internal/metadata"
)

var _ encoding.MetricsUnmarshalerExtension = (*encodingExtension)(nil)

type encodingExtension struct {
	unmarshaller pmetric.Unmarshaler
}

func newExtension(cfg *Config, settings extension.Settings) (*encodingExtension, error) {
	switch cfg.Format {
	case formatJSON, formatOpenTelemetry10:
	default:
		// Format will have been validated by Config.Validate,
		// so we'll only get here if we haven't handled a valid
		// format.
		return nil, fmt.Errorf("unimplemented format %q", cfg.Format)
	}
	unmarshaller, err := newUnmarshaler(
		cfg.Format,
		metadata.Type.String(),
		settings.BuildInfo,
		settings.Logger,
	)
	if err != nil {
		return nil, err
	}
	return &encodingExtension{unmarshaller: unmarshaller}, nil
}

func (*encodingExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (*encodingExtension) Shutdown(_ context.Context) error {
	return nil
}

func (e *encodingExtension) UnmarshalMetrics(buf []byte) (pmetric.Metrics, error) {
	if e.unmarshaller == nil {
		return pmetric.Metrics{}, errors.New("no unmarshaler defined")
	}
	return e.unmarshaller.UnmarshalMetrics(buf)
}
