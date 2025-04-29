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

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/pmetricutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
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
	set connector.Settings,
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
		logger: set.Logger,
		config: cfg,
		router: r,
	}, nil
}

func (c *metricsConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (c *metricsConnector) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	groups := make(map[consumer.Metrics]pmetric.Metrics)
	var errs error
	for i := 0; i < len(c.router.routeSlice) && md.ResourceMetrics().Len() > 0; i++ {
		route := c.router.routeSlice[i]
		matchedMetrics := pmetric.NewMetrics()
		switch route.statementContext {
		case "request":
			if route.requestCondition.matchRequest(ctx) {
				groupAllMetrics(groups, route.consumer, md)
				md = pmetric.NewMetrics() // all metrics have been routed
			}
		case "", "resource":
			pmetricutil.MoveResourcesIf(md, matchedMetrics,
				func(rs pmetric.ResourceMetrics) bool {
					rtx := ottlresource.NewTransformContext(rs.Resource(), rs)
					_, isMatch, err := route.resourceStatement.Execute(ctx, rtx)
					errs = errors.Join(errs, err)
					return isMatch
				},
			)
		case "metric":
			pmetricutil.MoveMetricsWithContextIf(md, matchedMetrics,
				func(rm pmetric.ResourceMetrics, sm pmetric.ScopeMetrics, m pmetric.Metric) bool {
					mtx := ottlmetric.NewTransformContext(m, sm.Metrics(), sm.Scope(), rm.Resource(), sm, rm)
					_, isMatch, err := route.metricStatement.Execute(ctx, mtx)
					errs = errors.Join(errs, err)
					return isMatch
				},
			)
		case "datapoint":
			pmetricutil.MoveDataPointsWithContextIf(md, matchedMetrics,
				func(rm pmetric.ResourceMetrics, sm pmetric.ScopeMetrics, m pmetric.Metric, dp any) bool {
					dptx := ottldatapoint.NewTransformContext(dp, m, sm.Metrics(), sm.Scope(), rm.Resource(), sm, rm)
					_, isMatch, err := route.dataPointStatement.Execute(ctx, dptx)
					errs = errors.Join(errs, err)
					return isMatch
				},
			)
		}
		if errs != nil {
			if c.config.ErrorMode == ottl.PropagateError {
				return errs
			}
			groupAllMetrics(groups, c.router.defaultConsumer, matchedMetrics)
		}
		groupAllMetrics(groups, route.consumer, matchedMetrics)
	}
	// anything left wasn't matched by any route. Send to default consumer
	groupAllMetrics(groups, c.router.defaultConsumer, md)
	for consumer, group := range groups {
		errs = errors.Join(errs, consumer.ConsumeMetrics(ctx, group))
	}
	return errs
}

func groupAllMetrics(
	groups map[consumer.Metrics]pmetric.Metrics,
	cons consumer.Metrics,
	metrics pmetric.Metrics,
) {
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		groupMetrics(groups, cons, metrics.ResourceMetrics().At(i))
	}
}

func groupMetrics(
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
