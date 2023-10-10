// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jaegerencodingextension"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var _ ptrace.Marshaler = &jaegerExtension{}
var _ ptrace.Unmarshaler = &jaegerExtension{}

type jaegerExtension struct {
	config      *Config
	marshaler   ptrace.Marshaler
	unmarshaler ptrace.Unmarshaler
}

func (e *jaegerExtension) MarshalTraces(traces ptrace.Traces) ([]byte, error) {
	return e.marshaler.MarshalTraces(traces)
}

func (e *jaegerExtension) UnmarshalTraces(buf []byte) (ptrace.Traces, error) {
	return e.unmarshaler.UnmarshalTraces(buf)
}

func (e *jaegerExtension) Start(_ context.Context, _ component.Host) error {
	switch e.config.Protocol {
	case JaegerProtocolProtobuf:
		jaegerProto := jaegerProtobufTrace{}
		e.marshaler = jaegerProto
		e.unmarshaler = jaegerProto
	default:
		return fmt.Errorf("unsupported protocol: %q", e.config.Protocol)
	}
	return nil
}

func (e *jaegerExtension) Shutdown(_ context.Context) error {
	return nil
}
