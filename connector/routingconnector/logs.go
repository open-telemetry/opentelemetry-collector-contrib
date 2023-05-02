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
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
)

type logsConnector struct {
	component.StartFunc
	component.ShutdownFunc

	logger *zap.Logger
	config *Config
	router *router[consumer.Logs, ottllog.TransformContext]
}

type logsGroup struct {
	consumer consumer.Logs
	logs     plog.Logs
}

func newLogsConnector(set connector.CreateSettings, config component.Config, logs consumer.Logs) (*logsConnector, error) {
	cfg := config.(*Config)

	logParser, _ := ottllog.NewParser(common.Functions[ottllog.TransformContext](), set.TelemetrySettings)

	lr, ok := logs.(connector.LogsRouter)
	if !ok {
		return nil, errTooFewPipelines
	}

	r, err := newRouter(
		cfg.Table,
		cfg.DefaultPipelines,
		lr.Consumer,
		set.TelemetrySettings,
		logParser)

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
	// routingEntry is used to group plog.ResourceLogs that are routed to
	// the same set of exporters.
	// This way we're not ending up with all the logs split up which would cause
	// higher CPU usage.
	groups := map[string]logsGroup{}
	var errs error

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rlogs := ld.ResourceLogs().At(i)
		ltx := ottllog.NewTransformContext(
			plog.LogRecord{},
			pcommon.InstrumentationScope{},
			rlogs.Resource(),
		)

		matchCount := len(c.router.routes)
		for key, route := range c.router.routes {
			_, isMatch, err := route.statement.Execute(ctx, ltx)
			if err != nil {
				if c.config.ErrorMode == ottl.PropagateError {
					return err
				}
				c.group("", groups, c.router.defaultConsumer, rlogs)
				continue
			}
			if !isMatch {
				matchCount--
				continue
			}
			c.group(key, groups, route.consumer, rlogs)
		}

		if matchCount == 0 {
			// no route conditions are matched, add resource logs to default exporters group
			c.group("", groups, c.router.defaultConsumer, rlogs)
		}
	}
	for _, g := range groups {
		if g.consumer != nil {
			errs = multierr.Append(errs, g.consumer.ConsumeLogs(ctx, g.logs))
		}
	}
	return errs
}

func (c *logsConnector) group(
	key string,
	groups map[string]logsGroup,
	consumer consumer.Logs,
	logs plog.ResourceLogs,
) {
	group, ok := groups[key]
	if !ok {
		group.logs = plog.NewLogs()
		group.consumer = consumer
	}
	logs.CopyTo(group.logs.ResourceLogs().AppendEmpty())
	groups[key] = group
}
