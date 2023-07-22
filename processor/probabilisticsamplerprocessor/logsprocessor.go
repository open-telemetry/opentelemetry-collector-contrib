// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"

import (
	"context"
	"strconv"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"
)

type logSamplerProcessor struct {
	scaledSamplingRate uint32
	hashSeed           uint32
	traceIDEnabled     bool
	samplingSource     string
	samplingPriority   string
	logger             *zap.Logger
}

// newLogsProcessor returns a processor.LogsProcessor that will perform head sampling according to the given
// configuration.
func newLogsProcessor(ctx context.Context, set processor.CreateSettings, nextConsumer consumer.Logs, cfg *Config) (processor.Logs, error) {

	lsp := &logSamplerProcessor{
		scaledSamplingRate: uint32(cfg.SamplingPercentage * percentageScaleFactor),
		hashSeed:           cfg.HashSeed,
		traceIDEnabled:     cfg.AttributeSource == traceIDAttributeSource,
		samplingPriority:   cfg.SamplingPriority,
		samplingSource:     cfg.FromAttribute,
		logger:             set.Logger,
	}

	return processorhelper.NewLogsProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		lsp.processLogs,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}))
}

func (lsp *logSamplerProcessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	ld.ResourceLogs().RemoveIf(func(rl plog.ResourceLogs) bool {
		rl.ScopeLogs().RemoveIf(func(ill plog.ScopeLogs) bool {
			ill.LogRecords().RemoveIf(func(l plog.LogRecord) bool {

				tagPolicyValue := "always_sampling"
				// pick the sampling source.
				var lidBytes []byte
				if lsp.traceIDEnabled && !l.TraceID().IsEmpty() {
					value := l.TraceID()
					tagPolicyValue = "trace_id_hash"
					lidBytes = value[:]
				}
				if lidBytes == nil && lsp.samplingSource != "" {
					if value, ok := l.Attributes().Get(lsp.samplingSource); ok {
						tagPolicyValue = lsp.samplingSource

						switch value.Type() {
						case pcommon.ValueTypeStr:
							lidBytes = []byte(value.Str())
						case pcommon.ValueTypeBytes:
							lidBytes = value.Bytes().AsRaw()
						default:
							lsp.logger.Warn("incompatible log record attribute, only String or Bytes supported; skipping log record",
								zap.String("log_record_attribute", lsp.samplingSource), zap.Stringer("attribute_type", value.Type()))
						}

					}
				}
				priority := lsp.scaledSamplingRate
				if lsp.samplingPriority != "" {
					if localPriority, ok := l.Attributes().Get(lsp.samplingPriority); ok {
						switch localPriority.Type() {
						case pcommon.ValueTypeDouble:
							priority = uint32(localPriority.Double() * percentageScaleFactor)
						case pcommon.ValueTypeInt:
							priority = uint32(float64(localPriority.Int()) * percentageScaleFactor)
						}
					}
				}

				sampled := computeHash(lidBytes, lsp.hashSeed)&bitMaskHashBuckets < priority
				var err error = stats.RecordWithTags(
					ctx,
					[]tag.Mutator{tag.Upsert(tagPolicyKey, tagPolicyValue), tag.Upsert(tagSampledKey, strconv.FormatBool(sampled))},
					statCountLogsSampled.M(int64(1)),
				)
				if err != nil {
					lsp.logger.Error(err.Error())
				}

				return !sampled
			})
			// Filter out empty ScopeLogs
			return ill.LogRecords().Len() == 0
		})
		// Filter out empty ResourceLogs
		return rl.ScopeLogs().Len() == 0
	})
	if ld.ResourceLogs().Len() == 0 {
		return ld, processorhelper.ErrSkipProcessingData
	}
	return ld, nil
}
