// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"

import (
	"context"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/otel/attribute"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/metadata"
)

type trigger int

const (
	triggerMetricDataPointsDropped trigger = iota
	triggerLogsDropped
	triggerSpansDropped
)

var (
	typeStr                      = metadata.Type
	processorTagKey              = tag.MustNewKey(typeStr)
	statMetricDataPointsFiltered = stats.Int64("datapoints.filtered", "Number of metric data points dropped by the filter processor", stats.UnitDimensionless)
	statLogsFiltered             = stats.Int64("logs.filtered", "Number of logs dropped by the filter processor", stats.UnitDimensionless)
	statSpansFiltered            = stats.Int64("spans.filtered", "Number of spans dropped by the filter processor", stats.UnitDimensionless)
)

func init() {
	// TODO: Find a way to handle the error.
	_ = view.Register(metricViews()...)
}

func metricViews() []*view.View {
	processorTagKeys := []tag.Key{processorTagKey}

	return []*view.View{
		{
			Name:        processorhelper.BuildCustomMetricName(typeStr, statMetricDataPointsFiltered.Name()),
			Measure:     statMetricDataPointsFiltered,
			Description: statMetricDataPointsFiltered.Description(),
			Aggregation: view.Sum(),
			TagKeys:     processorTagKeys,
		},
		{
			Name:        processorhelper.BuildCustomMetricName(typeStr, statLogsFiltered.Name()),
			Measure:     statLogsFiltered,
			Description: statLogsFiltered.Description(),
			Aggregation: view.Sum(),
			TagKeys:     processorTagKeys,
		},
		{
			Name:        processorhelper.BuildCustomMetricName(typeStr, statSpansFiltered.Name()),
			Measure:     statSpansFiltered,
			Description: statSpansFiltered.Description(),
			Aggregation: view.Sum(),
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
	case triggerMetricDataPointsDropped:
		triggerMeasure = statMetricDataPointsFiltered
	case triggerLogsDropped:
		triggerMeasure = statLogsFiltered
	case triggerSpansDropped:
		triggerMeasure = statSpansFiltered
	}

	stats.Record(fpt.exportCtx, triggerMeasure.M(dropped))
}
