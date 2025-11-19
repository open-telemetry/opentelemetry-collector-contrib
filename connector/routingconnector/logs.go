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

func (*logsConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (c *logsConnector) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	groups := make(map[consumer.Logs]plog.Logs)
	matched := plog.NewLogs()
	for i := 0; i < len(c.router.routeSlice) && ld.ResourceLogs().Len() > 0; i++ {
		var errs error
		route := c.router.routeSlice[i]
		switch route.statementContext {
		case "request":
			if route.requestCondition.matchRequest(ctx) {
				// all logs are routed
				ld.MoveTo(matched)
			}
		case "", "resource":
			plogutil.MoveResourcesIf(ld, matched,
				func(rl plog.ResourceLogs) bool {
					rtx := ottlresource.NewTransformContext(rl.Resource(), rl)
					_, isMatch, err := route.resourceStatement.Execute(ctx, rtx)
					// If error during statement evaluation consider it as not a match.
					if err != nil {
						errs = errors.Join(errs, err)
						return false
					}
					return isMatch
				},
			)
		case "log":
			plogutil.MoveRecordsWithContextIf(ld, matched,
				func(rl plog.ResourceLogs, sl plog.ScopeLogs, lr plog.LogRecord) bool {
					ltx := ottllog.NewTransformContext(lr, sl.Scope(), rl.Resource(), sl, rl)
					_, isMatch, err := route.logStatement.Execute(ctx, ltx)
					// If error during statement evaluation consider it as not a match.
					if err != nil {
						errs = errors.Join(errs, err)
						return false
					}
					return isMatch
				},
			)
		}
		if errs != nil && c.config.ErrorMode == ottl.PropagateError {
			return errs
		}
		groupAllLogs(groups, route.consumer, matched)
	}
	// anything left wasn't matched by any route. Send to default consumer
	groupAllLogs(groups, c.router.defaultConsumer, ld)
	var errs error
	for consumer, group := range groups {
		err := consumer.ConsumeLogs(ctx, group)
		if err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

func groupAllLogs(
	groups map[consumer.Logs]plog.Logs,
	cons consumer.Logs,
	logs plog.Logs,
) {
	if cons == nil {
		return
	}
	if logs.ResourceLogs().Len() == 0 {
		return
	}
	group, ok := groups[cons]
	if !ok {
		group = plog.NewLogs()
		groups[cons] = group
	}
	logs.ResourceLogs().MoveAndAppendTo(group.ResourceLogs())
}
