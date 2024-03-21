// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/idutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

// defaultHashSeed is used throughout to ensure that the HashSeed is real
// and does not fall back to proportional-mode sampling due to HashSeed == 0.
const defaultHashSeed = 4312

func TestHashBucketsLog2(t *testing.T) {
	require.Equal(t, numHashBuckets, 1<<numHashBucketsLg2)
}

func TestNewTracesProcessor(t *testing.T) {
	tests := []struct {
		name         string
		nextConsumer consumer.Traces
		cfg          *Config
		wantErr      bool
	}{
		{
			name:         "happy_path_default",
			nextConsumer: consumertest.NewNop(),
			cfg: &Config{
				SamplingPercentage: 15.5,
			},
		},
		{
			name:         "happy_path_hash_seed",
			nextConsumer: consumertest.NewNop(),
			cfg: &Config{
				SamplingPercentage: 13.33,
				HashSeed:           defaultHashSeed,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), tt.cfg, tt.nextConsumer)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
			}
		})
	}
}

// Test_tracesamplerprocessor_SamplingPercentageRange checks for different sampling rates and ensures
// that they are within acceptable deltas.
func Test_tracesamplerprocessor_SamplingPercentageRange(t *testing.T) {
	tests := []struct {
		name              string
		cfg               *Config
		numBatches        int
		numTracesPerBatch int
		acceptableDelta   float64
	}{
		{
			name: "random_sampling_tiny",
			cfg: &Config{
				SamplingPercentage: 0.03,
			},
			numBatches:        1e5,
			numTracesPerBatch: 2,
			acceptableDelta:   0.02,
		},
		{
			name: "random_sampling_small",
			cfg: &Config{
				SamplingPercentage: 5,
			},
			numBatches:        1e6,
			numTracesPerBatch: 2,
			acceptableDelta:   0.1,
		},
		{
			name: "random_sampling_medium",
			cfg: &Config{
				SamplingPercentage: 50.0,
			},
			numBatches:        1e5,
			numTracesPerBatch: 4,
			acceptableDelta:   0.2,
		},
		{
			name: "random_sampling_high",
			cfg: &Config{
				SamplingPercentage: 90.0,
			},
			numBatches:        1e5,
			numTracesPerBatch: 1,
			acceptableDelta:   0.2,
		},
		{
			name: "random_sampling_all",
			cfg: &Config{
				SamplingPercentage: 100.0,
			},
			numBatches:        1e5,
			numTracesPerBatch: 1,
			acceptableDelta:   0.0,
		},
	}
	const testSvcName = "test-svc"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.HashSeed = defaultHashSeed

			sink := new(consumertest.TracesSink)
			tsp, err := newTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), tt.cfg, sink)
			if err != nil {
				t.Errorf("error when creating traceSamplerProcessor: %v", err)
				return
			}
			for _, td := range genRandomTestData(tt.numBatches, tt.numTracesPerBatch, testSvcName, 1) {
				assert.NoError(t, tsp.ConsumeTraces(context.Background(), td))
			}
			_, sampled := assertSampledData(t, sink.AllTraces(), testSvcName)
			actualPercentageSamplingPercentage := float32(sampled) / float32(tt.numBatches*tt.numTracesPerBatch) * 100.0
			delta := math.Abs(float64(actualPercentageSamplingPercentage - tt.cfg.SamplingPercentage))
			if delta > tt.acceptableDelta {
				t.Errorf(
					"got %f percentage sampling rate, want %f (allowed delta is %f but got %f)",
					actualPercentageSamplingPercentage,
					tt.cfg.SamplingPercentage,
					tt.acceptableDelta,
					delta,
				)
			}
		})
	}
}

// Test_tracesamplerprocessor_SamplingPercentageRange_MultipleResourceSpans checks for number of spans sent to xt consumer. This is to avoid duplicate spans
func Test_tracesamplerprocessor_SamplingPercentageRange_MultipleResourceSpans(t *testing.T) {
	tests := []struct {
		name                 string
		cfg                  *Config
		numBatches           int
		numTracesPerBatch    int
		acceptableDelta      float64
		resourceSpanPerTrace int
	}{
		{
			name: "single_batch_single_trace_two_resource_spans",
			cfg: &Config{
				SamplingPercentage: 100.0,
			},
			numBatches:           1,
			numTracesPerBatch:    1,
			acceptableDelta:      0.0,
			resourceSpanPerTrace: 2,
		},
		{
			name: "single_batch_two_traces_two_resource_spans",
			cfg: &Config{
				SamplingPercentage: 100.0,
			},
			numBatches:           1,
			numTracesPerBatch:    2,
			acceptableDelta:      0.0,
			resourceSpanPerTrace: 2,
		},
	}
	const testSvcName = "test-svc"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.HashSeed = defaultHashSeed

			sink := new(consumertest.TracesSink)
			tsp, err := newTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), tt.cfg, sink)
			if err != nil {
				t.Errorf("error when creating traceSamplerProcessor: %v", err)
				return
			}

			for _, td := range genRandomTestData(tt.numBatches, tt.numTracesPerBatch, testSvcName, tt.resourceSpanPerTrace) {
				assert.NoError(t, tsp.ConsumeTraces(context.Background(), td))
				assert.Equal(t, tt.resourceSpanPerTrace*tt.numTracesPerBatch, sink.SpanCount())
				sink.Reset()
			}

		})
	}
}

// Test_tracesamplerprocessor_SpanSamplingPriority checks if handling of "sampling.priority" is correct.
func Test_tracesamplerprocessor_SpanSamplingPriority(t *testing.T) {
	singleSpanWithAttrib := func(key string, attribValue pcommon.Value) ptrace.Traces {
		traces := ptrace.NewTraces()
		initSpanWithAttribute(key, attribValue, traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty())
		return traces
	}
	tests := []struct {
		name    string
		cfg     *Config
		td      ptrace.Traces
		sampled bool
	}{
		{
			name: "must_sample",
			cfg: &Config{
				SamplingPercentage: 0.0,
			},
			td: singleSpanWithAttrib(
				"sampling.priority",
				pcommon.NewValueInt(2)),
			sampled: true,
		},
		{
			name: "must_sample_double",
			cfg: &Config{
				SamplingPercentage: 0.0,
			},
			td: singleSpanWithAttrib(
				"sampling.priority",
				pcommon.NewValueDouble(1)),
			sampled: true,
		},
		{
			name: "must_sample_string",
			cfg: &Config{
				SamplingPercentage: 0.0,
			},
			td: singleSpanWithAttrib(
				"sampling.priority",
				pcommon.NewValueStr("1")),
			sampled: true,
		},
		{
			name: "must_not_sample",
			cfg: &Config{
				SamplingPercentage: 100.0,
			},
			td: singleSpanWithAttrib(
				"sampling.priority",
				pcommon.NewValueInt(0)),
		},
		{
			name: "must_not_sample_double",
			cfg: &Config{
				SamplingPercentage: 100.0,
			},
			td: singleSpanWithAttrib(
				"sampling.priority",
				pcommon.NewValueDouble(0)),
		},
		{
			name: "must_not_sample_string",
			cfg: &Config{
				SamplingPercentage: 100.0,
			},
			td: singleSpanWithAttrib(
				"sampling.priority",
				pcommon.NewValueStr("0")),
		},
		{
			name: "defer_sample_expect_not_sampled",
			cfg: &Config{
				SamplingPercentage: 0.0,
			},
			td: singleSpanWithAttrib(
				"no.sampling.priority",
				pcommon.NewValueInt(2)),
		},
		{
			name: "defer_sample_expect_sampled",
			cfg: &Config{
				SamplingPercentage: 100.0,
			},
			td: singleSpanWithAttrib(
				"no.sampling.priority",
				pcommon.NewValueInt(2)),
			sampled: true,
		},
	}
	for _, mode := range AllModes {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				sink := new(consumertest.TracesSink)
				cfg := &Config{}
				if tt.cfg != nil {
					*cfg = *tt.cfg
				}
				cfg.SamplerMode = mode
				cfg.HashSeed = defaultHashSeed
				tsp, err := newTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, sink)
				require.NoError(t, err)

				err = tsp.ConsumeTraces(context.Background(), tt.td)
				require.NoError(t, err)

				sampledData := sink.AllTraces()
				if tt.sampled {
					require.Equal(t, 1, len(sampledData))
					assert.Equal(t, 1, sink.SpanCount())
				} else {
					require.Equal(t, 0, len(sampledData))
					assert.Equal(t, 0, sink.SpanCount())
				}
			})
		}
	}
}

// Test_parseSpanSamplingPriority ensures that the function parsing the attributes is taking "sampling.priority"
// attribute correctly.
func Test_parseSpanSamplingPriority(t *testing.T) {
	tests := []struct {
		name string
		span ptrace.Span
		want samplingPriority
	}{
		{
			name: "nil_span",
			span: ptrace.NewSpan(),
			want: deferDecision,
		},
		{
			name: "nil_attributes",
			span: ptrace.NewSpan(),
			want: deferDecision,
		},
		{
			name: "no_sampling_priority",
			span: getSpanWithAttributes("key", pcommon.NewValueBool(true)),
			want: deferDecision,
		},
		{
			name: "sampling_priority_int_zero",
			span: getSpanWithAttributes("sampling.priority", pcommon.NewValueInt(0)),
			want: doNotSampleSpan,
		},
		{
			name: "sampling_priority_int_gt_zero",
			span: getSpanWithAttributes("sampling.priority", pcommon.NewValueInt(1)),
			want: mustSampleSpan,
		},
		{
			name: "sampling_priority_int_lt_zero",
			span: getSpanWithAttributes("sampling.priority", pcommon.NewValueInt(-1)),
			want: deferDecision,
		},
		{
			name: "sampling_priority_double_zero",
			span: getSpanWithAttributes("sampling.priority", pcommon.NewValueDouble(0)),
			want: doNotSampleSpan,
		},
		{
			name: "sampling_priority_double_gt_zero",
			span: getSpanWithAttributes("sampling.priority", pcommon.NewValueDouble(1)),
			want: mustSampleSpan,
		},
		{
			name: "sampling_priority_double_lt_zero",
			span: getSpanWithAttributes("sampling.priority", pcommon.NewValueDouble(-1)),
			want: deferDecision,
		},
		{
			name: "sampling_priority_string_zero",
			span: getSpanWithAttributes("sampling.priority", pcommon.NewValueStr("0.0")),
			want: doNotSampleSpan,
		},
		{
			name: "sampling_priority_string_gt_zero",
			span: getSpanWithAttributes("sampling.priority", pcommon.NewValueStr("0.5")),
			want: mustSampleSpan,
		},
		{
			name: "sampling_priority_string_lt_zero",
			span: getSpanWithAttributes("sampling.priority", pcommon.NewValueStr("-0.5")),
			want: deferDecision,
		},
		{
			name: "sampling_priority_string_NaN",
			span: getSpanWithAttributes("sampling.priority", pcommon.NewValueStr("NaN")),
			want: deferDecision,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseSpanSamplingPriority(tt.span))
		})
	}
}

// Test_tracesamplerprocessor_TraceState checks if handling of the context
// tracestate is correct with a number o cases that exercise the two
// consistent sampling modes.
func Test_tracesamplerprocessor_TraceState(t *testing.T) {
	// This hard-coded TraceID will sample at 50% and not at 49%.
	// The equivalent randomness is 0x80000000000000.
	var defaultTID = mustParseTID("fefefefefefefefefe80000000000000")

	// improbableTraceID will sample at all supported probabilities.  In
	// hex, the leading 18 digits do not matter, the trailing 14 are all `f`.
	var improbableTraceID = mustParseTID("111111111111111111ffffffffffffff")

	sid := idutils.UInt64ToSpanID(0xfefefefe)
	tests := []struct {
		name  string
		tid   pcommon.TraceID
		cfg   *Config
		ts    string
		key   string
		value pcommon.Value
		log   string
		sf    func(SamplerMode) (sampled bool, adjCount float64, tracestate string)
	}{
		{
			name: "100 percent",
			cfg: &Config{
				SamplingPercentage: 100,
			},
			ts: "",
			sf: func(SamplerMode) (bool, float64, string) {
				return true, 1, "ot=th:0"
			},
		},
		{
			name: "50 percent sampled",
			cfg: &Config{
				SamplingPercentage: 50,
			},
			ts: "",
			sf: func(SamplerMode) (bool, float64, string) { return true, 2, "ot=th:8" },
		},
		{
			name: "25 percent sampled",
			tid:  mustParseTID("ddddddddddddddddddc0000000000000"),
			cfg: &Config{
				SamplingPercentage: 25,
			},
			ts: "",
			sf: func(SamplerMode) (bool, float64, string) { return true, 4, "ot=th:c" },
		},
		{
			name: "25 percent unsampled",
			tid:  mustParseTID("ddddddddddddddddddb0000000000000"),
			cfg: &Config{
				SamplingPercentage: 25,
			},
			ts: "",
			sf: func(SamplerMode) (bool, float64, string) { return false, 0, "" },
		},
		{
			name: "1 percent sampled",
			cfg: &Config{
				SamplingPercentage: 1,
				SamplingPrecision:  0,
			},
			//    99/100 = .fd70a3d70a3d70a3d
			ts: "ot=rv:FD70A3D70A3D71", // note upper case passes through, is not generated
			sf: func(SamplerMode) (bool, float64, string) {
				return true, 1 / 0.01, "ot=rv:FD70A3D70A3D71;th:fd70a3d70a3d71"
			},
		},
		{
			// with precision 4, the 1% probability rounds down and the
			// exact R-value here will sample.  see below, where the
			// opposite is true.
			name: "1 percent sampled with rvalue and precision 4",
			cfg: &Config{
				SamplingPercentage: 1,
				SamplingPrecision:  4,
			},
			ts: "ot=rv:FD70A3D70A3D71",
			sf: func(SamplerMode) (bool, float64, string) {
				return true, 1 / 0.01, "ot=rv:FD70A3D70A3D71;th:fd70a"
			},
		},
		{
			// at precision 3, the 1% probability rounds
			// up to fd71 and so this does not sample.
			// see above, where the opposite is true.
			name: "1 percent sampled with rvalue and precision 3",
			cfg: &Config{
				SamplingPercentage: 1,
				SamplingPrecision:  3,
			},
			ts: "ot=rv:FD70A3D70A3D71",
			sf: func(SamplerMode) (bool, float64, string) {
				return false, 0, ""
			},
		},
		{
			name: "1 percent not sampled with rvalue",
			cfg: &Config{
				SamplingPercentage: 1,
			},
			// this r-value is slightly below the t-value threshold,
			// off-by-one compared with the case above in the least-
			// significant digit.
			ts: "ot=rv:FD70A3D70A3D70",
		},
		{
			name: "49 percent not sampled with default tid",
			cfg: &Config{
				SamplingPercentage: 49,
			},
		},
		{
			name: "1 percent sampled with rvalue",
			cfg: &Config{
				SamplingPercentage: 1,
			},
			// 99/100 = .FD70A3D70A3D70A3D
			ts: "ot=rv:fd70B000000000",
			sf: func(SamplerMode) (bool, float64, string) {
				return true, 1 / 0.01, "ot=rv:fd70B000000000;th:fd70a3d70a3d71"
			},
		},
		{
			name: "1 percent sampled with tid",
			tid:  mustParseTID("a0a0a0a0a0a0a0a0a0fe000000000000"),
			cfg: &Config{
				SamplingPercentage: 1,
				SamplingPrecision:  4,
			},
			// 99/100 = .FD70A3D70A3D70A3D
			ts: "",
			sf: func(SamplerMode) (bool, float64, string) {
				return true, 1 / 0.01, "ot=th:fd70a"
			},
		},
		{
			name: "sampled by priority",
			cfg: &Config{
				SamplingPercentage: 1,
			},
			ts:    "",
			key:   "sampling.priority",
			value: pcommon.NewValueInt(2),
			sf:    func(SamplerMode) (bool, float64, string) { return true, 1, "ot=th:0" },
		},
		{
			name: "not sampled by priority",
			cfg: &Config{
				SamplingPercentage: 99,
			},
			ts:    "",
			key:   "sampling.priority",
			value: pcommon.NewValueInt(0),
		},
		{
			name: "incoming 50 percent with rvalue",
			cfg: &Config{
				SamplingPercentage: 50,
			},
			ts: "ot=rv:90000000000000;th:80000000000000", // note extra zeros in th are erased
			sf: func(mode SamplerMode) (bool, float64, string) {
				if mode == Equalizing {
					return true, 2, "ot=rv:90000000000000;th:8"
				}
				// Proportionally, 50% less is 25% absolute sampling
				return false, 0, ""
			},
		},
		{
			name: "incoming 50 percent at 25 percent not sampled",
			cfg: &Config{
				SamplingPercentage: 25,
			},
			ts: "ot=th:8", // 50%
			sf: func(mode SamplerMode) (bool, float64, string) {
				return false, 0, ""
			},
		},
		{
			name: "incoming 50 percent at 25 percent sampled",
			cfg: &Config{
				SamplingPercentage: 25,
			},
			tid: mustParseTID("ffffffffffffffffffffffffffffffff"), // always sampled
			ts:  "ot=th:8",                                        // 50%
			sf: func(mode SamplerMode) (bool, float64, string) {
				if mode == Equalizing {
					return true, 4, "ot=th:c"
				}
				return true, 8, "ot=th:e"
			},
		},
		{
			name: "equalizing vs proportional",
			cfg: &Config{
				SamplingPercentage: 50,
			},
			ts: "ot=rv:c0000000000000;th:8",
			sf: func(mode SamplerMode) (bool, float64, string) {
				if mode == Equalizing {
					return true, 2, "ot=rv:c0000000000000;th:8"
				}
				return true, 4, "ot=rv:c0000000000000;th:c"
			},
		},
		{
			name: "inconsistent arriving t-value",
			log:  "inconsistent arriving t-value",
			cfg: &Config{
				SamplingPercentage: 100,
			},
			ts: "ot=rv:40000000000000;th:8",
			sf: func(SamplerMode) (bool, float64, string) {
				return true, 1, "ot=rv:40000000000000;th:0"
			},
		},
		{
			name: "inconsistent arriving t-value not sampled",
			log:  "inconsistent arriving t-value",
			cfg: &Config{
				SamplingPercentage: 1,
			},
			ts: "ot=rv:40000000000000;th:8",
			sf: func(SamplerMode) (bool, float64, string) {
				return false, 0, ""
			},
		},
		{
			name: "40 percent precision 3 with rvalue",
			cfg: &Config{
				SamplingPercentage: 40,
				SamplingPrecision:  3,
			},
			ts: "ot=rv:a0000000000000",
			sf: func(SamplerMode) (bool, float64, string) {
				return true, 1 / 0.4, "ot=rv:a0000000000000;th:99a"
			},
		},
		{
			name: "arriving 50 percent sampled at 40 percent precision 6 with tid",
			cfg: &Config{
				SamplingPercentage: 40,
				SamplingPrecision:  6,
			},
			tid: mustParseTID("a0a0a0a0a0a0a0a0a0d0000000000000"),
			ts:  "ot=th:8", // 50%
			sf: func(mode SamplerMode) (bool, float64, string) {
				if mode == Proportional {
					// 5 == 1 / (0.4 * 0.5)
					return true, 5, "ot=th:cccccd"
				}
				// 2.5 == 1 / 0.4
				return true, 2.5, "ot=th:99999a"
			},
		},
		{
			name: "arriving 50 percent sampled at 40 percent partly sampled",
			cfg: &Config{
				SamplingPercentage: 40,
				SamplingPrecision:  3,
			},
			tid: mustParseTID("a0a0a0a0a0a0a0a0a0b0000000000000"),
			ts:  "ot=th:8", // 50%
			sf: func(mode SamplerMode) (bool, float64, string) {
				if mode == Proportional {
					return false, 0, ""
				}
				// 2.5 == 1 / 0.4
				return true, 2.5, "ot=th:99a"
			},
		},
		{
			name: "arriving 50 percent sampled at 40 percent not sampled",
			cfg: &Config{
				SamplingPercentage: 40,
				SamplingPrecision:  3,
			},
			tid: mustParseTID("a0a0a0a0a0a0a0a0a080000000000000"),
			ts:  "ot=th:8", // 50%
			sf: func(mode SamplerMode) (bool, float64, string) {
				return false, 0, ""
			},
		},
		{
			name: "200 percent equals 100 percent",
			cfg: &Config{
				SamplingPercentage: 200,
			},
			ts: "",
			sf: func(SamplerMode) (bool, float64, string) {
				return true, 1, "ot=th:0"
			},
		},
		{
			name: "tiny probability rounding",
			cfg: &Config{
				SamplingPercentage: 100 * 0x1p-14,
			},
			tid: improbableTraceID,
			ts:  "ot=th:fffc",
			sf: func(mode SamplerMode) (bool, float64, string) {
				if mode == Equalizing {
					return true, 1 << 14, "ot=th:fffc"
				}
				return true, 1 << 28, "ot=th:fffffff"
			},
		},
		{
			// Note this test tests a probability value very close
			// to the limit near 100.0% expressible in a float32,
			// which is how the SamplingPercentage field is declared.
			// it's impossible to have 10 significant figures at
			// at this extreme.
			name: "almost 100pct sampling",
			cfg: &Config{
				SamplingPercentage: (1 - 8e-7) * 100, // very close to 100%
				SamplingPrecision:  10,               // 10 sig figs is impossible
			},
			tid: improbableTraceID,
			sf: func(mode SamplerMode) (bool, float64, string) {
				// The adjusted count is very close to 1.0.
				// The threshold has 8 significant figures.
				return true, 1 / (1 - 8e-7), "ot=th:00000cccccccd"
			},
		},
	}
	for _, tt := range tests {
		for _, mode := range []SamplerMode{Equalizing, Proportional} {
			t.Run(fmt.Sprint(mode, "_", tt.name), func(t *testing.T) {

				sink := new(consumertest.TracesSink)
				cfg := &Config{}
				if tt.cfg != nil {
					*cfg = *tt.cfg
				}
				cfg.SamplerMode = mode
				cfg.HashSeed = defaultHashSeed

				set := processortest.NewNopCreateSettings()
				logger, observed := observer.New(zap.DebugLevel)
				set.Logger = zap.New(logger)

				tsp, err := newTracesProcessor(context.Background(), set, cfg, sink)
				require.NoError(t, err)

				tid := defaultTID

				if !tt.tid.IsEmpty() {
					tid = tt.tid
				}

				td := makeSingleSpanWithAttrib(tid, sid, tt.ts, tt.key, tt.value)

				err = tsp.ConsumeTraces(context.Background(), td)
				require.NoError(t, err)

				sampledData := sink.AllTraces()

				var expectSampled bool
				var expectCount float64
				var expectTS string
				if tt.sf != nil {
					expectSampled, expectCount, expectTS = tt.sf(mode)
				}
				if expectSampled {
					require.Equal(t, 1, len(sampledData))
					assert.Equal(t, 1, sink.SpanCount())
					got := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
					gotTs, err := sampling.NewW3CTraceState(got.TraceState().AsRaw())
					require.NoError(t, err)
					if expectCount == 0 {
						assert.Equal(t, 0.0, gotTs.OTelValue().AdjustedCount())
					} else if cfg.SamplingPrecision == 0 {
						assert.InEpsilon(t, expectCount, gotTs.OTelValue().AdjustedCount(), 1e-9,
							"compare %v %v", expectCount, gotTs.OTelValue().AdjustedCount())
					} else {
						assert.InEpsilon(t, expectCount, gotTs.OTelValue().AdjustedCount(), 1e-3,
							"compare %v %v", expectCount, gotTs.OTelValue().AdjustedCount())
					}
					require.Equal(t, expectTS, got.TraceState().AsRaw())
				} else {
					require.Equal(t, 0, len(sampledData))
					assert.Equal(t, 0, sink.SpanCount())
					require.Equal(t, "", expectTS)
				}

				if len(tt.log) == 0 {
					require.Equal(t, 0, len(observed.All()), "should not have logs: %v", observed.All())
				} else {
					require.Equal(t, 1, len(observed.All()), "should have one log: %v", observed.All())
					require.Equal(t, observed.All()[0].Message, "tracestate")
					require.Contains(t, observed.All()[0].Context[0].Interface.(error).Error(), tt.log)
				}
			})
		}
	}
}

// Test_tracesamplerprocessor_StrictTraceState checks that when
// strictness checking is enabled, certain spans do not pass, with
// errors.
func Test_tracesamplerprocessor_StrictTraceState(t *testing.T) {
	defaultTID := mustParseTID("fefefefefefefefefe80000000000000")
	sid := idutils.UInt64ToSpanID(0xfefefefe)
	tests := []struct {
		name string
		tid  pcommon.TraceID
		cfg  *Config
		ts   string
		sf   func(SamplerMode) string
	}{
		{
			name: "missing randomness",
			cfg: &Config{
				SamplingPercentage: 100,
			},
			ts: "",
			sf: func(SamplerMode) string {
				return "missing randomness"
			},
		},
		{
			name: "invalid r-value",
			cfg: &Config{
				SamplingPercentage: 100,
			},
			ts: "ot=rv:abababababababab", // 16 digits is too many
			sf: func(SamplerMode) string {
				return "r-value must have 14 hex digits"
			},
		},
		{
			name: "invalid t-value",
			cfg: &Config{
				SamplingPercentage: 100,
			},
			ts: "ot=th:abababababababab", // 16 digits is too many
			sf: func(SamplerMode) string {
				return "t-value exceeds 14 hex digits"
			},
		},
		{
			name: "t-value syntax",
			cfg: &Config{
				SamplingPercentage: 100,
			},
			ts: "ot=th:-1",
			sf: func(SamplerMode) string {
				return "invalid syntax"
			},
		},
		{
			name: "inconsistent t-value trace ID",
			cfg: &Config{
				SamplingPercentage: 100,
			},
			tid: mustParseTID("ffffffffffffffffff70000000000000"),
			ts:  "ot=th:8",
			sf: func(SamplerMode) string {
				return "inconsistent arriving t-value"
			},
		},
		{
			name: "inconsistent t-value r-value",
			cfg: &Config{
				SamplingPercentage: 100,
			},
			ts: "ot=th:8;rv:70000000000000",
			sf: func(SamplerMode) string {
				return "inconsistent arriving t-value"
			},
		},
	}
	for _, tt := range tests {
		for _, mode := range []SamplerMode{Equalizing, Proportional} {
			t.Run(fmt.Sprint(mode, "_", tt.name), func(t *testing.T) {
				sink := new(consumertest.TracesSink)
				cfg := &Config{}
				if tt.cfg != nil {
					*cfg = *tt.cfg
				}
				cfg.StrictRandomness = true
				cfg.SamplerMode = mode

				set := processortest.NewNopCreateSettings()
				logger, observed := observer.New(zap.DebugLevel)
				set.Logger = zap.New(logger)

				expectMessage := ""
				if tt.sf != nil {
					expectMessage = tt.sf(mode)

				}

				tsp, err := newTracesProcessor(context.Background(), set, cfg, sink)
				if err != nil {
					// Sometimes the constructor fails.
					require.Contains(t, err.Error(), expectMessage)
					return
				}

				tid := defaultTID

				if !tt.tid.IsEmpty() {
					tid = tt.tid
				}

				td := makeSingleSpanWithAttrib(tid, sid, tt.ts, "", pcommon.Value{})

				err = tsp.ConsumeTraces(context.Background(), td)
				require.NoError(t, err)

				sampledData := sink.AllTraces()

				require.Equal(t, 0, len(sampledData))
				assert.Equal(t, 0, sink.SpanCount())

				require.Equal(t, 1, len(observed.All()), "should have one log: %v", observed.All())
				require.Equal(t, observed.All()[0].Message, "tracestate")
				require.Contains(t, observed.All()[0].Context[0].Interface.(error).Error(), expectMessage)
			})
		}
	}
}

// Test_tracesamplerprocessor_HashSeedTraceState tests that non-strict
// HashSeed modes generate trace state to indicate sampling.
func Test_tracesamplerprocessor_HashSeedTraceState(t *testing.T) {
	sid := idutils.UInt64ToSpanID(0xfefefefe)
	tests := []struct {
		pct   float32
		tvout string
	}{
		{
			pct:   100,
			tvout: "0",
		},
		{
			pct:   75,
			tvout: "4",
		},
		{
			pct:   50,
			tvout: "8",
		},
		{
			pct:   25,
			tvout: "c",
		},
		{
			pct:   10,
			tvout: "e668", // 14-bit rounding means e668 vs e666.
		},
		{
			pct:   100.0 / 3,
			tvout: "aaac", // 14-bit rounding means aaac, vs aaab.
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.pct, "pct"), func(t *testing.T) {
			sink := new(consumertest.TracesSink)
			cfg := &Config{}
			cfg.SamplingPercentage = tt.pct
			cfg.SamplerMode = HashSeed
			cfg.HashSeed = defaultHashSeed
			cfg.SamplingPrecision = 4

			tsp, err := newTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, sink)
			require.NoError(t, err)

			// Repeat until we find 10 sampled cases; each sample will have
			// an independent R-value.
			const find = 10
			found := 0
			for {
				sink.Reset()
				tid := idutils.UInt64ToTraceID(rand.Uint64(), rand.Uint64())
				td := makeSingleSpanWithAttrib(tid, sid, "", "", pcommon.Value{})

				err = tsp.ConsumeTraces(context.Background(), td)
				require.NoError(t, err)

				sampledData := sink.AllTraces()

				if len(sampledData) == 0 {
					continue
				}
				assert.Equal(t, 1, sink.SpanCount())

				span := sampledData[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
				spanTs, err := sampling.NewW3CTraceState(span.TraceState().AsRaw())
				require.NoError(t, err)

				threshold, hasT := spanTs.OTelValue().TValueThreshold()
				require.True(t, hasT)
				require.Equal(t, tt.tvout, spanTs.OTelValue().TValue())
				rnd, hasR := spanTs.OTelValue().RValueRandomness()
				require.True(t, hasR)
				require.True(t, threshold.ShouldSample(rnd))

				if found++; find == found {
					break
				}
			}
		})
	}
}

func getSpanWithAttributes(key string, value pcommon.Value) ptrace.Span {
	span := ptrace.NewSpan()
	initSpanWithAttribute(key, value, span)
	return span
}

func initSpanWithAttribute(key string, value pcommon.Value, dest ptrace.Span) {
	dest.SetName("spanName")
	value.CopyTo(dest.Attributes().PutEmpty(key))
}

// genRandomTestData generates a slice of ptrace.Traces with the numBatches elements which one with
// numTracesPerBatch spans (ie.: each span has a different trace ID). All spans belong to the specified
// serviceName.
func genRandomTestData(numBatches, numTracesPerBatch int, serviceName string, resourceSpanCount int) (tdd []ptrace.Traces) {
	r := rand.New(rand.NewSource(1))
	var traceBatches []ptrace.Traces
	for i := 0; i < numBatches; i++ {
		traces := ptrace.NewTraces()
		traces.ResourceSpans().EnsureCapacity(resourceSpanCount)
		for j := 0; j < resourceSpanCount; j++ {
			rs := traces.ResourceSpans().AppendEmpty()
			rs.Resource().Attributes().PutStr("service.name", serviceName)
			rs.Resource().Attributes().PutBool("bool", true)
			rs.Resource().Attributes().PutStr("string", "yes")
			rs.Resource().Attributes().PutInt("int64", 10000000)
			ils := rs.ScopeSpans().AppendEmpty()
			ils.Spans().EnsureCapacity(numTracesPerBatch)

			for k := 0; k < numTracesPerBatch; k++ {
				span := ils.Spans().AppendEmpty()
				span.SetTraceID(idutils.UInt64ToTraceID(r.Uint64(), r.Uint64()))
				span.SetSpanID(idutils.UInt64ToSpanID(r.Uint64()))
				span.Attributes().PutInt(conventions.AttributeHTTPStatusCode, 404)
				span.Attributes().PutStr("http.status_text", "Not Found")
			}
		}
		traceBatches = append(traceBatches, traces)
	}

	return traceBatches
}

// assertSampledData checks for no repeated traceIDs and counts the number of spans on the sampled data for
// the given service.
func assertSampledData(t *testing.T, sampled []ptrace.Traces, serviceName string) (traceIDs map[[16]byte]bool, spanCount int) {
	traceIDs = make(map[[16]byte]bool)
	for _, td := range sampled {
		rspans := td.ResourceSpans()
		for i := 0; i < rspans.Len(); i++ {
			rspan := rspans.At(i)
			ilss := rspan.ScopeSpans()
			for j := 0; j < ilss.Len(); j++ {
				ils := ilss.At(j)
				if svcNameAttr, _ := rspan.Resource().Attributes().Get("service.name"); svcNameAttr.Str() != serviceName {
					continue
				}
				for k := 0; k < ils.Spans().Len(); k++ {
					spanCount++
					span := ils.Spans().At(k)
					key := span.TraceID()
					if traceIDs[key] {
						t.Errorf("same traceID used more than once %q", key)
						return
					}
					traceIDs[key] = true
				}
			}
		}
	}
	return
}

// mustParseTID generates TraceIDs from their hex encoding, for
// testing probability sampling.
func mustParseTID(in string) pcommon.TraceID {
	b, err := hex.DecodeString(in)
	if err != nil {
		panic(err)
	}
	if len(b) != len(pcommon.TraceID{}) {
		panic("incorrect size input")
	}
	return pcommon.TraceID(b)
}

// makeSingleSpanWithAttrib is used to construct test data with
// a specific TraceID and a single attribute.
func makeSingleSpanWithAttrib(tid pcommon.TraceID, sid pcommon.SpanID, ts string, key string, attribValue pcommon.Value) ptrace.Traces {
	traces := ptrace.NewTraces()
	span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.TraceState().FromRaw(ts)
	span.SetTraceID(tid)
	span.SetSpanID(sid)
	if key != "" {
		attribValue.CopyTo(span.Attributes().PutEmpty(key))
	}
	return traces
}
