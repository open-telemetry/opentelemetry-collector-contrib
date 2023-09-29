// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/otlpencodingextension"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var _ ptrace.Unmarshaler = &otlpjsonencoding{}
var _ plog.Unmarshaler = &otlpjsonencoding{}
var _ pmetric.Unmarshaler = &otlpjsonencoding{}
var _ pmetric.Marshaler = &otlpjsonencoding{}
var _ plog.Marshaler = &otlpjsonencoding{}
var _ ptrace.Marshaler = &otlpjsonencoding{}

type otlpjsonencoding struct {
	config             *Config
	logsUnmarshaler    plog.Unmarshaler
	metricsUnmarshaler pmetric.Unmarshaler
	tracesUnmarshaler  ptrace.Unmarshaler
	logsMarshaler      plog.Marshaler
	metricsMarshaler   pmetric.Marshaler
	tracesMarshaler    ptrace.Marshaler
}

func (e *otlpjsonencoding) MarshalLogs(logs plog.Logs) ([]byte, error) {
	return e.logsMarshaler.MarshalLogs(logs)
}

func (e *otlpjsonencoding) MarshalMetrics(metrics pmetric.Metrics) ([]byte, error) {
	return e.metricsMarshaler.MarshalMetrics(metrics)
}

func (e *otlpjsonencoding) MarshalTraces(traces ptrace.Traces) ([]byte, error) {
	return e.tracesMarshaler.MarshalTraces(traces)
}

func (e *otlpjsonencoding) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	return e.logsUnmarshaler.UnmarshalLogs(buf)
}

func (e *otlpjsonencoding) UnmarshalMetrics(buf []byte) (pmetric.Metrics, error) {
	return e.metricsUnmarshaler.UnmarshalMetrics(buf)
}

func (e *otlpjsonencoding) UnmarshalTraces(buf []byte) (ptrace.Traces, error) {
	return e.tracesUnmarshaler.UnmarshalTraces(buf)
}

func (e *otlpjsonencoding) Start(_ context.Context, _ component.Host) error {
	switch e.config.Protocol {
	case OTLPProtocolJSON:
		e.logsMarshaler = &plog.JSONMarshaler{}
		e.metricsMarshaler = &pmetric.JSONMarshaler{}
		e.tracesMarshaler = &ptrace.JSONMarshaler{}
		e.logsUnmarshaler = &plog.JSONUnmarshaler{}
		e.metricsUnmarshaler = &pmetric.JSONUnmarshaler{}
		e.tracesUnmarshaler = &ptrace.JSONUnmarshaler{}
	default:
		return fmt.Errorf("unsupported protocol: %q", e.config.Protocol)
	}
	return nil
}

func (e *otlpjsonencoding) Shutdown(_ context.Context) error {
	return nil
}
