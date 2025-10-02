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

// FailoverMode defines the interface for different failover strategies
type FailoverMode string

const (
	FailoverModeStandard    FailoverMode = "standard"
	FailoverModeProgressive FailoverMode = "progressive"
)

// TracesFailoverStrategy defines the interface for traces failover strategies
type TracesFailoverStrategy interface {
	ConsumeTraces(ctx context.Context, td ptrace.Traces) error
	Shutdown()
}

// MetricsFailoverStrategy defines the interface for metrics failover strategies
type MetricsFailoverStrategy interface {
	ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error
	Shutdown()
}

// LogsFailoverStrategy defines the interface for logs failover strategies
type LogsFailoverStrategy interface {
	ConsumeLogs(ctx context.Context, ld plog.Logs) error
	Shutdown()
}

// FailoverStrategyFactory creates failover strategies based on mode
type FailoverStrategyFactory interface {
	CreateTracesStrategy(router *baseFailoverRouter[consumer.Traces]) TracesFailoverStrategy
	CreateMetricsStrategy(router *baseFailoverRouter[consumer.Metrics]) MetricsFailoverStrategy
	CreateLogsStrategy(router *baseFailoverRouter[consumer.Logs]) LogsFailoverStrategy
}

// GetFailoverStrategyFactory returns the appropriate factory for the given mode
func GetFailoverStrategyFactory(mode FailoverMode) FailoverStrategyFactory {
	switch mode {
	case FailoverModeProgressive:
		return &progressiveFailoverFactory{}
	case FailoverModeStandard:
		fallthrough
	default:
		return &standardFailoverFactory{}
	}
}
