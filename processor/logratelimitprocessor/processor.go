// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logratelimitprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/logratelimitprocessor"

import (
	"context"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/logratelimitprocessor/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

// logratelimitprocessor is an opentelemetry-collector processor that rate limits the log records based on specified rate_limit_fields
type logratelimitprocessor struct {
	conditions       *ottl.ConditionSequence[ottllog.TransformContext]
	keyHelper        *keyHelper
	rateLimiter      *RateLimiter
	nextConsumer     consumer.Logs
	logger           *zap.Logger
	telemetryBuilder *metadata.TelemetryBuilder
}

func newProcessor(cfg *Config, nextConsumer consumer.Logs, settings processor.Settings) (*logratelimitprocessor, error) {
	telemetryBuilder, err := metadata.NewTelemetryBuilder(settings.TelemetrySettings)
	if err != nil {
		return nil, fmt.Errorf("failed to create telemetry builder: %w", err)
	}

	return &logratelimitprocessor{
		keyHelper:        newKeyHelper(cfg.RateLimitFields, settings.Logger),
		rateLimiter:      NewRateLimiter(cfg.AllowedRate, cfg.Interval, settings.Logger),
		nextConsumer:     nextConsumer,
		logger:           settings.Logger,
		telemetryBuilder: telemetryBuilder,
	}, nil
}

// Start starts the processor.
func (p *logratelimitprocessor) Start(ctx context.Context, _ component.Host) error {
	return nil
}

// Capabilities returns the consumer's capabilities.
func (p *logratelimitprocessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// Shutdown stops the processor.
func (p *logratelimitprocessor) Shutdown(_ context.Context) error {
	return nil
}

// ConsumeLogs processes the logs.
func (p *logratelimitprocessor) ConsumeLogs(ctx context.Context, pl plog.Logs) error {
	for i := 0; i < pl.ResourceLogs().Len(); i++ {
		rl := pl.ResourceLogs().At(i)
		resource := rl.Resource()

		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			scope := sl.Scope()
			logs := sl.LogRecords()

			logs.RemoveIf(func(logRecord plog.LogRecord) bool {
				if p.conditions == nil {
					return !p.isLogAllowed(ctx, logRecord, resource)
				}

				logCtx := ottllog.NewTransformContext(logRecord, scope, resource, sl, rl)
				logMatch, err := p.conditions.Eval(ctx, logCtx)
				if err != nil {
					p.logger.Error("error while matching conditions", zap.Error(err))
					return false
				}
				if logMatch {
					return !p.isLogAllowed(ctx, logRecord, resource)
				}
				return false
			})
		}
	}

	// immediately consume logs which are allowed (not rate limited)
	if pl.LogRecordCount() > 0 {
		err := p.nextConsumer.ConsumeLogs(ctx, pl)
		if err != nil {
			p.logger.Error("failed to consume logs", zap.Error(err))
		}
	}

	return nil
}

// isLogAllowed checks if a log record should be allowed or drop instead wrt configured AllowedRate in configured interval
func (p *logratelimitprocessor) isLogAllowed(ctx context.Context, logRecord plog.LogRecord, resource pcommon.Resource) bool {
	// generate key for log
	key, appendedFieldValsStr := p.keyHelper.GenerateKey(logRecord, resource)

	// check if log is allowed
	isAllowed := p.rateLimiter.IsRequestAllowed(key)
	if !isAllowed {
		// increment dropped log metric
		p.telemetryBuilder.RatelimitProcessorDroppedLogs.Add(
			ctx,
			1,
			metric.WithAttributes(attribute.String("rate_limit_fields", appendedFieldValsStr)),
		)
	}
	return isAllowed
}
