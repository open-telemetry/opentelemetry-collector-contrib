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

// Strategy defines the interface for different failover strategies
type Strategy string

const (
	StrategyStandard Strategy = "standard"
)

// tracesFailoverStrategy defines the interface for traces failover strategies
type tracesFailoverStrategy interface {
	ConsumeTraces(ctx context.Context, td ptrace.Traces) error
	Shutdown()
}

// metricsFailoverStrategy defines the interface for metrics failover strategies
type metricsFailoverStrategy interface {
	ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error
	Shutdown()
}

// logsFailoverStrategy defines the interface for logs failover strategies
type logsFailoverStrategy interface {
	ConsumeLogs(ctx context.Context, ld plog.Logs) error
	Shutdown()
}

// failoverStrategyFactory creates failover strategies based on mode
type failoverStrategyFactory interface {
	CreateTracesStrategy(router *baseFailoverRouter[consumer.Traces]) tracesFailoverStrategy
	CreateMetricsStrategy(router *baseFailoverRouter[consumer.Metrics]) metricsFailoverStrategy
	CreateLogsStrategy(router *baseFailoverRouter[consumer.Logs]) logsFailoverStrategy
}

// GetFailoverStrategyFactory returns the appropriate factory for the given strategy
func getFailoverStrategyFactory(strategy Strategy) failoverStrategyFactory {
	switch strategy {
	case StrategyStandard, "":
		return &standardFailoverFactory{}
	default:
		panic("failoverconnector: unrecognized strategy " + string(strategy) + " (Validate should have rejected it)")
	}
}
