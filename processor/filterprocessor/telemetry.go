// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"

import (
	"context"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/multierr"
)

const (
	processorKey = "filter_processor"
	scopeName    = "go.opentelemetry.io/collector/processor/filterprocessor"
)

var (
	processorTagKey      = tag.MustNewKey(processorKey)
	statMetricsFiltered  = stats.Int64("metrics_filtered", "Number of metrics dropped by the filter processor", stats.UnitDimensionless)
	statMetricsProcessed = stats.Int64("metrics_processed", "Number of metrics processed by the filter processor", stats.UnitDimensionless)
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
			Name:        statMetricsProcessed.Name(),
			Measure:     statMetricsProcessed,
			Description: statMetricsProcessed.Description(),
			Aggregation: view.Count(),
			TagKeys:     processorTagKeys,
		},
	}
}

type filterProcessorTelemetry struct {
	useOtel bool

	exportCtx context.Context

	processorAttr     []attribute.KeyValue
	droppedByFilter   metric.Int64Counter
	processedByFilter metric.Int64Counter
}

func newfilterProcessorTelemetry(set component.TelemetrySettings) (*filterProcessorTelemetry, error) {
	id, _ := set.Resource.Attributes().Get("ID")

	exportCtx, err := tag.New(context.Background(), tag.Insert(processorTagKey, id.Str()))
	if err != nil {
		return nil, err
	}

	fpt := &filterProcessorTelemetry{
		useOtel:       false,
		processorAttr: []attribute.KeyValue{attribute.String(processorKey, id.Str())},
		exportCtx:     exportCtx,
	}

	if err = fpt.createOtelMetrics(set.MeterProvider); err != nil {
		return nil, err
	}

	return fpt, nil
}

func (fpt *filterProcessorTelemetry) createOtelMetrics(mp metric.MeterProvider) error {
	if !fpt.useOtel {
		return nil
	}

	var errors, err error
	meter := mp.Meter(scopeName)

	fpt.droppedByFilter, err = meter.Int64Counter(
		processorhelper.BuildCustomMetricName(typeStr, "metrics_filtered"),
		metric.WithDescription("Number of metrics dropped by the filter processor"),
		metric.WithUnit("1"),
	)
	errors = multierr.Append(errors, err)
	fpt.processedByFilter, err = meter.Int64Counter(
		processorhelper.BuildCustomMetricName(typeStr, "metrics_processed"),
		metric.WithDescription("Number of metrics processed by the filter processor"),
		metric.WithUnit("1"),
	)
	errors = multierr.Append(errors, err)

	return errors
}

func (fpt *filterProcessorTelemetry) record(dropped, total int64) {
	if fpt.useOtel {
		fpt.recordWithOtel(dropped, total)
	} else {
		fpt.recordWithOC(dropped, total)
	}
}

func (fpt *filterProcessorTelemetry) recordWithOC(dropped, total int64) {
	stats.Record(fpt.exportCtx, statMetricsFiltered.M(dropped))
	stats.Record(fpt.exportCtx, statMetricsProcessed.M(total))
}

func (fpt *filterProcessorTelemetry) recordWithOtel(dropped, total int64) {
	fpt.droppedByFilter.Add(fpt.exportCtx, dropped, metric.WithAttributes(fpt.processorAttr...))
	fpt.processedByFilter.Add(fpt.exportCtx, total, metric.WithAttributes(fpt.processorAttr...))
}
