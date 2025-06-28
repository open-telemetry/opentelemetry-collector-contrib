// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package countconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/countconnector"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/countconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofile"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
)

// count can count spans, span event, metrics, data points, log records or
// profiles and emit the counts onto a metrics pipeline.
type count struct {
	metricsConsumer consumer.Metrics
	component.StartFunc
	component.ShutdownFunc

	spansMetricDefs      map[string]metricDef[ottlspan.TransformContext]
	spanEventsMetricDefs map[string]metricDef[ottlspanevent.TransformContext]
	metricsMetricDefs    map[string]metricDef[ottlmetric.TransformContext]
	dataPointsMetricDefs map[string]metricDef[ottldatapoint.TransformContext]
	logsMetricDefs       map[string]metricDef[ottllog.TransformContext]
	profilesMetricDefs   map[string]metricDef[ottlprofile.TransformContext]
}

func (c *count) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *count) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	var multiError error
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
				sCtx := ottlspan.NewTransformContext(span, scopeSpan.Scope(), resourceSpan.Resource(), scopeSpan, resourceSpan)
				multiError = errors.Join(multiError, spansCounter.update(ctx, span.Attributes(), sCtx))

				for l := 0; l < span.Events().Len(); l++ {
					event := span.Events().At(l)
					eCtx := ottlspanevent.NewTransformContext(event, span, scopeSpan.Scope(), resourceSpan.Resource(), scopeSpan, resourceSpan)
					multiError = errors.Join(multiError, spanEventsCounter.update(ctx, event.Attributes(), eCtx))
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
		countScope.Scope().SetName(metadata.ScopeName)

		spansCounter.appendMetricsTo(countScope.Metrics())
		spanEventsCounter.appendMetricsTo(countScope.Metrics())
	}
	if multiError != nil {
		return multiError
	}
	return c.metricsConsumer.ConsumeMetrics(ctx, countMetrics)
}

func (c *count) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	var multiError error
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
				mCtx := ottlmetric.NewTransformContext(metric, scopeMetrics.Metrics(), scopeMetrics.Scope(), resourceMetric.Resource(), scopeMetrics, resourceMetric)
				multiError = errors.Join(multiError, metricsCounter.update(ctx, pcommon.NewMap(), mCtx))

				//exhaustive:enforce
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					dps := metric.Gauge().DataPoints()
					for i := 0; i < dps.Len(); i++ {
						dCtx := ottldatapoint.NewTransformContext(dps.At(i), metric, scopeMetrics.Metrics(), scopeMetrics.Scope(), resourceMetric.Resource(), scopeMetrics, resourceMetric)
						multiError = errors.Join(multiError, dataPointsCounter.update(ctx, dps.At(i).Attributes(), dCtx))
					}
				case pmetric.MetricTypeSum:
					dps := metric.Sum().DataPoints()
					for i := 0; i < dps.Len(); i++ {
						dCtx := ottldatapoint.NewTransformContext(dps.At(i), metric, scopeMetrics.Metrics(), scopeMetrics.Scope(), resourceMetric.Resource(), scopeMetrics, resourceMetric)
						multiError = errors.Join(multiError, dataPointsCounter.update(ctx, dps.At(i).Attributes(), dCtx))
					}
				case pmetric.MetricTypeSummary:
					dps := metric.Summary().DataPoints()
					for i := 0; i < dps.Len(); i++ {
						dCtx := ottldatapoint.NewTransformContext(dps.At(i), metric, scopeMetrics.Metrics(), scopeMetrics.Scope(), resourceMetric.Resource(), scopeMetrics, resourceMetric)
						multiError = errors.Join(multiError, dataPointsCounter.update(ctx, dps.At(i).Attributes(), dCtx))
					}
				case pmetric.MetricTypeHistogram:
					dps := metric.Histogram().DataPoints()
					for i := 0; i < dps.Len(); i++ {
						dCtx := ottldatapoint.NewTransformContext(dps.At(i), metric, scopeMetrics.Metrics(), scopeMetrics.Scope(), resourceMetric.Resource(), scopeMetrics, resourceMetric)
						multiError = errors.Join(multiError, dataPointsCounter.update(ctx, dps.At(i).Attributes(), dCtx))
					}
				case pmetric.MetricTypeExponentialHistogram:
					dps := metric.ExponentialHistogram().DataPoints()
					for i := 0; i < dps.Len(); i++ {
						dCtx := ottldatapoint.NewTransformContext(dps.At(i), metric, scopeMetrics.Metrics(), scopeMetrics.Scope(), resourceMetric.Resource(), scopeMetrics, resourceMetric)
						multiError = errors.Join(multiError, dataPointsCounter.update(ctx, dps.At(i).Attributes(), dCtx))
					}
				case pmetric.MetricTypeEmpty:
					multiError = errors.Join(multiError, fmt.Errorf("metric %q: invalid metric type: %v", metric.Name(), metric.Type()))
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
		countScope.Scope().SetName(metadata.ScopeName)

		metricsCounter.appendMetricsTo(countScope.Metrics())
		dataPointsCounter.appendMetricsTo(countScope.Metrics())
	}
	if multiError != nil {
		return multiError
	}
	return c.metricsConsumer.ConsumeMetrics(ctx, countMetrics)
}

func (c *count) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	var multiError error
	countMetrics := pmetric.NewMetrics()
	countMetrics.ResourceMetrics().EnsureCapacity(ld.ResourceLogs().Len())
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		resourceLog := ld.ResourceLogs().At(i)
		counter := newCounter[ottllog.TransformContext](c.logsMetricDefs)

		for j := 0; j < resourceLog.ScopeLogs().Len(); j++ {
			scopeLogs := resourceLog.ScopeLogs().At(j)

			for k := 0; k < scopeLogs.LogRecords().Len(); k++ {
				logRecord := scopeLogs.LogRecords().At(k)

				lCtx := ottllog.NewTransformContext(logRecord, scopeLogs.Scope(), resourceLog.Resource(), scopeLogs, resourceLog)
				multiError = errors.Join(multiError, counter.update(ctx, logRecord.Attributes(), lCtx))
			}
		}

		if len(counter.counts) == 0 {
			continue // don't add an empty resource
		}

		countResource := countMetrics.ResourceMetrics().AppendEmpty()
		resourceLog.Resource().Attributes().CopyTo(countResource.Resource().Attributes())

		countResource.ScopeMetrics().EnsureCapacity(resourceLog.ScopeLogs().Len())
		countScope := countResource.ScopeMetrics().AppendEmpty()
		countScope.Scope().SetName(metadata.ScopeName)

		counter.appendMetricsTo(countScope.Metrics())
	}
	if multiError != nil {
		return multiError
	}
	return c.metricsConsumer.ConsumeMetrics(ctx, countMetrics)
}

func (c *count) ConsumeProfiles(ctx context.Context, ld pprofile.Profiles) error {
	var multiError error
	countMetrics := pmetric.NewMetrics()
	countMetrics.ResourceMetrics().EnsureCapacity(ld.ResourceProfiles().Len())
	for i := 0; i < ld.ResourceProfiles().Len(); i++ {
		resourceProfile := ld.ResourceProfiles().At(i)
		counter := newCounter[ottlprofile.TransformContext](c.profilesMetricDefs)

		for j := 0; j < resourceProfile.ScopeProfiles().Len(); j++ {
			scopeProfile := resourceProfile.ScopeProfiles().At(j)

			for k := 0; k < scopeProfile.Profiles().Len(); k++ {
				profile := scopeProfile.Profiles().At(k)

				pCtx := ottlprofile.NewTransformContext(profile, ld.ProfilesDictionary(), scopeProfile.Scope(), resourceProfile.Resource(), scopeProfile, resourceProfile)
				attributes := pprofile.FromAttributeIndices(ld.ProfilesDictionary().AttributeTable(), profile)
				multiError = errors.Join(multiError, counter.update(ctx, attributes, pCtx))
			}
		}

		if len(counter.counts) == 0 {
			continue // don't add an empty resource
		}

		countResource := countMetrics.ResourceMetrics().AppendEmpty()
		resourceProfile.Resource().Attributes().CopyTo(countResource.Resource().Attributes())

		countResource.ScopeMetrics().EnsureCapacity(resourceProfile.ScopeProfiles().Len())
		countScope := countResource.ScopeMetrics().AppendEmpty()
		countScope.Scope().SetName(metadata.ScopeName)

		counter.appendMetricsTo(countScope.Metrics())
	}
	if multiError != nil {
		return multiError
	}
	return c.metricsConsumer.ConsumeMetrics(ctx, countMetrics)
}
