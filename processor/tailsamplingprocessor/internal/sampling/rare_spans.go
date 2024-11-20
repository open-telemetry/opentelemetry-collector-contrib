// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/cms"
)

const (
	spansPerSecondSampledDefaultLimit   = 1000
	spansPerSecondProcessedDefaultLimit = 100000
	spainUniqIdBufferSize               = 20 * 1024
)

type RareSpansSampler struct {
	// cntMinSketch implementation of the count-min sketch used to estimate the
	// frequency of spans.
	cntMinSketch cms.CountMinSketch
	// maxSpanFreq span frequency value below which span should be sampled.
	maxSpanFreq uint32
	// spsSampledLimit maximum number of sampled spans per second.
	spsSampledLimit int64
	// spsSampled the number of already sampled spans for the current second.
	spsSampled int64
	// spsProcessingLimit maximum number of spans that can be processed per
	// second.
	spsProcessingLimit int64
	// spsPrecessed the number of already processed spans for the current
	// second.
	spsPrecessed int64
	// currentSecond current second in unix time stamp format. Is used for
	// counting the number of spans already sampled and processed.
	currentSecond int64
	// idBuff buffer for creating and storing span CMS key. Used for optimization
	// purposes to avoid unnecessary copying and resource allocation at strings
	// concatenation.
	idBuff []byte

	tmProvider TimeProvider

	logger *zap.Logger
}

// ShouldBeSampled returns a decision about whether the span should be sampled
// based on its name and the service name.
func (r *RareSpansSampler) ShouldBeSampled(svcName, operationName string) bool {
	r.idBuff = r.idBuff[:len(svcName)]
	copy(r.idBuff[:], svcName)
	copy(r.idBuff[len(svcName):len(svcName)+1], []byte{':'})
	copy(r.idBuff[len(svcName)+1:len(svcName)+1+len(operationName)], operationName)
	r.idBuff = r.idBuff[:len(svcName)+1+len(operationName)]

	cnt := r.cntMinSketch.InsertWithCount(r.idBuff)
	if cnt <= r.maxSpanFreq {
		return true
	}
	return false
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (r *RareSpansSampler) Evaluate(_ context.Context, _ pcommon.TraceID, trace *TraceData) (Decision, error) {
	var (
		shouldBeSampled bool
		decision        = NotSampled
	)

	currentSecond := r.tmProvider.getCurSecond()
	if r.currentSecond != currentSecond {
		r.currentSecond = currentSecond
		r.spsSampled = 0
		r.spsPrecessed = 0
	}

	sps := trace.SpanCount.Load()
	possibleSpsSampled := r.spsSampled + sps
	possibleProcessed := r.spsPrecessed + sps
	if possibleSpsSampled > r.spsSampledLimit || possibleProcessed > r.spsProcessingLimit {
		return decision, nil
	}

	trace.Lock()
	defer trace.Unlock()

	for i := 0; i < trace.ReceivedBatches.ResourceSpans().Len(); i++ {
		rs := trace.ReceivedBatches.ResourceSpans().At(i)
		svcName, _ := rs.Resource().Attributes().Get(semconv.AttributeServiceName)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			rss := rs.ScopeSpans().At(j).Spans()
			for k := 0; k < rss.Len(); k++ {
				keyLen := len(svcName.Str()) + 1 + len(rss.At(k).Name())
				if keyLen > spainUniqIdBufferSize {
					r.logger.Error("too long span key", zap.Int("key_len", keyLen))
					continue
				}
				operationName := rss.At(k).Name()
				if r.ShouldBeSampled(svcName.Str(), operationName) {
					shouldBeSampled = true
				}
			}
		}
	}

	sps = trace.SpanCount.Load()
	if shouldBeSampled {
		decision = Sampled
		r.spsSampled += sps
	}

	r.spsPrecessed += sps

	return decision, nil
}

// NewRareSpansSamplerWithCms creates a policy evaluator that samples traces
// based on spans frequency. CMS (Count-min sketch) is used to estimate the
// frequency of occurrence of a `span key`, where the `span key` consists of the
// span service name and the span name (operation name).
func NewRareSpansSamplerWithCms(
	rareSpansFreq uint32,
	spsSampledLimit int64,
	spsProcessedLimit int64,
	tmProvider TimeProvider,
	cms cms.CountMinSketch,
	settings component.TelemetrySettings,
) *RareSpansSampler {

	return &RareSpansSampler{
		cntMinSketch:       cms,
		maxSpanFreq:        rareSpansFreq,
		spsSampledLimit:    spsSampledLimit,
		spsProcessingLimit: spsProcessedLimit,
		currentSecond:      tmProvider.getCurSecond(),
		idBuff:             make([]byte, 0, spainUniqIdBufferSize),
		tmProvider:         tmProvider,
		logger:             settings.Logger,
	}
}

// NewRareSpansSampler creates a policy evaluator that samples traces based
// on spans frequency. Unlike NewRareSpansSamplerWithCms, this function takes
// explicit CMS parameters as input.
func NewRareSpansSampler(
	cmsCfg cms.CountMinSketchCfg,
	bucketsCfg cms.BucketsCfg,
	spanFreq uint32,
	spsSampledLimit int64,
	spsProcessedLimit int64,
	tmProvider TimeProvider,
	settings component.TelemetrySettings,
) (*RareSpansSampler, error) {

	if spsSampledLimit == 0 {
		spsSampledLimit = spansPerSecondSampledDefaultLimit
	}

	if spsProcessedLimit == 0 {
		spsProcessedLimit = spansPerSecondProcessedDefaultLimit
	}

	slidingCms, err := cms.NewSlidingCMSWithStartPoint(bucketsCfg, cmsCfg, time.Now())
	if err != nil {
		return nil, err
	}

	return NewRareSpansSamplerWithCms(
		spanFreq,
		spsSampledLimit,
		spsProcessedLimit,
		tmProvider,
		slidingCms,
		settings,
	), nil
}
