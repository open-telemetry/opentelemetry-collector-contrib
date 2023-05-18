// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter"

import (
	"errors"

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
}

var (
	ErrUnknownMarshaler = errors.New("unknown marshaler")
)

func NewMarshaler(mType MarshalerType, logger *zap.Logger) (marshaler, error) {
	marshaler := &s3Marshaler{logger: logger}
	switch mType {
	case OtlpJSON:
		marshaler.logsMarshaler = &plog.JSONMarshaler{}
		marshaler.tracesMarshaler = &ptrace.JSONMarshaler{}
		marshaler.metricsMarshaler = &pmetric.JSONMarshaler{}
		marshaler.fileFormat = "json"
	default:
		return nil, ErrUnknownMarshaler
	}
	return marshaler, nil
}
