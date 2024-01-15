// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
)

type metricsConnector struct {
	component.StartFunc
	component.ShutdownFunc

	logger *zap.Logger
	config *Config
	router *router[consumer.Metrics]
}

func newMetricsConnector(
	set connector.CreateSettings,
	config component.Config,
	metrics consumer.Metrics,
) (*metricsConnector, error) {
	cfg := config.(*Config)

	mr, ok := metrics.(connector.MetricsRouterAndConsumer)
	if !ok {
		return nil, errUnexpectedConsumer
	}

	r, err := newRouter(
		cfg.Table,
		cfg.DefaultPipelines,
		mr.Consumer,
		set.TelemetrySettings)

	if err != nil {
		return nil, err
	}

	return &metricsConnector{
		logger: set.TelemetrySettings.Logger,
		config: cfg,
		router: r,
	}, nil
}

func (c *metricsConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *metricsConnector) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	// groups is used to group pmetric.ResourceMetrics that are routed to
	// the same set of exporters. This way we're not ending up with all the
	// metrics split up which would cause higher CPU usage.
	groups := make(map[consumer.Metrics]pmetric.Metrics)

	var errs error

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rmetrics := md.ResourceMetrics().At(i)
		rtx := ottlresource.NewTransformContext(rmetrics.Resource())

		noRoutesMatch := true
		for _, route := range c.router.routeSlice {
			_, isMatch, err := route.statement.Execute(ctx, rtx)
			if err != nil {
				if c.config.ErrorMode == ottl.PropagateError {
					return err
				}
				c.group(groups, c.router.defaultConsumer, rmetrics)
				continue
			}
			if isMatch {
				noRoutesMatch = false
				c.group(groups, route.consumer, rmetrics)
				if c.config.MatchOnce {
					break
				}
			}

		}

		if noRoutesMatch {
			// no route conditions are matched, add resource metrics to default exporters group
			c.group(groups, c.router.defaultConsumer, rmetrics)
		}
	}

	for consumer, group := range groups {
		errs = errors.Join(errs, consumer.ConsumeMetrics(ctx, group))
	}
	return errs
}

func (c *metricsConnector) group(
	groups map[consumer.Metrics]pmetric.Metrics,
	consumer consumer.Metrics,
	metrics pmetric.ResourceMetrics,
) {
	if consumer == nil {
		return
	}
	group, ok := groups[consumer]
	if !ok {
		group = pmetric.NewMetrics()
	}
	metrics.CopyTo(group.ResourceMetrics().AppendEmpty())
	groups[consumer] = group
}
