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
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

var (
	errEmptyRoute                   = errors.New("empty routing attribute provided")
	errNoExporters                  = errors.New("no exporters defined for the route")
	errNoTableItems                 = errors.New("the routing table is empty")
	errNoMissingFromAttribute       = errors.New("the FromAttribute property is empty")
	errDefaultExporterNotFound      = errors.New("default exporter not found")
	errExporterNotFound             = errors.New("exporter not found")
	errNoExportersAfterRegistration = errors.New("provided configuration resulted in no exporter available to accept data")
)

var (
	_ component.TracesProcessor  = (*processorImp)(nil)
	_ component.MetricsProcessor = (*processorImp)(nil)
	_ component.LogsProcessor    = (*processorImp)(nil)
)

type processorImp struct {
	logger *zap.Logger
	router *router
}

// newProcessor creates new processor
func newProcessor(logger *zap.Logger, cfg config.Processor) *processorImp {
	logger.Info("building processor")

	oCfg := cfg.(*Config)

	return &processorImp{
		logger: logger,
		router: newRouter(*oCfg, logger),
	}
}

func (e *processorImp) Start(_ context.Context, host component.Host) error {
	return e.router.registerExporters(host.GetExporters())
}

func (e *processorImp) Shutdown(context.Context) error {
	return nil
}

func (e *processorImp) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	routedTraces := e.router.RouteTraces(ctx, td)
	for _, rt := range routedTraces {
		for _, exp := range rt.exporters {
			// TODO: determine the proper action when errors happen
			if err := exp.ConsumeTraces(ctx, rt.traces); err != nil {
				return err
			}
		}
	}

	return nil
}

func (e *processorImp) ConsumeMetrics(ctx context.Context, tm pmetric.Metrics) error {
	routedMetrics := e.router.RouteMetrics(ctx, tm)
	for _, rm := range routedMetrics {
		for _, exp := range rm.exporters {
			// TODO: determine the proper action when errors happen
			if err := exp.ConsumeMetrics(ctx, rm.metrics); err != nil {
				return err
			}
		}
	}

	return nil
}

func (e *processorImp) ConsumeLogs(ctx context.Context, tl plog.Logs) error {
	routedLogs := e.router.RouteLogs(ctx, tl)
	for _, rl := range routedLogs {
		for _, exp := range rl.exporters {
			// TODO: determine the proper action when errors happen
			if err := exp.ConsumeLogs(ctx, rl.logs); err != nil {
				return err
			}
		}
	}

	return nil
}

func (e *processorImp) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}
