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

	// resAttrsCache caches FilterResourceAttributes results per metric-def index within
	// a single ResourceSpan. Nil until first match
	var (
		resAttrsCache      []pcommon.Map
		resAttrsCacheReady []bool
	)

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		resourceSpan := td.ResourceSpans().At(i)
		resourceAttrs := resourceSpan.Resource().Attributes()
		// Reset cache for this new resource span
		resAttrsCache = resAttrsCache[:0]
		resAttrsCacheReady = resAttrsCacheReady[:0]

		for j := 0; j < resourceSpan.ScopeSpans().Len(); j++ {
			scopeSpan := resourceSpan.ScopeSpans().At(j)
			for k := 0; k < scopeSpan.Spans().Len(); k++ {
				span := scopeSpan.Spans().At(k)
				spanAttrs := span.Attributes()
				for mdIdx, md := range sm.spanMetricDefs {
					// FilterAttributesID checks required attrs and returns a hash ID
					// without allocating a pcommon.Map
					attrID, ok := md.FilterAttributesID(spanAttrs)
					if !ok {
						continue
					}

					// The transform context is created from original attributes so that the
					// OTTL expressions are also applied on the original attributes.
					tCtx := ottlspan.NewTransformContextPtr(resourceSpan, scopeSpan, span)
					if md.Conditions != nil {
						match, err := md.Conditions.Eval(ctx, tCtx)
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

					// Lazy-compute and cache filtered resource attrs per metric def
					for len(resAttrsCache) <= mdIdx {
						resAttrsCache = append(resAttrsCache, pcommon.Map{})
						resAttrsCacheReady = append(resAttrsCacheReady, false)
					}
					if !resAttrsCacheReady[mdIdx] {
						resAttrsCache[mdIdx] = md.FilterResourceAttributes(resourceAttrs, sm.collectorInstanceInfo)
						resAttrsCacheReady[mdIdx] = true
					}

					err := aggregator.Aggregate(ctx, tCtx, md, resAttrsCache[mdIdx], spanAttrs, attrID, 1)
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

	// resAttrsCache caches FilterResourceAttributes results per metric-def index within
	// a single ResourceMetric. Nil until first match
	var (
		resAttrsCache      []pcommon.Map
		resAttrsCacheReady []bool
	)

	for i := 0; i < m.ResourceMetrics().Len(); i++ {
		resourceMetric := m.ResourceMetrics().At(i)
		resourceAttrs := resourceMetric.Resource().Attributes()
		// Reset cache for this new resource metric
		resAttrsCache = resAttrsCache[:0]
		resAttrsCacheReady = resAttrsCacheReady[:0]

		for j := 0; j < resourceMetric.ScopeMetrics().Len(); j++ {
			scopeMetric := resourceMetric.ScopeMetrics().At(j)
			for k := 0; k < scopeMetric.Metrics().Len(); k++ {
				metrics := scopeMetric.Metrics()
				metric := metrics.At(k)
				for mdIdx, md := range sm.dpMetricDefs {
					// aggregate is called only when a DP passes the attribute filter.
					// filteredResAttrs is computed lazily inside and cached per metric-def
					// index within the current ResourceMetric
					aggregate := func(dp any, originalDPAttrs pcommon.Map, attrID [16]byte) error {
						// Lazy-compute and cache filtered resource attrs
						for len(resAttrsCache) <= mdIdx {
							resAttrsCache = append(resAttrsCache, pcommon.Map{})
							resAttrsCacheReady = append(resAttrsCacheReady, false)
						}
						if !resAttrsCacheReady[mdIdx] {
							resAttrsCache[mdIdx] = md.FilterResourceAttributes(resourceAttrs, sm.collectorInstanceInfo)
							resAttrsCacheReady[mdIdx] = true
						}
						// The transform context is created from original attributes so that the
						// OTTL expressions are also applied on the original attributes.
						tCtx := ottldatapoint.NewTransformContextPtr(resourceMetric, scopeMetric, metric, dp)
						defer tCtx.Close()
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
						return aggregator.Aggregate(ctx, tCtx, md, resAttrsCache[mdIdx], originalDPAttrs, attrID, 1)
					}

					//exhaustive:enforce
					switch metric.Type() {
					case pmetric.MetricTypeGauge:
						dps := metric.Gauge().DataPoints()
						for l := 0; l < dps.Len(); l++ {
							dp := dps.At(l)
							attrID, ok := md.FilterAttributesID(dp.Attributes())
							if !ok {
								continue
							}
							if err := aggregate(dp, dp.Attributes(), attrID); err != nil {
								return err
							}
						}
					case pmetric.MetricTypeSum:
						dps := metric.Sum().DataPoints()
						for l := 0; l < dps.Len(); l++ {
							dp := dps.At(l)
							attrID, ok := md.FilterAttributesID(dp.Attributes())
							if !ok {
								continue
							}
							if err := aggregate(dp, dp.Attributes(), attrID); err != nil {
								return err
							}
						}
					case pmetric.MetricTypeSummary:
						dps := metric.Summary().DataPoints()
						for l := 0; l < dps.Len(); l++ {
							dp := dps.At(l)
							attrID, ok := md.FilterAttributesID(dp.Attributes())
							if !ok {
								continue
							}
							if err := aggregate(dp, dp.Attributes(), attrID); err != nil {
								return err
							}
						}
					case pmetric.MetricTypeHistogram:
						dps := metric.Histogram().DataPoints()
						for l := 0; l < dps.Len(); l++ {
							dp := dps.At(l)
							attrID, ok := md.FilterAttributesID(dp.Attributes())
							if !ok {
								continue
							}
							if err := aggregate(dp, dp.Attributes(), attrID); err != nil {
								return err
							}
						}
					case pmetric.MetricTypeExponentialHistogram:
						dps := metric.ExponentialHistogram().DataPoints()
						for l := 0; l < dps.Len(); l++ {
							dp := dps.At(l)
							attrID, ok := md.FilterAttributesID(dp.Attributes())
							if !ok {
								continue
							}
							if err := aggregate(dp, dp.Attributes(), attrID); err != nil {
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

	// resAttrsCache caches FilterResourceAttributes results per metric-def index within
	// a single ResourceLog. Nil until first match (Exp4: lazy allocation).
	var (
		resAttrsCache      []pcommon.Map
		resAttrsCacheReady []bool
	)

	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		resourceLog := logs.ResourceLogs().At(i)
		resourceAttrs := resourceLog.Resource().Attributes()
		// Reset cache for this new resource log (keep backing array).
		resAttrsCache = resAttrsCache[:0]
		resAttrsCacheReady = resAttrsCacheReady[:0]

		for j := 0; j < resourceLog.ScopeLogs().Len(); j++ {
			scopeLog := resourceLog.ScopeLogs().At(j)
			for k := 0; k < scopeLog.LogRecords().Len(); k++ {
				log := scopeLog.LogRecords().At(k)
				logAttrs := log.Attributes()
				for mdIdx, md := range sm.logMetricDefs {
					// FilterAttributesID checks required attrs and returns a hash ID
					// without allocating a pcommon.Map (Exp8).
					attrID, ok := md.FilterAttributesID(logAttrs)
					if !ok {
						continue
					}

					// The transform context is created from original attributes so that the
					// OTTL expressions are also applied on the original attributes.
					tCtx := ottllog.NewTransformContextPtr(resourceLog, scopeLog, log)
					if md.Conditions != nil {
						match, err := md.Conditions.Eval(ctx, tCtx)
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

					// Lazy-compute and cache filtered resource attrs per metric def (Exp2+3).
					for len(resAttrsCache) <= mdIdx {
						resAttrsCache = append(resAttrsCache, pcommon.Map{})
						resAttrsCacheReady = append(resAttrsCacheReady, false)
					}
					if !resAttrsCacheReady[mdIdx] {
						resAttrsCache[mdIdx] = md.FilterResourceAttributes(resourceAttrs, sm.collectorInstanceInfo)
						resAttrsCacheReady[mdIdx] = true
					}

					err := aggregator.Aggregate(ctx, tCtx, md, resAttrsCache[mdIdx], logAttrs, attrID, 1)
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

	for i := 0; i < profiles.ResourceProfiles().Len(); i++ {
		resourceProfile := profiles.ResourceProfiles().At(i)
		resourceAttrs := resourceProfile.Resource().Attributes()

		for j := 0; j < resourceProfile.ScopeProfiles().Len(); j++ {
			scopeProfile := resourceProfile.ScopeProfiles().At(j)

			for k := 0; k < scopeProfile.Profiles().Len(); k++ {
				profile := scopeProfile.Profiles().At(k)
				profileAttrs := pprofile.FromAttributeIndices(profiles.Dictionary().AttributeTable(), profile, profiles.Dictionary())

				for _, md := range sm.profileMetricDefs {
					// FilterAttributesID checks required attrs and returns a hash ID
					// without allocating a pcommon.Map (Exp8).
					attrID, ok := md.FilterAttributesID(profileAttrs)
					if !ok {
						continue
					}

					// The transform context is created from original attributes so that the
					// OTTL expressions are also applied on the original attributes.
					tCtx := ottlprofile.NewTransformContextPtr(resourceProfile, scopeProfile, profile, profiles.Dictionary())
					if md.Conditions != nil {
						match, err := md.Conditions.Eval(ctx, tCtx)
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
					filteredResAttrs := md.FilterResourceAttributes(resourceAttrs, sm.collectorInstanceInfo)
					if err := aggregator.Aggregate(ctx, tCtx, md, filteredResAttrs, profileAttrs, attrID, 1); err != nil {
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
