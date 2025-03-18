// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor/internal/metadata"
)

func TestNewLogs(t *testing.T) {
	tests := []struct {
		name         string
		nextConsumer consumer.Logs
		cfg          *Config
		wantErr      bool
	}{
		{
			name:         "happy_path",
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
				HashSeed:           4321,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newLogsProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), tt.nextConsumer, tt.cfg)
			if tt.wantErr {
				assert.Nil(t, got)
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
			}
		})
	}
}

func TestLogsSampling(t *testing.T) {
	// Note: in the tests below, &Config{} objects are created w/o
	// use of factory-supplied defaults. Therefore, we explicitly
	// set FailClosed: true in cases where the legacy test
	// included coverage of data with missing randomness.
	tests := []struct {
		name     string
		cfg      *Config
		received int
	}{
		{
			name: "happy_path",
			cfg: &Config{
				SamplingPercentage: 100,
			},
			received: 100,
		},
		{
			name: "nothing",
			cfg: &Config{
				SamplingPercentage: 0,

				// FailClosed because the test
				// includes one empty TraceID which
				// would otherwise fail open.
				FailClosed: true,
			},
			received: 0,
		},
		{
			name: "roughly half",
			cfg: &Config{
				SamplingPercentage: 50,
				AttributeSource:    traceIDAttributeSource,
				Mode:               HashSeed,
				FailClosed:         true,
			},
			// Note: This count excludes one empty TraceID
			// that fails closed.  If this test had been
			// written for 63% or greater, it would have been
			// counted.
			received: 45,
		},
		{
			name: "sampling_source no sampling",
			cfg: &Config{
				SamplingPercentage: 0,
				AttributeSource:    recordAttributeSource,
				FromAttribute:      "foo",
			},

			received: 0,
		},
		{
			name: "sampling_source all sampling",
			cfg: &Config{
				SamplingPercentage: 100,
				AttributeSource:    recordAttributeSource,
				FromAttribute:      "foo",
			},
			received: 100,
		},
		{
			name: "sampling_source sampling",
			cfg: &Config{
				SamplingPercentage: 50,
				AttributeSource:    recordAttributeSource,
				FromAttribute:      "foo",

				// FailClosed: true so that we do not
				// sample when the attribute is
				// missing.
				FailClosed: true,
			},
			received: 23,
		},
		{
			name: "sampling_source sampling as string",
			cfg: &Config{
				SamplingPercentage: 50,
				AttributeSource:    recordAttributeSource,
				FromAttribute:      "bar",

				// FailClosed: true so that we do not
				// sample when the attribute is
				// missing.
				FailClosed: true,
			},
			received: 29, // probabilistic... doesn't yield the same results as foo
		},
		{
			name: "sampling_priority_0_record",
			cfg: &Config{
				SamplingPercentage: 0,
				SamplingPriority:   "priority",
				AttributeSource:    recordAttributeSource,
				FromAttribute:      "rando",
			},
			received: 25,
		},
		{
			name: "sampling_priority_50_record",
			cfg: &Config{
				SamplingPercentage: 50,
				SamplingPriority:   "priority",
				AttributeSource:    recordAttributeSource,
				FromAttribute:      "rando",
			},
			// Expected value is (0.5*75)+25 == 62.5
			received: 64,
		},
		{
			name: "sampling_priority_100_record",
			cfg: &Config{
				SamplingPercentage: 100,
				SamplingPriority:   "priority",
				AttributeSource:    recordAttributeSource,
				FromAttribute:      "rando",
			},
			received: 100,
		},
		{
			name: "sampling_priority_0_traceid",
			cfg: &Config{
				SamplingPercentage: 0,
				SamplingPriority:   "priority",
				AttributeSource:    traceIDAttributeSource,
			},
			received: 25,
		},
		{
			name: "sampling_priority_50_traceid",
			cfg: &Config{
				SamplingPercentage: 50,
				SamplingPriority:   "priority",
				AttributeSource:    traceIDAttributeSource,
			},
			// Expected value is (0.5*75)+25 == 62.5
			received: 57,
		},
		{
			name: "sampling_priority_100_traceid",
			cfg: &Config{
				SamplingPercentage: 100,
				SamplingPriority:   "priority",
				AttributeSource:    traceIDAttributeSource,
			},
			received: 100,
		},
		{
			name: "sampling_priority with sampling field",
			cfg: &Config{
				SamplingPercentage: 0,
				AttributeSource:    recordAttributeSource,
				FromAttribute:      "foo",
				SamplingPriority:   "priority",
			},
			received: 25,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(consumertest.LogsSink)
			processor, err := newLogsProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), sink, tt.cfg)
			require.NoError(t, err)
			logs := plog.NewLogs()
			lr := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			for i := 0; i < 100; i++ {
				record := lr.AppendEmpty()
				record.SetTimestamp(pcommon.Timestamp(time.Unix(1649400860, 0).Unix()))
				record.SetSeverityNumber(plog.SeverityNumberDebug)
				ib := byte(i)
				// Note this TraceID is invalid when i==0.  Since this test
				// encodes historical behavior, we leave it as-is.
				traceID := [16]byte{0, 0, 0, 0, 0, 0, 0, 0, ib, ib, ib, ib, ib, ib, ib, ib}
				record.SetTraceID(traceID)
				record.Attributes().PutInt("rando", int64(computeHash(traceID[:], 123)))
				// set half of records with a foo (bytes) and a bar (string) attribute
				if i%2 == 0 {
					b := record.Attributes().PutEmptyBytes("foo")
					b.FromRaw(traceID[:])
					record.Attributes().PutStr("bar", record.TraceID().String())
				}
				// set a fourth of records with a priority attribute
				if i%4 == 0 {
					record.Attributes().PutDouble("priority", 100)
				}
			}
			err = processor.ConsumeLogs(context.Background(), logs)
			require.NoError(t, err)
			sunk := sink.AllLogs()
			numReceived := 0
			if len(sunk) > 0 && sunk[0].ResourceLogs().Len() > 0 {
				numReceived = sunk[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len()
			}
			assert.Equal(t, tt.received, numReceived)
		})
	}
}

func TestLogsSamplingState(t *testing.T) {
	// This hard-coded TraceID will sample at 50% and not at 49%.
	// The equivalent randomness is 0x80000000000000.
	defaultTID := mustParseTID("fefefefefefefefefe80000000000000")

	tests := []struct {
		name     string
		cfg      *Config
		tid      pcommon.TraceID
		attrs    map[string]any
		log      string
		sampled  bool
		adjCount float64
		expect   map[string]any
	}{
		{
			name: "100 percent traceID",
			cfg: &Config{
				SamplingPercentage: 100,
				AttributeSource:    traceIDAttributeSource,
				Mode:               Proportional,
			},
			tid: defaultTID,
			attrs: map[string]any{
				"ignored": "value",
			},
			sampled:  true,
			adjCount: 1,
			expect: map[string]any{
				"sampling.threshold": "0",
				"ignored":            "value",
			},
		},
		{
			name: "100 percent traceID hash_seed",
			cfg: &Config{
				SamplingPercentage: 100,
				AttributeSource:    traceIDAttributeSource,
				Mode:               "hash_seed",
				HashSeed:           22,
			},
			attrs: map[string]any{
				"K": "V",
			},
			tid:      defaultTID,
			sampled:  true,
			adjCount: 1,
			expect: map[string]any{
				"K":                   "V",
				"sampling.threshold":  "0",
				"sampling.randomness": randomnessFromBytes(defaultTID[:], 22).RValue(),
			},
		},
		{
			name: "100 percent attribute",
			cfg: &Config{
				SamplingPercentage: 100,
				AttributeSource:    recordAttributeSource,
				FromAttribute:      "veryrandom",
				HashSeed:           49,
			},
			attrs: map[string]any{
				"veryrandom": "1234",
			},
			sampled:  true,
			adjCount: 1,
			expect: map[string]any{
				"sampling.threshold":  "0",
				"sampling.randomness": randomnessFromBytes([]byte("1234"), 49).RValue(),
				"veryrandom":          "1234",
			},
		},
		{
			name: "0 percent traceID",
			cfg: &Config{
				SamplingPercentage: 0,
				AttributeSource:    traceIDAttributeSource,
			},
			tid:     defaultTID,
			sampled: false,
		},
		{
			name: "10 percent priority sampled incoming randomness",
			cfg: &Config{
				SamplingPercentage: 0,
				AttributeSource:    traceIDAttributeSource,
				SamplingPriority:   "veryrandom",
				SamplingPrecision:  6,
			},
			tid: defaultTID,
			attrs: map[string]any{
				"sampling.randomness": "e6147c00000000",
				"veryrandom":          10.125,
			},
			sampled:  true,
			adjCount: 9.876654321,
			expect: map[string]any{
				"sampling.randomness": "e6147c00000000",
				"sampling.threshold":  "e6147b",
				"veryrandom":          10.125,
			},
		},
		{
			name: "25 percent incoming",
			cfg: &Config{
				SamplingPercentage: 50,
				AttributeSource:    traceIDAttributeSource,
				Mode:               Proportional,
			},
			tid: mustParseTID("fefefefefefefefefef0000000000000"),
			attrs: map[string]any{
				"sampling.threshold": "c",
			},
			sampled:  true,
			adjCount: 8,
			expect: map[string]any{
				"sampling.threshold": "e",
			},
		},
		{
			name: "25 percent arriving inconsistent",
			cfg: &Config{
				SamplingPercentage: 50,
				AttributeSource:    traceIDAttributeSource,
				Mode:               Equalizing,
				FailClosed:         true,
			},
			tid: mustParseTID("fefefefefefefefefeb0000000000000"),
			attrs: map[string]any{
				// "c" is an invalid threshold for the TraceID
				// i.e., T <= R is false, should be rejected.
				"sampling.threshold": "c", // Corresponds with 25%
			},
			log:     "inconsistent arriving threshold",
			sampled: false,
		},
		{
			name: "25 percent arriving equalizing",
			cfg: &Config{
				SamplingPercentage: 50,
				AttributeSource:    traceIDAttributeSource,
				Mode:               Equalizing,
				SamplingPriority:   "prio",
			},
			tid: mustParseTID("fefefefefefefefefefefefefefefefe"),
			attrs: map[string]any{
				"sampling.threshold": "c", // Corresponds with 25%
				"prio":               37,  // Lower than 50, greater than 25
			},
			sampled:  true,
			adjCount: 4,
			expect: map[string]any{
				"sampling.threshold": "c",
				"prio":               int64(37),
			},
			log: "cannot raise existing sampling probability",
		},
		{
			name: "hash_seed with spec randomness",
			cfg: &Config{
				SamplingPercentage: 100,
				AttributeSource:    traceIDAttributeSource,
				Mode:               HashSeed,
			},
			tid: defaultTID,
			attrs: map[string]any{
				"sampling.randomness": "f2341234123412",
			},
			sampled:  true,
			adjCount: 0, // No threshold
			log:      "item has sampling randomness",
			expect: map[string]any{
				"sampling.randomness": "f2341234123412",
			},
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.name), func(t *testing.T) {
			sink := new(consumertest.LogsSink)
			cfg := &Config{}
			if tt.cfg != nil {
				*cfg = *tt.cfg
			}

			set := processortest.NewNopSettings(metadata.Type)
			logger, observed := observer.New(zap.DebugLevel)
			set.Logger = zap.New(logger)

			tsp, err := newLogsProcessor(context.Background(), set, sink, cfg)
			require.NoError(t, err)

			logs := plog.NewLogs()
			lr := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
			record := lr.AppendEmpty()
			record.SetTimestamp(pcommon.Timestamp(time.Unix(1649400860, 0).Unix()))
			record.SetSeverityNumber(plog.SeverityNumberDebug)
			record.SetTraceID(tt.tid)
			require.NoError(t, record.Attributes().FromRaw(tt.attrs))

			err = tsp.ConsumeLogs(context.Background(), logs)
			require.NoError(t, err)

			if len(tt.log) == 0 {
				require.Empty(t, observed.All(), "should not have logs: %v", observed.All())
				require.Equal(t, "", tt.log)
			} else {
				require.Len(t, observed.All(), 1, "should have one log: %v", observed.All())
				require.Contains(t, observed.All()[0].Message, "logs sampler")
				require.ErrorContains(t, observed.All()[0].Context[0].Interface.(error), tt.log)
			}

			sampledData := sink.AllLogs()

			if tt.sampled {
				require.Len(t, sampledData, 1)
				assert.Equal(t, 1, sink.LogRecordCount())
				got := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
				gotAttrs := got.Attributes()
				require.Equal(t, tt.expect, gotAttrs.AsRaw())
				thVal, hasTh := gotAttrs.Get("sampling.threshold")
				if tt.adjCount == 0 {
					require.False(t, hasTh)
				} else {
					th, err := sampling.TValueToThreshold(thVal.Str())
					require.NoError(t, err)
					if cfg.SamplingPrecision == 0 {
						assert.InEpsilon(t, tt.adjCount, th.AdjustedCount(), 1e-9,
							"compare %v %v", tt.adjCount, th.AdjustedCount())
					} else {
						assert.InEpsilon(t, tt.adjCount, th.AdjustedCount(), 1e-3,
							"compare %v %v", tt.adjCount, th.AdjustedCount())
					}
				}
			}
		})
	}
}

func TestLogsMissingRandomness(t *testing.T) {
	type test struct {
		pct        float32
		source     AttributeSource
		failClosed bool
		sampled    bool
	}

	for _, mode := range AllModes {
		for _, tt := range []test{
			{0, recordAttributeSource, true, false},
			{50, recordAttributeSource, true, false},
			{100, recordAttributeSource, true, false},

			{0, recordAttributeSource, false, false},
			{50, recordAttributeSource, false, true},
			{100, recordAttributeSource, false, true},

			{0, traceIDAttributeSource, true, false},
			{50, traceIDAttributeSource, true, false},
			{100, traceIDAttributeSource, true, false},

			{0, traceIDAttributeSource, false, false},
			{50, traceIDAttributeSource, false, true},
			{100, traceIDAttributeSource, false, true},
		} {
			t.Run(fmt.Sprint(tt.pct, "_", tt.source, "_", tt.failClosed, "_", mode), func(t *testing.T) {
				ctx := context.Background()
				logs := plog.NewLogs()
				record := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
				record.SetTraceID(pcommon.TraceID{}) // invalid TraceID
				record.Attributes().PutStr("unused", "")

				cfg := &Config{
					SamplingPercentage: tt.pct,
					Mode:               mode,
					HashSeed:           defaultHashSeed,
					FailClosed:         tt.failClosed,
					AttributeSource:    tt.source,
					FromAttribute:      "unused",
				}

				sink := new(consumertest.LogsSink)
				set := processortest.NewNopSettings(metadata.Type)
				// Note: there is a debug-level log we are expecting when FailClosed
				// causes a drop.
				logger, observed := observer.New(zap.DebugLevel)
				set.Logger = zap.New(logger)

				lp, err := newLogsProcessor(ctx, set, sink, cfg)
				require.NoError(t, err)

				err = lp.ConsumeLogs(ctx, logs)
				require.NoError(t, err)

				sampledData := sink.AllLogs()
				if tt.sampled {
					require.Len(t, sampledData, 1)
					assert.Equal(t, 1, sink.LogRecordCount())
				} else {
					require.Empty(t, sampledData)
					assert.Equal(t, 0, sink.LogRecordCount())
				}

				if tt.pct != 0 {
					// pct==0 bypasses the randomness check
					require.Len(t, observed.All(), 1, "should have one log: %v", observed.All())
					require.Contains(t, observed.All()[0].Message, "logs sampler")
					require.ErrorContains(t, observed.All()[0].Context[0].Interface.(error), "missing randomness")
				} else {
					require.Empty(t, observed.All(), "should have no logs: %v", observed.All())
				}
			})
		}
	}
}
