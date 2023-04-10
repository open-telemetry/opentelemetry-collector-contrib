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
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor/internal/common"
)

var _ processor.Metrics = (*metricsProcessor)(nil)

type metricsProcessor struct {
	logger *zap.Logger
	config *Config

	extractor extractor
	router    router[exporter.Metrics, ottldatapoint.TransformContext]
}

func newMetricProcessor(settings component.TelemetrySettings, config component.Config) *metricsProcessor {
	cfg := rewriteRoutingEntriesToOTTL(config.(*Config))

	dataPointParser, _ := ottldatapoint.NewParser(common.Functions[ottldatapoint.TransformContext](), settings)

	return &metricsProcessor{
		logger: settings.Logger,
		config: cfg,
		router: newRouter[exporter.Metrics](
			cfg.Table,
			cfg.DefaultExporters,
			settings,
			dataPointParser,
		),
		extractor: newExtractor(cfg.FromAttribute, settings.Logger),
	}
}

func (p *metricsProcessor) Start(_ context.Context, host component.Host) error {
	err := p.router.registerExporters(host.GetExporters()[component.DataTypeMetrics])
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
	exporters []exporter.Metrics
	metrics   pmetric.Metrics
}

func (p *metricsProcessor) route(ctx context.Context, tm pmetric.Metrics) error {
	// groups is used to group pmetric.ResourceMetrics that are routed to
	// the same set of exporters. This way we're not ending up with all the
	// metrics split up which would cause higher CPU usage.
	groups := map[string]metricsGroup{}

	var errs error

	for i := 0; i < tm.ResourceMetrics().Len(); i++ {
		rmetrics := tm.ResourceMetrics().At(i)
		mtx := ottldatapoint.NewTransformContext(
			nil,
			pmetric.Metric{},
			pmetric.MetricSlice{},
			pcommon.InstrumentationScope{},
			rmetrics.Resource(),
		)

		matchCount := len(p.router.routes)
		for key, route := range p.router.routes {
			_, isMatch, err := route.statement.Execute(ctx, mtx)
			if err != nil {
				if p.config.ErrorMode == ottl.PropagateError {
					return err
				}
				p.group("", groups, p.router.defaultExporters, rmetrics)
				continue
			}
			if !isMatch {
				matchCount--
				continue
			}
			p.group(key, groups, route.exporters, rmetrics)
		}

		if matchCount == 0 {
			// no route conditions are matched, add resource metrics to default exporters group
			p.group("", groups, p.router.defaultExporters, rmetrics)
		}
	}

	for _, g := range groups {
		for _, e := range g.exporters {
			errs = multierr.Append(errs, e.ConsumeMetrics(ctx, g.metrics))
		}
	}
	return errs
}

func (p *metricsProcessor) group(
	key string,
	groups map[string]metricsGroup,
	exporters []exporter.Metrics,
	metrics pmetric.ResourceMetrics,
) {
	group, ok := groups[key]
	if !ok {
		group.metrics = pmetric.NewMetrics()
		group.exporters = exporters
	}
	metrics.CopyTo(group.metrics.ResourceMetrics().AppendEmpty())
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
