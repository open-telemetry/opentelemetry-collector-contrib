// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/metadata"
)

type trigger int

const (
	triggerMetricDataPointsDropped trigger = iota
	triggerLogsDropped
	triggerSpansDropped
)

type filterProcessorTelemetry struct {
	exportCtx context.Context

	processorAttr []attribute.KeyValue

	telemetryBuilder *metadata.TelemetryBuilder
}

func newfilterProcessorTelemetry(set processor.Settings) (*filterProcessorTelemetry, error) {
	telemetryBuilder, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	return &filterProcessorTelemetry{
		processorAttr:    []attribute.KeyValue{attribute.String(metadata.Type.String(), set.ID.String())},
		exportCtx:        context.Background(),
		telemetryBuilder: telemetryBuilder,
	}, nil
}

func (fpt *filterProcessorTelemetry) record(trigger trigger, dropped int64) {
	switch trigger {
	case triggerMetricDataPointsDropped:
		fpt.telemetryBuilder.ProcessorFilterDatapointsFiltered.Add(fpt.exportCtx, dropped, metric.WithAttributes(fpt.processorAttr...))
	case triggerLogsDropped:
		fpt.telemetryBuilder.ProcessorFilterLogsFiltered.Add(fpt.exportCtx, dropped, metric.WithAttributes(fpt.processorAttr...))
	case triggerSpansDropped:
		fpt.telemetryBuilder.ProcessorFilterSpansFiltered.Add(fpt.exportCtx, dropped, metric.WithAttributes(fpt.processorAttr...))
	}
}
