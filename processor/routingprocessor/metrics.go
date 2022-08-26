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
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

var _ component.MetricsProcessor = (*metricsProcessor)(nil)

type metricsProcessor struct {
	logger *zap.Logger
	config *Config

	extractor extractor
	router    router[component.MetricsExporter]
}

func newMetricProcessor(logger *zap.Logger, cfg config.Processor) *metricsProcessor {
	oCfg := cfg.(*Config)

	return &metricsProcessor{
		logger: logger,
		config: oCfg,

		extractor: newExtractor(oCfg.FromAttribute, logger),
		router:    newRouter[component.MetricsExporter](*oCfg, logger),
	}
}

func (p *metricsProcessor) Start(_ context.Context, host component.Host) error {
	err := p.router.registerExporters(host.GetExporters()[config.MetricsDataType])
	if err != nil {
		return err
	}
	return nil
}

func (p *metricsProcessor) ConsumeMetrics(ctx context.Context, m pmetric.Metrics) error {
	var errs error
	switch p.config.AttributeSource {
	case resourceAttributeSource:
		errs = multierr.Append(errs, p.route(ctx, m))
	case contextAttributeSource:
		fallthrough
	default:
		errs = multierr.Append(errs, p.routeForContext(ctx, m))
	}
	// TODO: determine the proper action when errors happen
	return errs
}

func (p *metricsProcessor) route(ctx context.Context, tm pmetric.Metrics) error {
	// routingEntry is used to group pmetric.ResourceMetrics that are routed to
	// the same set of exporters.
	// This way we're not ending up with all the metrics split up which would cause
	// higher CPU usage.
	groups := map[string]struct {
		exporters  []component.MetricsExporter
		resMetrics pmetric.ResourceMetricsSlice
	}{}

	var errs error
	resMetricsSlice := tm.ResourceMetrics()
	for i := 0; i < resMetricsSlice.Len(); i++ {
		resMetrics := resMetricsSlice.At(i)

		attrValue := p.extractor.extractAttrFromResource(resMetrics.Resource())
		exp := p.router.defaultExporters
		// If we have an exporter list defined for that attribute value then use it.
		if e, ok := p.router.exporters[attrValue]; ok {
			exp = e
			if p.config.DropRoutingResourceAttribute {
				resMetrics.Resource().Attributes().Remove(p.config.FromAttribute)
			}
		}

		if rEntry, ok := groups[attrValue]; ok {
			resMetrics.MoveTo(rEntry.resMetrics.AppendEmpty())
		} else {
			newResMetrics := pmetric.NewResourceMetricsSlice()
			resMetrics.MoveTo(newResMetrics.AppendEmpty())

			groups[attrValue] = struct {
				exporters  []component.MetricsExporter
				resMetrics pmetric.ResourceMetricsSlice
			}{
				exporters:  exp,
				resMetrics: newResMetrics,
			}
		}
	}

	for _, g := range groups {
		m := pmetric.NewMetrics()
		m.ResourceMetrics().EnsureCapacity(g.resMetrics.Len())
		g.resMetrics.MoveAndAppendTo(m.ResourceMetrics())

		for _, e := range g.exporters {
			errs = multierr.Append(errs, e.ConsumeMetrics(ctx, m))
		}
	}
	return errs
}

func (p *metricsProcessor) routeForContext(ctx context.Context, m pmetric.Metrics) error {
	value := p.extractor.extractFromContext(ctx)
	exporters, ok := p.router.exporters[value]
	if !ok {
		exporters = p.router.defaultExporters
	}

	var errs error
	for _, e := range exporters {
		errs = multierr.Append(errs, e.ConsumeMetrics(ctx, m))
	}
	return errs
}

func (p *metricsProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (p *metricsProcessor) Shutdown(context.Context) error {
	return nil
}
