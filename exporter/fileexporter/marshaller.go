// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
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

var profilesMarshalers = map[string]pprofile.Marshaler{
	formatTypeJSON:  &pprofile.JSONMarshaler{},
	formatTypeProto: &pprofile.ProtoMarshaler{},
}

type marshaller struct {
	tracesMarshaler   ptrace.Marshaler
	metricsMarshaler  pmetric.Marshaler
	logsMarshaler     plog.Marshaler
	profilesMarshaler pprofile.Marshaler

	compression string
	compressor  compressFunc

	formatType string
}

func newMarshaller(conf *Config, host component.Host) (*marshaller, error) {
	if conf.Encoding != nil {
		encoding := host.GetExtensions()[*conf.Encoding]
		if encoding == nil {
			return nil, fmt.Errorf("unknown encoding %q", conf.Encoding)
		}
		// cast with ok to avoid panics.
		tm, _ := encoding.(ptrace.Marshaler)
		mm, _ := encoding.(pmetric.Marshaler)
		lm, _ := encoding.(plog.Marshaler)
		pm, _ := encoding.(pprofile.Marshaler)
		return &marshaller{
			tracesMarshaler:   tm,
			metricsMarshaler:  mm,
			logsMarshaler:     lm,
			profilesMarshaler: pm,
			compression:       conf.Compression,
			compressor:        buildCompressor(conf.Compression),
		}, nil
	}
	return &marshaller{
		formatType:        conf.FormatType,
		tracesMarshaler:   tracesMarshalers[conf.FormatType],
		metricsMarshaler:  metricsMarshalers[conf.FormatType],
		logsMarshaler:     logsMarshalers[conf.FormatType],
		profilesMarshaler: profilesMarshalers[conf.FormatType],
		compression:       conf.Compression,
		compressor:        buildCompressor(conf.Compression),
	}, nil
}

func (m *marshaller) marshalTraces(td ptrace.Traces) ([]byte, error) {
	if m.tracesMarshaler == nil {
		return nil, errors.New("traces are not supported by encoding")
	}
	buf, err := m.tracesMarshaler.MarshalTraces(td)
	if err != nil {
		return nil, err
	}
	buf = m.compressor(buf)
	return buf, nil
}

func (m *marshaller) marshalMetrics(md pmetric.Metrics) ([]byte, error) {
	if m.metricsMarshaler == nil {
		return nil, errors.New("metrics are not supported by encoding")
	}
	buf, err := m.metricsMarshaler.MarshalMetrics(md)
	if err != nil {
		return nil, err
	}
	buf = m.compressor(buf)
	return buf, nil
}

func (m *marshaller) marshalLogs(ld plog.Logs) ([]byte, error) {
	if m.logsMarshaler == nil {
		return nil, errors.New("logs are not supported by encoding")
	}
	buf, err := m.logsMarshaler.MarshalLogs(ld)
	if err != nil {
		return nil, err
	}
	buf = m.compressor(buf)
	return buf, nil
}

func (m *marshaller) marshalProfiles(pd pprofile.Profiles) ([]byte, error) {
	if m.profilesMarshaler == nil {
		return nil, errors.New("profiles are not supported by encoding")
	}
	buf, err := m.profilesMarshaler.MarshalProfiles(pd)
	if err != nil {
		return nil, err
	}
	buf = m.compressor(buf)
	return buf, nil
}
