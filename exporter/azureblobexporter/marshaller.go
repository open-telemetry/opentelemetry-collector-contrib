// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// Marshaler configuration used for marhsaling Protobuf
var tracesMarshalers = map[string]ptrace.Marshaler{
	formatTypeJSON:  &ptrace.JSONMarshaler{},
	formatTypeProto: &ptrace.ProtoMarshaler{},
}

var metricsMarshalers = map[string]pmetric.Marshaler{
	formatTypeJSON:  &pmetric.JSONMarshaler{},
	formatTypeProto: &pmetric.ProtoMarshaler{},
}

var logsMarshalers = map[string]plog.Marshaler{
	formatTypeJSON:  &plog.JSONMarshaler{},
	formatTypeProto: &plog.ProtoMarshaler{},
}

type marshaller struct {
	tracesMarshaler  ptrace.Marshaler
	metricsMarshaler pmetric.Marshaler
	logsMarshaler    plog.Marshaler
}

func newMarshaller(config *Config, host component.Host) (*marshaller, error) {
	if config.FormatType != formatTypeJSON && config.FormatType != formatTypeProto {
		return nil, fmt.Errorf("unsupported format type %q", config.FormatType)
	}
	marshaller := &marshaller{
		tracesMarshaler:  tracesMarshalers[config.FormatType],
		metricsMarshaler: metricsMarshalers[config.FormatType],
		logsMarshaler:    logsMarshalers[config.FormatType],
	}

	if config.Encodings.Logs != nil {
		encoding := host.GetExtensions()[*config.Encodings.Logs]
		if encoding == nil {
			return nil, fmt.Errorf("unknown encoding %q", config.Encodings.Logs)
		}
		// cast with ok to avoid panics.
		lm, ok := encoding.(plog.Marshaler)
		if !ok {
			return nil, fmt.Errorf("encoding %q does not support logs", *config.Encodings.Logs)
		}
		marshaller.logsMarshaler = lm
	}

	if config.Encodings.Metrics != nil {
		encoding := host.GetExtensions()[*config.Encodings.Metrics]
		if encoding == nil {
			return nil, fmt.Errorf("unknown encoding %q", config.Encodings.Metrics)
		}
		// cast with ok to avoid panics.
		mm, ok := encoding.(pmetric.Marshaler)
		if !ok {
			return nil, fmt.Errorf("encoding %q does not support metrics", *config.Encodings.Metrics)
		}
		marshaller.metricsMarshaler = mm
	}

	if config.Encodings.Traces != nil {
		encoding := host.GetExtensions()[*config.Encodings.Traces]
		if encoding == nil {
			return nil, fmt.Errorf("unknown encoding %q", config.Encodings.Traces)
		}
		// cast with ok to avoid panics.
		tm, ok := encoding.(ptrace.Marshaler)
		if !ok {
			return nil, fmt.Errorf("encoding %q does not support traces", *config.Encodings.Traces)
		}
		marshaller.tracesMarshaler = tm
	}

	return marshaller, nil
}

func (m *marshaller) marshalTraces(td ptrace.Traces) ([]byte, error) {
	if m.tracesMarshaler == nil {
		return nil, errors.New("traces are not supported by encoding")
	}
	return m.tracesMarshaler.MarshalTraces(td)
}

func (m *marshaller) marshalMetrics(md pmetric.Metrics) ([]byte, error) {
	if m.metricsMarshaler == nil {
		return nil, errors.New("metrics are not supported by encoding")
	}
	return m.metricsMarshaler.MarshalMetrics(md)
}

func (m *marshaller) marshalLogs(ld plog.Logs) ([]byte, error) {
	if m.logsMarshaler == nil {
		return nil, errors.New("logs are not supported by encoding")
	}
	return m.logsMarshaler.MarshalLogs(ld)
}
