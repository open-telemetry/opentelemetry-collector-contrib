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
		logger: set.Logger,
		config: cfg,
		router: r,
	}, nil
}

func (*tracesConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (c *tracesConnector) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	groups := make(map[consumer.Traces]ptrace.Traces)
	matched := ptrace.NewTraces()
	for i := 0; i < len(c.router.routeSlice) && td.ResourceSpans().Len() > 0; i++ {
		var errs error
		route := c.router.routeSlice[i]
		switch route.statementContext {
		case "request":
			if route.requestCondition.matchRequest(ctx) {
				switch route.action {
				case Copy:
					td.CopyTo(matched)
				default:
					// all traces are routed
					td.MoveTo(matched)
				}
			}
		case "", "resource":
			switch route.action {
			case Copy:
				ptraceutil.CopyResourcesIf(td, matched,
					func(rs ptrace.ResourceSpans) bool {
						rtx := ottlresource.NewTransformContextPtr(rs.Resource(), rs)
						defer rtx.Close()
						_, isMatch, err := route.resourceStatement.Execute(ctx, rtx)
						// If error during statement evaluation consider it as not a match.
						if err != nil {
							errs = errors.Join(errs, err)
							return false
						}
						return isMatch
					},
				)
			default:
				ptraceutil.MoveResourcesIf(td, matched,
					func(rs ptrace.ResourceSpans) bool {
						rtx := ottlresource.NewTransformContextPtr(rs.Resource(), rs)
						defer rtx.Close()
						_, isMatch, err := route.resourceStatement.Execute(ctx, rtx)
						// If error during statement evaluation consider it as not a match.
						if err != nil {
							errs = errors.Join(errs, err)
							return false
						}
						return isMatch
					},
				)
			}
		case "span":
			switch route.action {
			case Copy:
				ptraceutil.CopySpansWithContextIf(td, matched,
					func(rs ptrace.ResourceSpans, ss ptrace.ScopeSpans, s ptrace.Span) bool {
						mtx := ottlspan.NewTransformContextPtr(rs, ss, s)
						defer mtx.Close()
						_, isMatch, err := route.spanStatement.Execute(ctx, mtx)
						// If error during statement evaluation consider it as not a match.
						if err != nil {
							errs = errors.Join(errs, err)
							return false
						}
						return isMatch
					},
				)
			default:
				ptraceutil.MoveSpansWithContextIf(td, matched,
					func(rs ptrace.ResourceSpans, ss ptrace.ScopeSpans, s ptrace.Span) bool {
						mtx := ottlspan.NewTransformContextPtr(rs, ss, s)
						defer mtx.Close()
						_, isMatch, err := route.spanStatement.Execute(ctx, mtx)
						// If error during statement evaluation consider it as not a match.
						if err != nil {
							errs = errors.Join(errs, err)
							return false
						}
						return isMatch
					},
				)
			}
		}
		if errs != nil && c.config.ErrorMode == ottl.PropagateError {
			return errs
		}
		groupAllTraces(groups, route.consumer, matched)
	}
	// anything left wasn't matched by any route. Send to default consumer
	groupAllTraces(groups, c.router.defaultConsumer, td)
	var errs error
	for consumer, group := range groups {
		err := consumer.ConsumeTraces(ctx, group)
		if err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

func groupAllTraces(
	groups map[consumer.Traces]ptrace.Traces,
	cons consumer.Traces,
	traces ptrace.Traces,
) {
	if cons == nil {
		return
	}
	if traces.ResourceSpans().Len() == 0 {
		return
	}
	group, ok := groups[cons]
	if !ok {
		group = ptrace.NewTraces()
		groups[cons] = group
	}
	traces.ResourceSpans().MoveAndAppendTo(group.ResourceSpans())
}
