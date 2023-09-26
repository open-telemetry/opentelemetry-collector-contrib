// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package text // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encodingextension/text"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encodingextension"
)

var _ encodingextension.Extension = &textExtension{}

type textExtension struct {
	config *Config
	c      *textLogCodec
}

func (e *textExtension) GetLogCodec() (encodingextension.Log, error) {
	return e.c, nil
}

func (e *textExtension) GetMetricCodec() (encodingextension.Metric, error) {
	return nil, errors.New("unimplemented")
}

func (e *textExtension) GetTraceCodec() (encodingextension.Trace, error) {
	return nil, errors.New("unimplemented")
}

func (e *textExtension) Start(_ context.Context, _ component.Host) error {
	var err error
	e.c, err = newLogCodec(e.config.encoding)
	return err
}

func (e *textExtension) Shutdown(_ context.Context) error {
	return nil
}
