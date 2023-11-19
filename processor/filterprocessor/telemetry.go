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

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/metadata"
)

const (
	scopeName = "go.opentelemetry.io/collector/processor/filterprocessor"

	metricsFilteredDesc = "Number of metrics dropped by the filter processor"
	logsFilteredDesc    = "Number of logs dropped by the filter processor"
	spansFilteredDesc   = "Number of spans dropped by the filter processor"
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
	statMetricsFiltered = stats.Int64("metrics.filtered", metricsFilteredDesc, stats.UnitDimensionless)
	statLogsFiltered    = stats.Int64("logs.filtered", logsFilteredDesc, stats.UnitDimensionless)
	statSpansFiltered   = stats.Int64("spans.filtered", spansFilteredDesc, stats.UnitDimensionless)
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
	useOtel bool

	exportCtx context.Context

	processorAttr []attribute.KeyValue

	metricsFiltered metric.Int64Counter
	logsFiltered    metric.Int64Counter
	spansFiltered   metric.Int64Counter
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

	fpt.metricsFiltered, err = meter.Int64Counter(
		processorhelper.BuildCustomMetricName(typeStr, "metrics_filtered"),
		metric.WithDescription(metricsFilteredDesc),
		metric.WithUnit("1"),
	)
	errors = multierr.Append(errors, err)
	fpt.logsFiltered, err = meter.Int64Counter(
		processorhelper.BuildCustomMetricName(typeStr, "logs_filtered"),
		metric.WithDescription(logsFilteredDesc),
		metric.WithUnit("1"),
	)
	errors = multierr.Append(errors, err)
	fpt.spansFiltered, err = meter.Int64Counter(
		processorhelper.BuildCustomMetricName(typeStr, "spans_filtered"),
		metric.WithDescription(spansFilteredDesc),
		metric.WithUnit("1"),
	)
	errors = multierr.Append(errors, err)

	return errors
}

func (fpt *filterProcessorTelemetry) record(trigger trigger, dropped int64) {
	if fpt.useOtel {
		fpt.recordWithOtel(trigger, dropped)
	} else {
		fpt.recordWithOC(trigger, dropped)
	}
}

func (fpt *filterProcessorTelemetry) recordWithOC(trigger trigger, dropped int64) {
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

func (fpt *filterProcessorTelemetry) recordWithOtel(trigger trigger, dropped int64) {
	switch trigger {
	case triggerMetricsDropped:
		fpt.metricsFiltered.Add(fpt.exportCtx, dropped, metric.WithAttributes(fpt.processorAttr...))
	case triggerLogsDropped:
		fpt.logsFiltered.Add(fpt.exportCtx, dropped, metric.WithAttributes(fpt.processorAttr...))
	case triggerSpansDropped:
		fpt.spansFiltered.Add(fpt.exportCtx, dropped, metric.WithAttributes(fpt.processorAttr...))
	}
}
