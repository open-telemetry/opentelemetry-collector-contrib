// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package throttlingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/throttlingprocessor"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/throttlingprocessor/internal/metadata"
)

// throttlingProcessor is a processor that throttles logs over a certain volume.
type throttlingProcessor struct {
	interval         time.Duration
	threshold        int64
	keyExpression    *ottl.ValueExpression[ottllog.TransformContext]
	conditions       *ottl.ConditionSequence[ottllog.TransformContext]
	buckets          map[string]int64
	telemetryBuilder *metadata.TelemetryBuilder
	nextConsumer     consumer.Logs
	logger           *zap.Logger
	cancel           context.CancelFunc
	mux              sync.Mutex
}

func newProcessor(cfg *Config, nextConsumer consumer.Logs, settings processor.Settings) (*throttlingProcessor, error) {
	telemetryBuilder, err := metadata.NewTelemetryBuilder(settings.TelemetrySettings)
	if err != nil {
		return nil, fmt.Errorf("failed to create telemetry builder: %w", err)
	}
	parser, err := ottllog.NewParser(
		ottlfuncs.StandardConverters[ottllog.TransformContext](),
		settings.TelemetrySettings,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating parser: %w", err)
	}
	keyExpression, err := parser.ParseValueExpression(cfg.KeyExpression)
	//keyExpression, err := parser.ParseStatement(cfg.KeyExpression)
	if err != nil {
		return nil, fmt.Errorf("error parsing keyExpression: %w", err)
	}
	var conditions *ottl.ConditionSequence[ottllog.TransformContext]
	if len(cfg.Conditions) != 0 {
		conditions, err = filterottl.NewBoolExprForLog(
			cfg.Conditions,
			filterottl.StandardLogFuncs(),
			ottl.PropagateError,
			settings.TelemetrySettings,
		)
		if err != nil {
			return nil, fmt.Errorf("invalid condition: %w", err)
		}
	}
	return &throttlingProcessor{
		interval:         cfg.Interval,
		threshold:        cfg.Threshold,
		keyExpression:    keyExpression,
		buckets:          make(map[string]int64),
		conditions:       conditions,
		nextConsumer:     nextConsumer,
		logger:           settings.Logger,
		telemetryBuilder: telemetryBuilder,
	}, nil
}

// Start starts the processor.
func (p *throttlingProcessor) Start(ctx context.Context, _ component.Host) error {
	ctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	go p.handleThrottlingResetInterval(ctx)

	return nil
}

// Capabilities returns the consumer's capabilities.
func (*throttlingProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// Shutdown stops the processor.
func (p *throttlingProcessor) Shutdown(context.Context) error {
	if p.cancel != nil {
		// Call cancel to stop the export interval goroutine and wait for it to finish.
		p.cancel()
	}
	return nil
}

// ConsumeLogs processes the logs.
func (p *throttlingProcessor) ConsumeLogs(ctx context.Context, pl plog.Logs) error {
	p.mux.Lock()
	defer p.mux.Unlock()
	for i := 0; i < pl.ResourceLogs().Len(); i++ {
		rl := pl.ResourceLogs().At(i)
		resource := rl.Resource()
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			scope := sl.Scope()
			logs := sl.LogRecords()
			logs.RemoveIf(func(logRecord plog.LogRecord) bool {
				logCtx := ottllog.NewTransformContext(logRecord, scope, resource, sl, rl)
				if p.conditions != nil {
					logMatch, err := p.conditions.Eval(ctx, logCtx)
					if err != nil {
						p.logger.Error("error matching conditions", zap.Error(err))
						return false
					}
					if !logMatch {
						return false
					}
				}
				result, err := p.keyExpression.Eval(ctx, logCtx)
				if err != nil {
					p.logger.Error("failed to execute key_expression", zap.Error(err))
					return false // do not drop the log record, error in key expression
				}
				bucket, ok := result.(string)
				if !ok {
					p.logger.Error("key_expression must produce a string")
				}
				if _, ok := p.buckets[bucket]; !ok {
					p.buckets[bucket] = 1
				} else {
					p.buckets[bucket]++
				}
				if p.buckets[bucket] > p.threshold {
					return true // drop the log record, over threshold for interval
				}
				return false
			})
		}
	}

	if pl.LogRecordCount() > 0 {
		err := p.nextConsumer.ConsumeLogs(ctx, pl)
		if err != nil {
			p.logger.Error("failed to consume logs", zap.Error(err))
		}
	}

	return nil
}

// handleThrottlingResetInterval reset bucket counts at the configured interval.
func (p *throttlingProcessor) handleThrottlingResetInterval(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.buckets = make(map[string]int64)
		}
	}
}
