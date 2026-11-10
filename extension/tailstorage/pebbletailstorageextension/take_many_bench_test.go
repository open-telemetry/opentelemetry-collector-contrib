// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !aix && !solaris

package pebbletailstorageextension

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// BenchmarkTickRead times one decision tick's worth of reads: every pending
// trace is taken once, either with one Take per trace or with one TakeMany.
func BenchmarkTickRead(b *testing.B) {
	for _, numTraces := range []int{100, 1000, 10000} {
		for _, mode := range []string{"take", "take_many"} {
			b.Run(fmt.Sprintf("traces_%d/%s", numTraces, mode), func(b *testing.B) {
				benchmarkTickRead(b, numTraces, mode == "take_many")
			})
		}
	}
}

func benchmarkTickRead(b *testing.B, numTraces int, batch bool) {
	s, err := newStorage(b.Context(), &Config{Directory: b.TempDir()}, zap.NewNop(), nil)
	require.NoError(b, err)
	b.Cleanup(func() { require.NoError(b, s.Close()) })

	const batchesPerTrace = 3
	ids := make([]pcommon.TraceID, numTraces)
	payloads := make([]ptrace.Traces, numTraces)
	for i := range ids {
		var id pcommon.TraceID
		binary.BigEndian.PutUint64(id[8:], uint64(i*7919+1))
		ids[i] = id
		td := ptrace.NewTraces()
		span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetTraceID(id)
		span.SetName("bench")
		span.Attributes().PutStr("payload", string(make([]byte, 512)))
		payloads[i] = td
	}
	// Traces arrive in an order unrelated to their key order, as in the processor.
	tickOrder := make([]pcommon.TraceID, numTraces)
	for i := range ids {
		tickOrder[i] = ids[(i*131)%numTraces]
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		b.StopTimer()
		for i, id := range ids {
			for range batchesPerTrace {
				td := ptrace.NewTraces()
				payloads[i].CopyTo(td)
				require.NoError(b, s.Append(id, td))
			}
		}
		b.StartTimer()

		if batch {
			out, errs := s.TakeMany(tickOrder)
			for i := range out {
				if errs[i] != nil || out[i].ResourceSpans().Len() != batchesPerTrace {
					b.Fatalf("trace %d: err=%v spans=%d", i, errs[i], out[i].ResourceSpans().Len())
				}
			}
			continue
		}
		for _, id := range tickOrder {
			td, err := s.Take(id)
			if err != nil || td.ResourceSpans().Len() != batchesPerTrace {
				b.Fatalf("trace %s: err=%v spans=%d", id, err, td.ResourceSpans().Len())
			}
		}
	}
}
