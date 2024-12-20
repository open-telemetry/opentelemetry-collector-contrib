// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqmessagereciever // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/rabbitmqmessagereciever"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type unmarshaler interface {
	plog.Unmarshaler
	ptrace.Unmarshaler
	pmetric.Unmarshaler
}

type otelUnmarshaler struct {
	logsMarshaler    plog.Unmarshaler
	tracesMarshaler  ptrace.Unmarshaler
	metricsMarshaler pmetric.Unmarshaler
}

func (o *otelUnmarshaler) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	return o.logsMarshaler.UnmarshalLogs(buf)
}

func (o *otelUnmarshaler) UnmarshalTraces(buf []byte) (ptrace.Traces, error) {
	return o.tracesMarshaler.UnmarshalTraces(buf)
}

func (o *otelUnmarshaler) UnmarshalMetrics(buf []byte) (pmetric.Metrics, error) {
	return o.metricsMarshaler.UnmarshalMetrics(buf)
}

func newUnmarshaler(encoding *component.ID, host component.Host) (unmarshaler, error) {
	return &otelUnmarshaler{}, nil
}
