// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package arrow // import "github.com/open-telemetry/otel-arrow/collector/exporter/otelarrowexporter/internal/arrow"

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	arrowpb "github.com/open-telemetry/otel-arrow/api/experimental/arrow/v1"
	arrowRecord "github.com/open-telemetry/otel-arrow/pkg/otel/arrow_record"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"go.opentelemetry.io/collector/component"
)

// Exporter is 1:1 with exporter, isolates arrow-specific
// functionality.
type Exporter struct {
	// numStreams is the number of streams that will be used.
	numStreams int

	maxStreamLifetime time.Duration

	// disableDowngrade prevents downgrade from occurring, supports
	// forcing Arrow transport.
	disableDowngrade bool

	// telemetry includes logger, tracer, meter.
	telemetry component.TelemetrySettings

	// grpcOptions includes options used by the unary RPC methods,
	// e.g., WaitForReady.
	grpcOptions []grpc.CallOption

	// newProducer returns a real (or mock) Producer.
	newProducer func() arrowRecord.ProducerAPI

	// client is a stream corresponding with the signal's payload
	// type. uses the exporter's gRPC ClientConn (or is a mock, in tests).
	streamClient StreamClientFunc

	// perRPCCredentials derived from the exporter's gRPC auth settings.
	perRPCCredentials credentials.PerRPCCredentials

	// returning is used to pass broken, gracefully-terminated,
	// and otherwise to the stream controller.
	returning chan *Stream

	// ready prioritizes streams that are ready to send
	ready *streamPrioritizer

	// cancel cancels the background context of this
	// Exporter, used for shutdown.
	cancel context.CancelFunc

	// wg counts one per active goroutine belonging to all strings
	// of this exporter.  The wait group has Add(1) called before
	// starting goroutines so that they can be properly waited for
	// in shutdown(), so the pattern is:
	//
	//   wg.Add(1)
	//   go func() {
	//     defer wg.Done()
	//     ...
	//   }()
	wg sync.WaitGroup
}

// AnyStreamClient is the interface supported by all Arrow streams,
// mixed signals or not.
type AnyStreamClient interface {
	Send(*arrowpb.BatchArrowRecords) error
	Recv() (*arrowpb.BatchStatus, error)
	grpc.ClientStream
}

// streamClientFunc is a constructor for AnyStreamClients.
type StreamClientFunc func(context.Context, ...grpc.CallOption) (AnyStreamClient, error)

// MakeAnyStreamClient accepts any Arrow-like stream (mixed signal or
// not) and turns it into an AnyStreamClient.
func MakeAnyStreamClient[T AnyStreamClient](clientFunc func(ctx context.Context, opts ...grpc.CallOption) (T, error)) StreamClientFunc {
	return func(ctx context.Context, opts ...grpc.CallOption) (AnyStreamClient, error) {
		client, err := clientFunc(ctx, opts...)
		return client, err
	}
}

// NewExporter configures a new Exporter.
func NewExporter(
	maxStreamLifetime time.Duration,
	numStreams int,
	disableDowngrade bool,
	telemetry component.TelemetrySettings,
	grpcOptions []grpc.CallOption,
	newProducer func() arrowRecord.ProducerAPI,
	streamClient StreamClientFunc,
	perRPCCredentials credentials.PerRPCCredentials,
) *Exporter {
	return &Exporter{
		maxStreamLifetime: maxStreamLifetime,
		numStreams:        numStreams,
		disableDowngrade:  disableDowngrade,
		telemetry:         telemetry,
		grpcOptions:       grpcOptions,
		newProducer:       newProducer,
		streamClient:      streamClient,
		perRPCCredentials: perRPCCredentials,
		returning:         make(chan *Stream, numStreams),
	}
}

// Start creates the background context used by all streams and starts
// a stream controller, which initializes the initial set of streams.
func (e *Exporter) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)

	e.cancel = cancel
	e.wg.Add(1)
	e.ready = newStreamPrioritizer(ctx, e.numStreams)

	go e.runStreamController(ctx)

	return nil
}

// runStreamController starts the initial set of streams, then waits for streams to
// terminate one at a time and restarts them.  If streams come back with a nil
// client (meaning that OTLP+Arrow was not supported by the endpoint), it will
// not be restarted.
func (e *Exporter) runStreamController(bgctx context.Context) {
	defer e.cancel()
	defer e.wg.Done()

	running := e.numStreams

	// Start the initial number of streams
	for i := 0; i < running; i++ {
		e.wg.Add(1)
		go e.runArrowStream(bgctx)
	}

	for {
		select {
		case stream := <-e.returning:
			if stream.client != nil || e.disableDowngrade {
				// The stream closed or broken.  Restart it.
				e.wg.Add(1)
				go e.runArrowStream(bgctx)
				continue
			}
			// Otherwise, the stream never got started.  It was
			// downgraded and senders will use the standard OTLP path.
			running--

			// None of the streams were able to connect to
			// an Arrow endpoint.
			if running == 0 {
				e.telemetry.Logger.Info("could not establish arrow streams, downgrading to standard OTLP export")
				e.ready.downgrade()
			}

		case <-bgctx.Done():
			// We are shutting down.
			return
		}
	}
}

// addJitter is used to subtract 0-5% from max_stream_lifetime.  Since
// the max_stream_lifetime value is expected to be close to the
// receiver's max_connection_age_grace setting, we do not add jitter,
// only subtract.
func addJitter(v time.Duration) time.Duration {
	if v == 0 {
		return 0
	}
	return v - time.Duration(rand.Int63n(int64(v/20)))
}

// runArrowStream begins one gRPC stream using a child of the background context.
// If the stream connection is successful, this goroutine starts another goroutine
// to call writeStream() and performs readStream() itself.  When the stream shuts
// down this call synchronously waits for and unblocks the consumers.
func (e *Exporter) runArrowStream(ctx context.Context) {
	producer := e.newProducer()

	stream := newStream(producer, e.ready, e.telemetry, e.perRPCCredentials)
	stream.maxStreamLifetime = addJitter(e.maxStreamLifetime)

	defer func() {
		if err := producer.Close(); err != nil {
			e.telemetry.Logger.Error("arrow producer close:", zap.Error(err))
		}
		e.wg.Done()
		e.returning <- stream
	}()

	stream.run(ctx, e.streamClient, e.grpcOptions)
}

// SendAndWait tries to send using an Arrow stream.  The results are:
//
// (true, nil):      Arrow send: success at consumer
// (false, nil):     Arrow is not supported by the server, caller expected to fallback.
// (true, non-nil):  Arrow send: server response may be permanent or allow retry.
// (false, non-nil): Context timeout prevents retry.
//
// consumer should fall back to standard OTLP, (true, nil)
func (e *Exporter) SendAndWait(ctx context.Context, data interface{}) (bool, error) {
	for {
		var stream *Stream
		var err error
		select {
		case <-ctx.Done():
			err = ctx.Err()
		case stream = <-e.ready.readyChannel():
		}

		if err != nil {
			return false, err // a Context error
		}
		if stream == nil {
			return false, nil // a downgraded connection
		}

		err = stream.SendAndWait(ctx, data)
		if err != nil && errors.Is(err, ErrStreamRestarting) {
			continue // an internal retry

		}
		// result from arrow server (may be nil, may be
		// permanent, etc.)
		return true, err
	}
}

// Shutdown returns when all Arrow-associated goroutines have returned.
func (e *Exporter) Shutdown(_ context.Context) error {
	e.cancel()
	e.wg.Wait()
	return nil
}
