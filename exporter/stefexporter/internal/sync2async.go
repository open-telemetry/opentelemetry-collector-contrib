// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stefexporter/internal"

import (
	"context"

	"go.uber.org/zap"
)

// Async is a function that executes an operation asynchronously.
// It takes some data and a channel to report the result.
// It returns a DataID, that will be reported via resultChan when
// the operation completes.
type Async func(
	ctx context.Context,
	data any,
	resultChan ResultChan,
) (DataID, error)

// DataID identifies the result of an asynchronous operation.
// Since Async function can be called repeatedly, the DataID is used to differentiate
// which of the operations started via Async() call have completed when reported
// via resultChan.
type DataID uint64

// AsyncResult is the result of an asynchronous operation completion.
type AsyncResult struct {
	// DataID is the ID of the data that was processed. This matches the DataID
	// returned by the Async function.
	DataID DataID

	// Err is the error that occurred during the processing of the data.
	// Equals to nil if the operation completed successfully.
	Err error
}

// ResultChan is a channel used to report the result of an asynchronous
// operation completion.
type ResultChan chan AsyncResult

// Sync2Async is an API converter that allows an asynchronous implementation
// of an Async function to be called synchronously.
type Sync2Async struct {
	logger *zap.Logger

	// The Async function to be called asynchronously.
	async Async

	// resultChannelsRing is a ring of channels that are used to report
	// the result of the async operation. The ring is used to limit the
	// number of concurrent async operations that can be in flight.
	resultChannelsRing chan chan AsyncResult
}

// NewSync2Async creates a new Sync2Async instance.
//
// concurrency is the number of concurrent calls that can be in flight.
// If more than concurrency DoSync() calls are made, the DoSync() call will block
// until one of the previous calls completes and returns a result.
func NewSync2Async(logger *zap.Logger, concurrency int, async Async) *Sync2Async {
	s := &Sync2Async{
		logger:             logger,
		async:              async,
		resultChannelsRing: make(chan chan AsyncResult, concurrency),
	}

	for i := 0; i < concurrency; i++ {
		// We need 1 element in the channel to make sure reporting the results via channel is not
		// blocked when the recipient of the channel gave up.
		s.resultChannelsRing <- make(chan AsyncResult, 1)
	}

	return s
}

// DoSync performs a synchronous operation. It will trigger the execution of the
// provided Async operation with supplied data and will block until the async
// operation completes (until the resultChan receives a result).
//
// If the number of calls to DoSync() exceeds the concurrency limit, the caller will block
// until one of the previous calls completes. This means that even if the number of
// concurrently executing DoSync() calls exceeds concurrency limit, the number of Async calls
// will never exceed the concurrency limit.
//
// Canceling the ctx will cause the async operation to be abandoned and error
// to be returned. The ctx is also passed to the Async function.
// If ctx is cancelled before the async operation is started (e.g. due to being
// blocked on concurrency limit) then an error will be returned as well.
func (s *Sync2Async) DoSync(ctx context.Context, data any) error {
	// Acquire a resultChan from the ring of channels if one is available.
	var resultChan chan AsyncResult
	select {
	case resultChan = <-s.resultChannelsRing:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() {
		// Put it back into the ring of channels when we are done with it.
		s.resultChannelsRing <- resultChan
	}()

	// Begin async operation. This will return immediately and the result
	// will be reported via the resultChan some time in the future.
	dataID, err := s.async(ctx, data, resultChan)
	if err != nil {
		return err
	}

	// Now we need to wait for the result of the async operation.
	select {
	case result := <-resultChan:
		// Async operation completed. We can return the result.
		if result.DataID != dataID {
			// Received ack on the wrong data item. This should normally not happen and indicates a bug somewhere.
			s.logger.Error(
				"Received ack on the wrong data item",
				zap.Uint64("expected", uint64(dataID)),
				zap.Uint64("actual", uint64(result.DataID)),
			)
		}
		return result.Err

	case <-ctx.Done():
		// Async operation is still executing, but we have to abandon it and return to our
		// caller immediately.
		// Abandon the ack channel that was given to async() because we don't know when/if
		// the Async operation will complete and that channel will fire, so we don't want to
		// touch it anymore.
		// Just allocate a new channel. The new resultChan will be returned to resultChannelsRing
		// when this func returns.
		// If after this the previously started Async operation completes, it will push the result
		// into an abandoned channel, which will have no effect.
		// Note that Async can push into an abandoned channel without blocking since the channel
		// has 1 buffered element.
		resultChan = make(chan AsyncResult)
		return ctx.Err()
	}
}
