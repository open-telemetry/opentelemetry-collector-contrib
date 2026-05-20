// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type marshaler interface {
	MarshalTraces(td ptrace.Traces) ([]byte, error)
	MarshalLogs(ld plog.Logs) ([]byte, error)
	MarshalMetrics(md pmetric.Metrics) ([]byte, error)
	format() string
	compressed() bool
}

var ErrUnknownMarshaler = errors.New("unknown marshaler")

func newMarshalerFromEncoding(encoding *component.ID, fileFormat string, host component.Host, logger *zap.Logger) (marshaler, error) {
	marshaler := &s3Marshaler{logger: logger}
	e, ok := host.GetExtensions()[*encoding]
	if !ok {
		return nil, fmt.Errorf("unknown encoding %q", encoding)
	}
	// cast with ok to avoid panics.
	marshaler.logsMarshaler, _ = e.(plog.Marshaler)
	marshaler.metricsMarshaler, _ = e.(pmetric.Marshaler)
	marshaler.tracesMarshaler, _ = e.(ptrace.Marshaler)
	marshaler.fileFormat = fileFormat
	marshaler.IsCompressed = false
	return marshaler, nil
}

func newMarshaler(mType MarshalerType, logger *zap.Logger) (marshaler, error) {
	marshaler := &s3Marshaler{logger: logger}
	switch mType {
	case OtlpProtobuf:
		marshaler.logsMarshaler = &plog.ProtoMarshaler{}
		marshaler.tracesMarshaler = &ptrace.ProtoMarshaler{}
		marshaler.metricsMarshaler = &pmetric.ProtoMarshaler{}
		marshaler.fileFormat = "binpb"
		marshaler.IsCompressed = false
	case OtlpJSON:
		marshaler.logsMarshaler = &plog.JSONMarshaler{}
		marshaler.tracesMarshaler = &ptrace.JSONMarshaler{}
		marshaler.metricsMarshaler = &pmetric.JSONMarshaler{}
		marshaler.fileFormat = "json"
		marshaler.IsCompressed = false
	case SumoIC:
		sumomarshaler := newSumoICMarshaler()
		marshaler.logsMarshaler = &sumomarshaler
		marshaler.fileFormat = "json"
		marshaler.IsCompressed = true
	case Body:
		exportbodyMarshaler := newbodyMarshaler()
		marshaler.logsMarshaler = &exportbodyMarshaler
		marshaler.fileFormat = exportbodyMarshaler.format()
		marshaler.IsCompressed = false
	default:
		return nil, ErrUnknownMarshaler
	}
	return marshaler, nil
}
