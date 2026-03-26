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
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/aggregator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/model"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofile"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
)

type signalToMetrics struct {
	next                  consumer.Metrics
	collectorInstanceInfo model.CollectorInstanceInfo
	logger                *zap.Logger
	errorMode             ottl.ErrorMode

	spanMetricDefs    []model.MetricDef[*ottlspan.TransformContext]
	dpMetricDefs      []model.MetricDef[*ottldatapoint.TransformContext]
	logMetricDefs     []model.MetricDef[*ottllog.TransformContext]
	profileMetricDefs []model.MetricDef[*ottlprofile.TransformContext]

	component.StartFunc
	component.ShutdownFunc
}

func (*signalToMetrics) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (sm *signalToMetrics) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	if len(sm.spanMetricDefs) == 0 {
		return nil
	}

	processedMetrics := pmetric.NewMetrics()
	processedMetrics.ResourceMetrics().EnsureCapacity(td.ResourceSpans().Len())
	aggregator := aggregator.NewAggregator[*ottlspan.TransformContext](processedMetrics, sm.errorMode, sm.logger)
	// resAttrsCache lazily caches the filtered resource attributes per
	// metric definition within a resource. Since resource attributes are
	// constant for all signals within a resource, the result only needs
	// to be computed once per metric definition per resource. The slice
	// is allocated on the first match and reused across resources via
	// clear(). A zero-value pcommon.Map entry indicates it has not been
	// computed yet for the current resource.
	var resAttrsCache []pcommon.Map

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		resourceSpan := td.ResourceSpans().At(i)
		resourceAttrs := resourceSpan.Resource().Attributes()
		clear(resAttrsCache)
		for j := 0; j < resourceSpan.ScopeSpans().Len(); j++ {
			scopeSpan := resourceSpan.ScopeSpans().At(j)
			for k := 0; k < scopeSpan.Spans().Len(); k++ {
				span := scopeSpan.Spans().At(k)
				spanAttrs := span.Attributes()
				for mdIdx, md := range sm.spanMetricDefs {
					if !md.MatchAttributes(spanAttrs) {
						continue
					}
					tCtx := ottlspan.NewTransformContextPtr(resourceSpan, scopeSpan, span)
					resolvedAttrs, err := md.ResolveAttributes(ctx, tCtx)
					if err != nil {
						tCtx.Close()
						return fmt.Errorf("failed to resolve attributes: %w", err)
					}
					attrID := md.ComputeAttributesHash(spanAttrs, resolvedAttrs)

					if md.Conditions != nil {
						var match bool
						match, err = md.Conditions.Eval(ctx, tCtx)
						if err != nil {
							tCtx.Close()
							return fmt.Errorf("failed to evaluate conditions: %w", err)
						}
						if !match {
							tCtx.Close()
							sm.logger.Debug("condition not matched, skipping", zap.String("name", md.Key.Name))
							continue
						}
					}

					if len(resAttrsCache) == 0 {
						resAttrsCache = make([]pcommon.Map, len(sm.spanMetricDefs))
					}
					if resAttrsCache[mdIdx] == (pcommon.Map{}) {
						resolvedResAttrs, resErr := md.ResolveIncludeResourceAttributes(ctx, tCtx)
						if resErr != nil {
							tCtx.Close()
							return fmt.Errorf("failed to resolve resource attributes: %w", resErr)
						}
						resAttrsCache[mdIdx] = md.FilterResourceAttributes(resourceAttrs, resolvedResAttrs, sm.collectorInstanceInfo)
					}

					filterAttrs := func() (pcommon.Map, error) {
						return md.FilterAttributes(spanAttrs, resolvedAttrs), nil
					}
					err = aggregator.Aggregate(ctx, tCtx, md, resAttrsCache[mdIdx], attrID, filterAttrs, 1)
					tCtx.Close()
					if err != nil {
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
	aggregator := aggregator.NewAggregator[*ottldatapoint.TransformContext](processedMetrics, sm.errorMode, sm.logger)
	// resAttrsCache lazily caches the filtered resource attributes per
	// metric definition within a resource. Since resource attributes are
	// constant for all signals within a resource, the result only needs
	// to be computed once per metric definition per resource. The slice
	// is allocated on the first match and reused across resources via
	// clear(). A zero-value pcommon.Map entry indicates it has not been
	// computed yet for the current resource.
	var resAttrsCache []pcommon.Map

	for i := 0; i < m.ResourceMetrics().Len(); i++ {
		resourceMetric := m.ResourceMetrics().At(i)
		resourceAttrs := resourceMetric.Resource().Attributes()
		clear(resAttrsCache)
		for j := 0; j < resourceMetric.ScopeMetrics().Len(); j++ {
			scopeMetric := resourceMetric.ScopeMetrics().At(j)
			for k := 0; k < scopeMetric.Metrics().Len(); k++ {
				metrics := scopeMetric.Metrics()
				metric := metrics.At(k)
				for mdIdx, md := range sm.dpMetricDefs {
					aggregate := func(dp any, dpAttrs pcommon.Map) error {
						if !md.MatchAttributes(dpAttrs) {
							return nil
						}
						tCtx := ottldatapoint.NewTransformContextPtr(resourceMetric, scopeMetric, metric, dp)
						defer tCtx.Close()
						resolvedAttrs, err := md.ResolveAttributes(ctx, tCtx)
						if err != nil {
							return fmt.Errorf("failed to resolve attributes: %w", err)
						}
						attrID := md.ComputeAttributesHash(dpAttrs, resolvedAttrs)

						if len(resAttrsCache) == 0 {
							resAttrsCache = make([]pcommon.Map, len(sm.dpMetricDefs))
						}
						if resAttrsCache[mdIdx] == (pcommon.Map{}) {
							resolvedResAttrs, resErr := md.ResolveIncludeResourceAttributes(ctx, tCtx)
							if resErr != nil {
								return fmt.Errorf("failed to resolve resource attributes: %w", resErr)
							}
							resAttrsCache[mdIdx] = md.FilterResourceAttributes(resourceAttrs, resolvedResAttrs, sm.collectorInstanceInfo)
						}

						if md.Conditions != nil {
							var match bool
							match, err = md.Conditions.Eval(ctx, tCtx)
							if err != nil {
								return fmt.Errorf("failed to evaluate conditions: %w", err)
							}
							if !match {
								sm.logger.Debug("condition not matched, skipping", zap.String("name", md.Key.Name))
								return nil
							}
						}
						filterAttrs := func() (pcommon.Map, error) {
							return md.FilterAttributes(dpAttrs, resolvedAttrs), nil
						}
						return aggregator.Aggregate(ctx, tCtx, md, resAttrsCache[mdIdx], attrID, filterAttrs, 1)
					}

					//exhaustive:enforce
					switch metric.Type() {
					case pmetric.MetricTypeGauge:
						dps := metric.Gauge().DataPoints()
						for l := 0; l < dps.Len(); l++ {
							if err := aggregate(dps.At(l), dps.At(l).Attributes()); err != nil {
								return err
							}
						}
					case pmetric.MetricTypeSum:
						dps := metric.Sum().DataPoints()
						for l := 0; l < dps.Len(); l++ {
							if err := aggregate(dps.At(l), dps.At(l).Attributes()); err != nil {
								return err
							}
						}
					case pmetric.MetricTypeSummary:
						dps := metric.Summary().DataPoints()
						for l := 0; l < dps.Len(); l++ {
							if err := aggregate(dps.At(l), dps.At(l).Attributes()); err != nil {
								return err
							}
						}
					case pmetric.MetricTypeHistogram:
						dps := metric.Histogram().DataPoints()
						for l := 0; l < dps.Len(); l++ {
							if err := aggregate(dps.At(l), dps.At(l).Attributes()); err != nil {
								return err
							}
						}
					case pmetric.MetricTypeExponentialHistogram:
						dps := metric.ExponentialHistogram().DataPoints()
						for l := 0; l < dps.Len(); l++ {
							if err := aggregate(dps.At(l), dps.At(l).Attributes()); err != nil {
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
	aggregator := aggregator.NewAggregator[*ottllog.TransformContext](processedMetrics, sm.errorMode, sm.logger)
	// resAttrsCache lazily caches the filtered resource attributes per
	// metric definition within a resource. Since resource attributes are
	// constant for all signals within a resource, the result only needs
	// to be computed once per metric definition per resource. The slice
	// is allocated on the first match and reused across resources via
	// clear(). A zero-value pcommon.Map entry indicates it has not been
	// computed yet for the current resource.
	var resAttrsCache []pcommon.Map

	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		resourceLog := logs.ResourceLogs().At(i)
		resourceAttrs := resourceLog.Resource().Attributes()
		clear(resAttrsCache)
		for j := 0; j < resourceLog.ScopeLogs().Len(); j++ {
			scopeLog := resourceLog.ScopeLogs().At(j)
			for k := 0; k < scopeLog.LogRecords().Len(); k++ {
				log := scopeLog.LogRecords().At(k)
				logAttrs := log.Attributes()
				for mdIdx, md := range sm.logMetricDefs {
					if !md.MatchAttributes(logAttrs) {
						continue
					}
					tCtx := ottllog.NewTransformContextPtr(resourceLog, scopeLog, log)
					resolvedAttrs, err := md.ResolveAttributes(ctx, tCtx)
					if err != nil {
						tCtx.Close()
						return fmt.Errorf("failed to resolve attributes: %w", err)
					}
					attrID := md.ComputeAttributesHash(logAttrs, resolvedAttrs)

					if md.Conditions != nil {
						var match bool
						match, err = md.Conditions.Eval(ctx, tCtx)
						if err != nil {
							tCtx.Close()
							return fmt.Errorf("failed to evaluate conditions: %w", err)
						}
						if !match {
							tCtx.Close()
							sm.logger.Debug("condition not matched, skipping", zap.String("name", md.Key.Name))
							continue
						}
					}

					if len(resAttrsCache) == 0 {
						resAttrsCache = make([]pcommon.Map, len(sm.logMetricDefs))
					}
					if resAttrsCache[mdIdx] == (pcommon.Map{}) {
						resolvedResAttrs, resErr := md.ResolveIncludeResourceAttributes(ctx, tCtx)
						if resErr != nil {
							tCtx.Close()
							return fmt.Errorf("failed to resolve resource attributes: %w", resErr)
						}
						resAttrsCache[mdIdx] = md.FilterResourceAttributes(resourceAttrs, resolvedResAttrs, sm.collectorInstanceInfo)
					}

					filterAttrs := func() (pcommon.Map, error) {
						return md.FilterAttributes(logAttrs, resolvedAttrs), nil
					}
					err = aggregator.Aggregate(ctx, tCtx, md, resAttrsCache[mdIdx], attrID, filterAttrs, 1)
					tCtx.Close()
					if err != nil {
						return err
					}
				}
			}
		}
	}
	aggregator.Finalize(sm.logMetricDefs)
	return sm.next.ConsumeMetrics(ctx, processedMetrics)
}

func (sm *signalToMetrics) ConsumeProfiles(ctx context.Context, profiles pprofile.Profiles) error {
	if len(sm.profileMetricDefs) == 0 {
		return nil
	}

	processedMetrics := pmetric.NewMetrics()
	processedMetrics.ResourceMetrics().EnsureCapacity(profiles.ResourceProfiles().Len())
	aggregator := aggregator.NewAggregator[*ottlprofile.TransformContext](processedMetrics, sm.errorMode, sm.logger)
	// resAttrsCache lazily caches the filtered resource attributes per
	// metric definition within a resource. Since resource attributes are
	// constant for all signals within a resource, the result only needs
	// to be computed once per metric definition per resource. The slice
	// is allocated on the first match and reused across resources via
	// clear(). A zero-value pcommon.Map entry indicates it has not been
	// computed yet for the current resource.
	var resAttrsCache []pcommon.Map

	for i := 0; i < profiles.ResourceProfiles().Len(); i++ {
		resourceProfile := profiles.ResourceProfiles().At(i)
		resourceAttrs := resourceProfile.Resource().Attributes()
		clear(resAttrsCache)
		for j := 0; j < resourceProfile.ScopeProfiles().Len(); j++ {
			scopeProfile := resourceProfile.ScopeProfiles().At(j)

			for k := 0; k < scopeProfile.Profiles().Len(); k++ {
				profile := scopeProfile.Profiles().At(k)
				profileAttrs := pprofile.FromAttributeIndices(profiles.Dictionary().AttributeTable(), profile, profiles.Dictionary())
				for mdIdx, md := range sm.profileMetricDefs {
					if !md.MatchAttributes(profileAttrs) {
						continue
					}
					tCtx := ottlprofile.NewTransformContextPtr(resourceProfile, scopeProfile, profile, profiles.Dictionary())
					resolvedAttrs, err := md.ResolveAttributes(ctx, tCtx)
					if err != nil {
						tCtx.Close()
						return fmt.Errorf("failed to resolve attributes: %w", err)
					}
					attrID := md.ComputeAttributesHash(profileAttrs, resolvedAttrs)

					if md.Conditions != nil {
						var match bool
						match, err = md.Conditions.Eval(ctx, tCtx)
						if err != nil {
							tCtx.Close()
							return fmt.Errorf("failed to evaluate conditions: %w", err)
						}
						if !match {
							sm.logger.Debug("condition not matched, skipping", zap.String("name", md.Key.Name))
							tCtx.Close()
							continue
						}
					}

					if len(resAttrsCache) == 0 {
						resAttrsCache = make([]pcommon.Map, len(sm.profileMetricDefs))
					}
					if resAttrsCache[mdIdx] == (pcommon.Map{}) {
						resolvedResAttrs, resErr := md.ResolveIncludeResourceAttributes(ctx, tCtx)
						if resErr != nil {
							tCtx.Close()
							return fmt.Errorf("failed to resolve resource attributes: %w", resErr)
						}
						resAttrsCache[mdIdx] = md.FilterResourceAttributes(resourceAttrs, resolvedResAttrs, sm.collectorInstanceInfo)
					}

					filterAttrs := func() (pcommon.Map, error) {
						return md.FilterAttributes(profileAttrs, resolvedAttrs), nil
					}
					if err := aggregator.Aggregate(ctx, tCtx, md, resAttrsCache[mdIdx], attrID, filterAttrs, 1); err != nil {
						tCtx.Close()
						return err
					}
					tCtx.Close()
				}
			}
		}
	}
	aggregator.Finalize(sm.profileMetricDefs)
	return sm.next.ConsumeMetrics(ctx, processedMetrics)
}
