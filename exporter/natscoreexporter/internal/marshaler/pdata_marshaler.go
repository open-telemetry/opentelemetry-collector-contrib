// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package marshaler // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter/internal/marshaler"

import (
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type pdataLogsMarshaler struct {
	marshaler plog.Marshaler
}

func (m *pdataLogsMarshaler) Marshal(data plog.Logs) ([]byte, error) {
	return m.marshaler.MarshalLogs(data)
}

var _ Marshaler[plog.Logs] = &pdataLogsMarshaler{}

func NewOtlpProtoLogsMarshaler() (Marshaler[plog.Logs], error) {
	return &pdataLogsMarshaler{
		marshaler: &plog.ProtoMarshaler{},
	}, nil
}

var _ NewMarshalerFunc[plog.Logs] = NewOtlpProtoLogsMarshaler

func NewOtlpJSONLogsMarshaler() (Marshaler[plog.Logs], error) {
	return &pdataLogsMarshaler{
		marshaler: &plog.JSONMarshaler{},
	}, nil
}

var _ NewMarshalerFunc[plog.Logs] = NewOtlpJSONLogsMarshaler

type pdataMetricsMarshaler struct {
	marshaler pmetric.Marshaler
}

var _ Marshaler[pmetric.Metrics] = &pdataMetricsMarshaler{}

func (m *pdataMetricsMarshaler) Marshal(data pmetric.Metrics) ([]byte, error) {
	return m.marshaler.MarshalMetrics(data)
}

func NewOtlpProtoMetricsMarshaler() (Marshaler[pmetric.Metrics], error) {
	return &pdataMetricsMarshaler{
		marshaler: &pmetric.ProtoMarshaler{},
	}, nil
}

var _ NewMarshalerFunc[pmetric.Metrics] = NewOtlpProtoMetricsMarshaler

func NewOtlpJSONMetricsMarshaler() (Marshaler[pmetric.Metrics], error) {
	return &pdataMetricsMarshaler{
		marshaler: &pmetric.JSONMarshaler{},
	}, nil
}

var _ NewMarshalerFunc[pmetric.Metrics] = NewOtlpJSONMetricsMarshaler

type pdataTracesMarshaler struct {
	marshaler ptrace.Marshaler
}

var _ Marshaler[ptrace.Traces] = &pdataTracesMarshaler{}

func (m *pdataTracesMarshaler) Marshal(data ptrace.Traces) ([]byte, error) {
	return m.marshaler.MarshalTraces(data)
}

func NewOtlpProtoTracesMarshaler() (Marshaler[ptrace.Traces], error) {
	return &pdataTracesMarshaler{
		marshaler: &ptrace.ProtoMarshaler{},
	}, nil
}

var _ NewMarshalerFunc[ptrace.Traces] = NewOtlpProtoTracesMarshaler

func NewOtlpJSONTracesMarshaler() (Marshaler[ptrace.Traces], error) {
	return &pdataTracesMarshaler{
		marshaler: &ptrace.JSONMarshaler{},
	}, nil
}

var _ NewMarshalerFunc[ptrace.Traces] = NewOtlpJSONTracesMarshaler
