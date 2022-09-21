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
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/oteltransformationlanguage/contexts/ottlmetrics"
)

var _ component.MetricsProcessor = (*metricsProcessor)(nil)

type metricsProcessor struct {
	logger *zap.Logger
	config *Config

	extractor extractor
	router    router[component.MetricsExporter]
}

func newMetricProcessor(logger *zap.Logger, config config.Processor) *metricsProcessor {
	cfg := rewriteRoutingEntriesToOTTL(config.(*Config))

	return &metricsProcessor{
		logger: logger,
		config: cfg,
		router: newRouter[component.MetricsExporter](
			cfg.Table,
			cfg.DefaultExporters,
			logger,
		),
		extractor: newExtractor(cfg.FromAttribute, logger),
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
	if p.config.FromAttribute == "" {
		err := p.route(ctx, m)
		if err != nil {
			return err
		}
		return nil
	}
	err := p.routeForContext(ctx, m)
	if err != nil {
		return err
	}
	return nil
}

type metricsGroup struct {
	exporters  []component.MetricsExporter
	resMetrics pmetric.ResourceMetricsSlice
}

func (p *metricsProcessor) route(ctx context.Context, tm pmetric.Metrics) error {
	// groups is used to group pmetric.ResourceMetrics that are routed to
	// the same set of exporters. This way we're not ending up with all the
	// metrics split up which would cause higher CPU usage.
	groups := map[string]metricsGroup{}

	var errs error

	for i := 0; i < tm.ResourceMetrics().Len(); i++ {
		rmetrics := tm.ResourceMetrics().At(i)
		mtx := ottlmetrics.NewTransformContext(
			nil,
			pmetric.Metric{},
			pmetric.MetricSlice{},
			pcommon.InstrumentationScope{},
			rmetrics.Resource(),
		)

		matchCount := len(p.router.routes)
		for key, route := range p.router.routes {
			if !route.expression.Condition(mtx) {
				matchCount--
				continue
			}
			route.expression.Function(mtx)
			p.group(key, groups, route.exporters, rmetrics)
		}

		if matchCount == 0 {
			// no route conditions are matched, add resource metrics to default exporters group
			p.group("", groups, p.router.defaultExporters, rmetrics)
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

func (p *metricsProcessor) group(
	key string,
	groups map[string]metricsGroup,
	exporters []component.MetricsExporter,
	metrics pmetric.ResourceMetrics,
) {
	group, ok := groups[key]
	if !ok {
		group.resMetrics = pmetric.NewResourceMetricsSlice()
		group.exporters = exporters
	}
	metrics.CopyTo(group.resMetrics.AppendEmpty())
	groups[key] = group
}

func (p *metricsProcessor) routeForContext(ctx context.Context, m pmetric.Metrics) error {
	value := p.extractor.extractFromContext(ctx)
	exporters := p.router.getExporters(value)

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
