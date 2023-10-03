package failoverconnector

import (
	"context"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"time"
)

const (
	Type             = "failover"
	TracesStability  = component.StabilityLevelDevelopment
	MetricsStability = component.StabilityLevelDevelopment
	LogsStability    = component.StabilityLevelDevelopment
)

func NewFactory() connector.Factory {
	return connector.NewFactory(
		Type,
		createDefaultConfig,
		connector.WithTracesToTraces(createTracesToTraces, TracesStability),
		connector.WithMetricsToMetrics(createMetricsToMetrics, MetricsStability),
		connector.WithLogsToLogs(createLogsToLogs, LogsStability),
	)
}

func createDefaultConfig() component.Config { // LOOK INTO THIS
	return &Config{
		RetryGap:      time.Minute,
		RetryInterval: 15 * time.Minute,
	}
}

func createTracesToTraces(
	_ context.Context,
	set connector.CreateSettings,
	cfg component.Config,
	traces consumer.Traces,
) (connector.Traces, error) {
	return newTracesToTraces(set, cfg, traces)
}

func createMetricsToMetrics(
	_ context.Context,
	set connector.CreateSettings,
	cfg component.Config,
	metrics consumer.Metrics,
) (connector.Metrics, error) {
	return newMetricsToMetrics(set, cfg, metrics)
}

func createLogsToLogs(
	_ context.Context,
	set connector.CreateSettings,
	cfg component.Config,
	logs consumer.Logs,
) (connector.Logs, error) {
	return newLogsToLogs(set, cfg, logs)
}
