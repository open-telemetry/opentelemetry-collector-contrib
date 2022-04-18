// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package groupbyattrsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor"

import (
	"context"

	"go.opencensus.io/stats"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type groupByAttrsProcessor struct {
	logger      *zap.Logger
	groupByKeys []string
}

// ProcessTraces process traces and groups traces by attribute.
func (gap *groupByAttrsProcessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	rss := td.ResourceSpans()
	groupedResourceSpans := newSpansGroupedByAttrs()

	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)

		ilss := rs.ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			for k := 0; k < ils.Spans().Len(); k++ {
				span := ils.Spans().At(k)

				toBeGrouped, requiredAttributes := gap.extractGroupingAttributes(span.Attributes())
				if toBeGrouped {
					stats.Record(ctx, mNumGroupedSpans.M(1))
					// Some attributes are going to be moved from span to resource level,
					// so we can delete those on the record level
					deleteAttributes(requiredAttributes, span.Attributes())
				} else {
					stats.Record(ctx, mNumNonGroupedSpans.M(1))
				}

				// Lets combine the base resource attributes + the extracted (grouped) attributes
				// and keep them in the grouping entry
				groupedSpans := groupedResourceSpans.findOrCreateResource(rs.Resource(), requiredAttributes)
				sp := matchingScopeSpans(groupedSpans, ils.Scope()).Spans().AppendEmpty()
				span.CopyTo(sp)
			}
		}
	}

	// Copy the grouped data into output
	groupedTraces := ptrace.NewTraces()
	groupedResourceSpans.MoveAndAppendTo(groupedTraces.ResourceSpans())
	stats.Record(ctx, mDistSpanGroups.M(int64(groupedTraces.ResourceSpans().Len())))

	return groupedTraces, nil
}

func (gap *groupByAttrsProcessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	rl := ld.ResourceLogs()
	groupedResourceLogs := newLogsGroupedByAttrs()

	for i := 0; i < rl.Len(); i++ {
		ls := rl.At(i)

		ills := ls.ScopeLogs()
		for j := 0; j < ills.Len(); j++ {
			sl := ills.At(j)
			for k := 0; k < sl.LogRecords().Len(); k++ {
				log := sl.LogRecords().At(k)

				toBeGrouped, requiredAttributes := gap.extractGroupingAttributes(log.Attributes())
				if toBeGrouped {
					stats.Record(ctx, mNumGroupedLogs.M(1))
					// Some attributes are going to be moved from log record to resource level,
					// so we can delete those on the record level
					deleteAttributes(requiredAttributes, log.Attributes())
				} else {
					stats.Record(ctx, mNumNonGroupedLogs.M(1))
				}

				// Lets combine the base resource attributes + the extracted (grouped) attributes
				// and keep them in the grouping entry
				groupedLogs := groupedResourceLogs.findResourceOrElseCreate(ls.Resource(), requiredAttributes)
				lr := matchingScopeLogs(groupedLogs, sl.Scope()).LogRecords().AppendEmpty()
				log.CopyTo(lr)
			}
		}

	}

	// Copy the grouped data into output
	groupedLogs := plog.NewLogs()
	groupedResourceLogs.MoveAndAppendTo(groupedLogs.ResourceLogs())
	stats.Record(ctx, mDistLogGroups.M(int64(groupedLogs.ResourceLogs().Len())))

	return groupedLogs, nil
}

func (gap *groupByAttrsProcessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	rms := md.ResourceMetrics()
	groupedResourceMetrics := newMetricsGroupedByAttrs()

	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)

		ilms := rm.ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ilm := ilms.At(j)
			for k := 0; k < ilm.Metrics().Len(); k++ {
				metric := ilm.Metrics().At(k)

				switch metric.DataType() {

				case pmetric.MetricDataTypeGauge:
					for pointIndex := 0; pointIndex < metric.Gauge().DataPoints().Len(); pointIndex++ {
						dataPoint := metric.Gauge().DataPoints().At(pointIndex)
						groupedMetric := gap.getGroupedMetricsFromAttributes(ctx, groupedResourceMetrics, rm, ilm, metric, dataPoint.Attributes())
						dataPoint.CopyTo(groupedMetric.Gauge().DataPoints().AppendEmpty())
					}

				case pmetric.MetricDataTypeSum:
					for pointIndex := 0; pointIndex < metric.Sum().DataPoints().Len(); pointIndex++ {
						dataPoint := metric.Sum().DataPoints().At(pointIndex)
						groupedMetric := gap.getGroupedMetricsFromAttributes(ctx, groupedResourceMetrics, rm, ilm, metric, dataPoint.Attributes())
						dataPoint.CopyTo(groupedMetric.Sum().DataPoints().AppendEmpty())
					}

				case pmetric.MetricDataTypeSummary:
					for pointIndex := 0; pointIndex < metric.Summary().DataPoints().Len(); pointIndex++ {
						dataPoint := metric.Summary().DataPoints().At(pointIndex)
						groupedMetric := gap.getGroupedMetricsFromAttributes(ctx, groupedResourceMetrics, rm, ilm, metric, dataPoint.Attributes())
						dataPoint.CopyTo(groupedMetric.Summary().DataPoints().AppendEmpty())
					}

				case pmetric.MetricDataTypeHistogram:
					for pointIndex := 0; pointIndex < metric.Histogram().DataPoints().Len(); pointIndex++ {
						dataPoint := metric.Histogram().DataPoints().At(pointIndex)
						groupedMetric := gap.getGroupedMetricsFromAttributes(ctx, groupedResourceMetrics, rm, ilm, metric, dataPoint.Attributes())
						dataPoint.CopyTo(groupedMetric.Histogram().DataPoints().AppendEmpty())
					}

				case pmetric.MetricDataTypeExponentialHistogram:
					for pointIndex := 0; pointIndex < metric.ExponentialHistogram().DataPoints().Len(); pointIndex++ {
						dataPoint := metric.ExponentialHistogram().DataPoints().At(pointIndex)
						groupedMetric := gap.getGroupedMetricsFromAttributes(ctx, groupedResourceMetrics, rm, ilm, metric, dataPoint.Attributes())
						dataPoint.CopyTo(groupedMetric.ExponentialHistogram().DataPoints().AppendEmpty())
					}

				}
			}
		}
	}

	// Copy the grouped data into output
	groupedMetrics := pmetric.NewMetrics()
	groupedResourceMetrics.MoveAndAppendTo(groupedMetrics.ResourceMetrics())
	stats.Record(ctx, mDistMetricGroups.M(int64(groupedMetrics.ResourceMetrics().Len())))

	return groupedMetrics, nil
}

func deleteAttributes(attrsForRemoval, targetAttrs pcommon.Map) {
	attrsForRemoval.Range(func(key string, _ pcommon.Value) bool {
		targetAttrs.Delete(key)
		return true
	})
}

// extractGroupingAttributes extracts the keys and values of the specified Attributes
// that match with the attributes keys that is used for grouping
// Returns:
//  - whether any attribute matched (true) or none (false)
//  - the extracted AttributeMap of matching keys and their corresponding values
func (gap *groupByAttrsProcessor) extractGroupingAttributes(attrMap pcommon.Map) (bool, pcommon.Map) {

	groupingAttributes := pcommon.NewMap()
	foundMatch := false

	for _, attrKey := range gap.groupByKeys {
		attrVal, found := attrMap.Get(attrKey)
		if found {
			groupingAttributes.Insert(attrKey, attrVal)
			foundMatch = true
		}
	}

	return foundMatch, groupingAttributes
}

// Searches for metric with same name in the specified InstrumentationLibrary and returns it. If nothing is found, create it.
func getMetricInInstrumentationLibrary(ilm pmetric.ScopeMetrics, searchedMetric pmetric.Metric, gap *groupByAttrsProcessor) pmetric.Metric {

	// Loop through all metrics and try to find the one that matches with the one we search for
	// (name and type)
	for i := 0; i < ilm.Metrics().Len(); i++ {
		metric := ilm.Metrics().At(i)
		if metric.Name() == searchedMetric.Name() && metric.DataType() == searchedMetric.DataType() {
			return metric
		}
	}

	// We're here, which means that we haven't found our metric, so we need to create a new one, with the same name and type
	metric := ilm.Metrics().AppendEmpty()
	metric.SetDataType(searchedMetric.DataType())
	metric.SetDescription(searchedMetric.Description())
	metric.SetName(searchedMetric.Name())
	metric.SetUnit(searchedMetric.Unit())

	// Move other special type specific values
	switch metric.DataType() {

	case pmetric.MetricDataTypeHistogram:
		metric.Histogram().SetAggregationTemporality(searchedMetric.Histogram().AggregationTemporality())

	case pmetric.MetricDataTypeExponentialHistogram:
		metric.ExponentialHistogram().SetAggregationTemporality(searchedMetric.ExponentialHistogram().AggregationTemporality())

	case pmetric.MetricDataTypeSum:
		metric.Sum().SetAggregationTemporality(searchedMetric.Sum().AggregationTemporality())

	case pmetric.MetricDataTypeGauge:
	case pmetric.MetricDataTypeSummary:

	default:
		gap.logger.Error("Unhandled metric data type.",
			zap.String("DataType", metric.DataType().String()),
			zap.String("Name", metric.Name()),
			zap.String("Unit", metric.Unit()),
		)
	}

	return metric
}

// Returns the Metric in the appropriate Resource matching with the specified Attributes
func (gap *groupByAttrsProcessor) getGroupedMetricsFromAttributes(
	ctx context.Context,
	groupedResourceMetrics *metricsGroupedByAttrs,
	originResourceMetrics pmetric.ResourceMetrics,
	ilm pmetric.ScopeMetrics,
	metric pmetric.Metric,
	attributes pcommon.Map,
) pmetric.Metric {

	toBeGrouped, requiredAttributes := gap.extractGroupingAttributes(attributes)
	if toBeGrouped {
		stats.Record(ctx, mNumGroupedMetrics.M(1))
		// These attributes are going to be moved from datapoint to resource level,
		// so we can delete those on the datapoint
		deleteAttributes(requiredAttributes, attributes)
	} else {
		stats.Record(ctx, mNumNonGroupedMetrics.M(1))
	}

	// Get the ResourceMetrics matching with these attributes
	groupedResource := groupedResourceMetrics.findResourceOrElseCreate(originResourceMetrics.Resource(), requiredAttributes)

	// Get the corresponding instrumentation library
	groupedInstrumentationLibrary := matchingScopeMetrics(groupedResource, ilm.Scope())

	// Return the metric in this resource
	return getMetricInInstrumentationLibrary(groupedInstrumentationLibrary, metric, gap)

}
