// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/plogutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
)

type logsConnector struct {
	component.StartFunc
	component.ShutdownFunc

	logger *zap.Logger
	config *Config
	router *router[consumer.Logs]
}

func newLogsConnector(
	set connector.Settings,
	config component.Config,
	logs consumer.Logs,
) (*logsConnector, error) {
	cfg := config.(*Config)
	lr, ok := logs.(connector.LogsRouterAndConsumer)
	if !ok {
		return nil, errUnexpectedConsumer
	}

	r, err := newRouter(
		cfg.Table,
		cfg.DefaultPipelines,
		lr.Consumer,
		set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	return &logsConnector{
		logger: set.Logger,
		config: cfg,
		router: r,
	}, nil
}

func (c *logsConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (c *logsConnector) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	groups := make(map[consumer.Logs]plog.Logs)
	var errs error
	for i := 0; i < len(c.router.routeSlice) && ld.ResourceLogs().Len() > 0; i++ {
		route := c.router.routeSlice[i]
		matchedLogs := plog.NewLogs()
		switch route.statementContext {
		case "request":
			if route.requestCondition.matchRequest(ctx) {
				groupAllLogs(groups, route.consumer, ld)
				ld = plog.NewLogs() // all logs have been routed
			}
		case "", "resource":
			plogutil.MoveResourcesIf(ld, matchedLogs,
				func(rl plog.ResourceLogs) bool {
					rtx := ottlresource.NewTransformContext(rl.Resource(), rl)
					_, isMatch, err := route.resourceStatement.Execute(ctx, rtx)
					errs = errors.Join(errs, err)
					return isMatch
				},
			)
		case "log":
			plogutil.MoveRecordsWithContextIf(ld, matchedLogs,
				func(rl plog.ResourceLogs, sl plog.ScopeLogs, lr plog.LogRecord) bool {
					ltx := ottllog.NewTransformContext(lr, sl.Scope(), rl.Resource(), sl, rl)
					_, isMatch, err := route.logStatement.Execute(ctx, ltx)
					errs = errors.Join(errs, err)
					return isMatch
				},
			)
		}
		if errs != nil {
			if c.config.ErrorMode == ottl.PropagateError {
				return errs
			}
			groupAllLogs(groups, c.router.defaultConsumer, matchedLogs)
		}
		groupAllLogs(groups, route.consumer, matchedLogs)
	}
	// anything left wasn't matched by any route. Send to default consumer
	groupAllLogs(groups, c.router.defaultConsumer, ld)
	for consumer, group := range groups {
		errs = errors.Join(errs, consumer.ConsumeLogs(ctx, group))
	}
	return errs
}

func groupAllLogs(
	groups map[consumer.Logs]plog.Logs,
	cons consumer.Logs,
	logs plog.Logs,
) {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		groupLogs(groups, cons, logs.ResourceLogs().At(i))
	}
}

func groupLogs(
	groups map[consumer.Logs]plog.Logs,
	cons consumer.Logs,
	logs plog.ResourceLogs,
) {
	if cons == nil {
		return
	}
	group, ok := groups[cons]
	if !ok {
		group = plog.NewLogs()
	}
	logs.CopyTo(group.ResourceLogs().AppendEmpty())
	groups[cons] = group
}
