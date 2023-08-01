// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package attributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterlog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filtermatcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor/internal/metadata"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

// NewFactory returns a new factory for the Attributes processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, metadata.TracesStability),
		processor.WithLogs(createLogsProcessor, metadata.LogsStability),
		processor.WithMetrics(createMetricsProcessor, metadata.MetricsStability))
}

// Note: This isn't a valid configuration because the processor would do no work.
func createDefaultConfig() component.Config {
	return &Config{}
}

func createTracesProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	oCfg := cfg.(*Config)
	attrProc, err := attraction.NewAttrProc(&oCfg.Settings)
	if err != nil {
		return nil, err
	}
	skipExpr, err := filterspan.NewSkipExpr(&oCfg.MatchConfig)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewTracesProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		newSpanAttributesProcessor(set.Logger, attrProc, skipExpr).processTraces,
		processorhelper.WithCapabilities(processorCapabilities))
}

func createLogsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	oCfg := cfg.(*Config)
	attrProc, err := attraction.NewAttrProc(&oCfg.Settings)
	if err != nil {
		return nil, err
	}

	skipExpr, err := filterlog.NewSkipExpr(&oCfg.MatchConfig)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewLogsProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		newLogAttributesProcessor(set.Logger, attrProc, skipExpr).processLogs,
		processorhelper.WithCapabilities(processorCapabilities))
}

func newSkipExpr(include *filterconfig.MatchProperties, exclude *filterconfig.MatchProperties) (expr.BoolExpr[ottlmetric.TransformContext], error) {
	var matchers []expr.BoolExpr[ottlmetric.TransformContext]
	inclExpr, err := newExpr(include)
	if err != nil {
		return nil, err
	}
	if inclExpr != nil {
		matchers = append(matchers, expr.Not(inclExpr))
	}
	exclExpr, err := newExpr(exclude)
	if err != nil {
		return nil, err
	}
	if exclExpr != nil {
		matchers = append(matchers, exclExpr)
	}
	return expr.Or(matchers...), nil
}

type anonMatcher[T any] struct {
	predicate func(tCtx T) (bool, error)
}

func (m *anonMatcher[T]) Eval(_ context.Context, tCtx T) (bool, error) {
	return m.predicate(tCtx)
}

func newAnonMatcher[T any](predicate func(tCtx T) (bool, error)) *anonMatcher[T] {
	return &anonMatcher[T]{
		predicate: predicate,
	}
}

// Creates an expr for matching whole metrics objects against metric name, resource attribute, and library(?) filters
func newExpr(mp *filterconfig.MatchProperties) (expr.BoolExpr[ottlmetric.TransformContext], error) {
	if mp == nil {
		return nil, nil
	}

	var matchers []expr.BoolExpr[ottlmetric.TransformContext]
	// do regex/strict match of metric names
	if len(mp.MetricNames) != 0 {
		filterSet, err := filterset.CreateFilterSet(mp.MetricNames, &filterset.Config{
			MatchType:    mp.MatchType,
			RegexpConfig: mp.RegexpConfig,
		})
		if err != nil {
			return nil, err
		}

		nameMatcher := newAnonMatcher(func(tCtx ottlmetric.TransformContext) (bool, error) {
			return filterSet.Matches(tCtx.GetMetric().Name()), nil
		})
		matchers = append(matchers, nameMatcher)
	}
	// do match of resource attributes
	if len(mp.Resources) != 0 {
		resourceAttributeMatcher, err := filtermatcher.NewAttributesMatcher(
			filterset.Config{
				MatchType:    mp.MatchType,
				RegexpConfig: mp.RegexpConfig,
			},
			mp.Resources,
		)
		if err != nil {
			return nil, err
		}
		resourceMatcher := newAnonMatcher(func(tCtx ottlmetric.TransformContext) (bool, error) {
			return resourceAttributeMatcher.Match(tCtx.GetResource().Attributes()), nil
		})
		matchers = append(matchers, resourceMatcher)
	}

	return expr.And(matchers...), nil
}

func newDataPointFilter(include, exclude *filterconfig.MatchProperties) (func(attrs pcommon.Map) bool, error) {
	inclFilter, err := newDataPointPredicate(include)
	if err != nil {
		return nil, err
	}
	if inclFilter == nil {
		inclFilter = func(_ pcommon.Map) bool {
			return true
		}
	}

	exclFilter, err2 := newDataPointPredicate(exclude)
	if err2 != nil {
		return nil, err
	}
	if exclFilter == nil {
		exclFilter = func(_ pcommon.Map) bool {
			return false
		}
	}

	return func(attrs pcommon.Map) bool {
		return inclFilter(attrs) && !exclFilter(attrs)
	}, nil
}

func newDataPointPredicate(mp *filterconfig.MatchProperties) (func(attrs pcommon.Map) bool, error) {
	if mp == nil || len(mp.Attributes) == 0 {
		return nil, nil
	}

	attributesMatcher, err := filtermatcher.NewAttributesMatcher(
		filterset.Config{
			MatchType:    mp.MatchType,
			RegexpConfig: mp.RegexpConfig,
		},
		mp.Attributes,
	)
	if err != nil {
		return nil, err
	}
	return attributesMatcher.Match, nil
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {

	oCfg := cfg.(*Config)
	attrProc, err := attraction.NewAttrProc(&oCfg.Settings)
	if err != nil {
		return nil, err
	}

	skipExpr, err := newSkipExpr(oCfg.Include, oCfg.Exclude)
	if err != nil {
		return nil, err
	}

	dataPointFilter, err := newDataPointFilter(oCfg.Include, oCfg.Exclude)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewMetricsProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		newMetricAttributesProcessor(set.Logger, attrProc, skipExpr, dataPointFilter).processMetrics,
		processorhelper.WithCapabilities(processorCapabilities),
	)
}
