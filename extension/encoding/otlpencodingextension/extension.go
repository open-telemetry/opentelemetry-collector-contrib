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

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

const (
	otlpProto = "otlp_proto"
	otlpJSON  = "otlp_json"
)

var (
	_ encoding.TracesMarshalerExtension    = (*otlpExtension)(nil)
	_ encoding.TracesUnmarshalerExtension  = (*otlpExtension)(nil)
	_ encoding.LogsMarshalerExtension      = (*otlpExtension)(nil)
	_ encoding.LogsUnmarshalerExtension    = (*otlpExtension)(nil)
	_ encoding.MetricsMarshalerExtension   = (*otlpExtension)(nil)
	_ encoding.MetricsUnmarshalerExtension = (*otlpExtension)(nil)
)

type otlpExtension struct {
	config            *Config
	traceMarshaler    ptrace.Marshaler
	traceUnmarshaler  ptrace.Unmarshaler
	logMarshaler      plog.Marshaler
	logUnmarshaler    plog.Unmarshaler
	metricMarshaler   pmetric.Marshaler
	metricUnmarshaler pmetric.Unmarshaler
}

func newExtension(config *Config) (*otlpExtension, error) {
	var ex *otlpExtension
	var err error
	protocol := config.Protocol
	switch protocol {
	case otlpProto:
		ex = &otlpExtension{
			config:            config,
			traceMarshaler:    &ptrace.ProtoMarshaler{},
			traceUnmarshaler:  &ptrace.ProtoUnmarshaler{},
			logMarshaler:      &plog.ProtoMarshaler{},
			logUnmarshaler:    &plog.ProtoUnmarshaler{},
			metricMarshaler:   &pmetric.ProtoMarshaler{},
			metricUnmarshaler: &pmetric.ProtoUnmarshaler{},
		}
	case otlpJSON:
		ex = &otlpExtension{
			config:            config,
			traceMarshaler:    &ptrace.JSONMarshaler{},
			traceUnmarshaler:  &ptrace.JSONUnmarshaler{},
			logMarshaler:      &plog.JSONMarshaler{},
			logUnmarshaler:    &plog.JSONUnmarshaler{},
			metricMarshaler:   &pmetric.JSONMarshaler{},
			metricUnmarshaler: &pmetric.JSONUnmarshaler{},
		}
	default:
		err = fmt.Errorf("unsupported protocol: %q", protocol)
	}

	return ex, err
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
	return nil
}

func (ex *otlpExtension) Shutdown(_ context.Context) error {
	return nil
}
