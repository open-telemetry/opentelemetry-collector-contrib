// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package arrow

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestPrioritizerSendOneReleasesOnCallerCancel verifies that a
// prioritizer goroutine stops waiting to hand work to a stream once
// the caller that submitted the work has given up.
//
// The prioritizer has a fixed number of goroutines, and they are the
// only means of dispatching work to any stream.  A work state whose
// stream is absent (for example, while the stream is reconnecting)
// scores as least-loaded and is therefore preferentially selected, so
// without this bound each dispatch to it permanently consumes one
// goroutine and the exporter eventually stops dispatching entirely.
func TestPrioritizerSendOneReleasesOnCallerCancel(t *testing.T) {
	for _, pname := range AllPrioritizers {
		t.Run(string(pname), func(t *testing.T) {
			_, dc := newDoneCancel(t.Context())
			// Note: no stream is started, so nothing reads toWrite.
			// This models a work state whose stream is absent.
			prio, state := newStreamPrioritizer(dc, pname, 1, time.Minute)
			defer dc.cancel()

			// Occupy the single-item buffer so the next hand-off blocks.
			state[0].toWrite <- writeItem{records: "prefill"}

			// Submit work whose caller then gives up waiting.
			abandonedCtx, abandon := context.WithCancel(t.Context())
			abandonedCh := make(chan error, 1)
			abandonedResult := make(chan error, 1)
			go func() {
				abandonedResult <- prio.nextWriter().sendAndWait(abandonedCtx, abandonedCh, writeItem{
					records:     "abandoned",
					producerCtx: abandonedCtx,
					errCh:       abandonedCh,
				})
			}()

			// Wait for the prioritizer to pick the item up and block on
			// the full buffer, then abandon it.
			require.Eventually(t, func() bool {
				return len(prio.(*bestOfNPrioritizer).input) == 0
			}, 30*time.Second, 10*time.Millisecond, "prioritizer never picked up the work")
			abandon()

			select {
			case err := <-abandonedResult:
				require.Error(t, err)
			case <-time.After(30 * time.Second):
				t.Fatal("sendAndWait did not return after the caller was canceled")
			}

			// Free the buffer.  A goroutine still blocked handing off
			// the abandoned item would immediately take this slot.
			require.Equal(t, "prefill", (<-state[0].toWrite).records)

			// Submit new work from a caller that is still waiting.
			liveCtx, liveCancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer liveCancel()
			liveCh := make(chan error, 1)
			go func() {
				_ = prio.nextWriter().sendAndWait(liveCtx, liveCh, writeItem{
					records:     "live",
					producerCtx: liveCtx,
					errCh:       liveCh,
				})
			}()

			// The next hand-off must be the live item.  Receiving the
			// abandoned item instead means a goroutine was still
			// blocked on behalf of a caller that had already left.
			select {
			case got := <-state[0].toWrite:
				require.Equal(t, "live", got.records,
					"prioritizer delivered work on behalf of a caller that had given up")
			case <-time.After(30 * time.Second):
				t.Fatal("prioritizer stopped dispatching; a goroutine leaked in sendOne")
			}
		})
	}
}
