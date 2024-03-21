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
	"go.uber.org/multierr"
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

func newLogRecordCarrier(l plog.LogRecord) (*recordCarrier, error) {
	var ret error
	carrier := &recordCarrier{
		record: l,
	}
	if tvalue := carrier.get("sampling.threshold"); len(tvalue) != 0 {
		th, err := sampling.TValueToThreshold(tvalue)
		if err != nil {
			ret = multierr.Append(err, ret)
		} else {
			carrier.parsed.tvalue = tvalue
			carrier.parsed.threshold = th
		}
	}
	if rvalue := carrier.get("sampling.randomness"); len(rvalue) != 0 {
		rnd, err := sampling.RValueToRandomness(rvalue)
		if err != nil {
			ret = multierr.Append(err, ret)
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
	rc.record.Attributes().PutStr("sampling.randomness", rnd.randomness().RValue())
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

				rnd, carrier, err := lsp.sampler.randomnessFromLogRecord(l)
				if err != nil {
					lsp.logger.Error("log sampling", zap.Error(err))
				} else if err := consistencyCheck(rnd, carrier, lsp.commonFields); err != nil {
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

				sampled := threshold.ShouldSample(rnd.randomness())

				if sampled && carrier != nil {
					if err := carrier.updateThreshold(threshold); err != nil {
						lsp.logger.Error("log sampling", zap.Error(err))
					}

					if err := carrier.reserialize(); err != nil {
						lsp.logger.Error("log sampling", zap.Error(err))
					}
				}

				if err := stats.RecordWithTags(
					ctx,
					[]tag.Mutator{tag.Upsert(tagPolicyKey, rnd.policyName()), tag.Upsert(tagSampledKey, strconv.FormatBool(sampled))},
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
			if th, err := sampling.ProbabilityToThresholdWithPrecision(localPriority.Double()/100.0, lsp.precision); err == nil {
				// The record has supplied a valid alternative sampling proabability
				return th
			}

		}
	}
	return sampling.NeverSampleThreshold
}
