// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
)

type metricsConnector struct {
	component.StartFunc
	component.ShutdownFunc

	logger *zap.Logger
	config *Config
	router *router[consumer.Metrics, ottldatapoint.TransformContext]
}

type metricsGroup struct {
	consumer consumer.Metrics
	metrics  pmetric.Metrics
}

func newMetricsConnector(set connector.CreateSettings, config component.Config, metrics consumer.Metrics) (*metricsConnector, error) {
	cfg := config.(*Config)

	dataPointParser, _ := ottldatapoint.NewParser(common.Functions[ottldatapoint.TransformContext](), set.TelemetrySettings)

	mr, ok := metrics.(connector.MetricsRouter)
	if !ok {
		return nil, errTooFewPipelines
	}

	r, err := newRouter(
		cfg.Table,
		cfg.DefaultPipelines,
		mr.Consumer,
		set.TelemetrySettings,
		dataPointParser)

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
	groups := map[string]metricsGroup{}

	var errs error

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rmetrics := md.ResourceMetrics().At(i)
		mtx := ottldatapoint.NewTransformContext(
			nil,
			pmetric.Metric{},
			pmetric.MetricSlice{},
			pcommon.InstrumentationScope{},
			rmetrics.Resource(),
		)

		matchCount := len(c.router.routes)
		for key, route := range c.router.routes {
			_, isMatch, err := route.statement.Execute(ctx, mtx)
			if err != nil {
				if c.config.ErrorMode == ottl.PropagateError {
					return err
				}
				c.group("", groups, c.router.defaultConsumer, rmetrics)
				continue
			}
			if !isMatch {
				matchCount--
				continue
			}
			c.group(key, groups, route.consumer, rmetrics)
		}

		if matchCount == 0 {
			// no route conditions are matched, add resource metrics to default exporters group
			c.group("", groups, c.router.defaultConsumer, rmetrics)
		}
	}

	for _, g := range groups {
		if g.consumer != nil {
			errs = multierr.Append(errs, g.consumer.ConsumeMetrics(ctx, g.metrics))
		}
	}
	return errs
}

func (c *metricsConnector) group(
	key string,
	groups map[string]metricsGroup,
	consumer consumer.Metrics,
	metrics pmetric.ResourceMetrics,
) {
	group, ok := groups[key]
	if !ok {
		group.metrics = pmetric.NewMetrics()
		group.consumer = consumer
	}
	metrics.CopyTo(group.metrics.ResourceMetrics().AppendEmpty())
	groups[key] = group
}
