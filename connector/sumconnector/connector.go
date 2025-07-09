// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/sumconnector"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
)

// sum can sum attribute values from spans, span event, metrics, data points, or log records
// and emit the sums onto a metrics pipeline.
type sum struct {
	metricsConsumer consumer.Metrics
	component.StartFunc
	component.ShutdownFunc

	spansMetricDefs      map[string]metricDef[ottlspan.TransformContext]
	spanEventsMetricDefs map[string]metricDef[ottlspanevent.TransformContext]
	metricsMetricDefs    map[string]metricDef[ottlmetric.TransformContext]
	dataPointsMetricDefs map[string]metricDef[ottldatapoint.TransformContext]
	logsMetricDefs       map[string]metricDef[ottllog.TransformContext]
}

func (c *sum) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *sum) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	var multiError error
	sumMetrics := pmetric.NewMetrics()
	sumMetrics.ResourceMetrics().EnsureCapacity(td.ResourceSpans().Len())
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		resourceSpan := td.ResourceSpans().At(i)
		spansSummer := newSummer[ottlspan.TransformContext](c.spansMetricDefs)
		spanEventsSummer := newSummer[ottlspanevent.TransformContext](c.spanEventsMetricDefs)

		for j := 0; j < resourceSpan.ScopeSpans().Len(); j++ {
			scopeSpan := resourceSpan.ScopeSpans().At(j)

			for k := 0; k < scopeSpan.Spans().Len(); k++ {
				span := scopeSpan.Spans().At(k)
				sCtx := ottlspan.NewTransformContext(span, scopeSpan.Scope(), resourceSpan.Resource(), scopeSpan, resourceSpan)
				multiError = errors.Join(multiError, spansSummer.update(ctx, span.Attributes(), sCtx))

				for l := 0; l < span.Events().Len(); l++ {
					event := span.Events().At(l)
					eCtx := ottlspanevent.NewTransformContext(event, span, scopeSpan.Scope(), resourceSpan.Resource(), scopeSpan, resourceSpan)
					multiError = errors.Join(multiError, spanEventsSummer.update(ctx, event.Attributes(), eCtx))
				}
			}
		}

		if len(spansSummer.sums)+len(spanEventsSummer.sums) == 0 {
			continue // don't add an empty resource
		}

		sumResource := sumMetrics.ResourceMetrics().AppendEmpty()
		resourceSpan.Resource().Attributes().CopyTo(sumResource.Resource().Attributes())

		sumResource.ScopeMetrics().EnsureCapacity(resourceSpan.ScopeSpans().Len())
		sumScope := sumResource.ScopeMetrics().AppendEmpty()

		spansSummer.appendMetricsTo(sumScope.Metrics())
		spanEventsSummer.appendMetricsTo(sumScope.Metrics())
	}
	if multiError != nil {
		return multiError
	}
	return c.metricsConsumer.ConsumeMetrics(ctx, sumMetrics)
}

func (c *sum) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	var multiError error
	sumMetrics := pmetric.NewMetrics()
	sumMetrics.ResourceMetrics().EnsureCapacity(md.ResourceMetrics().Len())
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		resourceMetric := md.ResourceMetrics().At(i)
		metricsSummer := newSummer[ottlmetric.TransformContext](c.metricsMetricDefs)
		dataPointsSummer := newSummer[ottldatapoint.TransformContext](c.dataPointsMetricDefs)

		for j := 0; j < resourceMetric.ScopeMetrics().Len(); j++ {
			scopeMetrics := resourceMetric.ScopeMetrics().At(j)

			for k := 0; k < scopeMetrics.Metrics().Len(); k++ {
				metric := scopeMetrics.Metrics().At(k)
				mCtx := ottlmetric.NewTransformContext(metric, scopeMetrics.Metrics(), scopeMetrics.Scope(), resourceMetric.Resource(), scopeMetrics, resourceMetric)
				multiError = errors.Join(multiError, metricsSummer.update(ctx, pcommon.NewMap(), mCtx))

				//exhaustive:enforce
				//  For metric types each must be handled in exactly the same way
				//  Switch case required because each type calls DataPoints() differently
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					dps := metric.Gauge().DataPoints()
					for i := 0; i < dps.Len(); i++ {
						dCtx := ottldatapoint.NewTransformContext(dps.At(i), metric, scopeMetrics.Metrics(), scopeMetrics.Scope(), resourceMetric.Resource(), scopeMetrics, resourceMetric)
						multiError = errors.Join(multiError, dataPointsSummer.update(ctx, dps.At(i).Attributes(), dCtx))
					}
				case pmetric.MetricTypeSum:
					dps := metric.Sum().DataPoints()
					for i := 0; i < dps.Len(); i++ {
						dCtx := ottldatapoint.NewTransformContext(dps.At(i), metric, scopeMetrics.Metrics(), scopeMetrics.Scope(), resourceMetric.Resource(), scopeMetrics, resourceMetric)
						multiError = errors.Join(multiError, dataPointsSummer.update(ctx, dps.At(i).Attributes(), dCtx))
					}
				case pmetric.MetricTypeSummary:
					dps := metric.Summary().DataPoints()
					for i := 0; i < dps.Len(); i++ {
						dCtx := ottldatapoint.NewTransformContext(dps.At(i), metric, scopeMetrics.Metrics(), scopeMetrics.Scope(), resourceMetric.Resource(), scopeMetrics, resourceMetric)
						multiError = errors.Join(multiError, dataPointsSummer.update(ctx, dps.At(i).Attributes(), dCtx))
					}
				case pmetric.MetricTypeHistogram:
					dps := metric.Histogram().DataPoints()
					for i := 0; i < dps.Len(); i++ {
						dCtx := ottldatapoint.NewTransformContext(dps.At(i), metric, scopeMetrics.Metrics(), scopeMetrics.Scope(), resourceMetric.Resource(), scopeMetrics, resourceMetric)
						multiError = errors.Join(multiError, dataPointsSummer.update(ctx, dps.At(i).Attributes(), dCtx))
					}
				case pmetric.MetricTypeExponentialHistogram:
					dps := metric.ExponentialHistogram().DataPoints()
					for i := 0; i < dps.Len(); i++ {
						dCtx := ottldatapoint.NewTransformContext(dps.At(i), metric, scopeMetrics.Metrics(), scopeMetrics.Scope(), resourceMetric.Resource(), scopeMetrics, resourceMetric)
						multiError = errors.Join(multiError, dataPointsSummer.update(ctx, dps.At(i).Attributes(), dCtx))
					}
				case pmetric.MetricTypeEmpty:
					multiError = errors.Join(multiError, fmt.Errorf("metric %q: invalid metric type: %v", metric.Name(), metric.Type()))
				}
			}
		}

		if len(metricsSummer.sums)+len(dataPointsSummer.sums) == 0 {
			continue // don't add an empty resource
		}

		sumResource := sumMetrics.ResourceMetrics().AppendEmpty()
		resourceMetric.Resource().Attributes().CopyTo(sumResource.Resource().Attributes())

		sumResource.ScopeMetrics().EnsureCapacity(resourceMetric.ScopeMetrics().Len())
		sumScope := sumResource.ScopeMetrics().AppendEmpty()

		metricsSummer.appendMetricsTo(sumScope.Metrics())
		dataPointsSummer.appendMetricsTo(sumScope.Metrics())
	}
	if multiError != nil {
		return multiError
	}
	return c.metricsConsumer.ConsumeMetrics(ctx, sumMetrics)
}

func (c *sum) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	var multiError error
	sumMetrics := pmetric.NewMetrics()
	sumMetrics.ResourceMetrics().EnsureCapacity(ld.ResourceLogs().Len())
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		resourceLog := ld.ResourceLogs().At(i)
		summer := newSummer[ottllog.TransformContext](c.logsMetricDefs)

		for j := 0; j < resourceLog.ScopeLogs().Len(); j++ {
			scopeLogs := resourceLog.ScopeLogs().At(j)

			for k := 0; k < scopeLogs.LogRecords().Len(); k++ {
				logRecord := scopeLogs.LogRecords().At(k)

				lCtx := ottllog.NewTransformContext(logRecord, scopeLogs.Scope(), resourceLog.Resource(), scopeLogs, resourceLog)
				multiError = errors.Join(multiError, summer.update(ctx, logRecord.Attributes(), lCtx))
			}
		}

		if len(summer.sums) == 0 {
			continue // don't add an empty resource
		}

		sumResource := sumMetrics.ResourceMetrics().AppendEmpty()
		resourceLog.Resource().Attributes().CopyTo(sumResource.Resource().Attributes())

		sumResource.ScopeMetrics().EnsureCapacity(resourceLog.ScopeLogs().Len())
		sumScope := sumResource.ScopeMetrics().AppendEmpty()

		summer.appendMetricsTo(sumScope.Metrics())
	}
	if multiError != nil {
		return multiError
	}
	return c.metricsConsumer.ConsumeMetrics(ctx, sumMetrics)
}
