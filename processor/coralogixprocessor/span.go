package coralogixprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor"

import (
	"context"

	"github.com/dgraph-io/ristretto"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

func fromConfigOrDefault(configValue int64, defaultValue int64) int64 {
	if configValue > 0 {
		return configValue
	}
	return defaultValue
}

type coralogixProcessor struct {
	config *Config
	component.StartFunc
	component.ShutdownFunc
	cache *ristretto.Cache
}

func newCoralogixProcessor(ctx context.Context, set processor.CreateSettings, cfg *Config, nextConsumer consumer.Traces) (processor.Traces, error) {
	sp := &coralogixProcessor{
		config: cfg,
	}

	if cfg.WithSampling {

		var cacheSize int64 = fromConfigOrDefault(cfg.CacheConfig.maxCacheSizeBytes, 1<<30) // Default to 1GB
		// 8 bytes per entry, 1GB / 8 Bytes = 134,217,728 entries by default
		// Num Counters recommended to be 10x the number of keys to track frequency of
		var numCounters int64 = fromConfigOrDefault(cfg.CacheConfig.maxCachedEntries*10, 1.5e6) // number of keys to track frequency of (1M).

		var err error
		sp.cache, err = ristretto.NewCache(&ristretto.Config{
			NumCounters: numCounters, // number of keys to track frequency of.
			MaxCost:     cacheSize,   // maximum cost of cache.
			BufferItems: 64,          // number of keys per Get buffer.
		})
		if err != nil {
			return nil, err
		}
	}

	return processorhelper.NewTracesProcessor(ctx,
		set,
		cfg,
		nextConsumer,
		sp.processTraces,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}))
}

func (sp *coralogixProcessor) processTraces(_ context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	return td, nil
}
