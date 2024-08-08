// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logdeduplicationprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/logdeduplicationprocessor"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

// logDedupProcessor is a logDedupProcessor that counts duplicate instances of logs.
type logDedupProcessor struct {
	emitInterval time.Duration
	aggregator   *logAggregator
	remover      *fieldRemover
	consumer     consumer.Logs
	logger       *zap.Logger
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	mux          sync.Mutex
}

func newProcessor(cfg *Config, consumer consumer.Logs, logger *zap.Logger) (*logDedupProcessor, error) {
	// This should not happen due to config validation but we check anyways.
	timezone, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone: %w", err)
	}

	return &logDedupProcessor{
		emitInterval: cfg.Interval,
		aggregator:   newLogAggregator(cfg.LogCountAttribute, timezone),
		remover:      newFieldRemover(cfg.ExcludeFields),
		consumer:     consumer,
		logger:       logger,
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
func (p *logDedupProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// Shutdown stops the processor.
func (p *logDedupProcessor) Shutdown(ctx context.Context) error {
	if p.cancel != nil {
		p.cancel()
	}

	doneChan := make(chan struct{})
	go func() {
		defer close(doneChan)
		p.wg.Wait()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-doneChan:
		return nil
	}
}

// ConsumeLogs processes the logs.
func (p *logDedupProcessor) ConsumeLogs(_ context.Context, pl plog.Logs) error {
	p.mux.Lock()
	defer p.mux.Unlock()

	for i := 0; i < pl.ResourceLogs().Len(); i++ {
		resourceLogs := pl.ResourceLogs().At(i)
		resourceAttrs := resourceLogs.Resource().Attributes()
		resourceKey := pdatautil.MapHash(resourceAttrs)
		for j := 0; j < resourceLogs.ScopeLogs().Len(); j++ {
			scope := resourceLogs.ScopeLogs().At(j)
			logs := scope.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				logRecord := logs.At(k)
				// Remove excluded fields if any
				p.remover.RemoveFields(logRecord)

				// Add the log to the aggregator
				p.aggregator.Add(resourceKey, resourceAttrs, logRecord)
			}
		}
	}

	return nil
}

// handleExportInterval sends metrics at the configured interval.
func (p *logDedupProcessor) handleExportInterval(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(p.emitInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.mux.Lock()

			logs := p.aggregator.Export()
			// Only send logs if we have some
			if logs.LogRecordCount() > 0 {
				err := p.consumer.ConsumeLogs(ctx, logs)
				if err != nil {
					p.logger.Error("failed to consume logs", zap.Error(err))
				}
			}
			p.aggregator.Reset()
			p.mux.Unlock()
		}
	}
}
