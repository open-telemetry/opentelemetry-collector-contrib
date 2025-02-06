// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"

import (
	"context"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type traceExporter struct {
	config           *Config
	transportChannel transportChannel
	logger           *zap.Logger
}

type traceVisitor struct {
	processed int
	err       error
	exporter  *traceExporter
}

// Called for each tuple of Resource, InstrumentationScope, and Span
func (v *traceVisitor) visit(
	resource pcommon.Resource,
	scope pcommon.InstrumentationScope,
	span ptrace.Span,
) (ok bool) {
	envelopes, err := spanToEnvelopes(resource, scope, span, v.exporter.config.SpanEventsEnabled, v.exporter.logger)
	if err != nil {
		// record the error and short-circuit
		v.err = consumererror.NewPermanent(err)
		return false
	}

	for _, envelope := range envelopes {
		envelope.IKey = string(v.exporter.config.InstrumentationKey)

		// This is a fire and forget operation
		v.exporter.transportChannel.Send(envelope)
	}

	// Flush the transport channel to force the telemetry to be sent
	v.exporter.transportChannel.Flush()
	v.processed++

	return true
}

func (exporter *traceExporter) onTraceData(_ context.Context, traceData ptrace.Traces) error {
	spanCount := traceData.SpanCount()
	if spanCount == 0 {
		return nil
	}

	visitor := &traceVisitor{exporter: exporter}
	accept(traceData, visitor)
	return visitor.err
}

// Returns a new instance of the trace exporter
func newTracesExporter(config *Config, transportChannel transportChannel, set exporter.Settings) (exporter.Traces, error) {
	exporter := &traceExporter{
		config:           config,
		transportChannel: transportChannel,
		logger:           set.Logger,
	}

	return exporterhelper.NewTraces(
		context.TODO(),
		set,
		config,
		exporter.onTraceData,
		exporterhelper.WithQueue(config.QueueSettings))
}
