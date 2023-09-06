// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package arrow // import "github.com/open-telemetry/otel-arrow/collector/exporter/otelarrowexporter/internal/arrow"

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var ErrStreamRestarting = status.Error(codes.Aborted, "stream is restarting")

// streamPrioritizer is a placeholder for a configurable mechanism
// that selects the next stream to write.
type streamPrioritizer struct {
	// done corresponds with the background context Done channel..
	done <-chan struct{}

	// channel will be closed to downgrade to standard OTLP,
	// otherwise it returns the first-available.
	channel chan *Stream
}

// newStreamPrioritizer constructs a channel-based first-available prioritizer.
func newStreamPrioritizer(bgctx context.Context, numStreams int) *streamPrioritizer {
	return &streamPrioritizer{
		done:    bgctx.Done(),
		channel: make(chan *Stream, numStreams),
	}
}

// downgrade indicates that streams are never going to be ready.  Note
// the caller is required to ensure that setReady() and removeReady()
// cannot be called concurrently; this is done by waiting for
// Stream.writeStream() calls to return before downgrading.
func (sp *streamPrioritizer) downgrade() {
	close(sp.channel)
}

// readyChannel returns channel to select a ready stream.  The caller
// is expected to select on this and ctx.Done() simultaneously.  If
// the exporter is downgraded, the channel will be closed.
func (sp *streamPrioritizer) readyChannel() chan *Stream {
	return sp.channel
}

// setReady marks this stream ready for use.
func (sp *streamPrioritizer) setReady(stream *Stream) {
	// Note: downgrade() can't be called concurrently.
	sp.channel <- stream
}

// removeReady removes this stream from the ready set, used in cases
// where the stream has broken unexpectedly.
func (sp *streamPrioritizer) removeReady(stream *Stream) {
	// Note: downgrade() can't be called concurrently.
	for {
		// Searching for this stream to get it out of the ready queue.
		select {
		case <-sp.done:
			// Shutdown case
			return
		case alternate := <-sp.channel:
			if alternate == stream {
				// Success: removed from ready queue.
				return
			}
			sp.channel <- alternate
		case wri := <-stream.toWrite:
			// A consumer got us first, means this stream has been removed
			// from the ready queue.
			//
			// Note: the top-level OTLP exporter will retry.
			wri.errCh <- ErrStreamRestarting
			return
		}
	}
}
