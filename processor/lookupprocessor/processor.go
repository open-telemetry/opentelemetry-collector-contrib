// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"
)

// lookupProcessor is generic over the OTTL transform context type.
type lookupProcessor[T any] struct {
	source  lookupsource.Source
	lookups []parsedLookup[T]
	logger  *zap.Logger
}

func newLookupProcessor[T any](source lookupsource.Source, lookups []parsedLookup[T], logger *zap.Logger) *lookupProcessor[T] {
	return &lookupProcessor[T]{
		source:  source,
		lookups: lookups,
		logger:  logger,
	}
}

func (p *lookupProcessor[T]) Start(ctx context.Context, host component.Host) error {
	return p.source.Start(ctx, host)
}

func (p *lookupProcessor[T]) Shutdown(ctx context.Context) error {
	return p.source.Shutdown(ctx)
}

func (p *lookupProcessor[T]) evalAndProcess(ctx context.Context, tCtx T, recordAttrs, resourceAttrs pcommon.Map) {
	for li := range p.lookups {
		lookup := &p.lookups[li]
		rawKey, err := lookup.keyExpr.Eval(ctx, tCtx)
		if err != nil {
			p.logger.Debug("failed to evaluate key expression", zap.Error(err))
			continue
		}
		processLookupResult(ctx, p.source, p.logger, rawKey, lookup.context, lookup.attributes, recordAttrs, resourceAttrs)
	}
}

// processLookupResult performs the source lookup and writes results to record or resource attributes.
// If the key is nil or empty, or if the lookup fails, the function returns without writing.
func processLookupResult(
	ctx context.Context,
	source lookupsource.Source,
	logger *zap.Logger,
	key any,
	lookupCtx ContextID,
	attributes []AttributeMapping,
	recordAttrs pcommon.Map,
	resourceAttrs pcommon.Map,
) {
	if key == nil {
		return
	}

	keyStr := anyToString(key)
	if keyStr == "" {
		return
	}

	result, found, err := source.Lookup(ctx, keyStr)
	if err != nil {
		logger.Debug("lookup failed", zap.String("key", keyStr), zap.Error(err))
		return
	}

	for ai := range attributes {
		attr := &attributes[ai]
		attrCtx := attr.GetContext(lookupCtx)

		var attrs pcommon.Map
		if attrCtx == ContextResource {
			attrs = resourceAttrs
		} else {
			attrs = recordAttrs
		}

		value, ok := extractValue(result, found, attr)
		if !ok {
			continue
		}

		if err := attrs.PutEmpty(attr.Destination).FromRaw(value); err != nil {
			attrs.PutStr(attr.Destination, fmt.Sprintf("%v", value))
		}
	}
}

type logsLookupProcessor struct {
	*lookupProcessor[*ottllog.TransformContext]
}

func (p *logsLookupProcessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		resourceAttrs := rl.Resource().Attributes()
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			for k := 0; k < sl.LogRecords().Len(); k++ {
				lr := sl.LogRecords().At(k)
				tCtx := ottllog.NewTransformContextPtr(rl, sl, lr)
				p.evalAndProcess(ctx, tCtx, lr.Attributes(), resourceAttrs)
				tCtx.Close()
			}
		}
	}
	return ld, nil
}

type tracesLookupProcessor struct {
	*lookupProcessor[*ottlspan.TransformContext]
}

func (p *tracesLookupProcessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)
		resourceAttrs := rs.Resource().Attributes()
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)
				tCtx := ottlspan.NewTransformContextPtr(rs, ss, span)
				p.evalAndProcess(ctx, tCtx, span.Attributes(), resourceAttrs)
				tCtx.Close()
			}
		}
	}
	return td, nil
}

type metricsLookupProcessor struct {
	*lookupProcessor[*ottldatapoint.TransformContext]
}

func (p *metricsLookupProcessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		resourceAttrs := rm.Resource().Attributes()
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)
				switch m.Type() {
				case pmetric.MetricTypeGauge:
					processDataPoints(ctx, p.lookupProcessor, m.Gauge().DataPoints(), rm, sm, m, resourceAttrs)
				case pmetric.MetricTypeSum:
					processDataPoints(ctx, p.lookupProcessor, m.Sum().DataPoints(), rm, sm, m, resourceAttrs)
				case pmetric.MetricTypeHistogram:
					processDataPoints(ctx, p.lookupProcessor, m.Histogram().DataPoints(), rm, sm, m, resourceAttrs)
				case pmetric.MetricTypeExponentialHistogram:
					processDataPoints(ctx, p.lookupProcessor, m.ExponentialHistogram().DataPoints(), rm, sm, m, resourceAttrs)
				case pmetric.MetricTypeSummary:
					processDataPoints(ctx, p.lookupProcessor, m.Summary().DataPoints(), rm, sm, m, resourceAttrs)
				}
			}
		}
	}
	return md, nil
}

// dataPointSlice is a generic interface for pmetric datapoint slices.
type dataPointSlice[DP dataPointWithAttributes] interface {
	Len() int
	At(int) DP
}

// dataPointWithAttributes is satisfied by all pmetric datapoint types.
type dataPointWithAttributes interface {
	Attributes() pcommon.Map
}

func processDataPoints[DP dataPointWithAttributes](
	ctx context.Context,
	p *lookupProcessor[*ottldatapoint.TransformContext],
	dps dataPointSlice[DP],
	rm pmetric.ResourceMetrics,
	sm pmetric.ScopeMetrics,
	m pmetric.Metric,
	resourceAttrs pcommon.Map,
) {
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		tCtx := ottldatapoint.NewTransformContextPtr(rm, sm, m, dp)
		p.evalAndProcess(ctx, tCtx, dp.Attributes(), resourceAttrs)
		tCtx.Close()
	}
}

// extractValue gets the value to write for a given attribute mapping.
// Returns (value, shouldWrite).
func extractValue(result any, found bool, attr *AttributeMapping) (any, bool) {
	if !found {
		if attr.Default == "" {
			return nil, false
		}
		return attr.Default, true
	}

	if attr.Source == "" {
		// 1:1 scalar lookup — use entire result
		return result, true
	}

	// 1:N map lookup — extract the named field
	m, ok := result.(map[string]any)
	if !ok {
		return nil, false
	}

	val, exists := m[attr.Source]
	if !exists {
		if attr.Default == "" {
			return nil, false
		}
		return attr.Default, true
	}

	return val, true
}

func anyToString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%g", val)
	case bool:
		return fmt.Sprintf("%t", val)
	default:
		return fmt.Sprintf("%v", v)
	}
}
