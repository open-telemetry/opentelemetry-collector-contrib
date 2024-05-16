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
)

func TestNewLogsProcessor(t *testing.T) {
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
			got, err := newLogsProcessor(context.Background(), processortest.NewNopCreateSettings(), tt.nextConsumer, tt.cfg)
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
			},
			received: 0,
		},
		{
			name: "roughly half",
			cfg: &Config{
				SamplingPercentage: 50,
				AttributeSource:    traceIDAttributeSource,
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
				FailClosed:         true,
			},
			received: 23,
		},
		{
			name: "sampling_source sampling as string",
			cfg: &Config{
				SamplingPercentage: 50,
				AttributeSource:    recordAttributeSource,
				FromAttribute:      "bar",
				FailClosed:         true,
			},
			received: 29, // probabilistic... doesn't yield the same results as foo
		},
		{
			name: "sampling_priority",
			cfg: &Config{
				SamplingPercentage: 0,
				SamplingPriority:   "priority",
			},
			received: 25,
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
			processor, err := newLogsProcessor(context.Background(), processortest.NewNopCreateSettings(), sink, tt.cfg)
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

func TestLogsMissingRandomness(t *testing.T) {
	type test struct {
		pct        float32
		source     AttributeSource
		failClosed bool
		sampled    bool
	}

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
		t.Run(fmt.Sprint(tt.pct, "_", tt.source, "_", tt.failClosed), func(t *testing.T) {

			ctx := context.Background()
			logs := plog.NewLogs()
			record := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
			record.SetTraceID(pcommon.TraceID{}) // invalid TraceID

			cfg := &Config{
				SamplingPercentage: tt.pct,
				HashSeed:           defaultHashSeed,
				FailClosed:         tt.failClosed,
				AttributeSource:    tt.source,
				FromAttribute:      "unused",
			}

			sink := new(consumertest.LogsSink)
			set := processortest.NewNopCreateSettings()
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
				require.Equal(t, 1, len(sampledData))
				assert.Equal(t, 1, sink.LogRecordCount())
			} else {
				require.Equal(t, 0, len(sampledData))
				assert.Equal(t, 0, sink.LogRecordCount())
			}

			if tt.pct != 0 {
				// pct==0 bypasses the randomness check
				require.Equal(t, 1, len(observed.All()), "should have one log: %v", observed.All())
				require.Contains(t, observed.All()[0].Message, "logs sampler")
				require.Contains(t, observed.All()[0].Context[0].Interface.(error).Error(), "missing randomness")
			} else {
				require.Equal(t, 0, len(observed.All()), "should have no logs: %v", observed.All())
			}
		})
	}
}
