// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor/internal/metadata"
)

type logsProcessor struct {
	sampler dataSampler

	samplingPriority string
	precision        int
	failClosed       bool
	logger           *zap.Logger
	telemetryBuilder *metadata.TelemetryBuilder
}

type recordCarrier struct {
	record plog.LogRecord

	parsed struct {
		tvalue    string
		threshold sampling.Threshold

		rvalue     string
		randomness sampling.Randomness
	}
}

var _ samplingCarrier = &recordCarrier{}

func (rc *recordCarrier) get(key string) string {
	val, ok := rc.record.Attributes().Get(key)
	if !ok || val.Type() != pcommon.ValueTypeStr {
		return ""
	}
	return val.Str()
}

func newLogRecordCarrier(l plog.LogRecord) (samplingCarrier, error) {
	var ret error
	carrier := &recordCarrier{
		record: l,
	}
	if tvalue := carrier.get("sampling.threshold"); len(tvalue) != 0 {
		th, err := sampling.TValueToThreshold(tvalue)
		if err != nil {
			ret = errors.Join(err, ret)
		} else {
			carrier.parsed.tvalue = tvalue
			carrier.parsed.threshold = th
		}
	}
	if rvalue := carrier.get("sampling.randomness"); len(rvalue) != 0 {
		rnd, err := sampling.RValueToRandomness(rvalue)
		if err != nil {
			ret = errors.Join(err, ret)
		} else {
			carrier.parsed.rvalue = rvalue
			carrier.parsed.randomness = rnd
		}
	}
	return carrier, ret
}

func (rc *recordCarrier) threshold() (sampling.Threshold, bool) {
	return rc.parsed.threshold, len(rc.parsed.tvalue) != 0
}

func (rc *recordCarrier) explicitRandomness() (randomnessNamer, bool) {
	if len(rc.parsed.rvalue) == 0 {
		return newMissingRandomnessMethod(), false
	}
	return newSamplingRandomnessMethod(rc.parsed.randomness), true
}

func (rc *recordCarrier) updateThreshold(th sampling.Threshold) error {
	exist, has := rc.threshold()
	if has && sampling.ThresholdLessThan(th, exist) {
		return sampling.ErrInconsistentSampling
	}
	rc.record.Attributes().PutStr("sampling.threshold", th.TValue())
	return nil
}

func (rc *recordCarrier) setExplicitRandomness(rnd randomnessNamer) {
	rc.parsed.randomness = rnd.randomness()
	rc.parsed.rvalue = rnd.randomness().RValue()
	rc.record.Attributes().PutStr("sampling.randomness", rnd.randomness().RValue())
}

func (rc *recordCarrier) clearThreshold() {
	rc.parsed.threshold = sampling.NeverSampleThreshold
	rc.parsed.tvalue = ""
	rc.record.Attributes().Remove("sampling.threshold")
}

func (rc *recordCarrier) reserialize() error {
	return nil
}

func (*neverSampler) randomnessFromLogRecord(logRec plog.LogRecord) (randomnessNamer, samplingCarrier, error) {
	// We return a fake randomness value, since it will not be used.
	// This avoids a consistency check error for missing randomness.
	lrc, err := newLogRecordCarrier(logRec)
	return newSamplingPriorityMethod(sampling.AllProbabilitiesRandomness), lrc, err
}

// randomnessFromLogRecord (hashingSampler) uses a hash function over
// the TraceID or logs attribute source.
func (th *hashingSampler) randomnessFromLogRecord(logRec plog.LogRecord) (randomnessNamer, samplingCarrier, error) {
	rnd := newMissingRandomnessMethod()
	lrc, err := newLogRecordCarrier(logRec)

	if th.logsTraceIDEnabled {
		value := logRec.TraceID()
		if !value.IsEmpty() {
			rnd = newTraceIDHashingMethod(randomnessFromBytes(value[:], th.hashSeed))
		}
	}

	if isMissing(rnd) && th.logsRandomnessSourceAttribute != "" {
		if value, ok := logRec.Attributes().Get(th.logsRandomnessSourceAttribute); ok {
			by := getBytesFromValue(value)
			if len(by) > 0 {
				rnd = newAttributeHashingMethod(
					th.logsRandomnessSourceAttribute,
					randomnessFromBytes(by, th.hashSeed),
				)
			}
		}
	}

	if err != nil {
		// The sampling.randomness or sampling.threshold attributes
		// had a parse error, in this case.
		lrc = nil
	} else if _, hasRnd := lrc.explicitRandomness(); hasRnd {
		// If the log record contains a randomness value, do not update.
		err = ErrRandomnessInUse
		lrc = nil
	} else if _, hasTh := lrc.threshold(); hasTh {
		// If the log record contains a threshold value, do not update.
		err = ErrThresholdInUse
		lrc = nil
	} else if !isMissing(rnd) {
		// When no sampling information is already present and we have
		// calculated new randomness, add it to the record.
		lrc.setExplicitRandomness(rnd)
	}

	return rnd, lrc, err
}

// randomnessFromLogRecord (hashingSampler) uses OTEP 235 semantic
// conventions basing its decision only on the TraceID.
func (ctc *consistentTracestateCommon) randomnessFromLogRecord(logRec plog.LogRecord) (randomnessNamer, samplingCarrier, error) {
	lrc, err := newLogRecordCarrier(logRec)
	rnd := newMissingRandomnessMethod()

	if err != nil {
		// Parse error in sampling.randomness or sampling.threshold
		lrc = nil
	} else if rv, hasRnd := lrc.explicitRandomness(); hasRnd {
		rnd = rv
	} else if tid := logRec.TraceID(); !tid.IsEmpty() {
		rnd = newTraceIDW3CSpecMethod(sampling.TraceIDToRandomness(tid))
	}

	return rnd, lrc, err
}

// newLogsProcessor returns a processor.LogsProcessor that will perform head sampling according to the given
// configuration.
func newLogsProcessor(ctx context.Context, set processor.Settings, nextConsumer consumer.Logs, cfg *Config) (processor.Logs, error) {
	telemetryBuilder, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	lsp := &logsProcessor{
		sampler:          makeSampler(cfg, true),
		samplingPriority: cfg.SamplingPriority,
		precision:        cfg.SamplingPrecision,
		failClosed:       cfg.FailClosed,
		logger:           set.Logger,
		telemetryBuilder: telemetryBuilder,
	}

	return processorhelper.NewLogs(
		ctx,
		set,
		cfg,
		nextConsumer,
		lsp.processLogs,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}))
}

func (lsp *logsProcessor) processLogs(ctx context.Context, logsData plog.Logs) (plog.Logs, error) {
	logsData.ResourceLogs().RemoveIf(func(rl plog.ResourceLogs) bool {
		rl.ScopeLogs().RemoveIf(func(ill plog.ScopeLogs) bool {
			ill.LogRecords().RemoveIf(func(l plog.LogRecord) bool {
				return !commonShouldSampleLogic(
					ctx,
					l,
					lsp.sampler,
					lsp.failClosed,
					lsp.sampler.randomnessFromLogRecord,
					lsp.priorityFunc,
					"logs sampler",
					lsp.logger,
					lsp.telemetryBuilder.ProcessorProbabilisticSamplerCountLogsSampled,
				)
			})
			// Filter out empty ScopeLogs
			return ill.LogRecords().Len() == 0
		})
		// Filter out empty ResourceLogs
		return rl.ScopeLogs().Len() == 0
	})
	if logsData.ResourceLogs().Len() == 0 {
		return logsData, processorhelper.ErrSkipProcessingData
	}
	return logsData, nil
}

func (lsp *logsProcessor) priorityFunc(logRec plog.LogRecord, rnd randomnessNamer, threshold sampling.Threshold) (randomnessNamer, sampling.Threshold) {
	// Note: in logs, unlike traces, the sampling priority
	// attribute is interpreted as a request to be sampled.
	if lsp.samplingPriority != "" {
		priorityThreshold, has := lsp.logRecordToPriorityThreshold(logRec)
		if has {
			threshold = priorityThreshold
			rnd = newSamplingPriorityMethod(rnd.randomness()) // override policy name
		}
	}
	return rnd, threshold
}

func (lsp *logsProcessor) logRecordToPriorityThreshold(logRec plog.LogRecord) (sampling.Threshold, bool) {
	if localPriority, ok := logRec.Attributes().Get(lsp.samplingPriority); ok {
		// Potentially raise the sampling probability to minProb
		minProb := 0.0
		switch localPriority.Type() {
		case pcommon.ValueTypeDouble:
			minProb = localPriority.Double() / 100.0
		case pcommon.ValueTypeInt:
			minProb = float64(localPriority.Int()) / 100.0
		}
		if minProb != 0 {
			if th, err := sampling.ProbabilityToThresholdWithPrecision(minProb, lsp.precision); err == nil {
				// The record has supplied a valid alternative sampling probability
				return th, true
			}
		}
	}
	return sampling.NeverSampleThreshold, false
}
