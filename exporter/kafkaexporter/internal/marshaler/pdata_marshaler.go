// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package marshaler // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/marshaler"

import (
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var (
	_ LogsMarshaler    = pdataLogsMarshaler{}
	_ MetricsMarshaler = pdataMetricsMarshaler{}
	_ TracesMarshaler  = pdataTracesMarshaler{}
)

type pdataLogsMarshaler struct {
	marshaler plog.Marshaler
}

func NewPdataLogsMarshaler(m plog.Marshaler) LogsMarshaler {
	return pdataLogsMarshaler{marshaler: m}
}

func (p pdataLogsMarshaler) MarshalLogs(ld plog.Logs) ([]Message, error) {
	bts, err := p.marshaler.MarshalLogs(ld)
	if err != nil {
		return nil, err
	}
	return []Message{{Value: bts}}, nil
}

type pdataMetricsMarshaler struct {
	marshaler pmetric.Marshaler
}

func NewPdataMetricsMarshaler(m pmetric.Marshaler) MetricsMarshaler {
	return pdataMetricsMarshaler{marshaler: m}
}

func (p pdataMetricsMarshaler) MarshalMetrics(ld pmetric.Metrics) ([]Message, error) {
	bts, err := p.marshaler.MarshalMetrics(ld)
	if err != nil {
		return nil, err
	}
	return []Message{{Value: bts}}, nil
}

type pdataTracesMarshaler struct {
	marshaler ptrace.Marshaler
}

func NewPdataTracesMarshaler(m ptrace.Marshaler) TracesMarshaler {
	return pdataTracesMarshaler{marshaler: m}
}

func (p pdataTracesMarshaler) MarshalTraces(td ptrace.Traces) ([]Message, error) {
	bts, err := p.marshaler.MarshalTraces(td)
	if err != nil {
		return nil, err
	}
	return []Message{{Value: bts}}, nil
}
