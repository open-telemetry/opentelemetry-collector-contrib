// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
)

type tracesConnector struct {
	component.StartFunc
	component.ShutdownFunc

	logger *zap.Logger
	config *Config
	router *router[consumer.Traces]
}

func newTracesConnector(
	set connector.CreateSettings,
	config component.Config,
	traces consumer.Traces,
) (*tracesConnector, error) {
	cfg := config.(*Config)

	tr, ok := traces.(connector.TracesRouter)
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

func (c *tracesConnector) ConsumeTraces(ctx context.Context, t ptrace.Traces) error {
	// groups is used to group ptrace.ResourceSpans that are routed to
	// the same set of pipelines. This way we're not ending up with all the
	// spans split up which would cause higher CPU usage.
	groups := make(map[consumer.Traces]ptrace.Traces)

	var errs error
	for i := 0; i < t.ResourceSpans().Len(); i++ {
		rspans := t.ResourceSpans().At(i)
		rtx := ottlresource.NewTransformContext(rspans.Resource())

		noRoutesMatch := true
		for _, route := range c.router.routes {
			_, isMatch, err := route.statement.Execute(ctx, rtx)
			if err != nil {
				if c.config.ErrorMode == ottl.PropagateError {
					return err
				}
				c.group(groups, c.router.defaultConsumer, rspans)
				continue
			}
			if isMatch {
				noRoutesMatch = false
				c.group(groups, route.consumer, rspans)
			}

		}

		if noRoutesMatch {
			// no route conditions are matched, add resource spans to default pipelines group
			c.group(groups, c.router.defaultConsumer, rspans)
		}
	}

	for consumer, group := range groups {
		errs = multierr.Append(errs, consumer.ConsumeTraces(ctx, group))
	}
	return errs
}

func (c *tracesConnector) group(
	groups map[consumer.Traces]ptrace.Traces,
	consumer consumer.Traces,
	spans ptrace.ResourceSpans,
) {
	if consumer == nil {
		return
	}
	group, ok := groups[consumer]
	if !ok {
		group = ptrace.NewTraces()
	}
	spans.CopyTo(group.ResourceSpans().AppendEmpty())
	groups[consumer] = group
}
