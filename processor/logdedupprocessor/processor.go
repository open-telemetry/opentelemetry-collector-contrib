// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logdedupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/logdedupprocessor"

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/logdedupprocessor/internal/metadata"
)

// shardedAggregator is the common interface for aggregating logs, either as a
// single bucket or as multiple buckets keyed by metadata combination.
type shardedAggregator interface {
	add(ctx context.Context, logRecord plog.LogRecord, scope pcommon.InstrumentationScope, resource pcommon.Resource) error
	flush(ctx context.Context, nextConsumer consumer.Logs, logger *zap.Logger)
}

// singleShardAggregator is used when no metadata_keys are configured.
// It wraps a single logAggregator with no runtime overhead compared to the original behavior.
type singleShardAggregator struct {
	aggregator *logAggregator
}

func (s *singleShardAggregator) add(_ context.Context, logRecord plog.LogRecord, scope pcommon.InstrumentationScope, resource pcommon.Resource) error {
	s.aggregator.Add(resource, scope, logRecord)
	return nil
}

func (s *singleShardAggregator) flush(ctx context.Context, nextConsumer consumer.Logs, logger *zap.Logger) {
	logs := s.aggregator.Export(ctx)
	if logs.LogRecordCount() > 0 {
		if err := nextConsumer.ConsumeLogs(ctx, logs); err != nil {
			logger.Error("failed to consume logs", zap.Error(err))
		}
		s.aggregator.Reset()
	}
}

// aggregatorShard holds a logAggregator and the client metadata for one metadata combination.
type aggregatorShard struct {
	aggregator *logAggregator
	clientInfo client.Info
}

// multiShardAggregator is used when metadata_keys are configured.
// It maintains one aggregatorShard per unique combination of metadata values.
type multiShardAggregator struct {
	metadataKeys             []string
	metadataCardinalityLimit int

	// Fields below are passed through to newLogAggregator for on-demand shard creation.
	logCountAttribute string
	timezone          *time.Location
	telemetryBuilder  *metadata.TelemetryBuilder
	includeFields     []string

	shards map[attribute.Set]*aggregatorShard
	// lock protects the shards map during concurrent lookups and creation.
	lock sync.Mutex
}

func (m *multiShardAggregator) add(ctx context.Context, logRecord plog.LogRecord, scope pcommon.InstrumentationScope, resource pcommon.Resource) error {
	info := client.FromContext(ctx)
	attrs := make([]attribute.KeyValue, 0, len(m.metadataKeys))
	for _, k := range m.metadataKeys {
		vs := info.Metadata.Get(k)
		if len(vs) == 1 {
			attrs = append(attrs, attribute.String(k, vs[0]))
		} else {
			attrs = append(attrs, attribute.StringSlice(k, vs))
		}
	}
	aset := attribute.NewSet(attrs...)

	shard, err := m.getOrCreateShard(info, aset)
	if err != nil {
		return err
	}

	shard.aggregator.Add(resource, scope, logRecord)
	return nil
}

func (m *multiShardAggregator) getOrCreateShard(info client.Info, aset attribute.Set) (*aggregatorShard, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	shard, ok := m.shards[aset]
	if ok {
		return shard, nil
	}

	if m.metadataCardinalityLimit > 0 && len(m.shards) >= m.metadataCardinalityLimit {
		return nil, consumererror.NewPermanent(errors.New("too many batcher metadata-value combinations"))
	}

	md := make(map[string][]string, len(m.metadataKeys))
	for _, k := range m.metadataKeys {
		md[k] = info.Metadata.Get(k)
	}
	shard = &aggregatorShard{
		aggregator: newLogAggregator(m.logCountAttribute, m.timezone, m.telemetryBuilder, m.includeFields),
		clientInfo: client.Info{
			Metadata: client.NewMetadata(md),
		},
	}
	m.shards[aset] = shard
	return shard, nil
}

func (m *multiShardAggregator) flush(_ context.Context, nextConsumer consumer.Logs, logger *zap.Logger) {
	m.lock.Lock()
	shards := make([]*aggregatorShard, 0, len(m.shards))
	for _, s := range m.shards {
		shards = append(shards, s)
	}
	m.lock.Unlock()

	for _, shard := range shards {
		exportCtx := client.NewContext(context.Background(), shard.clientInfo)
		logs := shard.aggregator.Export(exportCtx)
		if logs.LogRecordCount() > 0 {
			if err := nextConsumer.ConsumeLogs(exportCtx, logs); err != nil {
				logger.Error("failed to consume logs", zap.Error(err))
			}
			shard.aggregator.Reset()
		}
	}
}

// logDedupProcessor is a logDedupProcessor that counts duplicate instances of logs.
type logDedupProcessor struct {
	emitInterval time.Duration
	conditions   *ottl.ConditionSequence[*ottllog.TransformContext]
	aggregator   shardedAggregator
	remover      *fieldRemover
	nextConsumer consumer.Logs
	logger       *zap.Logger
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	mux          sync.Mutex
}

func newProcessor(cfg *Config, nextConsumer consumer.Logs, settings processor.Settings) (*logDedupProcessor, error) {
	telemetryBuilder, err := metadata.NewTelemetryBuilder(settings.TelemetrySettings)
	if err != nil {
		return nil, fmt.Errorf("failed to create telemetry builder: %w", err)
	}

	// This should not happen due to config validation but we check anyways.
	timezone, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone: %w", err)
	}

	// Normalize metadata keys to lowercase. HTTP/2 headers are case-insensitive,
	// but attribute.Set key lookup is case-sensitive — normalizing avoids
	// duplicate shards for the same logical key.
	metadataKeys := make([]string, len(cfg.MetadataKeys))
	for i, k := range cfg.MetadataKeys {
		metadataKeys[i] = strings.ToLower(k)
	}
	sort.Strings(metadataKeys)

	var agg shardedAggregator
	if len(metadataKeys) == 0 {
		agg = &singleShardAggregator{
			aggregator: newLogAggregator(cfg.LogCountAttribute, timezone, telemetryBuilder, cfg.IncludeFields),
		}
	} else {
		if cfg.MetadataCardinalityLimit == 0 {
			settings.Logger.Warn("metadata_keys is set but metadata_cardinality_limit is 0; the number of metadata combinations is unbounded, which may lead to unbounded memory growth. Set metadata_cardinality_limit to a positive value to cap it.",
				zap.Strings("metadata_keys", metadataKeys),
			)
		}
		agg = &multiShardAggregator{
			metadataKeys:             metadataKeys,
			metadataCardinalityLimit: int(cfg.MetadataCardinalityLimit),
			logCountAttribute:        cfg.LogCountAttribute,
			timezone:                 timezone,
			telemetryBuilder:         telemetryBuilder,
			includeFields:            cfg.IncludeFields,
			shards:                   make(map[attribute.Set]*aggregatorShard),
		}
	}

	return &logDedupProcessor{
		emitInterval: cfg.Interval,
		aggregator:   agg,
		remover:      newFieldRemover(cfg.ExcludeFields),
		nextConsumer: nextConsumer,
		logger:       settings.Logger,
	}, nil
}

// Start starts the processor.
func (p *logDedupProcessor) Start(ctx context.Context, _ component.Host) error {
	ctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel

	p.wg.Add(1)
	go p.handleExportInterval(ctx)

	return nil
}

// Capabilities returns the consumer's capabilities.
func (*logDedupProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// Shutdown stops the processor.
func (p *logDedupProcessor) Shutdown(context.Context) error {
	if p.cancel != nil {
		// Call cancel to stop the export interval goroutine and wait for it to finish.
		p.cancel()
		p.wg.Wait()
	}
	return nil
}

// ConsumeLogs processes the logs.
func (p *logDedupProcessor) ConsumeLogs(ctx context.Context, pl plog.Logs) error {
	p.mux.Lock()
	defer p.mux.Unlock()

	var aggregateErr error
	pl.ResourceLogs().RemoveIf(func(rl plog.ResourceLogs) bool {
		resource := rl.Resource()

		rl.ScopeLogs().RemoveIf(func(sl plog.ScopeLogs) bool {
			scope := sl.Scope()
			logs := sl.LogRecords()

			logs.RemoveIf(func(logRecord plog.LogRecord) bool {
				if p.conditions == nil {
					if err := p.aggregateLog(ctx, logRecord, scope, resource); err != nil {
						aggregateErr = err
						return false
					}
					return true
				}

				logCtx := ottllog.NewTransformContextPtr(rl, sl, logRecord)
				defer logCtx.Close()
				logMatch, err := p.conditions.Eval(ctx, logCtx)
				if err != nil {
					p.logger.Error("error matching conditions", zap.Error(err))
					return false
				}
				if !logMatch {
					return false
				}
				if err := p.aggregateLog(ctx, logRecord, scope, resource); err != nil {
					aggregateErr = err
					return false
				}
				return true
			})
			return sl.LogRecords().Len() == 0
		})
		return rl.ScopeLogs().Len() == 0
	})

	// immediately consume any logs that didn't match any conditions
	if pl.LogRecordCount() > 0 {
		err := p.nextConsumer.ConsumeLogs(ctx, pl)
		if err != nil {
			p.logger.Error("failed to consume logs", zap.Error(err))
		}
	}

	return aggregateErr
}

func (p *logDedupProcessor) aggregateLog(ctx context.Context, logRecord plog.LogRecord, scope pcommon.InstrumentationScope, resource pcommon.Resource) error {
	p.remover.RemoveFields(logRecord)
	return p.aggregator.add(ctx, logRecord, scope, resource)
}

// handleExportInterval sends metrics at the configured interval.
func (p *logDedupProcessor) handleExportInterval(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(p.emitInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Export any remaining logs
			p.exportLogs(ctx)
			if err := ctx.Err(); err != context.Canceled {
				p.logger.Error("context error", zap.Error(err))
			}
			return
		case <-ticker.C:
			p.exportLogs(ctx)
		}
	}
}

// exportLogs exports the logs to the next consumer.
func (p *logDedupProcessor) exportLogs(ctx context.Context) {
	p.mux.Lock()
	defer p.mux.Unlock()

	p.aggregator.flush(ctx, p.nextConsumer, p.logger)
}
