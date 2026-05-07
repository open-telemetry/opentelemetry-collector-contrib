// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type tracesFailoverStrategy interface {
	ConsumeTraces(ctx context.Context, td ptrace.Traces) error
	Shutdown()
}

type metricsFailoverStrategy interface {
	ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error
	Shutdown()
}

type logsFailoverStrategy interface {
	ConsumeLogs(ctx context.Context, ld plog.Logs) error
	Shutdown()
}

type failoverStrategyFactory interface {
	CreateTracesStrategy(router *baseFailoverRouter[consumer.Traces]) tracesFailoverStrategy
	CreateMetricsStrategy(router *baseFailoverRouter[consumer.Metrics]) metricsFailoverStrategy
	CreateLogsStrategy(router *baseFailoverRouter[consumer.Logs]) logsFailoverStrategy
}

// selectFactory dispatches on which variant of the discriminated Strategy union
// is set. The empty Strategy value selects the standard variant with defaults,
// matching the documented zero-config behavior.
func (Strategy) selectFactory() failoverStrategyFactory {
	return &standardFailoverFactory{}
}
