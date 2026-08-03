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

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter/internal/metadata"
)

// lineDelimitedLogsMarshaler is optionally implemented by encoding extensions whose marshaled
// logs output already uses newline as its record separator. Such output is appended to the file
// newline-delimited rather than length-prefixed, so the result stays readable by standard
// tooling. Encodings that do not implement it keep length-prefix framing.
type lineDelimitedLogsMarshaler interface {
	LogsLineDelimited() bool
}

// Marshaler configuration used for marshaling Protobuf
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

	// encodingLineDelimited is true when the configured encoding marshals records
	// newline-separated, so length-prefix framing is unnecessary.
	encodingLineDelimited bool
}

func newMarshaller(conf *Config, host component.Host) (*marshaller, error) {
	// When native compression is enabled, skip message-level compression
	// since the compressingWriter handles it at the file stream level.
	compression := conf.Compression
	compressor := buildCompressor(conf.Compression)
	if conf.Compression != "" && metadata.ExporterFileNativeCompressionFeatureGate.IsEnabled() {
		compression = ""
		compressor = noneCompress
	}

	if conf.Encoding != nil {
		encodingExt := host.GetExtensions()[*conf.Encoding]
		if encodingExt == nil {
			return nil, fmt.Errorf("unknown encoding %q", conf.Encoding)
		}
		// cast with ok to avoid panics.
		tm, _ := encodingExt.(ptrace.Marshaler)
		mm, _ := encodingExt.(pmetric.Marshaler)
		lm, _ := encodingExt.(plog.Marshaler)
		pm, _ := encodingExt.(pprofile.Marshaler)
		// A single export function is shared across every signal (see the sharedcomponent use
		// in factory.go), so newline framing is only safe when logs is the only signal this
		// encoding marshals. Drop the extra conditions once framing is chosen per signal.
		lineDelimited := false
		if ldm, ok := encodingExt.(lineDelimitedLogsMarshaler); ok && ldm.LogsLineDelimited() {
			lineDelimited = tm == nil && mm == nil && pm == nil
		}
		return &marshaller{
			tracesMarshaler:       tm,
			metricsMarshaler:      mm,
			logsMarshaler:         lm,
			profilesMarshaler:     pm,
			compression:           compression,
			compressor:            compressor,
			encodingLineDelimited: lineDelimited,
		}, nil
	}
	return &marshaller{
		formatType:        conf.FormatType,
		tracesMarshaler:   tracesMarshalers[conf.FormatType],
		metricsMarshaler:  metricsMarshalers[conf.FormatType],
		logsMarshaler:     logsMarshalers[conf.FormatType],
		profilesMarshaler: profilesMarshalers[conf.FormatType],
		compression:       compression,
		compressor:        compressor,
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
