// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package routingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottltraces"
)

var _ component.TracesProcessor = (*tracesProcessor)(nil)

type tracesProcessor struct {
	logger *zap.Logger
	config *Config

	extractor extractor
	router    router[component.TracesExporter]
}

func newTracesProcessor(settings component.TelemetrySettings, config config.Processor) *tracesProcessor {
	cfg := rewriteRoutingEntriesToOTTL(config.(*Config))

	return &tracesProcessor{
		logger: settings.Logger,
		config: cfg,
		router: newRouter[component.TracesExporter](
			cfg.Table,
			cfg.DefaultExporters,
			settings,
		),
		extractor: newExtractor(cfg.FromAttribute, settings.Logger),
	}
}

func (p *tracesProcessor) Start(_ context.Context, host component.Host) error {
	err := p.router.registerExporters(host.GetExporters()[config.TracesDataType])
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
	exporters []component.TracesExporter
	resSpans  ptrace.ResourceSpansSlice
}

func (p *tracesProcessor) route(ctx context.Context, t ptrace.Traces) error {
	// groups is used to group ptrace.ResourceSpans that are routed to
	// the same set of exporters. This way we're not ending up with all the
	// logs split up which would cause higher CPU usage.
	groups := map[string]spanGroup{}

	var errs error
	for i := 0; i < t.ResourceSpans().Len(); i++ {
		rspans := t.ResourceSpans().At(i)
		stx := ottltraces.NewTransformContext(
			ptrace.Span{},
			pcommon.InstrumentationScope{},
			rspans.Resource(),
		)

		matchCount := len(p.router.routes)
		for key, route := range p.router.routes {
			if !route.expression.Condition(stx) {
				matchCount--
				continue
			}
			route.expression.Function(stx)
			p.group(key, groups, route.exporters, rspans)
		}

		if matchCount == 0 {
			// no route conditions are matched, add resource spans to default exporters group
			p.group("", groups, p.router.defaultExporters, rspans)
		}
	}

	for _, g := range groups {
		t := ptrace.NewTraces()
		t.ResourceSpans().EnsureCapacity(g.resSpans.Len())
		g.resSpans.MoveAndAppendTo(t.ResourceSpans())

		for _, e := range g.exporters {
			errs = multierr.Append(errs, e.ConsumeTraces(ctx, t))
		}
	}
	return errs
}

func (p *tracesProcessor) group(key string, groups map[string]spanGroup, exporters []component.TracesExporter, spans ptrace.ResourceSpans) {
	group, ok := groups[key]
	if !ok {
		group.resSpans = ptrace.NewResourceSpansSlice()
		group.exporters = exporters
	}
	spans.CopyTo(group.resSpans.AppendEmpty())
	groups[key] = group
}

func (p *tracesProcessor) routeForContext(ctx context.Context, t ptrace.Traces) error {
	value := p.extractor.extractFromContext(ctx)
	exporters := p.router.getExporters(value)

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
