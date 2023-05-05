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
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
)

type tracesConnector struct {
	component.StartFunc
	component.ShutdownFunc

	logger *zap.Logger
	config *Config
	router *router[consumer.Traces, ottlspan.TransformContext]
}

type spanGroup struct {
	consumer consumer.Traces
	traces   ptrace.Traces
}

func newTracesConnector(
	set connector.CreateSettings,
	config component.Config,
	traces consumer.Traces,
) (*tracesConnector, error) {
	cfg := config.(*Config)
	spanParser, _ := ottlspan.NewParser(
		common.Functions[ottlspan.TransformContext](),
		set.TelemetrySettings,
	)

	tr, ok := traces.(connector.TracesRouter)
	if !ok {
		return nil, errTooFewPipelines
	}

	r, err := newRouter(
		cfg.Table,
		cfg.DefaultPipelines,
		tr.Consumer,
		set.TelemetrySettings,
		spanParser)

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
	groups := map[string]spanGroup{}

	var errs error
	for i := 0; i < t.ResourceSpans().Len(); i++ {
		rspans := t.ResourceSpans().At(i)
		stx := ottlspan.NewTransformContext(
			ptrace.Span{},
			pcommon.InstrumentationScope{},
			rspans.Resource(),
		)

		matchCount := len(c.router.routes)
		for key, route := range c.router.routes {
			_, isMatch, err := route.statement.Execute(ctx, stx)
			if err != nil {
				if c.config.ErrorMode == ottl.PropagateError {
					return err
				}
				c.group("", groups, c.router.defaultConsumer, rspans)
				continue
			}
			if !isMatch {
				matchCount--
				continue
			}
			c.group(key, groups, route.consumer, rspans)
		}

		if matchCount == 0 {
			// no route conditions are matched, add resource spans to default pipelines group
			c.group("", groups, c.router.defaultConsumer, rspans)
		}
	}

	for _, g := range groups {
		if g.consumer != nil {
			errs = multierr.Append(errs, g.consumer.ConsumeTraces(ctx, g.traces))
		}
	}
	return errs
}

func (c *tracesConnector) group(
	key string,
	groups map[string]spanGroup,
	consumer consumer.Traces,
	spans ptrace.ResourceSpans,
) {
	group, ok := groups[key]
	if !ok {
		group.traces = ptrace.NewTraces()
		group.consumer = consumer
	}
	spans.CopyTo(group.traces.ResourceSpans().AppendEmpty())
	groups[key] = group
}
