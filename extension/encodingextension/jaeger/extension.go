// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaeger // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encodingextension/jaeger"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encodingextension"
)

var _ encodingextension.Extension = &jaegerExtension{}

type jaegerExtension struct {
	config *Config
	c      encodingextension.Trace
}

func (e *jaegerExtension) GetLogCodec() (encodingextension.Log, error) {
	return nil, errors.New("unimplemented")
}

func (e *jaegerExtension) GetMetricCodec() (encodingextension.Metric, error) {
	return nil, errors.New("unimplemented")
}

func (e *jaegerExtension) GetTraceCodec() (encodingextension.Trace, error) {
	return nil, errors.New("unimplemented")
}

func (e *jaegerExtension) Start(_ context.Context, _ component.Host) error {
	switch e.config.Protocol {
	case "protobuf":
		e.c = jaegerProtobufTrace{}
	case "thrift":

	}
	return nil
}

func (e *jaegerExtension) Shutdown(_ context.Context) error {
	return nil
}
