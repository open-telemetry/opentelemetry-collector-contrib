// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor/internal/metadata"
)

var _ processor.Traces = (*tracesProcessor)(nil)

type tracesProcessor struct {
	logger    *zap.Logger
	telemetry *metadata.TelemetryBuilder
	config    *Config

	extractor extractor
	router    router[exporter.Traces, ottlspan.TransformContext]
}

func newTracesProcessor(settings component.TelemetrySettings, config component.Config) (*tracesProcessor, error) {
	telemetryBuilder, err := metadata.NewTelemetryBuilder(settings)
	if err != nil {
		return nil, err
	}

	cfg := rewriteRoutingEntriesToOTTL(config.(*Config))

	spanParser, err := ottlspan.NewParser(common.Functions[ottlspan.TransformContext](), settings)
	if err != nil {
		return nil, err
	}

	return &tracesProcessor{
		logger:    settings.Logger,
		telemetry: telemetryBuilder,
		config:    cfg,
		router: newRouter[exporter.Traces, ottlspan.TransformContext](
			cfg.Table,
			cfg.DefaultExporters,
			settings,
			spanParser,
		),
		extractor: newExtractor(cfg.FromAttribute, settings.Logger),
	}, nil
}

func (p *tracesProcessor) Start(_ context.Context, host component.Host) error {
	ge, ok := host.(getExporters)
	if !ok {
		return errors.New("unable to get exporters")
	}
	err := p.router.registerExporters(ge.GetExporters()[pipeline.SignalTraces])
	if err != nil {
		return err
	}
	return nil
}

func (p *tracesProcessor) ConsumeTraces(ctx context.Context, t ptrace.Traces) error {
	// TODO: determine the proper action when errors happen
	if p.config.FromAttribute == "" {
		err := p.route(ctx, t)
		if err != nil {
			return err
		}
		return nil
	}
	err := p.routeForContext(ctx, t)
	if err != nil {
		return err
	}
	return nil
}

type spanGroup struct {
	exporters []exporter.Traces
	traces    ptrace.Traces
}

func (p *tracesProcessor) route(ctx context.Context, t ptrace.Traces) error {
	// groups is used to group ptrace.ResourceSpans that are routed to
	// the same set of exporters. This way we're not ending up with all the
	// logs split up which would cause higher CPU usage.
	groups := map[string]spanGroup{}

	var errs error
	for i := 0; i < t.ResourceSpans().Len(); i++ {
		rspans := t.ResourceSpans().At(i)
		stx := ottlspan.NewTransformContext(
			ptrace.NewSpan(),
			pcommon.NewInstrumentationScope(),
			rspans.Resource(),
			ptrace.NewScopeSpans(),
			rspans,
		)

		matchCount := len(p.router.routes)
		for key, route := range p.router.routes {
			_, isMatch, err := route.statement.Execute(ctx, stx)
			if err != nil {
				if p.config.ErrorMode == ottl.PropagateError {
					return err
				}
				p.group("", groups, p.router.defaultExporters, rspans)
				p.recordNonRoutedResourceSpans(ctx, key, rspans)
				continue
			}
			if !isMatch {
				matchCount--
				continue
			}
			p.group(key, groups, route.exporters, rspans)
		}

		if matchCount == 0 {
			// no route conditions are matched, add resource spans to default exporters group
			p.group("", groups, p.router.defaultExporters, rspans)
			p.recordNonRoutedResourceSpans(ctx, "", rspans)
		}
	}

	for _, g := range groups {
		for _, e := range g.exporters {
			errs = multierr.Append(errs, e.ConsumeTraces(ctx, g.traces))
		}
	}
	return errs
}

func (p *tracesProcessor) group(key string, groups map[string]spanGroup, exporters []exporter.Traces, spans ptrace.ResourceSpans) {
	group, ok := groups[key]
	if !ok {
		group.traces = ptrace.NewTraces()
		group.exporters = exporters
	}
	spans.CopyTo(group.traces.ResourceSpans().AppendEmpty())
	groups[key] = group
}

func (p *tracesProcessor) recordNonRoutedResourceSpans(ctx context.Context, routingKey string, rspans ptrace.ResourceSpans) {
	spanCount := 0
	ilss := rspans.ScopeSpans()
	for j := 0; j < ilss.Len(); j++ {
		spanCount += ilss.At(j).Spans().Len()
	}

	p.telemetry.RoutingProcessorNonRoutedSpans.Add(
		ctx,
		int64(spanCount),
		metric.WithAttributes(
			attribute.String("routing_key", routingKey),
		),
	)
}

func (p *tracesProcessor) routeForContext(ctx context.Context, t ptrace.Traces) error {
	value := p.extractor.extractFromContext(ctx)
	exporters := p.router.getExporters(value)
	if value == "" { // "" is a key for default exporters
		p.telemetry.RoutingProcessorNonRoutedSpans.Add(
			ctx,
			int64(t.SpanCount()),
			metric.WithAttributes(
				attribute.String("routing_key", p.extractor.fromAttr),
			),
		)
	}

	var errs error
	for _, e := range exporters {
		errs = multierr.Append(errs, e.ConsumeTraces(ctx, t))
	}
	return errs
}

func (p *tracesProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (p *tracesProcessor) Shutdown(context.Context) error {
	return nil
}
