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
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor/internal/common"
)

var _ processor.Logs = (*logProcessor)(nil)

type logProcessor struct {
	logger *zap.Logger
	config *Config

	extractor extractor
	router    router[exporter.Logs, ottllog.TransformContext]
}

func newLogProcessor(settings component.TelemetrySettings, config component.Config) *logProcessor {
	cfg := rewriteRoutingEntriesToOTTL(config.(*Config))

	logParser, _ := ottllog.NewParser(common.Functions[ottllog.TransformContext](), settings)

	return &logProcessor{
		logger: settings.Logger,
		config: cfg,
		router: newRouter[exporter.Logs, ottllog.TransformContext](
			cfg.Table,
			cfg.DefaultExporters,
			settings,
			logParser,
		),
		extractor: newExtractor(cfg.FromAttribute, settings.Logger),
	}
}

func (p *logProcessor) Start(_ context.Context, host component.Host) error {
	err := p.router.registerExporters(host.GetExporters()[component.DataTypeLogs])
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
			plog.LogRecord{},
			pcommon.InstrumentationScope{},
			rlogs.Resource(),
		)

		matchCount := len(p.router.routes)
		for key, route := range p.router.routes {
			_, isMatch, err := route.statement.Execute(ctx, ltx)
			if err != nil {
				if p.config.ErrorMode == ottl.PropagateError {
					return err
				}
				p.group("", groups, p.router.defaultExporters, rlogs)
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

func (p *logProcessor) routeForContext(ctx context.Context, l plog.Logs) error {
	value := p.extractor.extractFromContext(ctx)
	exporters := p.router.getExporters(value)

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
