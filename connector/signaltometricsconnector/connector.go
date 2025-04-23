// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signaltometricsconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/aggregator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/model"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
)

type signalToMetrics struct {
	next                  consumer.Metrics
	collectorInstanceInfo model.CollectorInstanceInfo
	logger                *zap.Logger

	spanMetricDefs []model.MetricDef[ottlspan.TransformContext]
	dpMetricDefs   []model.MetricDef[ottldatapoint.TransformContext]
	logMetricDefs  []model.MetricDef[ottllog.TransformContext]

	component.StartFunc
	component.ShutdownFunc
}

func (sm *signalToMetrics) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (sm *signalToMetrics) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	if len(sm.spanMetricDefs) == 0 {
		return nil
	}

	processedMetrics := pmetric.NewMetrics()
	processedMetrics.ResourceMetrics().EnsureCapacity(td.ResourceSpans().Len())
	aggregator := aggregator.NewAggregator[ottlspan.TransformContext](processedMetrics)

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		resourceSpan := td.ResourceSpans().At(i)
		resourceAttrs := resourceSpan.Resource().Attributes()
		for j := 0; j < resourceSpan.ScopeSpans().Len(); j++ {
			scopeSpan := resourceSpan.ScopeSpans().At(j)
			for k := 0; k < scopeSpan.Spans().Len(); k++ {
				span := scopeSpan.Spans().At(k)
				spanAttrs := span.Attributes()
				for _, md := range sm.spanMetricDefs {
					filteredSpanAttrs, ok := md.FilterAttributes(spanAttrs)
					if !ok {
						continue
					}

					// The transform context is created from original attributes so that the
					// OTTL expressions are also applied on the original attributes.
					tCtx := ottlspan.NewTransformContext(span, scopeSpan.Scope(), resourceSpan.Resource(), scopeSpan, resourceSpan)
					if md.Conditions != nil {
						match, err := md.Conditions.Eval(ctx, tCtx)
						if err != nil {
							return fmt.Errorf("failed to evaluate conditions: %w", err)
						}
						if !match {
							sm.logger.Debug("condition not matched, skipping", zap.String("name", md.Key.Name))
							continue
						}
					}

					filteredResAttrs := md.FilterResourceAttributes(resourceAttrs, sm.collectorInstanceInfo)
					if err := aggregator.Aggregate(ctx, tCtx, md, filteredResAttrs, filteredSpanAttrs, 1); err != nil {
						return err
					}
				}
			}
		}
	}
	aggregator.Finalize(sm.spanMetricDefs)
	return sm.next.ConsumeMetrics(ctx, processedMetrics)
}

func (sm *signalToMetrics) ConsumeMetrics(ctx context.Context, m pmetric.Metrics) error {
	if len(sm.dpMetricDefs) == 0 {
		return nil
	}

	processedMetrics := pmetric.NewMetrics()
	processedMetrics.ResourceMetrics().EnsureCapacity(m.ResourceMetrics().Len())
	aggregator := aggregator.NewAggregator[ottldatapoint.TransformContext](processedMetrics)
	for i := 0; i < m.ResourceMetrics().Len(); i++ {
		resourceMetric := m.ResourceMetrics().At(i)
		resourceAttrs := resourceMetric.Resource().Attributes()
		for j := 0; j < resourceMetric.ScopeMetrics().Len(); j++ {
			scopeMetric := resourceMetric.ScopeMetrics().At(j)
			for k := 0; k < scopeMetric.Metrics().Len(); k++ {
				metrics := scopeMetric.Metrics()
				metric := metrics.At(k)
				for _, md := range sm.dpMetricDefs {
					filteredResAttrs := md.FilterResourceAttributes(resourceAttrs, sm.collectorInstanceInfo)
					aggregate := func(dp any, dpAttrs pcommon.Map) error {
						// The transform context is created from original attributes so that the
						// OTTL expressions are also applied on the original attributes.
						tCtx := ottldatapoint.NewTransformContext(dp, metric, metrics, scopeMetric.Scope(), resourceMetric.Resource(), scopeMetric, resourceMetric)
						if md.Conditions != nil {
							match, err := md.Conditions.Eval(ctx, tCtx)
							if err != nil {
								return fmt.Errorf("failed to evaluate conditions: %w", err)
							}
							if !match {
								sm.logger.Debug("condition not matched, skipping", zap.String("name", md.Key.Name))
								return nil
							}
						}
						return aggregator.Aggregate(ctx, tCtx, md, filteredResAttrs, dpAttrs, 1)
					}

					//exhaustive:enforce
					switch metric.Type() {
					case pmetric.MetricTypeGauge:
						dps := metric.Gauge().DataPoints()
						for l := 0; l < dps.Len(); l++ {
							dp := dps.At(l)
							filteredDPAttrs, ok := md.FilterAttributes(dp.Attributes())
							if !ok {
								continue
							}
							if err := aggregate(dp, filteredDPAttrs); err != nil {
								return err
							}
						}
					case pmetric.MetricTypeSum:
						dps := metric.Sum().DataPoints()
						for l := 0; l < dps.Len(); l++ {
							dp := dps.At(l)
							filteredDPAttrs, ok := md.FilterAttributes(dp.Attributes())
							if !ok {
								continue
							}
							if err := aggregate(dp, filteredDPAttrs); err != nil {
								return err
							}
						}
					case pmetric.MetricTypeSummary:
						dps := metric.Summary().DataPoints()
						for l := 0; l < dps.Len(); l++ {
							dp := dps.At(l)
							filteredDPAttrs, ok := md.FilterAttributes(dp.Attributes())
							if !ok {
								continue
							}
							if err := aggregate(dp, filteredDPAttrs); err != nil {
								return err
							}
						}
					case pmetric.MetricTypeHistogram:
						dps := metric.Histogram().DataPoints()
						for l := 0; l < dps.Len(); l++ {
							dp := dps.At(l)
							filteredDPAttrs, ok := md.FilterAttributes(dp.Attributes())
							if !ok {
								continue
							}
							if err := aggregate(dp, filteredDPAttrs); err != nil {
								return err
							}
						}
					case pmetric.MetricTypeExponentialHistogram:
						dps := metric.ExponentialHistogram().DataPoints()
						for l := 0; l < dps.Len(); l++ {
							dp := dps.At(l)
							filteredDPAttrs, ok := md.FilterAttributes(dp.Attributes())
							if !ok {
								continue
							}
							if err := aggregate(dp, filteredDPAttrs); err != nil {
								return err
							}
						}
					case pmetric.MetricTypeEmpty:
						continue
					}
				}
			}
		}
	}
	aggregator.Finalize(sm.dpMetricDefs)
	return sm.next.ConsumeMetrics(ctx, processedMetrics)
}

func (sm *signalToMetrics) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	if len(sm.logMetricDefs) == 0 {
		return nil
	}

	processedMetrics := pmetric.NewMetrics()
	processedMetrics.ResourceMetrics().EnsureCapacity(logs.ResourceLogs().Len())
	aggregator := aggregator.NewAggregator[ottllog.TransformContext](processedMetrics)
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		resourceLog := logs.ResourceLogs().At(i)
		resourceAttrs := resourceLog.Resource().Attributes()
		for j := 0; j < resourceLog.ScopeLogs().Len(); j++ {
			scopeLog := resourceLog.ScopeLogs().At(j)
			for k := 0; k < scopeLog.LogRecords().Len(); k++ {
				log := scopeLog.LogRecords().At(k)
				logAttrs := log.Attributes()
				for _, md := range sm.logMetricDefs {
					filteredLogAttrs, ok := md.FilterAttributes(logAttrs)
					if !ok {
						continue
					}

					// The transform context is created from original attributes so that the
					// OTTL expressions are also applied on the original attributes.
					tCtx := ottllog.NewTransformContext(log, scopeLog.Scope(), resourceLog.Resource(), scopeLog, resourceLog)
					if md.Conditions != nil {
						match, err := md.Conditions.Eval(ctx, tCtx)
						if err != nil {
							return fmt.Errorf("failed to evaluate conditions: %w", err)
						}
						if !match {
							sm.logger.Debug("condition not matched, skipping", zap.String("name", md.Key.Name))
							continue
						}
					}
					filteredResAttrs := md.FilterResourceAttributes(resourceAttrs, sm.collectorInstanceInfo)
					if err := aggregator.Aggregate(ctx, tCtx, md, filteredResAttrs, filteredLogAttrs, 1); err != nil {
						return err
					}
				}
			}
		}
	}
	aggregator.Finalize(sm.logMetricDefs)
	return sm.next.ConsumeMetrics(ctx, processedMetrics)
}
