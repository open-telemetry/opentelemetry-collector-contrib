// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package arrow // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter/internal/arrow"

import (
	"context"
)

// fifoPrioritizer is a prioritizer that selects the next stream to write.
// It is the simplest prioritizer implementation.
type fifoPrioritizer struct {
	doneCancel

	// shared is shared by all streams.  will be closed to
	// downgrade to standard OTLP, otherwise it returns the
	// first-available.
	shared chan writeItem
}

var _ streamPrioritizer = &fifoPrioritizer{}

// newFifoPrioritizer constructs a channel-based first-available prioritizer.
func newFifoPrioritizer(dc doneCancel, numStreams int) (*fifoPrioritizer, []*streamWorkState) {
	var state []*streamWorkState

	shared := make(chan writeItem, numStreams)

	for i := 0; i < numStreams; i++ {
		state = append(state, &streamWorkState{
			waiters: map[int64]chan<- error{},
			toWrite: shared,
		})
	}

	return &fifoPrioritizer{
		doneCancel: dc,
		shared:     shared,
	}, state
}

// downgrade indicates that streams are never going to be ready.  Note
// the caller is required to ensure that setReady() and unsetReady()
// cannot be called concurrently; this is done by waiting for
// Stream.writeStream() calls to return before downgrading.
func (fp *fifoPrioritizer) downgrade(ctx context.Context) {
	go drain(fp.shared, ctx.Done())
}

// nextWriter returns the first-available stream.
func (fp *fifoPrioritizer) nextWriter(ctx context.Context) streamWriter {
	select {
	case <-fp.done:
		// In case of downgrade, return nil to return into a
		// non-Arrow code path.
		return nil
	default:
		// Fall through to sendAndWait().
		return fp
	}
}

func (fp *fifoPrioritizer) sendAndWait(ctx context.Context, errCh <-chan error, wri writeItem) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case fp.shared <- wri:
		return waitForWrite(ctx, errCh, fp.done)
	case <-fp.done:
		return ErrStreamRestarting
	}
}
