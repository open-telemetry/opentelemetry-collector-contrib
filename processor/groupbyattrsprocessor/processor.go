// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbyattrsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor/internal/metadata"
)

type groupByAttrsProcessor struct {
	logger           *zap.Logger
	groupByKeys      []string
	telemetryBuilder *metadata.TelemetryBuilder
}

// ProcessTraces process traces and groups traces by attribute.
func (gap *groupByAttrsProcessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	rss := td.ResourceSpans()
	tg := newTracesGroup()

	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)

		ilss := rs.ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			for k := 0; k < ils.Spans().Len(); k++ {
				span := ils.Spans().At(k)

				toBeGrouped, requiredAttributes := gap.extractGroupingAttributes(span.Attributes())
				if toBeGrouped {
					gap.telemetryBuilder.ProcessorGroupbyattrsNumGroupedSpans.Add(ctx, 1)
					// Some attributes are going to be moved from span to resource level,
					// so we can delete those on the record level
					deleteAttributes(requiredAttributes, span.Attributes())
				} else {
					gap.telemetryBuilder.ProcessorGroupbyattrsNumNonGroupedSpans.Add(ctx, 1)
				}

				// Lets combine the base resource attributes + the extracted (grouped) attributes
				// and keep them in the grouping entry
				groupedResourceSpans := tg.findOrCreateResourceSpans(rs.Resource(), requiredAttributes)
				sp := matchingScopeSpans(groupedResourceSpans, ils.Scope()).Spans().AppendEmpty()
				span.CopyTo(sp)
			}
		}
	}

	// Copy the grouped data into output
	gap.telemetryBuilder.ProcessorGroupbyattrsSpanGroups.Record(ctx, int64(tg.traces.ResourceSpans().Len()))

	return tg.traces, nil
}

func (gap *groupByAttrsProcessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	rl := ld.ResourceLogs()
	lg := newLogsGroup()

	for i := 0; i < rl.Len(); i++ {
		ls := rl.At(i)

		ills := ls.ScopeLogs()
		for j := 0; j < ills.Len(); j++ {
			sl := ills.At(j)
			for k := 0; k < sl.LogRecords().Len(); k++ {
				log := sl.LogRecords().At(k)

				toBeGrouped, requiredAttributes := gap.extractGroupingAttributes(log.Attributes())
				if toBeGrouped {
					gap.telemetryBuilder.ProcessorGroupbyattrsNumGroupedLogs.Add(ctx, 1)
					// Some attributes are going to be moved from log record to resource level,
					// so we can delete those on the record level
					deleteAttributes(requiredAttributes, log.Attributes())
				} else {
					gap.telemetryBuilder.ProcessorGroupbyattrsNumNonGroupedLogs.Add(ctx, 1)
				}

				// Lets combine the base resource attributes + the extracted (grouped) attributes
				// and keep them in the grouping entry
				groupedResourceLogs := lg.findOrCreateResourceLogs(ls.Resource(), requiredAttributes)
				lr := matchingScopeLogs(groupedResourceLogs, sl.Scope()).LogRecords().AppendEmpty()
				log.CopyTo(lr)
			}
		}
	}

	// Copy the grouped data into output
	gap.telemetryBuilder.ProcessorGroupbyattrsLogGroups.Record(ctx, int64(lg.logs.ResourceLogs().Len()))

	return lg.logs, nil
}

func (gap *groupByAttrsProcessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	rms := md.ResourceMetrics()
	mg := newMetricsGroup()

	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)

		ilms := rm.ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ilm := ilms.At(j)
			for k := 0; k < ilm.Metrics().Len(); k++ {
				metric := ilm.Metrics().At(k)

				//exhaustive:enforce
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					for pointIndex := 0; pointIndex < metric.Gauge().DataPoints().Len(); pointIndex++ {
						dataPoint := metric.Gauge().DataPoints().At(pointIndex)
						groupedMetric := gap.getGroupedMetricsFromAttributes(ctx, mg, rm, ilm, metric, dataPoint.Attributes())
						dataPoint.CopyTo(groupedMetric.Gauge().DataPoints().AppendEmpty())
					}

				case pmetric.MetricTypeSum:
					for pointIndex := 0; pointIndex < metric.Sum().DataPoints().Len(); pointIndex++ {
						dataPoint := metric.Sum().DataPoints().At(pointIndex)
						groupedMetric := gap.getGroupedMetricsFromAttributes(ctx, mg, rm, ilm, metric, dataPoint.Attributes())
						dataPoint.CopyTo(groupedMetric.Sum().DataPoints().AppendEmpty())
					}

				case pmetric.MetricTypeSummary:
					for pointIndex := 0; pointIndex < metric.Summary().DataPoints().Len(); pointIndex++ {
						dataPoint := metric.Summary().DataPoints().At(pointIndex)
						groupedMetric := gap.getGroupedMetricsFromAttributes(ctx, mg, rm, ilm, metric, dataPoint.Attributes())
						dataPoint.CopyTo(groupedMetric.Summary().DataPoints().AppendEmpty())
					}

				case pmetric.MetricTypeHistogram:
					for pointIndex := 0; pointIndex < metric.Histogram().DataPoints().Len(); pointIndex++ {
						dataPoint := metric.Histogram().DataPoints().At(pointIndex)
						groupedMetric := gap.getGroupedMetricsFromAttributes(ctx, mg, rm, ilm, metric, dataPoint.Attributes())
						dataPoint.CopyTo(groupedMetric.Histogram().DataPoints().AppendEmpty())
					}

				case pmetric.MetricTypeExponentialHistogram:
					for pointIndex := 0; pointIndex < metric.ExponentialHistogram().DataPoints().Len(); pointIndex++ {
						dataPoint := metric.ExponentialHistogram().DataPoints().At(pointIndex)
						groupedMetric := gap.getGroupedMetricsFromAttributes(ctx, mg, rm, ilm, metric, dataPoint.Attributes())
						dataPoint.CopyTo(groupedMetric.ExponentialHistogram().DataPoints().AppendEmpty())
					}

				case pmetric.MetricTypeEmpty:
				}
			}
		}
	}

	gap.telemetryBuilder.ProcessorGroupbyattrsMetricGroups.Record(ctx, int64(mg.metrics.ResourceMetrics().Len()))

	return mg.metrics, nil
}

func deleteAttributes(attrsForRemoval, targetAttrs pcommon.Map) {
	for key := range attrsForRemoval.All() {
		targetAttrs.Remove(key)
	}
}

// extractGroupingAttributes extracts the keys and values of the specified Attributes
// that match with the attributes keys that is used for grouping
// Returns:
//   - whether any attribute matched (true) or none (false)
//   - the extracted AttributeMap of matching keys and their corresponding values
func (gap *groupByAttrsProcessor) extractGroupingAttributes(attrMap pcommon.Map) (bool, pcommon.Map) {
	groupingAttributes := pcommon.NewMap()
	foundMatch := false

	for _, attrKey := range gap.groupByKeys {
		attrVal, found := attrMap.Get(attrKey)
		if found {
			attrVal.CopyTo(groupingAttributes.PutEmpty(attrKey))
			foundMatch = true
		}
	}

	return foundMatch, groupingAttributes
}

// Searches for metric with same name in the specified InstrumentationLibrary and returns it. If nothing is found, create it.
func getMetricInInstrumentationLibrary(ilm pmetric.ScopeMetrics, searchedMetric pmetric.Metric) pmetric.Metric {
	// Loop through all metrics and try to find the one that matches with the one we search for
	// (name and type)
	for i := 0; i < ilm.Metrics().Len(); i++ {
		metric := ilm.Metrics().At(i)
		if metric.Name() == searchedMetric.Name() && metric.Type() == searchedMetric.Type() {
			return metric
		}
	}

	// We're here, which means that we haven't found our metric, so we need to create a new one, with the same name and type
	metric := ilm.Metrics().AppendEmpty()
	metric.SetDescription(searchedMetric.Description())
	metric.SetName(searchedMetric.Name())
	metric.SetUnit(searchedMetric.Unit())
	searchedMetric.Metadata().CopyTo(metric.Metadata())

	// Move other special type specific values
	//exhaustive:enforce
	switch searchedMetric.Type() {
	case pmetric.MetricTypeHistogram:
		metric.SetEmptyHistogram().SetAggregationTemporality(searchedMetric.Histogram().AggregationTemporality())

	case pmetric.MetricTypeExponentialHistogram:
		metric.SetEmptyExponentialHistogram().SetAggregationTemporality(searchedMetric.ExponentialHistogram().AggregationTemporality())

	case pmetric.MetricTypeSum:
		metric.SetEmptySum().SetAggregationTemporality(searchedMetric.Sum().AggregationTemporality())
		metric.Sum().SetIsMonotonic(searchedMetric.Sum().IsMonotonic())

	case pmetric.MetricTypeGauge:
		metric.SetEmptyGauge()

	case pmetric.MetricTypeSummary:
		metric.SetEmptySummary()

	case pmetric.MetricTypeEmpty:
	}

	return metric
}

// Returns the Metric in the appropriate Resource matching with the specified Attributes
func (gap *groupByAttrsProcessor) getGroupedMetricsFromAttributes(
	ctx context.Context,
	mg *metricsGroup,
	originResourceMetrics pmetric.ResourceMetrics,
	ilm pmetric.ScopeMetrics,
	metric pmetric.Metric,
	attributes pcommon.Map,
) pmetric.Metric {
	toBeGrouped, requiredAttributes := gap.extractGroupingAttributes(attributes)
	if toBeGrouped {
		gap.telemetryBuilder.ProcessorGroupbyattrsNumGroupedMetrics.Add(ctx, 1)
		// These attributes are going to be moved from datapoint to resource level,
		// so we can delete those on the datapoint
		deleteAttributes(requiredAttributes, attributes)
	} else {
		gap.telemetryBuilder.ProcessorGroupbyattrsNumNonGroupedMetrics.Add(ctx, 1)
	}

	// Get the ResourceMetrics matching with these attributes
	groupedResourceMetrics := mg.findOrCreateResourceMetrics(originResourceMetrics.Resource(), requiredAttributes)

	// Get the corresponding instrumentation library
	groupedInstrumentationLibrary := matchingScopeMetrics(groupedResourceMetrics, ilm.Scope())

	// Return the metric in this resource
	return getMetricInInstrumentationLibrary(groupedInstrumentationLibrary, metric)
}
