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
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

var _ component.TracesProcessor = (*tracesProcessor)(nil)

type tracesProcessor struct {
	logger *zap.Logger
	config *Config

	extractor extractor
	router    router[component.TracesExporter]
}

func newTracesProcessor(logger *zap.Logger, cfg config.Processor) *tracesProcessor {
	oCfg := cfg.(*Config)

	return &tracesProcessor{
		logger: logger,
		config: oCfg,

		extractor: newExtractor(oCfg.FromAttribute, logger),
		router:    newRouter[component.TracesExporter](*oCfg, logger),
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
	var errs error
	switch p.config.AttributeSource {
	case resourceAttributeSource:
		errs = multierr.Append(errs, p.route(ctx, t))
	case contextAttributeSource:
		fallthrough
	default:
		errs = multierr.Append(errs, p.routeForContext(ctx, t))
	}
	// TODO: determine the proper action when errors happen
	return errs
}

func (p *tracesProcessor) route(ctx context.Context, t ptrace.Traces) error {
	// routingEntry is used to group ptrace.ResourceSpans that are routed to
	// the same set of exporters.
	// This way we're not ending up with all the logs split up which would cause
	// higher CPU usage.
	groups := map[string]struct {
		exporters []component.TracesExporter
		resSpans  ptrace.ResourceSpansSlice
	}{}

	var errs error
	resSpansSlice := t.ResourceSpans()
	for i := 0; i < resSpansSlice.Len(); i++ {
		resSpans := resSpansSlice.At(i)

		attrValue := p.extractor.extractAttrFromResource(resSpans.Resource())
		exp := p.router.defaultExporters
		// If we have an exporter list defined for that attribute value then use it.
		if e, ok := p.router.exporters[attrValue]; ok {
			exp = e
			if p.config.DropRoutingResourceAttribute {
				resSpans.Resource().Attributes().Remove(p.config.FromAttribute)
			}
		}

		if rEntry, ok := groups[attrValue]; ok {
			resSpans.MoveTo(rEntry.resSpans.AppendEmpty())
		} else {
			newResSpans := ptrace.NewResourceSpansSlice()
			resSpans.MoveTo(newResSpans.AppendEmpty())

			groups[attrValue] = struct {
				exporters []component.TracesExporter
				resSpans  ptrace.ResourceSpansSlice
			}{
				exporters: exp,
				resSpans:  newResSpans,
			}
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

func (p *tracesProcessor) routeForContext(ctx context.Context, t ptrace.Traces) error {
	value := p.extractor.extractFromContext(ctx)
	exporters, ok := p.router.exporters[value]
	if !ok {
		exporters = p.router.defaultExporters
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
