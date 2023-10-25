// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jaegerencodingextension"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

var _ encoding.TracesUnmarshalerExtension = &jaegerExtension{}
var _ ptrace.Unmarshaler = &jaegerExtension{}
var _ ptrace.Marshaler = &jaegerExtension{}

type jaegerExtension struct {
	config      *Config
	unmarshaler ptrace.Unmarshaler
	marshaler   ptrace.Marshaler
}

func (e *jaegerExtension) UnmarshalTraces(buf []byte) (ptrace.Traces, error) {
	return e.unmarshaler.UnmarshalTraces(buf)
}

func (e *jaegerExtension) MarshalTraces(td ptrace.Traces) ([]byte, error) {
	if e.marshaler == nil {
		return nil, errors.New("unsupported encoding")
	}
	return e.marshaler.MarshalTraces(td)
}

func (e *jaegerExtension) Start(_ context.Context, _ component.Host) error {
	e.marshaler = nil
	switch e.config.Protocol {
	case JaegerProtocolProtobuf:
		e.unmarshaler = jaegerProtobufTrace{}
	case JaegerProtocolJson:
		e.unmarshaler = jaegerJsonTrace{}
	default:
		return fmt.Errorf("unsupported protocol: %q", e.config.Protocol)
	}
	return nil
}

func (e *jaegerExtension) Shutdown(_ context.Context) error {
	return nil
}
