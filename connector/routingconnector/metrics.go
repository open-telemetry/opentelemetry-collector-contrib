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

func (*metricsConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (c *metricsConnector) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	groups := make(map[consumer.Metrics]pmetric.Metrics)
	matched := pmetric.NewMetrics()
	for i := 0; i < len(c.router.routeSlice) && md.ResourceMetrics().Len() > 0; i++ {
		var errs error
		route := c.router.routeSlice[i]
		switch route.statementContext {
		case "request":
			if route.requestCondition.matchRequest(ctx) {
				switch route.action {
				case Copy:
					md.CopyTo(matched)
				default:
					// all metrics are routed
					md.MoveTo(matched)
				}
			}
		case "", "resource":
			switch route.action {
			case Copy:
				pmetricutil.CopyResourcesIf(md, matched,
					func(rs pmetric.ResourceMetrics) bool {
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
				pmetricutil.MoveResourcesIf(md, matched,
					func(rs pmetric.ResourceMetrics) bool {
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
		case "metric":
			switch route.action {
			case Copy:
				pmetricutil.CopyMetricsWithContextIf(md, matched,
					func(rm pmetric.ResourceMetrics, sm pmetric.ScopeMetrics, m pmetric.Metric) bool {
						mtx := ottlmetric.NewTransformContextPtr(rm, sm, m)
						_, isMatch, err := route.metricStatement.Execute(ctx, mtx)
						mtx.Close()
						// If error during statement evaluation consider it as not a match.
						if err != nil {
							errs = errors.Join(errs, err)
							return false
						}
						return isMatch
					},
				)
			default:
				pmetricutil.MoveMetricsWithContextIf(md, matched,
					func(rm pmetric.ResourceMetrics, sm pmetric.ScopeMetrics, m pmetric.Metric) bool {
						mtx := ottlmetric.NewTransformContextPtr(rm, sm, m)
						_, isMatch, err := route.metricStatement.Execute(ctx, mtx)
						mtx.Close()
						// If error during statement evaluation consider it as not a match.
						if err != nil {
							errs = errors.Join(errs, err)
							return false
						}
						return isMatch
					},
				)
			}
		case "datapoint":
			switch route.action {
			case Copy:
				pmetricutil.CopyDataPointsWithContextIf(md, matched,
					func(rm pmetric.ResourceMetrics, sm pmetric.ScopeMetrics, m pmetric.Metric, dp any) bool {
						dptx := ottldatapoint.NewTransformContextPtr(rm, sm, m, dp)
						_, isMatch, err := route.dataPointStatement.Execute(ctx, dptx)
						dptx.Close()
						// If error during statement evaluation consider it as not a match.
						if err != nil {
							errs = errors.Join(errs, err)
							return false
						}
						return isMatch
					},
				)
			default:
				pmetricutil.MoveDataPointsWithContextIf(md, matched,
					func(rm pmetric.ResourceMetrics, sm pmetric.ScopeMetrics, m pmetric.Metric, dp any) bool {
						dptx := ottldatapoint.NewTransformContextPtr(rm, sm, m, dp)
						_, isMatch, err := route.dataPointStatement.Execute(ctx, dptx)
						dptx.Close()
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
		groupAllMetrics(groups, route.consumer, matched)
	}
	// anything left wasn't matched by any route. Send to default consumer
	groupAllMetrics(groups, c.router.defaultConsumer, md)
	var errs error
	for consumer, group := range groups {
		err := consumer.ConsumeMetrics(ctx, group)
		if err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

func groupAllMetrics(
	groups map[consumer.Metrics]pmetric.Metrics,
	cons consumer.Metrics,
	metrics pmetric.Metrics,
) {
	if cons == nil {
		return
	}
	if metrics.ResourceMetrics().Len() == 0 {
		return
	}
	group, ok := groups[cons]
	if !ok {
		group = pmetric.NewMetrics()
		groups[cons] = group
	}
	metrics.ResourceMetrics().MoveAndAppendTo(group.ResourceMetrics())
}
