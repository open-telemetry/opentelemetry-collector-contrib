// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"

import (
	"context"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/metadata"
)

type trigger int

const (
	triggerMetricsDropped trigger = iota
	triggerLogsDropped
	triggerSpansDropped
)

var (
	typeStr             = metadata.Type
	processorTagKey     = tag.MustNewKey(typeStr)
	statMetricsFiltered = stats.Int64("metrics.filtered", "Number of metrics dropped by the filter processor", stats.UnitDimensionless)
	statLogsFiltered    = stats.Int64("logs.filtered", "Number of logs dropped by the filter processor", stats.UnitDimensionless)
	statSpansFiltered   = stats.Int64("spans.filtered", "Number of spans dropped by the filter processor", stats.UnitDimensionless)
)

func init() {
	// TODO: Find a way to handle the error.
	_ = view.Register(metricViews()...)
}

func metricViews() []*view.View {
	processorTagKeys := []tag.Key{processorTagKey}

	return []*view.View{
		{
			Name:        statMetricsFiltered.Name(),
			Measure:     statMetricsFiltered,
			Description: statMetricsFiltered.Description(),
			Aggregation: view.Count(),
			TagKeys:     processorTagKeys,
		},
		{
			Name:        statLogsFiltered.Name(),
			Measure:     statLogsFiltered,
			Description: statLogsFiltered.Description(),
			Aggregation: view.Count(),
			TagKeys:     processorTagKeys,
		},
		{
			Name:        statSpansFiltered.Name(),
			Measure:     statSpansFiltered,
			Description: statSpansFiltered.Description(),
			Aggregation: view.Count(),
			TagKeys:     processorTagKeys,
		},
	}
}

type filterProcessorTelemetry struct {
	exportCtx context.Context

	processorAttr []attribute.KeyValue
}

func newfilterProcessorTelemetry(set processor.CreateSettings) (*filterProcessorTelemetry, error) {
	processorID := set.ID.String()

	exportCtx, err := tag.New(context.Background(), tag.Insert(processorTagKey, processorID))
	if err != nil {
		return nil, err
	}
	fpt := &filterProcessorTelemetry{
		processorAttr: []attribute.KeyValue{attribute.String(typeStr, processorID)},
		exportCtx:     exportCtx,
	}

	return fpt, nil
}

func (fpt *filterProcessorTelemetry) record(trigger trigger, dropped int64) {
	var triggerMeasure *stats.Int64Measure
	switch trigger {
	case triggerMetricsDropped:
		triggerMeasure = statMetricsFiltered
	case triggerLogsDropped:
		triggerMeasure = statLogsFiltered
	case triggerSpansDropped:
		triggerMeasure = statSpansFiltered
	}

	stats.Record(fpt.exportCtx, triggerMeasure.M(dropped))
}
