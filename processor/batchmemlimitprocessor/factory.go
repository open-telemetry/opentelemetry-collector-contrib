package batchmemlimitprocessor

import (
	"context"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/batchmemlimitprocessor/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
)

func NewFactory() processor.Factory {
	return processor.NewFactory(metadata.Type, createDefaultConfig,
		processor.WithLogs(createMemoryLimiterProcessor,
			metadata.LogsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		Timeout:        1 * time.Second,
		SendMemorySize: 1000000,
		SendBatchSize:  1000,
	}
}

func createMemoryLimiterProcessor(
	_ context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	return newBatchMemoryLimiterProcessor(nextConsumer, set.Logger, cfg.(*Config)), nil
}
