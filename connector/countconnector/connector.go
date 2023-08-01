// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package countconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/countconnector"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
)

const scopeName = "otelcol/countconnector"

// count can count spans, span event, metrics, data points, or log records
// and emit the counts onto a metrics pipeline.
type count struct {
	metricsConsumer consumer.Metrics
	component.StartFunc
	component.ShutdownFunc

	spansMetricDefs      map[string]metricDef[ottlspan.TransformContext]
	spanEventsMetricDefs map[string]metricDef[ottlspanevent.TransformContext]
	metricsMetricDefs    map[string]metricDef[ottlmetric.TransformContext]
	dataPointsMetricDefs map[string]metricDef[ottldatapoint.TransformContext]
	logsMetricDefs       map[string]metricDef[ottllog.TransformContext]
}

func (c *count) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *count) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	var errors error
	countMetrics := pmetric.NewMetrics()
	countMetrics.ResourceMetrics().EnsureCapacity(td.ResourceSpans().Len())
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		resourceSpan := td.ResourceSpans().At(i)
		spansCounter := newCounter[ottlspan.TransformContext](c.spansMetricDefs)
		spanEventsCounter := newCounter[ottlspanevent.TransformContext](c.spanEventsMetricDefs)

		for j := 0; j < resourceSpan.ScopeSpans().Len(); j++ {
			scopeSpan := resourceSpan.ScopeSpans().At(j)

			for k := 0; k < scopeSpan.Spans().Len(); k++ {
				span := scopeSpan.Spans().At(k)
				sCtx := ottlspan.NewTransformContext(span, scopeSpan.Scope(), resourceSpan.Resource())
				errors = multierr.Append(errors, spansCounter.update(ctx, span.Attributes(), sCtx))

				for l := 0; l < span.Events().Len(); l++ {
					event := span.Events().At(l)
					eCtx := ottlspanevent.NewTransformContext(event, span, scopeSpan.Scope(), resourceSpan.Resource())
					errors = multierr.Append(errors, spanEventsCounter.update(ctx, event.Attributes(), eCtx))
				}
			}
		}

		if len(spansCounter.counts)+len(spanEventsCounter.counts) == 0 {
			continue // don't add an empty resource
		}

		countResource := countMetrics.ResourceMetrics().AppendEmpty()
		resourceSpan.Resource().Attributes().CopyTo(countResource.Resource().Attributes())

		countResource.ScopeMetrics().EnsureCapacity(resourceSpan.ScopeSpans().Len())
		countScope := countResource.ScopeMetrics().AppendEmpty()
		countScope.Scope().SetName(scopeName)

		spansCounter.appendMetricsTo(countScope.Metrics())
		spanEventsCounter.appendMetricsTo(countScope.Metrics())
	}
	if errors != nil {
		return errors
	}
	return c.metricsConsumer.ConsumeMetrics(ctx, countMetrics)
}

func (c *count) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	var errors error
	countMetrics := pmetric.NewMetrics()
	countMetrics.ResourceMetrics().EnsureCapacity(md.ResourceMetrics().Len())
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		resourceMetric := md.ResourceMetrics().At(i)
		metricsCounter := newCounter[ottlmetric.TransformContext](c.metricsMetricDefs)
		dataPointsCounter := newCounter[ottldatapoint.TransformContext](c.dataPointsMetricDefs)

		for j := 0; j < resourceMetric.ScopeMetrics().Len(); j++ {
			scopeMetrics := resourceMetric.ScopeMetrics().At(j)

			for k := 0; k < scopeMetrics.Metrics().Len(); k++ {
				metric := scopeMetrics.Metrics().At(k)
				mCtx := ottlmetric.NewTransformContext(metric, scopeMetrics.Metrics(), scopeMetrics.Scope(), resourceMetric.Resource())
				errors = multierr.Append(errors, metricsCounter.update(ctx, pcommon.NewMap(), mCtx))

				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					dps := metric.Gauge().DataPoints()
					for i := 0; i < dps.Len(); i++ {
						dCtx := ottldatapoint.NewTransformContext(dps.At(i), metric, scopeMetrics.Metrics(), scopeMetrics.Scope(), resourceMetric.Resource())
						errors = multierr.Append(errors, dataPointsCounter.update(ctx, dps.At(i).Attributes(), dCtx))
					}
				case pmetric.MetricTypeSum:
					dps := metric.Sum().DataPoints()
					for i := 0; i < dps.Len(); i++ {
						dCtx := ottldatapoint.NewTransformContext(dps.At(i), metric, scopeMetrics.Metrics(), scopeMetrics.Scope(), resourceMetric.Resource())
						errors = multierr.Append(errors, dataPointsCounter.update(ctx, dps.At(i).Attributes(), dCtx))
					}
				case pmetric.MetricTypeSummary:
					dps := metric.Summary().DataPoints()
					for i := 0; i < dps.Len(); i++ {
						dCtx := ottldatapoint.NewTransformContext(dps.At(i), metric, scopeMetrics.Metrics(), scopeMetrics.Scope(), resourceMetric.Resource())
						errors = multierr.Append(errors, dataPointsCounter.update(ctx, dps.At(i).Attributes(), dCtx))
					}
				case pmetric.MetricTypeHistogram:
					dps := metric.Histogram().DataPoints()
					for i := 0; i < dps.Len(); i++ {
						dCtx := ottldatapoint.NewTransformContext(dps.At(i), metric, scopeMetrics.Metrics(), scopeMetrics.Scope(), resourceMetric.Resource())
						errors = multierr.Append(errors, dataPointsCounter.update(ctx, dps.At(i).Attributes(), dCtx))
					}
				case pmetric.MetricTypeExponentialHistogram:
					dps := metric.ExponentialHistogram().DataPoints()
					for i := 0; i < dps.Len(); i++ {
						dCtx := ottldatapoint.NewTransformContext(dps.At(i), metric, scopeMetrics.Metrics(), scopeMetrics.Scope(), resourceMetric.Resource())
						errors = multierr.Append(errors, dataPointsCounter.update(ctx, dps.At(i).Attributes(), dCtx))
					}
				case pmetric.MetricTypeEmpty:
					errors = multierr.Append(errors, fmt.Errorf("metric %q: invalid metric type: %v", metric.Name(), metric.Type()))
				}
			}
		}

		if len(metricsCounter.counts)+len(dataPointsCounter.counts) == 0 {
			continue // don't add an empty resource
		}

		countResource := countMetrics.ResourceMetrics().AppendEmpty()
		resourceMetric.Resource().Attributes().CopyTo(countResource.Resource().Attributes())

		countResource.ScopeMetrics().EnsureCapacity(resourceMetric.ScopeMetrics().Len())
		countScope := countResource.ScopeMetrics().AppendEmpty()
		countScope.Scope().SetName(scopeName)

		metricsCounter.appendMetricsTo(countScope.Metrics())
		dataPointsCounter.appendMetricsTo(countScope.Metrics())
	}
	if errors != nil {
		return errors
	}
	return c.metricsConsumer.ConsumeMetrics(ctx, countMetrics)
}

func (c *count) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	var errors error
	countMetrics := pmetric.NewMetrics()
	countMetrics.ResourceMetrics().EnsureCapacity(ld.ResourceLogs().Len())
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		resourceLog := ld.ResourceLogs().At(i)
		counter := newCounter[ottllog.TransformContext](c.logsMetricDefs)

		for j := 0; j < resourceLog.ScopeLogs().Len(); j++ {
			scopeLogs := resourceLog.ScopeLogs().At(j)

			for k := 0; k < scopeLogs.LogRecords().Len(); k++ {
				logRecord := scopeLogs.LogRecords().At(k)

				lCtx := ottllog.NewTransformContext(logRecord, scopeLogs.Scope(), resourceLog.Resource())
				errors = multierr.Append(errors, counter.update(ctx, logRecord.Attributes(), lCtx))
			}
		}

		if len(counter.counts) == 0 {
			continue // don't add an empty resource
		}

		countResource := countMetrics.ResourceMetrics().AppendEmpty()
		resourceLog.Resource().Attributes().CopyTo(countResource.Resource().Attributes())

		countResource.ScopeMetrics().EnsureCapacity(resourceLog.ScopeLogs().Len())
		countScope := countResource.ScopeMetrics().AppendEmpty()
		countScope.Scope().SetName(scopeName)

		counter.appendMetricsTo(countScope.Metrics())
	}
	if errors != nil {
		return errors
	}
	return c.metricsConsumer.ConsumeMetrics(ctx, countMetrics)
}
