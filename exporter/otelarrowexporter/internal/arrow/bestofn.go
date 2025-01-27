// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package arrow // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter/internal/arrow"

import (
	"context"
	"math/rand"
	"runtime"
	"sort"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// bestOfNPrioritizer is a prioritizer that selects a less-loaded stream to write.
// https://smallrye.io/smallrye-stork/1.1.1/load-balancer/power-of-two-choices/
type bestOfNPrioritizer struct {
	doneCancel

	// input from the pipeline, as processed data with headers and
	// a return channel for the result.  This channel is never
	// closed and is buffered.  At shutdown, items of telemetry can
	// be left in this channel, but users are expected to complete
	// their requests before calling shutdown (and the collector's
	// graph package ensures this).
	input chan writeItem

	// state tracks the work being handled by all streams.
	state []*streamWorkState

	// numChoices is the number of streams to consider in each decision.
	numChoices int

	// loadFunc is the load function.
	loadFunc loadFunc
}

type loadFunc func(*streamWorkState) float64

type streamSorter struct {
	work *streamWorkState
	load float64
}

var _ streamPrioritizer = &bestOfNPrioritizer{}

func newBestOfNPrioritizer(dc doneCancel, numChoices, numStreams int, lf loadFunc, maxLifetime time.Duration) (*bestOfNPrioritizer, []*streamWorkState) {
	var state []*streamWorkState

	// Limit numChoices to the number of streams.
	numChoices = min(numStreams, numChoices)

	for i := 0; i < numStreams; i++ {
		ws := &streamWorkState{
			maxStreamLifetime: addJitter(maxLifetime),
			waiters:           map[int64]chan<- error{},
			toWrite:           make(chan writeItem, 1),
		}

		state = append(state, ws)
	}

	lp := &bestOfNPrioritizer{
		doneCancel: dc,
		input:      make(chan writeItem, runtime.NumCPU()),
		state:      state,
		numChoices: numChoices,
		loadFunc:   lf,
	}

	for i := 0; i < numStreams; i++ {
		// TODO It's not clear if/when the prioritizer can
		// become a bottleneck.
		go lp.run()
	}

	return lp, state
}

func (lp *bestOfNPrioritizer) downgrade(ctx context.Context) {
	for _, ws := range lp.state {
		go drain(ws.toWrite, ctx.Done())
	}
}

func (lp *bestOfNPrioritizer) sendOne(item writeItem, rnd *rand.Rand, tmp []streamSorter) {
	stream := lp.streamFor(item, rnd, tmp)
	writeCh := stream.toWrite
	select {
	case writeCh <- item:
		return

	case <-lp.done:
		// All other cases: signal restart.
	}
	item.errCh <- ErrStreamRestarting
}

func (lp *bestOfNPrioritizer) run() {
	tmp := make([]streamSorter, len(lp.state))
	rnd := rand.New(rand.NewSource(rand.Int63()))
	for {
		select {
		case <-lp.done:
			return
		case item := <-lp.input:
			lp.sendOne(item, rnd, tmp)
		}
	}
}

// sendAndWait implements streamWriter
func (lp *bestOfNPrioritizer) sendAndWait(ctx context.Context, errCh <-chan error, wri writeItem) error {
	select {
	case <-lp.done:
		return ErrStreamRestarting
	case <-ctx.Done():
		return status.Errorf(codes.Canceled, "stream wait: %v", ctx.Err())
	case lp.input <- wri:
		return waitForWrite(ctx, errCh, lp.done)
	}
}

func (lp *bestOfNPrioritizer) nextWriter() streamWriter {
	select {
	case <-lp.done:
		// In case of downgrade, return nil to return into a
		// non-Arrow code path.
		return nil
	default:
		// Fall through to sendAndWait().
		return lp
	}
}

func (lp *bestOfNPrioritizer) streamFor(_ writeItem, rnd *rand.Rand, tmp []streamSorter) *streamWorkState {
	// Place all streams into the temporary slice.
	for idx, item := range lp.state {
		tmp[idx].work = item
	}
	// Select numChoices at random by shifting the selection into the start
	// of the temporary slice.
	for i := 0; i < lp.numChoices; i++ {
		pick := rnd.Intn(lp.numChoices - i)
		tmp[i], tmp[i+pick] = tmp[i+pick], tmp[i]
	}
	for i := 0; i < lp.numChoices; i++ {
		// TODO: skip channels w/ a pending item (maybe)
		tmp[i].load = lp.loadFunc(tmp[i].work)
	}
	sort.Slice(tmp[0:lp.numChoices], func(i, j int) bool {
		return tmp[i].load < tmp[j].load
	})
	return tmp[0].work
}
