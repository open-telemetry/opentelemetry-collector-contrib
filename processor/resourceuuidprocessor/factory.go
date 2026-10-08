package resourceuuidprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceuuidprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceuuidprocessor/internal/metadata"
)

// NewFactory returns the factory of the resource uuid processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithLogs(createLogsProcessor, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Endpoint:        defaultEndpoint,
		RefreshInterval: defaultRefreshInterval,
		RetryInterval:   defaultRetryInterval,
		CacheTTL:        defaultCacheTTL,
		CacheSize:       defaultCacheSize,
		Timeout:         defaultTimeout,
		PodUIDAttribute: defaultPodUIDAttribute,
		TargetAttribute: defaultTargetAttribute,
	}
}

func createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	next consumer.Logs,
) (processor.Logs, error) {
	p := newResourceUUIDProcessor(set.Logger, cfg.(*Config))
	return processorhelper.NewLogs(
		ctx,
		set,
		cfg,
		next,
		p.processLogs,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
		processorhelper.WithStart(p.start),
		processorhelper.WithShutdown(p.shutdown),
	)
}
