package azureeventhubsexporter
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureeventhubsexporter"

import (
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type marshaller interface {
	MarshalTraces(td ptrace.Traces) ([]byte, error)
	MarshalLogs(ld plog.Logs) ([]byte, error)
	MarshalMetrics(md pmetric.Metrics) ([]byte, error)
	format() string
}

type protoMarshaller struct {
	tracesMarshaler  ptrace.Marshaler
	logsMarshaler    plog.Marshaler
	metricsMarshaler pmetric.Marshaler
}

func newProtoMarshaller() *protoMarshaller {
	return &protoMarshaller{
		tracesMarshaler:  &ptrace.ProtoMarshaler{},
		logsMarshaler:    &plog.ProtoMarshaler{},
		metricsMarshaler: &pmetric.ProtoMarshaler{},
	}
}

func (p *protoMarshaller) MarshalTraces(td ptrace.Traces) ([]byte, error) {
	return p.tracesMarshaler.MarshalTraces(td)
}

func (p *protoMarshaller) MarshalLogs(ld plog.Logs) ([]byte, error) {
	return p.logsMarshaler.MarshalLogs(ld)
}

func (p *protoMarshaller) MarshalMetrics(md pmetric.Metrics) ([]byte, error) {
	return p.metricsMarshaler.MarshalMetrics(md)
}

func (p *protoMarshaller) format() string {
	return formatTypeProto
}

type jsonMarshaller struct {
	tracesMarshaler  ptrace.Marshaler
	logsMarshaler    plog.Marshaler
	metricsMarshaler pmetric.Marshaler
}

func newJSONMarshaller() *jsonMarshaller {
	return &jsonMarshaller{
		tracesMarshaler:  &ptrace.JSONMarshaler{},
		logsMarshaler:    &plog.JSONMarshaler{},
		metricsMarshaler: &pmetric.JSONMarshaler{},
	}
}

func (j *jsonMarshaller) MarshalTraces(td ptrace.Traces) ([]byte, error) {
	return j.tracesMarshaler.MarshalTraces(td)
}

func (j *jsonMarshaller) MarshalLogs(ld plog.Logs) ([]byte, error) {
	return j.logsMarshaler.MarshalLogs(ld)
}

func (j *jsonMarshaller) MarshalMetrics(md pmetric.Metrics) ([]byte, error) {
	return j.metricsMarshaler.MarshalMetrics(md)
}

func (j *jsonMarshaller) format() string {
	return formatTypeJSON
}
