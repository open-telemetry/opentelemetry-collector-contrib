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

const (
	otlpProto = "otlp_proto"
	otlpJSON  = "otlp_json"
)

var _ component.Component = (*otlpExtension)(nil)
var _ ptrace.Marshaler = (*otlpExtension)(nil)
var _ ptrace.Unmarshaler = (*otlpExtension)(nil)
var _ plog.Marshaler = (*otlpExtension)(nil)
var _ plog.Unmarshaler = (*otlpExtension)(nil)
var _ pmetric.Marshaler = (*otlpExtension)(nil)
var _ pmetric.Unmarshaler = (*otlpExtension)(nil)

type otlpExtension struct {
	config            *Config
	traceMarshaler    ptrace.Marshaler
	traceUnmarshaler  ptrace.Unmarshaler
	logMarshaler      plog.Marshaler
	logUnmarshaler    plog.Unmarshaler
	metricMarshaler   pmetric.Marshaler
	metricUnmarshaler pmetric.Unmarshaler
}

func (ex *otlpExtension) UnmarshalTraces(buf []byte) (ptrace.Traces, error) {
	return ex.traceUnmarshaler.UnmarshalTraces(buf)
}

func (ex *otlpExtension) MarshalTraces(traces ptrace.Traces) ([]byte, error) {
	return ex.traceMarshaler.MarshalTraces(traces)
}

func (ex *otlpExtension) UnmarshalMetrics(buf []byte) (pmetric.Metrics, error) {
	return ex.metricUnmarshaler.UnmarshalMetrics(buf)
}

func (ex *otlpExtension) MarshalMetrics(metrics pmetric.Metrics) ([]byte, error) {
	return ex.metricMarshaler.MarshalMetrics(metrics)
}

func (ex *otlpExtension) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	return ex.logUnmarshaler.UnmarshalLogs(buf)
}

func (ex *otlpExtension) MarshalLogs(logs plog.Logs) ([]byte, error) {
	return ex.logMarshaler.MarshalLogs(logs)
}

func (ex *otlpExtension) Start(_ context.Context, _ component.Host) error {

	protocol := ex.config.Protocol
	switch protocol {
	case otlpProto:
		ex.traceMarshaler = &ptrace.ProtoMarshaler{}
		ex.traceUnmarshaler = &ptrace.ProtoUnmarshaler{}
		ex.logMarshaler = &plog.ProtoMarshaler{}
		ex.logUnmarshaler = &plog.ProtoUnmarshaler{}
		ex.metricMarshaler = &pmetric.ProtoMarshaler{}
		ex.metricUnmarshaler = &pmetric.ProtoUnmarshaler{}
	case otlpJSON:
		ex.traceMarshaler = &ptrace.JSONMarshaler{}
		ex.traceUnmarshaler = &ptrace.JSONUnmarshaler{}
		ex.logMarshaler = &plog.JSONMarshaler{}
		ex.logUnmarshaler = &plog.JSONUnmarshaler{}
		ex.metricMarshaler = &pmetric.JSONMarshaler{}
		ex.metricUnmarshaler = &pmetric.JSONUnmarshaler{}
	default:
		return fmt.Errorf("unsupported protocol: %q", protocol)
	}
	return nil
}

func (ex *otlpExtension) Shutdown(_ context.Context) error {
	return nil
}
