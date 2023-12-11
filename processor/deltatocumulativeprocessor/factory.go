package deltatocumulativeprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"

import (
	"context"
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
)

func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithMetrics(createMetricsProcessor, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Interval: 60 * time.Second,
	}
}

func createMetricsProcessor(ctx context.Context, set processor.CreateSettings, cfg component.Config, next consumer.Metrics) (processor.Metrics, error) {
	pcfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("configuration parsing error")
	}

	return newProcessor(pcfg, set.Logger, next), nil
}
