// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"
)

type logsProcessor struct {
	sampler dataSampler

	samplingPriority string
	failClosed       bool
	logger           *zap.Logger
}

type recordCarrier struct {
	record plog.LogRecord
}

var _ samplingCarrier = &recordCarrier{}

func newLogRecordCarrier(l plog.LogRecord) (samplingCarrier, error) {
	carrier := &recordCarrier{
		record: l,
	}
	return carrier, nil
}

func (*neverSampler) randomnessFromLogRecord(_ plog.LogRecord) (randomnessNamer, samplingCarrier, error) {
	return newMissingRandomnessMethod(), nil, nil
}

// randomnessFromLogRecord (hashingSampler) uses a hash function over
// the TraceID
func (th *hashingSampler) randomnessFromLogRecord(l plog.LogRecord) (randomnessNamer, samplingCarrier, error) {
	rnd := newMissingRandomnessMethod()
	lrc, err := newLogRecordCarrier(l)

	if th.logsTraceIDEnabled {
		value := l.TraceID()
		// Note: this admits empty TraceIDs.
		rnd = newTraceIDHashingMethod(randomnessFromBytes(value[:], th.hashSeed))
	}

	if isMissing(rnd) && th.logsRandomnessSourceAttribute != "" {
		if value, ok := l.Attributes().Get(th.logsRandomnessSourceAttribute); ok {
			// Note: this admits zero-byte values.
			rnd = newAttributeHashingMethod(
				th.logsRandomnessSourceAttribute,
				randomnessFromBytes(getBytesFromValue(value), th.hashSeed),
			)
		}
	}

	if err != nil {
		// The sampling.randomness or sampling.threshold attributes
		// had a parse error, in this case.
		lrc = nil
	}

	return rnd, lrc, err
}

// newLogsProcessor returns a processor.LogsProcessor that will perform head sampling according to the given
// configuration.
func newLogsProcessor(ctx context.Context, set processor.CreateSettings, nextConsumer consumer.Logs, cfg *Config) (processor.Logs, error) {
	lsp := &logsProcessor{
		sampler:          makeSampler(cfg),
		samplingPriority: cfg.SamplingPriority,
		failClosed:       cfg.FailClosed,
		logger:           set.Logger,
	}

	return processorhelper.NewLogsProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		lsp.processLogs,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}))
}

func (lsp *logsProcessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	ld.ResourceLogs().RemoveIf(func(rl plog.ResourceLogs) bool {
		rl.ScopeLogs().RemoveIf(func(ill plog.ScopeLogs) bool {
			ill.LogRecords().RemoveIf(func(l plog.LogRecord) bool {
				return commonSamplingLogic(
					ctx,
					l,
					lsp.sampler,
					lsp.failClosed,
					lsp.sampler.randomnessFromLogRecord,
					lsp.priorityFunc,
					"logs sampler",
					lsp.logger,
				)
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

func (lsp *logsProcessor) priorityFunc(l plog.LogRecord, rnd randomnessNamer, threshold sampling.Threshold) (randomnessNamer, sampling.Threshold) {
	// Note: in logs, unlike traces, the sampling priority
	// attribute is interpreted as a request to be sampled.
	if lsp.samplingPriority != "" {
		priorityThreshold := lsp.logRecordToPriorityThreshold(l)

		if priorityThreshold == sampling.NeverSampleThreshold {
			threshold = priorityThreshold
			rnd = newSamplingPriorityMethod(rnd.randomness()) // override policy name
		} else if sampling.ThresholdLessThan(priorityThreshold, threshold) {
			threshold = priorityThreshold
			rnd = newSamplingPriorityMethod(rnd.randomness()) // override policy name
		}
	}
	return rnd, threshold
}

func (lsp *logsProcessor) logRecordToPriorityThreshold(l plog.LogRecord) sampling.Threshold {
	if localPriority, ok := l.Attributes().Get(lsp.samplingPriority); ok {
		// Potentially raise the sampling probability to minProb
		minProb := 0.0
		switch localPriority.Type() {
		case pcommon.ValueTypeDouble:
			minProb = localPriority.Double() / 100.0
		case pcommon.ValueTypeInt:
			minProb = float64(localPriority.Int()) / 100.0
		}
		if minProb != 0 {
			if th, err := sampling.ProbabilityToThresholdWithPrecision(minProb, defaultPrecision); err == nil {
				// The record has supplied a valid alternative sampling proabability
				return th
			}

		}
	}
	return sampling.NeverSampleThreshold
}
