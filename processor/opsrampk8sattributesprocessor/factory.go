package opsrampk8sattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/opsrampk8sattributesprocessor"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/opsrampk8sattributesprocessor/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

var consumerCapabilities = consumer.Capabilities{MutatesData: true}

// NewFactory returns a new factory for the k8s processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, metadata.TracesStability),
		processor.WithMetrics(createMetricsProcessor, metadata.MetricsStability),
		processor.WithLogs(createLogsProcessor, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		RedisPort: "6379",
	}
}

func createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	next consumer.Traces,
) (processor.Traces, error) {
	oCfg := cfg.(*Config)
	op := newOpsrampK8sAttributesProcessor(set.Logger, oCfg.RedisHost, oCfg.RedisPort, oCfg.RedisPass, oCfg.ClusterName, oCfg.ClusterUid, oCfg.NodeName)

	return processorhelper.NewTraces(
		ctx,
		set,
		cfg,
		next,
		op.processTraces,
		processorhelper.WithCapabilities(consumerCapabilities),
		processorhelper.WithStart(op.Start),
		processorhelper.WithShutdown(op.Shutdown))
}

func createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextLogsConsumer consumer.Logs,
) (processor.Logs, error) {
	oCfg := cfg.(*Config)
	op := newOpsrampK8sAttributesProcessor(set.Logger, oCfg.RedisHost, oCfg.RedisPort, oCfg.RedisPass, oCfg.ClusterName, oCfg.ClusterUid, oCfg.NodeName)

	return processorhelper.NewLogs(
		ctx,
		set,
		cfg,
		nextLogsConsumer,
		op.processLogs,
		processorhelper.WithCapabilities(consumerCapabilities),
		processorhelper.WithStart(op.Start),
		processorhelper.WithShutdown(op.Shutdown))
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextMetricsConsumer consumer.Metrics,
) (processor.Metrics, error) {
	oCfg := cfg.(*Config)
	op := newOpsrampK8sAttributesProcessor(set.Logger, oCfg.RedisHost, oCfg.RedisPort, oCfg.RedisPass, oCfg.ClusterName, oCfg.ClusterUid, oCfg.NodeName)

	return processorhelper.NewMetrics(
		ctx,
		set,
		cfg,
		nextMetricsConsumer,
		op.processMetrics,
		processorhelper.WithCapabilities(consumerCapabilities),
		processorhelper.WithStart(op.Start),
		processorhelper.WithShutdown(op.Shutdown))
}
