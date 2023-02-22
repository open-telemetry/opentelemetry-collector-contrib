// Copyright The OpenTelemetry Authors
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

package countconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/countconnector"

import (
	"context"

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

	spansCounterFactory      *counterFactory[ottlspan.TransformContext]
	spanEventsCounterFactory *counterFactory[ottlspanevent.TransformContext]
	metricsCounterFactory    *counterFactory[ottlmetric.TransformContext]
	dataPointsCounterFactory *counterFactory[ottldatapoint.TransformContext]
	logsCounterFactory       *counterFactory[ottllog.TransformContext]
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

		countResource := countMetrics.ResourceMetrics().AppendEmpty()
		resourceSpan.Resource().Attributes().CopyTo(countResource.Resource().Attributes())

		countResource.ScopeMetrics().EnsureCapacity(resourceSpan.ScopeSpans().Len())
		countScope := countResource.ScopeMetrics().AppendEmpty()
		countScope.Scope().SetName(scopeName)

		spansCounter := c.spansCounterFactory.newCounter()
		spanEventsCounter := c.spanEventsCounterFactory.newCounter()
		for j := 0; j < resourceSpan.ScopeSpans().Len(); j++ {
			scopeSpan := resourceSpan.ScopeSpans().At(j)

			for k := 0; k < scopeSpan.Spans().Len(); k++ {
				span := scopeSpan.Spans().At(k)
				sCtx := ottlspan.NewTransformContext(span, scopeSpan.Scope(), resourceSpan.Resource())
				errors = multierr.Append(errors, spansCounter.update(ctx, sCtx))

				for l := 0; l < span.Events().Len(); l++ {
					event := span.Events().At(l)
					eCtx := ottlspanevent.NewTransformContext(event, span, scopeSpan.Scope(), resourceSpan.Resource())
					errors = multierr.Append(errors, spanEventsCounter.update(ctx, eCtx))
				}
			}
		}
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

		countResource := countMetrics.ResourceMetrics().AppendEmpty()
		resourceMetric.Resource().Attributes().CopyTo(countResource.Resource().Attributes())

		countResource.ScopeMetrics().EnsureCapacity(resourceMetric.ScopeMetrics().Len())
		countScope := countResource.ScopeMetrics().AppendEmpty()
		countScope.Scope().SetName(scopeName)

		metricsCounter := c.metricsCounterFactory.newCounter()
		datapointsCounter := c.dataPointsCounterFactory.newCounter()
		for j := 0; j < resourceMetric.ScopeMetrics().Len(); j++ {
			scopeMetrics := resourceMetric.ScopeMetrics().At(j)

			for k := 0; k < scopeMetrics.Metrics().Len(); k++ {
				metric := scopeMetrics.Metrics().At(k)
				mCtx := ottlmetric.NewTransformContext(metric, scopeMetrics.Scope(), resourceMetric.Resource())
				errors = multierr.Append(errors, metricsCounter.update(ctx, mCtx))

				dCtxs := dataPointContexts(metric, scopeMetrics.Metrics(), scopeMetrics.Scope(), resourceMetric.Resource())
				for l := 0; l < len(dCtxs); l++ {
					errors = multierr.Append(errors, datapointsCounter.update(ctx, dCtxs[l]))
				}
			}
		}
		metricsCounter.appendMetricsTo(countScope.Metrics())
		datapointsCounter.appendMetricsTo(countScope.Metrics())
	}
	if errors != nil {
		return errors
	}
	return c.metricsConsumer.ConsumeMetrics(ctx, countMetrics)
}

func dataPointContexts(metric pmetric.Metric, metrics pmetric.MetricSlice, scope pcommon.InstrumentationScope, resource pcommon.Resource) []ottldatapoint.TransformContext {
	dCtxs := []ottldatapoint.TransformContext{}
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dps := metric.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dCtxs = append(dCtxs, ottldatapoint.NewTransformContext(dps.At(i), metric, metrics, scope, resource))
		}
	case pmetric.MetricTypeSum:
		dps := metric.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dCtxs = append(dCtxs, ottldatapoint.NewTransformContext(dps.At(i), metric, metrics, scope, resource))
		}
	case pmetric.MetricTypeSummary:
		dps := metric.Summary().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dCtxs = append(dCtxs, ottldatapoint.NewTransformContext(dps.At(i), metric, metrics, scope, resource))
		}
	case pmetric.MetricTypeHistogram:
		dps := metric.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dCtxs = append(dCtxs, ottldatapoint.NewTransformContext(dps.At(i), metric, metrics, scope, resource))
		}
	case pmetric.MetricTypeExponentialHistogram:
		dps := metric.ExponentialHistogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dCtxs = append(dCtxs, ottldatapoint.NewTransformContext(dps.At(i), metric, metrics, scope, resource))
		}
	}
	return dCtxs
}

func (c *count) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	var errors error
	countMetrics := pmetric.NewMetrics()
	countMetrics.ResourceMetrics().EnsureCapacity(ld.ResourceLogs().Len())
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		resourceLog := ld.ResourceLogs().At(i)

		countResource := countMetrics.ResourceMetrics().AppendEmpty()
		resourceLog.Resource().Attributes().CopyTo(countResource.Resource().Attributes())

		countResource.ScopeMetrics().EnsureCapacity(resourceLog.ScopeLogs().Len())
		countScope := countResource.ScopeMetrics().AppendEmpty()
		countScope.Scope().SetName(scopeName)

		counter := c.logsCounterFactory.newCounter()
		for j := 0; j < resourceLog.ScopeLogs().Len(); j++ {
			scopeLogs := resourceLog.ScopeLogs().At(j)

			for k := 0; k < scopeLogs.LogRecords().Len(); k++ {
				logRecord := scopeLogs.LogRecords().At(k)
				lCtx := ottllog.NewTransformContext(logRecord, scopeLogs.Scope(), resourceLog.Resource())
				errors = multierr.Append(errors, counter.update(ctx, lCtx))
			}
		}
		counter.appendMetricsTo(countScope.Metrics())
	}
	if errors != nil {
		return errors
	}
	return c.metricsConsumer.ConsumeMetrics(ctx, countMetrics)
}
