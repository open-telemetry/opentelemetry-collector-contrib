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
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor/internal/metadata"
)

var _ processor.Metrics = (*metricsProcessor)(nil)

type metricsProcessor struct {
	logger    *zap.Logger
	telemetry *metadata.TelemetryBuilder
	config    *Config

	extractor extractor
	router    router[exporter.Metrics, ottldatapoint.TransformContext]
}

func newMetricProcessor(settings component.TelemetrySettings, config component.Config) (*metricsProcessor, error) {
	telemetryBuilder, err := metadata.NewTelemetryBuilder(settings)
	if err != nil {
		return nil, err
	}
	cfg := rewriteRoutingEntriesToOTTL(config.(*Config))

	dataPointParser, err := ottldatapoint.NewParser(common.Functions[ottldatapoint.TransformContext](), settings)
	if err != nil {
		return nil, err
	}

	return &metricsProcessor{
		logger:    settings.Logger,
		telemetry: telemetryBuilder,
		config:    cfg,
		router: newRouter[exporter.Metrics](
			cfg.Table,
			cfg.DefaultExporters,
			settings,
			dataPointParser,
		),
		extractor: newExtractor(cfg.FromAttribute, settings.Logger),
	}, nil
}

func (p *metricsProcessor) Start(_ context.Context, host component.Host) error {
	ge, ok := host.(getExporters)
	if !ok {
		return errors.New("unable to get exporters")
	}
	err := p.router.registerExporters(ge.GetExporters()[pipeline.SignalMetrics])
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
			pmetric.NewMetric(),
			pmetric.NewMetricSlice(),
			pcommon.NewInstrumentationScope(),
			rmetrics.Resource(),
			pmetric.NewScopeMetrics(),
			rmetrics,
		)

		matchCount := len(p.router.routes)
		for key, route := range p.router.routes {
			_, isMatch, err := route.statement.Execute(ctx, mtx)
			if err != nil {
				if p.config.ErrorMode == ottl.PropagateError {
					return err
				}
				p.group("", groups, p.router.defaultExporters, rmetrics)
				p.recordNonRoutedForResourceMetrics(ctx, "", rmetrics)
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
			p.recordNonRoutedForResourceMetrics(ctx, "", rmetrics)
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

func (p *metricsProcessor) recordNonRoutedForResourceMetrics(ctx context.Context, routingKey string, rm pmetric.ResourceMetrics) {
	metricPointsCount := 0
	sm := rm.ScopeMetrics()
	for j := 0; j < sm.Len(); j++ {
		metricPointsCount += sm.At(j).Metrics().Len()
	}

	p.telemetry.RoutingProcessorNonRoutedMetricPoints.Add(
		ctx,
		int64(metricPointsCount),
		metric.WithAttributes(
			attribute.String("routing_key", routingKey),
		),
	)
}

func (p *metricsProcessor) routeForContext(ctx context.Context, m pmetric.Metrics) error {
	value := p.extractor.extractFromContext(ctx)
	exporters := p.router.getExporters(value)
	if value == "" { // "" is a key for default exporters
		p.telemetry.RoutingProcessorNonRoutedMetricPoints.Add(
			ctx,
			int64(m.MetricCount()),
			metric.WithAttributes(
				attribute.String("routing_key", p.extractor.fromAttr),
			),
		)
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
