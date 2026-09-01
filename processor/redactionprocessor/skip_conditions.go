// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redactionprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor"

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
)

var knownSkipContexts = []string{"resource", "scope", "span", "spanevent", "log", "metric", "datapoint"}

type traceSkipper struct {
	resource  *ottl.ConditionSequence[*ottlresource.TransformContext]
	scope     *ottl.ConditionSequence[*ottlscope.TransformContext]
	span      *ottl.ConditionSequence[*ottlspan.TransformContext]
	spanEvent *ottl.ConditionSequence[*ottlspanevent.TransformContext]
}

type logSkipper struct {
	resource *ottl.ConditionSequence[*ottlresource.TransformContext]
	scope    *ottl.ConditionSequence[*ottlscope.TransformContext]
	log      *ottl.ConditionSequence[*ottllog.TransformContext]
}

type metricSkipper struct {
	resource  *ottl.ConditionSequence[*ottlresource.TransformContext]
	scope     *ottl.ConditionSequence[*ottlscope.TransformContext]
	metric    *ottl.ConditionSequence[*ottlmetric.TransformContext]
	dataPoint *ottl.ConditionSequence[*ottldatapoint.TransformContext]
}

type parsedTraceSkipConditions struct {
	resourceConditions  []*ottl.Condition[*ottlresource.TransformContext]
	scopeConditions     []*ottl.Condition[*ottlscope.TransformContext]
	spanConditions      []*ottl.Condition[*ottlspan.TransformContext]
	spanEventConditions []*ottl.Condition[*ottlspanevent.TransformContext]
}

type parsedLogSkipConditions struct {
	resourceConditions []*ottl.Condition[*ottlresource.TransformContext]
	scopeConditions    []*ottl.Condition[*ottlscope.TransformContext]
	logConditions      []*ottl.Condition[*ottllog.TransformContext]
}

type parsedMetricSkipConditions struct {
	resourceConditions  []*ottl.Condition[*ottlresource.TransformContext]
	scopeConditions     []*ottl.Condition[*ottlscope.TransformContext]
	metricConditions    []*ottl.Condition[*ottlmetric.TransformContext]
	dataPointConditions []*ottl.Condition[*ottldatapoint.TransformContext]
}

func newTraceSkipper(conditions []string, set component.TelemetrySettings) (*traceSkipper, error) {
	filtered := filterSkipConditionsForSignal(conditions, map[string]struct{}{
		"resource":  {},
		"scope":     {},
		"span":      {},
		"spanevent": {},
	})
	if len(filtered) == 0 {
		return nil, nil
	}

	pc, err := ottl.NewParserCollection[parsedTraceSkipConditions](
		set,
		ottl.EnableParserCollectionModifiedPathsLogging[parsedTraceSkipConditions](true),
		traceResourceParser(),
		traceScopeParser(),
		traceSpanParser(set),
		traceSpanEventParser(set),
	)
	if err != nil {
		return nil, err
	}

	parsed, err := parseTraceSkipConditions(pc, filtered)
	if err != nil {
		return nil, err
	}

	return &traceSkipper{
		resource:  newTraceResourceSequence(parsed.resourceConditions, set),
		scope:     newTraceScopeSequence(parsed.scopeConditions, set),
		span:      newTraceSpanSequence(parsed.spanConditions, set),
		spanEvent: newTraceSpanEventSequence(parsed.spanEventConditions, set),
	}, nil
}

func newLogSkipper(conditions []string, set component.TelemetrySettings) (*logSkipper, error) {
	filtered := filterSkipConditionsForSignal(conditions, map[string]struct{}{
		"resource": {},
		"scope":    {},
		"log":      {},
	})
	if len(filtered) == 0 {
		return nil, nil
	}

	pc, err := ottl.NewParserCollection[parsedLogSkipConditions](
		set,
		ottl.EnableParserCollectionModifiedPathsLogging[parsedLogSkipConditions](true),
		logResourceParser(),
		logScopeParser(),
		logRecordParser(set),
	)
	if err != nil {
		return nil, err
	}

	parsed, err := parseLogSkipConditions(pc, filtered)
	if err != nil {
		return nil, err
	}

	return &logSkipper{
		resource: newLogResourceSequence(parsed.resourceConditions, set),
		scope:    newLogScopeSequence(parsed.scopeConditions, set),
		log:      newLogRecordSequence(parsed.logConditions, set),
	}, nil
}

func newMetricSkipper(conditions []string, set component.TelemetrySettings) (*metricSkipper, error) {
	filtered := filterSkipConditionsForSignal(conditions, map[string]struct{}{
		"resource":  {},
		"scope":     {},
		"metric":    {},
		"datapoint": {},
	})
	if len(filtered) == 0 {
		return nil, nil
	}

	pc, err := ottl.NewParserCollection[parsedMetricSkipConditions](
		set,
		ottl.EnableParserCollectionModifiedPathsLogging[parsedMetricSkipConditions](true),
		metricResourceParser(),
		metricScopeParser(),
		metricParser(set),
		dataPointParser(set),
	)
	if err != nil {
		return nil, err
	}

	parsed, err := parseMetricSkipConditions(pc, filtered)
	if err != nil {
		return nil, err
	}

	return &metricSkipper{
		resource:  newMetricResourceSequence(parsed.resourceConditions, set),
		scope:     newMetricScopeSequence(parsed.scopeConditions, set),
		metric:    newMetricSequence(parsed.metricConditions, set),
		dataPoint: newDataPointSequence(parsed.dataPointConditions, set),
	}, nil
}

func filterSkipConditionsForSignal(conditions []string, supported map[string]struct{}) []string {
	filtered := make([]string, 0, len(conditions))
	for _, condition := range conditions {
		if skipConditionUsesUnsupportedContext(condition, supported) {
			continue
		}
		filtered = append(filtered, condition)
	}
	return filtered
}

func skipConditionUsesUnsupportedContext(condition string, supported map[string]struct{}) bool {
	lower := strings.ToLower(condition)
	for _, contextName := range knownSkipContexts {
		if !strings.Contains(lower, contextName+".") {
			continue
		}
		if _, ok := supported[contextName]; !ok {
			return true
		}
	}
	return false
}

func parseTraceSkipConditions(pc *ottl.ParserCollection[parsedTraceSkipConditions], conditions []string) (parsedTraceSkipConditions, error) {
	var aggregated parsedTraceSkipConditions
	for _, condition := range conditions {
		parsed, err := pc.ParseConditions(ottl.NewConditionsGetter([]string{condition}))
		if err != nil {
			return parsedTraceSkipConditions{}, fmt.Errorf("invalid skip condition %q: %w", condition, err)
		}
		aggregated.resourceConditions = append(aggregated.resourceConditions, parsed.resourceConditions...)
		aggregated.scopeConditions = append(aggregated.scopeConditions, parsed.scopeConditions...)
		aggregated.spanConditions = append(aggregated.spanConditions, parsed.spanConditions...)
		aggregated.spanEventConditions = append(aggregated.spanEventConditions, parsed.spanEventConditions...)
	}
	return aggregated, nil
}

func parseLogSkipConditions(pc *ottl.ParserCollection[parsedLogSkipConditions], conditions []string) (parsedLogSkipConditions, error) {
	var aggregated parsedLogSkipConditions
	for _, condition := range conditions {
		parsed, err := pc.ParseConditions(ottl.NewConditionsGetter([]string{condition}))
		if err != nil {
			return parsedLogSkipConditions{}, fmt.Errorf("invalid skip condition %q: %w", condition, err)
		}
		aggregated.resourceConditions = append(aggregated.resourceConditions, parsed.resourceConditions...)
		aggregated.scopeConditions = append(aggregated.scopeConditions, parsed.scopeConditions...)
		aggregated.logConditions = append(aggregated.logConditions, parsed.logConditions...)
	}
	return aggregated, nil
}

func parseMetricSkipConditions(pc *ottl.ParserCollection[parsedMetricSkipConditions], conditions []string) (parsedMetricSkipConditions, error) {
	var aggregated parsedMetricSkipConditions
	for _, condition := range conditions {
		parsed, err := pc.ParseConditions(ottl.NewConditionsGetter([]string{condition}))
		if err != nil {
			return parsedMetricSkipConditions{}, fmt.Errorf("invalid skip condition %q: %w", condition, err)
		}
		aggregated.resourceConditions = append(aggregated.resourceConditions, parsed.resourceConditions...)
		aggregated.scopeConditions = append(aggregated.scopeConditions, parsed.scopeConditions...)
		aggregated.metricConditions = append(aggregated.metricConditions, parsed.metricConditions...)
		aggregated.dataPointConditions = append(aggregated.dataPointConditions, parsed.dataPointConditions...)
	}
	return aggregated, nil
}

func traceResourceParser() ottl.ParserCollectionOption[parsedTraceSkipConditions] {
	return commonResourceParser(func(parsed []*ottl.Condition[*ottlresource.TransformContext]) parsedTraceSkipConditions {
		return parsedTraceSkipConditions{resourceConditions: parsed}
	})
}

func traceScopeParser() ottl.ParserCollectionOption[parsedTraceSkipConditions] {
	return commonScopeParser(func(parsed []*ottl.Condition[*ottlscope.TransformContext]) parsedTraceSkipConditions {
		return parsedTraceSkipConditions{scopeConditions: parsed}
	})
}

func traceSpanParser(set component.TelemetrySettings) ottl.ParserCollectionOption[parsedTraceSkipConditions] {
	return func(pc *ottl.ParserCollection[parsedTraceSkipConditions]) error {
		parser, err := ottlspan.NewParser(filterottl.StandardSpanFuncs(), set, ottlspan.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(
			ottlspan.ContextName,
			&parser,
			ottl.WithConditionConverter(func(_ *ottl.ParserCollection[parsedTraceSkipConditions], _ ottl.ConditionsGetter, parsed []*ottl.Condition[*ottlspan.TransformContext]) (parsedTraceSkipConditions, error) {
				return parsedTraceSkipConditions{spanConditions: parsed}, nil
			}),
		)(pc)
	}
}

func traceSpanEventParser(set component.TelemetrySettings) ottl.ParserCollectionOption[parsedTraceSkipConditions] {
	return func(pc *ottl.ParserCollection[parsedTraceSkipConditions]) error {
		parser, err := ottlspanevent.NewParser(filterottl.StandardSpanEventFuncs(), set, ottlspanevent.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(
			ottlspanevent.ContextName,
			&parser,
			ottl.WithConditionConverter(func(_ *ottl.ParserCollection[parsedTraceSkipConditions], _ ottl.ConditionsGetter, parsed []*ottl.Condition[*ottlspanevent.TransformContext]) (parsedTraceSkipConditions, error) {
				return parsedTraceSkipConditions{spanEventConditions: parsed}, nil
			}),
		)(pc)
	}
}

func logResourceParser() ottl.ParserCollectionOption[parsedLogSkipConditions] {
	return commonResourceParser(func(parsed []*ottl.Condition[*ottlresource.TransformContext]) parsedLogSkipConditions {
		return parsedLogSkipConditions{resourceConditions: parsed}
	})
}

func logScopeParser() ottl.ParserCollectionOption[parsedLogSkipConditions] {
	return commonScopeParser(func(parsed []*ottl.Condition[*ottlscope.TransformContext]) parsedLogSkipConditions {
		return parsedLogSkipConditions{scopeConditions: parsed}
	})
}

func logRecordParser(set component.TelemetrySettings) ottl.ParserCollectionOption[parsedLogSkipConditions] {
	return func(pc *ottl.ParserCollection[parsedLogSkipConditions]) error {
		parser, err := ottllog.NewParser(filterottl.StandardLogFuncs(), set, ottllog.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(
			ottllog.ContextName,
			&parser,
			ottl.WithConditionConverter(func(_ *ottl.ParserCollection[parsedLogSkipConditions], _ ottl.ConditionsGetter, parsed []*ottl.Condition[*ottllog.TransformContext]) (parsedLogSkipConditions, error) {
				return parsedLogSkipConditions{logConditions: parsed}, nil
			}),
		)(pc)
	}
}

func metricResourceParser() ottl.ParserCollectionOption[parsedMetricSkipConditions] {
	return commonResourceParser(func(parsed []*ottl.Condition[*ottlresource.TransformContext]) parsedMetricSkipConditions {
		return parsedMetricSkipConditions{resourceConditions: parsed}
	})
}

func metricScopeParser() ottl.ParserCollectionOption[parsedMetricSkipConditions] {
	return commonScopeParser(func(parsed []*ottl.Condition[*ottlscope.TransformContext]) parsedMetricSkipConditions {
		return parsedMetricSkipConditions{scopeConditions: parsed}
	})
}

func metricParser(set component.TelemetrySettings) ottl.ParserCollectionOption[parsedMetricSkipConditions] {
	return func(pc *ottl.ParserCollection[parsedMetricSkipConditions]) error {
		parser, err := ottlmetric.NewParser(filterottl.StandardMetricFuncs(), set, ottlmetric.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(
			ottlmetric.ContextName,
			&parser,
			ottl.WithConditionConverter(func(_ *ottl.ParserCollection[parsedMetricSkipConditions], _ ottl.ConditionsGetter, parsed []*ottl.Condition[*ottlmetric.TransformContext]) (parsedMetricSkipConditions, error) {
				return parsedMetricSkipConditions{metricConditions: parsed}, nil
			}),
		)(pc)
	}
}

func dataPointParser(set component.TelemetrySettings) ottl.ParserCollectionOption[parsedMetricSkipConditions] {
	return func(pc *ottl.ParserCollection[parsedMetricSkipConditions]) error {
		parser, err := ottldatapoint.NewParser(filterottl.StandardDataPointFuncs(), set, ottldatapoint.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(
			ottldatapoint.ContextName,
			&parser,
			ottl.WithConditionConverter(func(_ *ottl.ParserCollection[parsedMetricSkipConditions], _ ottl.ConditionsGetter, parsed []*ottl.Condition[*ottldatapoint.TransformContext]) (parsedMetricSkipConditions, error) {
				return parsedMetricSkipConditions{dataPointConditions: parsed}, nil
			}),
		)(pc)
	}
}

func commonResourceParser[R any](converter func([]*ottl.Condition[*ottlresource.TransformContext]) R) ottl.ParserCollectionOption[R] {
	return func(pc *ottl.ParserCollection[R]) error {
		parser, err := ottlresource.NewParser(filterottl.StandardResourceFuncs(), pc.Settings, ottlresource.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(
			ottlresource.ContextName,
			&parser,
			ottl.WithConditionConverter(func(_ *ottl.ParserCollection[R], _ ottl.ConditionsGetter, parsed []*ottl.Condition[*ottlresource.TransformContext]) (R, error) {
				return converter(parsed), nil
			}),
		)(pc)
	}
}

func commonScopeParser[R any](converter func([]*ottl.Condition[*ottlscope.TransformContext]) R) ottl.ParserCollectionOption[R] {
	return func(pc *ottl.ParserCollection[R]) error {
		parser, err := ottlscope.NewParser(filterottl.StandardScopeFuncs(), pc.Settings, ottlscope.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(
			ottlscope.ContextName,
			&parser,
			ottl.WithConditionConverter(func(_ *ottl.ParserCollection[R], _ ottl.ConditionsGetter, parsed []*ottl.Condition[*ottlscope.TransformContext]) (R, error) {
				return converter(parsed), nil
			}),
		)(pc)
	}
}

func newTraceResourceSequence(conditions []*ottl.Condition[*ottlresource.TransformContext], set component.TelemetrySettings) *ottl.ConditionSequence[*ottlresource.TransformContext] {
	if len(conditions) == 0 {
		return nil
	}
	sequence := ottlresource.NewConditionSequence(conditions, set, ottlresource.WithConditionSequenceErrorMode(ottl.PropagateError))
	return &sequence
}

func newTraceScopeSequence(conditions []*ottl.Condition[*ottlscope.TransformContext], set component.TelemetrySettings) *ottl.ConditionSequence[*ottlscope.TransformContext] {
	if len(conditions) == 0 {
		return nil
	}
	sequence := ottlscope.NewConditionSequence(conditions, set, ottlscope.WithConditionSequenceErrorMode(ottl.PropagateError))
	return &sequence
}

func newTraceSpanSequence(conditions []*ottl.Condition[*ottlspan.TransformContext], set component.TelemetrySettings) *ottl.ConditionSequence[*ottlspan.TransformContext] {
	if len(conditions) == 0 {
		return nil
	}
	sequence := ottlspan.NewConditionSequence(conditions, set, ottlspan.WithConditionSequenceErrorMode(ottl.PropagateError))
	return &sequence
}

func newTraceSpanEventSequence(conditions []*ottl.Condition[*ottlspanevent.TransformContext], set component.TelemetrySettings) *ottl.ConditionSequence[*ottlspanevent.TransformContext] {
	if len(conditions) == 0 {
		return nil
	}
	sequence := ottlspanevent.NewConditionSequence(conditions, set, ottlspanevent.WithConditionSequenceErrorMode(ottl.PropagateError))
	return &sequence
}

func newLogResourceSequence(conditions []*ottl.Condition[*ottlresource.TransformContext], set component.TelemetrySettings) *ottl.ConditionSequence[*ottlresource.TransformContext] {
	if len(conditions) == 0 {
		return nil
	}
	sequence := ottlresource.NewConditionSequence(conditions, set, ottlresource.WithConditionSequenceErrorMode(ottl.PropagateError))
	return &sequence
}

func newLogScopeSequence(conditions []*ottl.Condition[*ottlscope.TransformContext], set component.TelemetrySettings) *ottl.ConditionSequence[*ottlscope.TransformContext] {
	if len(conditions) == 0 {
		return nil
	}
	sequence := ottlscope.NewConditionSequence(conditions, set, ottlscope.WithConditionSequenceErrorMode(ottl.PropagateError))
	return &sequence
}

func newLogRecordSequence(conditions []*ottl.Condition[*ottllog.TransformContext], set component.TelemetrySettings) *ottl.ConditionSequence[*ottllog.TransformContext] {
	if len(conditions) == 0 {
		return nil
	}
	sequence := ottllog.NewConditionSequence(conditions, set, ottllog.WithConditionSequenceErrorMode(ottl.PropagateError))
	return &sequence
}

func newMetricResourceSequence(conditions []*ottl.Condition[*ottlresource.TransformContext], set component.TelemetrySettings) *ottl.ConditionSequence[*ottlresource.TransformContext] {
	if len(conditions) == 0 {
		return nil
	}
	sequence := ottlresource.NewConditionSequence(conditions, set, ottlresource.WithConditionSequenceErrorMode(ottl.PropagateError))
	return &sequence
}

func newMetricScopeSequence(conditions []*ottl.Condition[*ottlscope.TransformContext], set component.TelemetrySettings) *ottl.ConditionSequence[*ottlscope.TransformContext] {
	if len(conditions) == 0 {
		return nil
	}
	sequence := ottlscope.NewConditionSequence(conditions, set, ottlscope.WithConditionSequenceErrorMode(ottl.PropagateError))
	return &sequence
}

func newMetricSequence(conditions []*ottl.Condition[*ottlmetric.TransformContext], set component.TelemetrySettings) *ottl.ConditionSequence[*ottlmetric.TransformContext] {
	if len(conditions) == 0 {
		return nil
	}
	sequence := ottlmetric.NewConditionSequence(conditions, set, ottlmetric.WithConditionSequenceErrorMode(ottl.PropagateError))
	return &sequence
}

func newDataPointSequence(conditions []*ottl.Condition[*ottldatapoint.TransformContext], set component.TelemetrySettings) *ottl.ConditionSequence[*ottldatapoint.TransformContext] {
	if len(conditions) == 0 {
		return nil
	}
	sequence := ottldatapoint.NewConditionSequence(conditions, set, ottldatapoint.WithConditionSequenceErrorMode(ottl.PropagateError))
	return &sequence
}

func (s *traceSkipper) skipResource(ctx context.Context, rs ptrace.ResourceSpans) (bool, error) {
	if s == nil || s.resource == nil {
		return false, nil
	}
	tCtx := ottlresource.NewTransformContextPtr(rs.Resource(), rs)
	defer tCtx.Close()
	return s.resource.Eval(ctx, tCtx)
}

func (s *traceSkipper) skipScope(ctx context.Context, rs ptrace.ResourceSpans, ss ptrace.ScopeSpans) (bool, error) {
	if s == nil || s.scope == nil {
		return false, nil
	}
	tCtx := ottlscope.NewTransformContextPtr(ss.Scope(), rs.Resource(), ss, rs)
	defer tCtx.Close()
	return s.scope.Eval(ctx, tCtx)
}

func (s *traceSkipper) skipSpan(ctx context.Context, rs ptrace.ResourceSpans, ss ptrace.ScopeSpans, span ptrace.Span) (bool, error) {
	if s == nil || s.span == nil {
		return false, nil
	}
	tCtx := ottlspan.NewTransformContextPtr(rs, ss, span)
	defer tCtx.Close()
	return s.span.Eval(ctx, tCtx)
}

func (s *traceSkipper) skipSpanEvent(ctx context.Context, rs ptrace.ResourceSpans, ss ptrace.ScopeSpans, span ptrace.Span, spanEvent ptrace.SpanEvent) (bool, error) {
	if s == nil || s.spanEvent == nil {
		return false, nil
	}
	tCtx := ottlspanevent.NewTransformContextPtr(rs, ss, span, spanEvent)
	defer tCtx.Close()
	return s.spanEvent.Eval(ctx, tCtx)
}

func (s *logSkipper) skipResource(ctx context.Context, rl plog.ResourceLogs) (bool, error) {
	if s == nil || s.resource == nil {
		return false, nil
	}
	tCtx := ottlresource.NewTransformContextPtr(rl.Resource(), rl)
	defer tCtx.Close()
	return s.resource.Eval(ctx, tCtx)
}

func (s *logSkipper) skipScope(ctx context.Context, rl plog.ResourceLogs, sl plog.ScopeLogs) (bool, error) {
	if s == nil || s.scope == nil {
		return false, nil
	}
	tCtx := ottlscope.NewTransformContextPtr(sl.Scope(), rl.Resource(), sl, rl)
	defer tCtx.Close()
	return s.scope.Eval(ctx, tCtx)
}

func (s *logSkipper) skipLogRecord(ctx context.Context, rl plog.ResourceLogs, sl plog.ScopeLogs, lr plog.LogRecord) (bool, error) {
	if s == nil || s.log == nil {
		return false, nil
	}
	tCtx := ottllog.NewTransformContextPtr(rl, sl, lr)
	defer tCtx.Close()
	return s.log.Eval(ctx, tCtx)
}

func (s *metricSkipper) skipResource(ctx context.Context, rm pmetric.ResourceMetrics) (bool, error) {
	if s == nil || s.resource == nil {
		return false, nil
	}
	tCtx := ottlresource.NewTransformContextPtr(rm.Resource(), rm)
	defer tCtx.Close()
	return s.resource.Eval(ctx, tCtx)
}

func (s *metricSkipper) skipScope(ctx context.Context, rm pmetric.ResourceMetrics, sm pmetric.ScopeMetrics) (bool, error) {
	if s == nil || s.scope == nil {
		return false, nil
	}
	tCtx := ottlscope.NewTransformContextPtr(sm.Scope(), rm.Resource(), sm, rm)
	defer tCtx.Close()
	return s.scope.Eval(ctx, tCtx)
}

func (s *metricSkipper) skipMetric(ctx context.Context, rm pmetric.ResourceMetrics, sm pmetric.ScopeMetrics, metric pmetric.Metric) (bool, error) {
	if s == nil || s.metric == nil {
		return false, nil
	}
	tCtx := ottlmetric.NewTransformContextPtr(rm, sm, metric)
	defer tCtx.Close()
	return s.metric.Eval(ctx, tCtx)
}

func (s *metricSkipper) skipNumberDataPoint(ctx context.Context, rm pmetric.ResourceMetrics, sm pmetric.ScopeMetrics, metric pmetric.Metric, dp pmetric.NumberDataPoint) (bool, error) {
	if s == nil || s.dataPoint == nil {
		return false, nil
	}
	tCtx := ottldatapoint.NewTransformContextPtr(rm, sm, metric, dp)
	defer tCtx.Close()
	return s.dataPoint.Eval(ctx, tCtx)
}

func (s *metricSkipper) skipHistogramDataPoint(ctx context.Context, rm pmetric.ResourceMetrics, sm pmetric.ScopeMetrics, metric pmetric.Metric, dp pmetric.HistogramDataPoint) (bool, error) {
	if s == nil || s.dataPoint == nil {
		return false, nil
	}
	tCtx := ottldatapoint.NewTransformContextPtr(rm, sm, metric, dp)
	defer tCtx.Close()
	return s.dataPoint.Eval(ctx, tCtx)
}

func (s *metricSkipper) skipExponentialHistogramDataPoint(ctx context.Context, rm pmetric.ResourceMetrics, sm pmetric.ScopeMetrics, metric pmetric.Metric, dp pmetric.ExponentialHistogramDataPoint) (bool, error) {
	if s == nil || s.dataPoint == nil {
		return false, nil
	}
	tCtx := ottldatapoint.NewTransformContextPtr(rm, sm, metric, dp)
	defer tCtx.Close()
	return s.dataPoint.Eval(ctx, tCtx)
}

func (s *metricSkipper) skipSummaryDataPoint(ctx context.Context, rm pmetric.ResourceMetrics, sm pmetric.ScopeMetrics, metric pmetric.Metric, dp pmetric.SummaryDataPoint) (bool, error) {
	if s == nil || s.dataPoint == nil {
		return false, nil
	}
	tCtx := ottldatapoint.NewTransformContextPtr(rm, sm, metric, dp)
	defer tCtx.Close()
	return s.dataPoint.Eval(ctx, tCtx)
}
