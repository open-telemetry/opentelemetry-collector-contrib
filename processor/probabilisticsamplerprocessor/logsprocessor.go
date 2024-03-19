// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"

import (
	"context"
	"strconv"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
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
	precision        int
	commonFields
}

type recordCarrier struct {
	record plog.LogRecord
	policy policy
}

var _ samplingCarrier = &recordCarrier{}

func newLogRecordCarrier(l plog.LogRecord, policy policy) *recordCarrier {
	return &recordCarrier{
		record: l,
		policy: policy,
	}
}

func (rc *recordCarrier) getPolicy() policy {
	return rc.policy
}

func (rc *recordCarrier) threshold() (th sampling.Threshold, _ bool) {
	th = sampling.AlwaysSampleThreshold
	val, ok := rc.record.Attributes().Get("sampling.threshold")
	if !ok {
		return th, false
	}
	if val.Type() != pcommon.ValueTypeStr {
		return th, false
	}
	th, err := sampling.TValueToThreshold(val.Str())
	return th, err != nil
}

func (rc *recordCarrier) explicitRandomness() (rnd sampling.Randomness, _ bool) {
	rnd = sampling.AllProbabilitiesRandomness
	val, ok := rc.record.Attributes().Get("sampling.randomness")
	if !ok {
		return rnd, false
	}
	if val.Type() != pcommon.ValueTypeStr {
		return rnd, false
	}
	rnd, err := sampling.RValueToRandomness(val.Str())
	return rnd, err != nil
}

func (rc *recordCarrier) updateThreshold(th sampling.Threshold, tv string) error {
	if tv == "" {
		rc.clearThreshold()
		return nil
	}
	exist, has := rc.threshold()
	if has && sampling.ThresholdLessThan(th, exist) {
		return sampling.ErrInconsistentSampling
	}

	rc.record.Attributes().PutStr("sampling.threshold", tv)
	return nil
}

func (rc *recordCarrier) setExplicitRandomness(rnd sampling.Randomness) {
	rc.record.Attributes().PutStr("sampling.randomness", rnd.RValue())
}

func (rc *recordCarrier) clearThreshold() {
	rc.record.Attributes().Remove("sampling.threshold")
}

func (rc *recordCarrier) reserialize() error {
	return nil
}

// newLogsProcessor returns a processor.LogsProcessor that will perform head sampling according to the given
// configuration.
func newLogsProcessor(ctx context.Context, set processor.CreateSettings, nextConsumer consumer.Logs, cfg *Config) (processor.Logs, error) {
	common := commonFields{
		strict: cfg.StrictRandomness,
		logger: set.Logger,
	}

	lsp := &logsProcessor{
		samplingPriority: cfg.SamplingPriority,
		precision:        cfg.SamplingPrecision,
		commonFields:     common,
	}

	lsp.sampler = makeSampler(cfg, common, true)

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

				randomness, carrier, err := lsp.sampler.randomnessFromLogRecord(l)
				if err != nil {
					lsp.logger.Error("log sampling", zap.Error(err))
					return true
				}

				policy := carrier.getPolicy()

				if err := consistencyCheck(randomness, carrier, lsp.commonFields); err != nil {
					// the consistency check resets the arriving
					// threshold if it is inconsistent with the
					// sampling decision.
					lsp.logger.Error("log sampling", zap.Error(err))
				}
				threshold := lsp.sampler.decide(carrier)

				// Note: in logs, unlike traces, the sampling priority
				// attribute is interpreted as a request to be sampled.
				if lsp.samplingPriority != "" {
					priorityThreshold := lsp.logRecordToPriorityThreshold(l)

					if sampling.ThresholdLessThan(priorityThreshold, threshold) {
						// Note: there is no effort to install
						// "sampling_priority" as the policy name,
						// which the traces processor will do.
						threshold = priorityThreshold
					}
				}

				sampled := threshold.ShouldSample(randomness)

				if err := stats.RecordWithTags(
					ctx,
					[]tag.Mutator{tag.Upsert(tagPolicyKey, string(policy)), tag.Upsert(tagSampledKey, strconv.FormatBool(sampled))},
					statCountLogsSampled.M(int64(1)),
				); err != nil {
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

func getBytesFromValue(value pcommon.Value) []byte {
	if value.Type() == pcommon.ValueTypeBytes {
		return value.Bytes().AsRaw()
	}
	return []byte(value.AsString())
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
			if th, err := sampling.ProbabilityToThresholdWithPrecision(localPriority.Double()/100.0, lsp.precision); err != nil {
				// The record has supplied a valid alternative sampling proabability
				return th
			}

		}
	}
	return sampling.NeverSampleThreshold
}
