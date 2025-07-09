// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
	conventions "go.opentelemetry.io/otel/semconv/v1.6.1"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	idutils "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/core/xidutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor/internal/metadata"
)

// defaultHashSeed is used throughout to ensure that the HashSeed is real
// and does not fall back to proportional-mode sampling due to HashSeed == 0.
const defaultHashSeed = 4312

func TestHashBucketsLog2(t *testing.T) {
	require.Equal(t, numHashBuckets, 1<<numHashBucketsLg2)
}

func TestEmptyHashFunction(t *testing.T) {
	// With zero bytes of randomness, seed=0:
	hashed32 := computeHash([]byte{}, 0)
	hashed := uint64(hashed32 & bitMaskHashBuckets)
	require.Equal(t, uint64(0x3515), hashed)
	require.InDelta(t, 0.829, float64(hashed)/float64(numHashBuckets), 0.001)

	// With 16 bytes of 0s, seed=0:
	var b [16]byte
	hashed32 = computeHash(b[:], 0)
	hashed = uint64(hashed32 & bitMaskHashBuckets)
	require.Equal(t, uint64(0x2455), hashed)
	require.InDelta(t, 0.568, float64(hashed)/float64(numHashBuckets), 0.001)
}

func TestNewTraces(t *testing.T) {
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
			got, err := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), tt.cfg, tt.nextConsumer)
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
	// Note on hard-coded "acceptableDelta" values below.  This
	// test uses a global random number generator: it is sensitive to
	// the seeding of the RNG and it is sensitive to the default
	// hash seed that is used.  This test depends on HashSeed==0.
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
			sink := newAssertTraces(t, testSvcName)

			tsp, err := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), tt.cfg, sink)
			require.NoError(t, err, "error when creating traceSamplerProcessor")
			for _, td := range genRandomTestData(tt.numBatches, tt.numTracesPerBatch, testSvcName, 1) {
				assert.NoError(t, tsp.ConsumeTraces(context.Background(), td))
			}
			sampled := sink.spanCount
			actualPercentageSamplingPercentage := float32(sampled) / float32(tt.numBatches*tt.numTracesPerBatch) * 100.0
			delta := math.Abs(float64(actualPercentageSamplingPercentage - tt.cfg.SamplingPercentage))
			assert.LessOrEqualf(t, delta, tt.acceptableDelta,
				"got %f percentage sampling rate, want %f (allowed delta is %f but got %f)",
				actualPercentageSamplingPercentage,
				tt.cfg.SamplingPercentage,
				tt.acceptableDelta,
				delta,
			)
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
			tsp, err := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), tt.cfg, sink)
			require.NoError(t, err, "error when creating traceSamplerProcessor")

			for _, td := range genRandomTestData(tt.numBatches, tt.numTracesPerBatch, testSvcName, tt.resourceSpanPerTrace) {
				assert.NoError(t, tsp.ConsumeTraces(context.Background(), td))
				assert.Equal(t, tt.resourceSpanPerTrace*tt.numTracesPerBatch, sink.SpanCount())
				sink.Reset()
			}
		})
	}
}

func Test_tracessamplerprocessor_MissingRandomness(t *testing.T) {
	type test struct {
		pct        float32
		failClosed bool
		sampled    bool
	}

	for _, tt := range []test{
		// When the TraceID is empty and failClosed==true, the span is not sampled.
		{0, true, false},
		{62, true, false},
		{100, true, false},

		// When the TraceID is empty and failClosed==false, the span is sampled when pct != 0.
		{0, false, false},
		{62, false, true},
		{100, false, true},
	} {
		t.Run(fmt.Sprint(tt.pct, "_", tt.failClosed), func(t *testing.T) {
			ctx := context.Background()
			traces := ptrace.NewTraces()
			span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
			span.SetTraceID(pcommon.TraceID{})                     // invalid TraceID
			span.SetSpanID(pcommon.SpanID{1, 2, 3, 4, 5, 6, 7, 8}) // valid SpanID
			span.SetName("testing")

			cfg := &Config{
				SamplingPercentage: tt.pct,
				HashSeed:           defaultHashSeed,
				FailClosed:         tt.failClosed,
			}

			sink := new(consumertest.TracesSink)

			set := processortest.NewNopSettings(metadata.Type)
			// Note: there is a debug-level log we are expecting when FailClosed
			// causes a drop.
			logger, observed := observer.New(zap.DebugLevel)
			set.Logger = zap.New(logger)

			tsp, err := newTracesProcessor(ctx, set, cfg, sink)
			require.NoError(t, err)

			err = tsp.ConsumeTraces(ctx, traces)
			require.NoError(t, err)

			sampledData := sink.AllTraces()
			if tt.sampled {
				require.Len(t, sampledData, 1)
				assert.Equal(t, 1, sink.SpanCount())
			} else {
				require.Empty(t, sampledData)
				assert.Equal(t, 0, sink.SpanCount())
			}

			if tt.pct != 0 {
				// pct==0 bypasses the randomness check
				require.Len(t, observed.All(), 1, "should have one log: %v", observed.All())
				require.Contains(t, observed.All()[0].Message, "traces sampler")
				require.ErrorContains(t, observed.All()[0].Context[0].Interface.(error), "missing randomness")
			} else {
				require.Empty(t, observed.All(), "should have no logs: %v", observed.All())
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
				cfg.Mode = mode
				cfg.HashSeed = defaultHashSeed

				tsp, err := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, sink)
				require.NoError(t, err)

				err = tsp.ConsumeTraces(context.Background(), tt.td)
				require.NoError(t, err)

				sampledData := sink.AllTraces()
				if tt.sampled {
					require.Len(t, sampledData, 1)
					assert.Equal(t, 1, sink.SpanCount())
				} else {
					require.Empty(t, sampledData)
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
	defaultTID := mustParseTID("fefefefefefefefefe80000000000000")

	// improbableTraceID will sample at all supported probabilities.  In
	// hex, the leading 18 digits do not matter, the trailing 14 are all `f`.
	improbableTraceID := mustParseTID("111111111111111111ffffffffffffff")

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
			sf: func(SamplerMode) (bool, float64, string) {
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
			name: "inconsistent arriving threshold",
			log:  "inconsistent arriving threshold",
			cfg: &Config{
				SamplingPercentage: 100,
			},
			ts: "ot=rv:40000000000000;th:8",
			sf: func(SamplerMode) (bool, float64, string) {
				return true, 1, "ot=rv:40000000000000;th:0"
			},
		},
		{
			name: "inconsistent arriving threshold not sampled",
			log:  "inconsistent arriving threshold",
			cfg: &Config{
				SamplingPercentage: 1,
				FailClosed:         true,
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
			sf: func(SamplerMode) (bool, float64, string) {
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
			sf: func(SamplerMode) (bool, float64, string) {
				// The adjusted count is very close to 1.0.
				// The threshold has 8 significant figures.
				return true, 1 / (1 - 8e-7), "ot=th:00000cccccccd"
			},
		},
		{
			name: "probability underflow",
			cfg: &Config{
				SamplingPercentage: 0x1p-4,
			},
			tid: improbableTraceID,
			ts:  "ot=th:fffffffffffff8",
			sf: func(mode SamplerMode) (bool, float64, string) {
				if mode == Equalizing {
					return true, 1 << 53, "ot=th:fffffffffffff8"
				}
				return false, 0, ""
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
				cfg.Mode = mode
				cfg.HashSeed = defaultHashSeed

				set := processortest.NewNopSettings(metadata.Type)
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
					require.Len(t, sampledData, 1)
					assert.Equal(t, 1, sink.SpanCount())
					got := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
					gotTs, err := sampling.NewW3CTraceState(got.TraceState().AsRaw())
					require.NoError(t, err)
					switch {
					case expectCount == 0:
						assert.Equal(t, 0.0, gotTs.OTelValue().AdjustedCount())
					case cfg.SamplingPrecision == 0:
						assert.InEpsilon(t, expectCount, gotTs.OTelValue().AdjustedCount(), 1e-9,
							"compare %v %v", expectCount, gotTs.OTelValue().AdjustedCount())
					default:
						assert.InEpsilon(t, expectCount, gotTs.OTelValue().AdjustedCount(), 1e-3,
							"compare %v %v", expectCount, gotTs.OTelValue().AdjustedCount())
					}
					require.Equal(t, expectTS, got.TraceState().AsRaw())
				} else {
					require.Empty(t, sampledData)
					assert.Equal(t, 0, sink.SpanCount())
					require.Empty(t, expectTS)
				}

				if len(tt.log) == 0 {
					require.Empty(t, observed.All(), "should not have logs: %v", observed.All())
				} else {
					require.Len(t, observed.All(), 1, "should have one log: %v", observed.All())
					require.Contains(t, observed.All()[0].Message, "traces sampler")
					require.ErrorContains(t, observed.All()[0].Context[0].Interface.(error), tt.log)
				}
			})
		}
	}
}

// Test_tracesamplerprocessor_TraceStateErrors checks that when
// FailClosed is true, certain spans do not pass, with errors.
func Test_tracesamplerprocessor_TraceStateErrors(t *testing.T) {
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
			ts:  "",
			tid: pcommon.TraceID{},
			sf: func(SamplerMode) string {
				return "missing randomness"
			},
		},
		{
			name: "invalid r-value",
			cfg: &Config{
				SamplingPercentage: 100,
			},
			tid: defaultTID,
			ts:  "ot=rv:abababababababab", // 16 digits is too many
			sf: func(SamplerMode) string {
				return "r-value must have 14 hex digits"
			},
		},
		{
			name: "invalid t-value",
			cfg: &Config{
				SamplingPercentage: 100,
			},
			tid: defaultTID,
			ts:  "ot=th:abababababababab", // 16 digits is too many
			sf: func(SamplerMode) string {
				return "t-value exceeds 14 hex digits"
			},
		},
		{
			name: "t-value syntax",
			cfg: &Config{
				SamplingPercentage: 100,
			},
			tid: defaultTID,
			ts:  "ot=th:-1",
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
				return "inconsistent arriving threshold"
			},
		},
		{
			name: "inconsistent t-value r-value",
			cfg: &Config{
				SamplingPercentage: 100,
			},
			tid: defaultTID,
			ts:  "ot=th:8;rv:70000000000000",
			sf: func(SamplerMode) string {
				return "inconsistent arriving threshold"
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
				cfg.Mode = mode
				cfg.FailClosed = true

				set := processortest.NewNopSettings(metadata.Type)
				logger, observed := observer.New(zap.DebugLevel)
				set.Logger = zap.New(logger)

				expectMessage := ""
				if tt.sf != nil {
					expectMessage = tt.sf(mode)
				}

				tsp, err := newTracesProcessor(context.Background(), set, cfg, sink)
				require.NoError(t, err)

				td := makeSingleSpanWithAttrib(tt.tid, sid, tt.ts, "", pcommon.Value{})

				err = tsp.ConsumeTraces(context.Background(), td)
				require.NoError(t, err)

				sampledData := sink.AllTraces()

				require.Empty(t, sampledData)
				assert.Equal(t, 0, sink.SpanCount())

				require.Len(t, observed.All(), 1, "should have one log: %v", observed.All())
				if observed.All()[0].Message == "trace sampler" {
					require.ErrorContains(t, observed.All()[0].Context[0].Interface.(error), expectMessage)
				} else {
					require.Contains(t, observed.All()[0].Message, "traces sampler")
					require.ErrorContains(t, observed.All()[0].Context[0].Interface.(error), expectMessage)
				}
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
			cfg.Mode = HashSeed
			cfg.HashSeed = defaultHashSeed
			cfg.SamplingPrecision = 4

			tsp, err := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, sink)
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

	// ensure a non-empty trace ID with a deterministic value, one that has
	// all zero bits for the w3c randomness portion.  this value, if sampled
	// with the OTel specification, has R-value 0 and sampled only at 100%.
	dest.SetTraceID(pcommon.TraceID{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
}

// genRandomTestData generates a slice of ptrace.Traces with the numBatches elements which one with
// numTracesPerBatch spans (ie.: each span has a different trace ID). All spans belong to the specified
// serviceName. A fixed-seed random generator is used to ensure tests are repeatable.
func genRandomTestData(numBatches, numTracesPerBatch int, serviceName string, resourceSpanCount int) (tdd []ptrace.Traces) {
	r := rand.New(rand.NewPCG(123, 456))
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
				span.Attributes().PutInt(string(conventions.HTTPStatusCodeKey), 404)
				span.Attributes().PutStr("http.status_text", "Not Found")
			}
		}
		traceBatches = append(traceBatches, traces)
	}

	return traceBatches
}

// assertTraces is a traces consumer.Traces
type assertTraces struct {
	*testing.T
	testName  string
	traceIDs  map[[16]byte]bool
	spanCount int
}

var _ consumer.Traces = &assertTraces{}

func (a *assertTraces) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (a *assertTraces) ConsumeTraces(_ context.Context, data ptrace.Traces) error {
	tt := [1]ptrace.Traces{
		data,
	}
	a.onSampledData(tt[:])
	return nil
}

func newAssertTraces(t *testing.T, name string) *assertTraces {
	return &assertTraces{
		T:         t,
		testName:  name,
		traceIDs:  map[[16]byte]bool{},
		spanCount: 0,
	}
}

// onSampledData checks for no repeated traceIDs and counts the number of spans on the sampled data for
// the given service.
func (a *assertTraces) onSampledData(sampled []ptrace.Traces) {
	for _, td := range sampled {
		rspans := td.ResourceSpans()
		for i := 0; i < rspans.Len(); i++ {
			rspan := rspans.At(i)
			ilss := rspan.ScopeSpans()
			for j := 0; j < ilss.Len(); j++ {
				ils := ilss.At(j)
				if svcNameAttr, _ := rspan.Resource().Attributes().Get("service.name"); svcNameAttr.Str() != a.testName {
					continue
				}
				for k := 0; k < ils.Spans().Len(); k++ {
					a.spanCount++
					span := ils.Spans().At(k)
					key := span.TraceID()
					if a.traceIDs[key] {
						a.Errorf("same traceID used more than once %q", key)
						return
					}
					a.traceIDs[key] = true
				}
			}
		}
	}
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

// TestHashingFunction verifies 100 examples of the legacy hash-seed
// based trace sampling decision.  This test is made prior to refactoring
// the hash calculation to ensure legacy behavior does not change.
func TestHashingFunction(t *testing.T) {
	type expect50PctHashed struct {
		seed    uint32
		traceID string
		sampled bool
	}

	expect50PctData := []expect50PctHashed{
		{653, "474a03c76d75951a4b4c537ced8f1122", true},
		{563, "53a518291e91307e43cd8467bb06f986", true},
		{142, "a56a02f843b9bc6ee0b13889249e90e6", true},
		{904, "4e40762d3ee97a1c0932e4fa584f89a8", false},
		{445, "5224507db93db513f0ea2a4b4e0578c8", true},
		{38, "0c8717ced36216037af657e9d7f8b35b", true},
		{561, "2a8aa76c18d08e1e8be935541f9318c7", false},
		{757, "9e3d0f9481dc422cb613ea550897ae71", false},
		{22, "66a66c516ac22054673e5da5e6492545", false},
		{172, "84a1ce7bcea3e66194e72b4aa2694e31", true},
		{552, "a811a7def34ca4b98d8e320afd115fad", false},
		{546, "e3a345cc8dbb6f014bfa1edad3981820", true},
		{315, "a71effb50e28d27cbdc9892f3765b8c2", false},
		{510, "55ee665a3fa22f8ea1b744ce15a7339d", false},
		{230, "7a5006be4d0ce7b542d59f83cd6f1c41", false},
		{544, "825b8fb9cfd45867794f4cd8a5a699bd", false},
		{790, "7629ecbea89398bfd9752a2f51c2c137", true},
		{555, "de6cdfb44d69e211f886c57120d7bda0", true},
		{147, "a8a5c3bb9205883fae17ead6675b2450", false},
		{238, "937e6cb3332dbe87062fa3997f48f425", false},
		{122, "5a357e150995e005847816c431ba502d", false},
		{963, "20cb3dcebe2cf8abe6102f4a2e548245", false},
		{141, "1b0afbd09abaaf7996cd26f8f6533795", false},
		{666, "3ee60b013303bcfda06be89071b90bd1", true},
		{305, "c90c7cf3471bbc3a804a8a831633705b", true},
		{270, "18dda74dfca45a7b0261510f385fb4de", true},
		{381, "600cf70c7bb4918e54aefd78c84f3996", true},
		{35, "855f493c5b5b1e2fcbc9993f8061eac8", false},
		{839, "89bc498feb21d969cf0eabf916aa621a", false},
		{561, "0a0af00f63e098a39883705a423b0aa8", true},
		{667, "0c38553d71f54dfc37155c22cc8bf243", false},
		{603, "49493809d1a49ea879e6aba37afde958", true},
		{92, "eb60d98b8f8fe22d8970f44f0e2b6bad", true},
		{70, "19c386ee7a9f2d56ff9ab2e6374540af", false},
		{567, "2a033c15405f1c7a311f653719ed47b7", false},
		{936, "62dda24e4be24f5198e4d8dd4010c811", true},
		{210, "bb134e26ee92e282e29cabdb1d00d333", true},
		{835, "6c77db08bbded7bdd5c99c6e2fea41d2", false},
		{864, "99f6a7e7b50845b4fb64b7c2ee49f53c", true},
		{775, "e908cb91224bee8fd4b5f3632f65717f", true},
		{687, "45a5ace7234d92d9983b4f3858bc0b8e", false},
		{761, "8c8a25d232fd4d3a37a5f70ccb82d752", false},
		{400, "54156d7434a894ef07f2a80dbf0f1138", true},
		{741, "67e3bfb02b0526dbb79420468d7b83dd", false},
		{871, "f6ea221283dcb42f89bfb15fa33398d2", false},
		{244, "b663ca45004decf8123a19fb5d7f7115", true},
		{885, "d0e299d54d6dc6469276fd4e48301d73", true},
		{607, "03240c2748aa67a185909a9345d84aba", false},
		{434, "b00de72ba67e6fe0ed661decac911f7e", true},
		{889, "20fff68a7cc715b30e4e6d69d53e0f60", false},
		{810, "6bf2bb105e594f6220803da5253551af", false},
		{494, "54fbb1d3ebe3883b0a01bbf2c9a2bf3e", true},
		{413, "a8a2ecba129d8537e360cf54de9d7460", false},
		{215, "2df034262b775136f2a313fdcc09738a", true},
		{557, "e3c1b943d9c1199d1108a69aa32a4587", false},
		{662, "1975d5f5640bac1064d53c2c21e02aae", false},
		{482, "6e4f16727dec3c09539b5f50d35d2c13", false},
		{223, "63a088446ef0ed60a9cace4698ede026", false},
		{261, "9b662cd6f67a4e3d1f904b4c5d4275b1", true},
		{112, "a2db788dbadc402b8c466b93b8749a6c", true},
		{6, "d6a68b47c66d1f94eed46b8ddc72faf4", false},
		{575, "e8a83c42f4515568d0942ec4472c9d2c", true},
		{568, "030e14c2954e3f08134b355f33414ba9", false},
		{965, "022315846d42a38322d6fd26250444b3", true},
		{512, "b3ba1ed226288dede87ac1f2ba88de2b", true},
		{108, "c57d0dcf43d5b154ef04c7953c94cd12", true},
		{248, "a835fe521d9cbfcfb724b603f87c7403", false},
		{46, "eda867e6df95e74abefac336c7f4cd1a", false},
		{879, "09e9e67a261ea3e00d817105b57ffd4e", false},
		{853, "6a780cf250cb3d2b699394042e6723a8", false},
		{639, "4c99d7f14c66b3123caf57980f8e2a31", false},
		{111, "79dc8d7a54bc3e8ef513b9cd8d830564", true},
		{135, "9c2e5d9d713e5219b0f9e5b884835e69", false},
		{209, "3ccb300bf7b983229979e0c46db267b0", true},
		{629, "0bb7b9da64da250c3934bb39130dc990", true},
		{910, "b25713ca4cea377871eaa334bc2dd382", true},
		{667, "69afc041003851cec60f41db97e005a9", true},
		{449, "c844b5428abe0cf82eaf02566781870a", true},
		{16, "2533c732bed8c1ba4721c25a1205f06c", false},
		{936, "ecc770b4be885dfc8d6fa135bc2c93bb", true},
		{595, "63b67cbb42de52e9916241ad94fcd5e8", true},
		{83, "fc4998bc53ccd42a5b8e7c86a93d4c88", false},
		{878, "59f0677dffe1a0a8c5895cb263e3a019", false},
		{206, "eb897eff9e7c7363e063b340a0c6b315", true},
		{710, "89e4c7e6af305be6cd139abcae953db5", true},
		{650, "97563d45ee254231e1ace05fb746bcce", false},
		{233, "3f580864f295ff13f179c3c907032ea9", true},
		{836, "e9b78e03706265a6936bff2a41530104", false},
		{568, "c458603ee921fa8711085c15b871b245", false},
		{816, "4bbfffab5b5975c1b007ebc518bf416d", false},
		{397, "61a1a65746287d78a431c6848ed1ffb3", false},
		{847, "53eee02f4672a72e93369b2c2ecf36eb", false},
		{354, "6ea23c3068a2c304488c8e67a072db97", false},
		{961, "eed247645e510ded87bd9afcf1d3e237", false},
		{799, "092af4ff2fdea5bb1c708b2169bdfd95", false},
		{99, "497f02db51c898ac441aae18a8b7ced9", false},
		{773, "988481445600bb91bbe23e3103034bcd", true},
		{928, "f1813835ac0f456721ef3aac39c0269a", true},
		{235, "1999920085682c007eb3a6984d2a7f05", true},
		{460, "60c3b9a2dde734d71ba5cca7eb164bce", true},
	}

	// Note the test data above was created by essentially a
	// one-off rewrite of the test body below, where instead of
	// verifying it printed the expected results.
	for _, tc := range expect50PctData {
		sink := new(consumertest.TracesSink)
		tsp, err := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), &Config{
			HashSeed:           tc.seed,
			SamplingPercentage: 50,
		}, sink)
		if err != nil {
			panic(err)
		}
		traces := ptrace.NewTraces()
		span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetTraceID(mustParseTID(tc.traceID))
		err = tsp.ConsumeTraces(context.Background(), traces)
		if err != nil {
			panic(err)
		}

		wasSampled := len(sink.AllTraces()) == 1

		require.Equal(t, tc.sampled, wasSampled)
	}
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
