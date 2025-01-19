// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/rabbitmqexporter"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type marshaler struct {
	logsMarshaler    plog.Marshaler
	tracesMarshaler  ptrace.Marshaler
	metricsMarshaler pmetric.Marshaler
}

func newMarshaler(encoding *component.ID, host component.Host) (*marshaler, error) {
	var (
		logsMarshaler    plog.Marshaler    = &plog.ProtoMarshaler{}
		tracesMarshaler  ptrace.Marshaler  = &ptrace.ProtoMarshaler{}
		metricsMarshaler pmetric.Marshaler = &pmetric.ProtoMarshaler{}
	)

	if encoding != nil {
		ext, ok := host.GetExtensions()[*encoding]
		if !ok {
			return nil, fmt.Errorf("unknown encoding %q", encoding)
		}

		logsMarshaler, _ = ext.(plog.Marshaler)
		tracesMarshaler, _ = ext.(ptrace.Marshaler)
		metricsMarshaler, _ = ext.(pmetric.Marshaler)
	}

	m := marshaler{
		logsMarshaler:    logsMarshaler,
		tracesMarshaler:  tracesMarshaler,
		metricsMarshaler: metricsMarshaler,
	}
	return &m, nil
}
