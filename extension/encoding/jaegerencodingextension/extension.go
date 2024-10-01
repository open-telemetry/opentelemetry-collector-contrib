// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jaegerencodingextension"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

var _ encoding.TracesUnmarshalerExtension = &jaegerExtension{}
var _ ptrace.Unmarshaler = &jaegerExtension{}

type jaegerExtension struct {
	config      *Config
	unmarshaler ptrace.Unmarshaler
}

func (e *jaegerExtension) UnmarshalTraces(buf []byte) (ptrace.Traces, error) {
	return e.unmarshaler.UnmarshalTraces(buf)
}

func (e *jaegerExtension) Start(_ context.Context, _ component.Host) error {
	switch e.config.Protocol {
	case JaegerProtocolProtobuf:
		e.unmarshaler = jaegerProtobufTrace{}
	case JaegerProtocolJSON:
		e.unmarshaler = jaegerJSONTrace{}
	default:
		return fmt.Errorf("unsupported protocol: %q", e.config.Protocol)
	}
	return nil
}

func (e *jaegerExtension) Shutdown(_ context.Context) error {
	return nil
}
