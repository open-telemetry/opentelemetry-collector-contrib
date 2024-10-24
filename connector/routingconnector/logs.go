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
		logger: set.TelemetrySettings.Logger,
		config: cfg,
		router: r,
	}, nil
}

func (c *logsConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *logsConnector) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	if c.config.MatchOnce {
		return c.switchLogs(ctx, ld)
	}
	return c.matchAll(ctx, ld)
}

// switchLogs removes items from the original plog.Logs as they are matched,
// and sends them to the appropriate consumer.
func (c *logsConnector) switchLogs(ctx context.Context, ld plog.Logs) error {
	groups := make(map[consumer.Logs]plog.Logs)
	var errs error
	for _, route := range c.router.routeSlice {
		matchedLogs := plog.NewLogs()

		plogutil.MoveResourcesIf(ld, matchedLogs,
			func(rl plog.ResourceLogs) bool {
				rtx := ottlresource.NewTransformContext(rl.Resource(), rl)
				_, isMatch, err := route.statement.Execute(ctx, rtx)
				errs = errors.Join(errs, err)
				return isMatch
			},
		)

		if errs != nil {
			if c.config.ErrorMode == ottl.PropagateError {
				return errs
			}
			groupAll(groups, c.router.defaultConsumer, matchedLogs)

		}
		groupAll(groups, route.consumer, matchedLogs)
	}

	// anything left wasn't matched by any route. Send to default consumer
	groupAll(groups, c.router.defaultConsumer, ld)

	for consumer, group := range groups {
		errs = errors.Join(errs, consumer.ConsumeLogs(ctx, group))
	}
	return errs
}

func (c *logsConnector) matchAll(ctx context.Context, ld plog.Logs) error {
	// routingEntry is used to group plog.ResourceLogs that are routed to
	// the same set of exporters.
	// This way we're not ending up with all the logs split up which would cause
	// higher CPU usage.
	groups := make(map[consumer.Logs]plog.Logs)
	var errs error

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rlogs := ld.ResourceLogs().At(i)
		rtx := ottlresource.NewTransformContext(rlogs.Resource(), rlogs)

		noRoutesMatch := true
		for _, route := range c.router.routeSlice {
			_, isMatch, err := route.statement.Execute(ctx, rtx)
			if err != nil {
				if c.config.ErrorMode == ottl.PropagateError {
					return err
				}
				group(groups, c.router.defaultConsumer, rlogs)
				continue
			}
			if isMatch {
				noRoutesMatch = false
				group(groups, route.consumer, rlogs)
			}

		}

		if noRoutesMatch {
			// no route conditions are matched, add resource logs to default exporters group
			group(groups, c.router.defaultConsumer, rlogs)
		}
	}
	for consumer, group := range groups {
		errs = errors.Join(errs, consumer.ConsumeLogs(ctx, group))
	}
	return errs
}

func groupAll(
	groups map[consumer.Logs]plog.Logs,
	cons consumer.Logs,
	logs plog.Logs,
) {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		group(groups, cons, logs.ResourceLogs().At(i))
	}
}

func group(
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
