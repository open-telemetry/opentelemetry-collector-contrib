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
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor/internal/metadata"
)

var _ processor.Logs = (*logProcessor)(nil)

type logProcessor struct {
	logger    *zap.Logger
	telemetry *metadata.TelemetryBuilder
	config    *Config

	extractor extractor
	router    router[exporter.Logs, ottllog.TransformContext]
}

func newLogProcessor(settings component.TelemetrySettings, config component.Config) (*logProcessor, error) {
	telemetryBuilder, err := metadata.NewTelemetryBuilder(settings)
	if err != nil {
		return nil, err
	}
	cfg := rewriteRoutingEntriesToOTTL(config.(*Config))

	logParser, err := ottllog.NewParser(common.Functions[ottllog.TransformContext](), settings)
	if err != nil {
		return nil, err
	}

	return &logProcessor{
		logger:    settings.Logger,
		telemetry: telemetryBuilder,
		config:    cfg,
		router: newRouter[exporter.Logs, ottllog.TransformContext](
			cfg.Table,
			cfg.DefaultExporters,
			settings,
			logParser,
		),
		extractor: newExtractor(cfg.FromAttribute, settings.Logger),
	}, nil
}

type getExporters interface {
	GetExporters() map[pipeline.Signal]map[component.ID]component.Component
}

func (p *logProcessor) Start(_ context.Context, host component.Host) error {
	ge, ok := host.(getExporters)
	if !ok {
		return errors.New("unable to get exporters")
	}
	err := p.router.registerExporters(ge.GetExporters()[pipeline.SignalLogs])
	if err != nil {
		return err
	}
	return nil
}

func (p *logProcessor) ConsumeLogs(ctx context.Context, l plog.Logs) error {
	if p.config.FromAttribute == "" {
		err := p.route(ctx, l)
		if err != nil {
			return err
		}
		return nil
	}
	err := p.routeForContext(ctx, l)
	if err != nil {
		return err
	}
	return nil
}

type logsGroup struct {
	exporters []exporter.Logs
	logs      plog.Logs
}

func (p *logProcessor) route(ctx context.Context, l plog.Logs) error {
	// routingEntry is used to group plog.ResourceLogs that are routed to
	// the same set of exporters.
	// This way we're not ending up with all the logs split up which would cause
	// higher CPU usage.
	groups := map[string]logsGroup{}
	var errs error

	for i := 0; i < l.ResourceLogs().Len(); i++ {
		rlogs := l.ResourceLogs().At(i)
		ltx := ottllog.NewTransformContext(
			plog.NewLogRecord(),
			pcommon.NewInstrumentationScope(),
			rlogs.Resource(),
			plog.NewScopeLogs(),
			rlogs,
		)

		matchCount := len(p.router.routes)
		for key, route := range p.router.routes {
			_, isMatch, err := route.statement.Execute(ctx, ltx)
			if err != nil {
				if p.config.ErrorMode == ottl.PropagateError {
					return err
				}
				p.group("", groups, p.router.defaultExporters, rlogs)
				p.recordNonRoutedResourceLogs(ctx, key, rlogs)
				continue
			}
			if !isMatch {
				matchCount--
				continue
			}
			p.group(key, groups, route.exporters, rlogs)
		}

		if matchCount == 0 {
			// no route conditions are matched, add resource logs to default exporters group
			p.group("", groups, p.router.defaultExporters, rlogs)
			p.recordNonRoutedResourceLogs(ctx, "", rlogs)
		}
	}
	for _, g := range groups {
		for _, e := range g.exporters {
			errs = multierr.Append(errs, e.ConsumeLogs(ctx, g.logs))
		}
	}
	return errs
}

func (p *logProcessor) group(
	key string,
	groups map[string]logsGroup,
	exporters []exporter.Logs,
	spans plog.ResourceLogs,
) {
	group, ok := groups[key]
	if !ok {
		group.logs = plog.NewLogs()
		group.exporters = exporters
	}
	spans.CopyTo(group.logs.ResourceLogs().AppendEmpty())
	groups[key] = group
}

func (p *logProcessor) recordNonRoutedResourceLogs(ctx context.Context, routingKey string, rlogs plog.ResourceLogs) {
	logRecordsCount := 0
	sl := rlogs.ScopeLogs()
	for j := 0; j < sl.Len(); j++ {
		logRecordsCount += sl.At(j).LogRecords().Len()
	}

	p.telemetry.RoutingProcessorNonRoutedLogRecords.Add(
		ctx,
		int64(logRecordsCount),
		metric.WithAttributes(
			attribute.String("routing_key", routingKey),
		),
	)
}

func (p *logProcessor) routeForContext(ctx context.Context, l plog.Logs) error {
	value := p.extractor.extractFromContext(ctx)
	exporters := p.router.getExporters(value)
	if value == "" { // "" is a key for default exporters
		p.telemetry.RoutingProcessorNonRoutedLogRecords.Add(
			ctx,
			int64(l.LogRecordCount()),
			metric.WithAttributes(
				attribute.String("routing_key", p.extractor.fromAttr),
			),
		)
	}

	var errs error
	for _, e := range exporters {
		errs = multierr.Append(errs, e.ConsumeLogs(ctx, l))
	}
	return errs
}

func (p *logProcessor) Shutdown(context.Context) error {
	return nil
}

func (p *logProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}
