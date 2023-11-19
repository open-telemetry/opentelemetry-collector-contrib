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
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/multierr"
)

const (
	scopeName = "go.opentelemetry.io/collector/processor/filterprocessor"

	metricFilteredDesc = "Number of metrics dropped by the filter processor"
)

var (
	processorTagKey     = tag.MustNewKey(typeStr)
	statMetricsFiltered = stats.Int64("metrics.filtered", metricFilteredDesc, stats.UnitDimensionless)
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
	}
}

type filterProcessorTelemetry struct {
	useOtel bool

	exportCtx context.Context

	processorAttr   []attribute.KeyValue
	droppedByFilter metric.Int64Counter
}

func newfilterProcessorTelemetry(set processor.CreateSettings) (*filterProcessorTelemetry, error) {
	processorID := set.ID.String()

	exportCtx, err := tag.New(context.Background(), tag.Insert(processorTagKey, processorID))
	if err != nil {
		return nil, err
	}

	fpt := &filterProcessorTelemetry{
		useOtel:       false,
		processorAttr: []attribute.KeyValue{attribute.String(typeStr, processorID)},
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
		metric.WithDescription(metricFilteredDesc),
		metric.WithUnit("1"),
	)
	errors = multierr.Append(errors, err)

	return errors
}

func (fpt *filterProcessorTelemetry) record(dropped int64) {
	if fpt.useOtel {
		fpt.recordWithOtel(dropped)
	} else {
		fpt.recordWithOC(dropped)
	}
}

func (fpt *filterProcessorTelemetry) recordWithOC(dropped int64) {
	stats.Record(fpt.exportCtx, statMetricsFiltered.M(dropped))
}

func (fpt *filterProcessorTelemetry) recordWithOtel(dropped int64) {
	fpt.droppedByFilter.Add(fpt.exportCtx, dropped, metric.WithAttributes(fpt.processorAttr...))
}
