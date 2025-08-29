// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mqttexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mqttexporter"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type marshaler struct {
	tracesMarshaler  ptrace.Marshaler
	metricsMarshaler pmetric.Marshaler
	logsMarshaler    plog.Marshaler
}

func newMarshaler(encodingExtensionID *component.ID, host component.Host) (*marshaler, error) {
	var tracesMarshaler ptrace.Marshaler
	var metricsMarshaler pmetric.Marshaler
	var logsMarshaler plog.Marshaler

	if encodingExtensionID != nil {
		extensions := host.GetExtensions()
		if extension, ok := extensions[*encodingExtensionID]; ok {
			if tracesExtension, ok := extension.(ptrace.Marshaler); ok {
				tracesMarshaler = tracesExtension
			}
			if metricsExtension, ok := extension.(pmetric.Marshaler); ok {
				metricsMarshaler = metricsExtension
			}
			if logsExtension, ok := extension.(plog.Marshaler); ok {
				logsMarshaler = logsExtension
			}
		}
	}

	if tracesMarshaler == nil {
		tracesMarshaler = &ptrace.ProtoMarshaler{}
	}
	if metricsMarshaler == nil {
		metricsMarshaler = &pmetric.ProtoMarshaler{}
	}
	if logsMarshaler == nil {
		logsMarshaler = &plog.ProtoMarshaler{}
	}

	return &marshaler{
		tracesMarshaler:  tracesMarshaler,
		metricsMarshaler: metricsMarshaler,
		logsMarshaler:    logsMarshaler,
	}, nil
}
