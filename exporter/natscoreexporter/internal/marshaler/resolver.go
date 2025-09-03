// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package marshaler // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter/internal/marshaler"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type BuiltinMarshalerName string

const (
	OtlpProtoBuiltinMarshalerName BuiltinMarshalerName = "otlp_proto"
	OtlpJSONBuiltinMarshalerName  BuiltinMarshalerName = "otlp_json"
)

type genericBuiltinMarshaler struct {
	logsMarshaler    plog.Marshaler
	metricsMarshaler pmetric.Marshaler
	tracesMarshaler  ptrace.Marshaler
}

func (g *genericBuiltinMarshaler) MarshalLogs(ld plog.Logs) ([]byte, error) {
	return g.logsMarshaler.MarshalLogs(ld)
}

func (g *genericBuiltinMarshaler) MarshalMetrics(md pmetric.Metrics) ([]byte, error) {
	return g.metricsMarshaler.MarshalMetrics(md)
}

func (g *genericBuiltinMarshaler) MarshalTraces(td ptrace.Traces) ([]byte, error) {
	return g.tracesMarshaler.MarshalTraces(td)
}

type builtinMarshalerResolver struct {
	genericMarshaler GenericMarshaler
}

func (r *builtinMarshalerResolver) Resolve(host component.Host) (GenericMarshaler, error) {
	return r.genericMarshaler, nil
}

var _ Resolver = (*builtinMarshalerResolver)(nil)

func NewBuiltinMarshalerResolver(builtinMarshalerName BuiltinMarshalerName) (Resolver, error) {
	var genericMarshaler GenericMarshaler
	switch builtinMarshalerName {
	case OtlpProtoBuiltinMarshalerName:
		genericMarshaler = &genericBuiltinMarshaler{
			logsMarshaler:    &plog.ProtoMarshaler{},
			metricsMarshaler: &pmetric.ProtoMarshaler{},
			tracesMarshaler:  &ptrace.ProtoMarshaler{},
		}
	case OtlpJSONBuiltinMarshalerName:
		genericMarshaler = &genericBuiltinMarshaler{
			logsMarshaler:    &plog.JSONMarshaler{},
			metricsMarshaler: &pmetric.JSONMarshaler{},
			tracesMarshaler:  &ptrace.JSONMarshaler{},
		}
	default:
		return nil, fmt.Errorf("unsupported built-in marshaler: %s", builtinMarshalerName)
	}
	return &builtinMarshalerResolver{
		genericMarshaler: genericMarshaler,
	}, nil
}

type encodingExtensionResolver struct {
	id component.ID
}

func (r *encodingExtensionResolver) Resolve(host component.Host) (GenericMarshaler, error) {
	encodingExtension, ok := host.GetExtensions()[r.id]
	if !ok {
		return nil, fmt.Errorf("encoding extension not found: %s", r.id)
	}
	return encodingExtension, nil
}

var _ Resolver = (*encodingExtensionResolver)(nil)

func NewEncodingExtensionResolver(encodingExtensionName string) (Resolver, error) {
	var id component.ID
	if err := id.UnmarshalText([]byte(encodingExtensionName)); err != nil {
		return nil, fmt.Errorf("failed to unmarshal encoding extension name: %w", err)
	}

	return &encodingExtensionResolver{
		id: id,
	}, nil
}
