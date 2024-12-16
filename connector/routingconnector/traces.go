// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/ptraceutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
)

type tracesConnector struct {
	component.StartFunc
	component.ShutdownFunc

	logger *zap.Logger
	config *Config
	router *router[consumer.Traces]
}

func newTracesConnector(
	set connector.Settings,
	config component.Config,
	traces consumer.Traces,
) (*tracesConnector, error) {
	cfg := config.(*Config)

	// TODO update log from warning to error in v0.116.0
	if !cfg.MatchOnce {
		set.Logger.Warn("The 'match_once' field has been deprecated. Set to 'true' to suppress this warning.")
	}

	tr, ok := traces.(connector.TracesRouterAndConsumer)
	if !ok {
		return nil, errUnexpectedConsumer
	}

	r, err := newRouter(
		cfg.Table,
		cfg.DefaultPipelines,
		tr.Consumer,
		set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	return &tracesConnector{
		logger: set.TelemetrySettings.Logger,
		config: cfg,
		router: r,
	}, nil
}

func (*tracesConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *tracesConnector) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	if c.config.MatchOnce {
		return c.switchTraces(ctx, td)
	}
	return c.matchAllTraces(ctx, td)
}

func (c *tracesConnector) switchTraces(ctx context.Context, td ptrace.Traces) error {
	groups := make(map[consumer.Traces]ptrace.Traces)
	var errs error
	for i := 0; i < len(c.router.routeSlice) && td.ResourceSpans().Len() > 0; i++ {
		route := c.router.routeSlice[i]
		matchedSpans := ptrace.NewTraces()
		switch route.statementContext {
		case "request":
			if route.requestCondition.matchRequest(ctx) {
				groupAllTraces(groups, route.consumer, td)
				td = ptrace.NewTraces() // all traces have been routed
			}
		case "", "resource":
			ptraceutil.MoveResourcesIf(td, matchedSpans,
				func(rs ptrace.ResourceSpans) bool {
					rtx := ottlresource.NewTransformContext(rs.Resource(), rs)
					_, isMatch, err := route.resourceStatement.Execute(ctx, rtx)
					errs = errors.Join(errs, err)
					return isMatch
				},
			)
		case "span":
			ptraceutil.MoveSpansWithContextIf(td, matchedSpans,
				func(rs ptrace.ResourceSpans, ss ptrace.ScopeSpans, s ptrace.Span) bool {
					mtx := ottlspan.NewTransformContext(s, ss.Scope(), rs.Resource(), ss, rs)
					_, isMatch, err := route.spanStatement.Execute(ctx, mtx)
					errs = errors.Join(errs, err)
					return isMatch
				},
			)
		}
		if errs != nil {
			if c.config.ErrorMode == ottl.PropagateError {
				return errs
			}
			groupAllTraces(groups, c.router.defaultConsumer, matchedSpans)
		}
		groupAllTraces(groups, route.consumer, matchedSpans)
	}
	// anything left wasn't matched by any route. Send to default consumer
	groupAllTraces(groups, c.router.defaultConsumer, td)
	for consumer, group := range groups {
		errs = errors.Join(errs, consumer.ConsumeTraces(ctx, group))
	}
	return errs
}

func (c *tracesConnector) matchAllTraces(ctx context.Context, td ptrace.Traces) error {
	// groups is used to group ptrace.ResourceSpans that are routed to
	// the same set of pipelines. This way we're not ending up with all the
	// spans split up which would cause higher CPU usage.
	groups := make(map[consumer.Traces]ptrace.Traces)

	var errs error
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		rtx := ottlresource.NewTransformContext(rspans.Resource(), rspans)
		noRoutesMatch := true
		for _, route := range c.router.routeSlice {
			_, isMatch, err := route.resourceStatement.Execute(ctx, rtx)
			if err != nil {
				if c.config.ErrorMode == ottl.PropagateError {
					return err
				}
				groupTraces(groups, c.router.defaultConsumer, rspans)
				continue
			}
			if isMatch {
				noRoutesMatch = false
				groupTraces(groups, route.consumer, rspans)
			}
		}
		if noRoutesMatch {
			// no route conditions are matched, add resource spans to default pipelines group
			groupTraces(groups, c.router.defaultConsumer, rspans)
		}
	}
	for consumer, group := range groups {
		errs = errors.Join(errs, consumer.ConsumeTraces(ctx, group))
	}
	return errs
}

func groupAllTraces(
	groups map[consumer.Traces]ptrace.Traces,
	cons consumer.Traces,
	traces ptrace.Traces,
) {
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		groupTraces(groups, cons, traces.ResourceSpans().At(i))
	}
}

func groupTraces(
	groups map[consumer.Traces]ptrace.Traces,
	cons consumer.Traces,
	spans ptrace.ResourceSpans,
) {
	if cons == nil {
		return
	}
	group, ok := groups[cons]
	if !ok {
		group = ptrace.NewTraces()
	}
	spans.CopyTo(group.ResourceSpans().AppendEmpty())
	groups[cons] = group
}
