// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package marshaler // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter/internal/marshaler"

import (
	"errors"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func PickMarshalLogs(genericMarshaler GenericMarshaler) (MarshalFunc[plog.Logs], error) {
	logsMarshaler, ok := genericMarshaler.(plog.Marshaler)
	if !ok {
		return nil, errors.New("genericMarshaler does not implement plog.Marshaler")
	}
	return logsMarshaler.MarshalLogs, nil
}

var _ PickFunc[plog.Logs] = PickMarshalLogs

func PickMarshalMetrics(genericMarshaler GenericMarshaler) (MarshalFunc[pmetric.Metrics], error) {
	metricsMarshaler, ok := genericMarshaler.(pmetric.Marshaler)
	if !ok {
		return nil, errors.New("genericMarshaler does not implement pmetric.Marshaler")
	}
	return metricsMarshaler.MarshalMetrics, nil
}

var _ PickFunc[pmetric.Metrics] = PickMarshalMetrics

func PickMarshalTraces(genericMarshaler GenericMarshaler) (MarshalFunc[ptrace.Traces], error) {
	tracesMarshaler, ok := genericMarshaler.(ptrace.Marshaler)
	if !ok {
		return nil, errors.New("genericMarshaler does not implement ptrace.Marshaler")
	}
	return tracesMarshaler.MarshalTraces, nil
}

var _ PickFunc[ptrace.Traces] = PickMarshalTraces
