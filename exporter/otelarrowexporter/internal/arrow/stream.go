// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package arrow // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter/internal/arrow"

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"time"

	arrowpb "github.com/open-telemetry/otel-arrow/go/api/experimental/arrow/v1"
	arrowRecord "github.com/open-telemetry/otel-arrow/go/pkg/otel/arrow_record"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"golang.org/x/net/http2/hpack"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/netstats"
)

// Stream is 1:1 with gRPC stream.
type Stream struct {
	// producer is exclusive to the holder of the stream.
	producer arrowRecord.ProducerAPI

	// prioritizer has a reference to the stream, this allows it to be severed.
	prioritizer streamPrioritizer

	// telemetry are a copy of the exporter's telemetry settings
	telemetry component.TelemetrySettings

	// tracer is used to create a span describing the export.
	tracer trace.Tracer

	// client uses the exporter's grpc.ClientConn.  this is
	// initially nil only set when ArrowStream() calls meaning the
	// endpoint recognizes OTel-Arrow.
	client AnyStreamClient

	// method the gRPC method name, used for additional instrumentation.
	method string

	// netReporter provides network-level metrics.
	netReporter netstats.Interface

	// streamWorkState is the interface to prioritizer/balancer, contains
	// outstanding request (by batch ID) and the write channel used by
	// the stream.  All of this state will be inherited by the successor
	// stream.
	workState *streamWorkState
}

// streamWorkState contains the state assigned to an Arrow stream.  When
// a stream shuts down, the work state is handed to the replacement stream.
type streamWorkState struct {
	// toWrite is used to pass pending data between a caller, the
	// prioritizer and a stream.
	toWrite chan writeItem

	// maxStreamLifetime is a limit on duration for streams.  A
	// slight "jitter" is applied relative to this value on a
	// per-stream basis.
	maxStreamLifetime time.Duration

	// lock protects waiters
	lock sync.Mutex

	// waiters is the response channel for each active batch.
	waiters map[int64]chan<- error
}

// writeItem is passed from the sender (a pipeline consumer) to the
// stream writer, which is not bound by the sender's context.
type writeItem struct {
	// records is a ptrace.Traces, plog.Logs, or pmetric.Metrics
	records any
	// md is the caller's metadata, derived from its context.
	md map[string]string
	// errCh is used by the stream reader to unblock the sender
	// to the stream side, this is a `chan<-`. to the send side,
	// this is a `<-chan`.
	errCh chan<- error
	// uncompSize is computed by the appropriate sizer (in the
	// caller's goroutine)
	uncompSize int
	// producerCtx is used for tracing purposes.
	producerCtx context.Context
}

// newStream constructs a stream
func newStream(
	producer arrowRecord.ProducerAPI,
	prioritizer streamPrioritizer,
	telemetry component.TelemetrySettings,
	netReporter netstats.Interface,
	workState *streamWorkState,
) *Stream {
	tracer := telemetry.TracerProvider.Tracer("otel-arrow-exporter")
	return &Stream{
		producer:    producer,
		prioritizer: prioritizer,
		telemetry:   telemetry,
		tracer:      tracer,
		netReporter: netReporter,
		workState:   workState,
	}
}

// setBatchChannel places a waiting consumer's batchID into the waiters map, where
// the stream reader may find it.
func (s *Stream) setBatchChannel(batchID int64, errCh chan<- error) {
	s.workState.lock.Lock()
	defer s.workState.lock.Unlock()

	s.workState.waiters[batchID] = errCh
}

// logStreamError decides how to log an error.  `where` indicates the
// error location, will be "reader" or "writer".
func (s *Stream) logStreamError(where string, err error) {
	var code codes.Code
	var msg string
	// gRPC tends to supply status-wrapped errors, so we always
	// unpack them.  A wrapped Canceled code indicates intentional
	// shutdown, which can be due to normal causes (EOF, e.g.,
	// max-stream-lifetime reached) or unusual causes (Canceled,
	// e.g., because the other stream direction reached an error).
	if status, ok := status.FromError(err); ok {
		code = status.Code()
		msg = status.Message()
	} else if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
		code = codes.Canceled
		msg = err.Error()
	} else {
		code = codes.Internal
		msg = err.Error()
	}
	if code == codes.Canceled {
		s.telemetry.Logger.Debug("arrow stream shutdown", zap.String("message", msg), zap.String("where", where))
	} else {
		s.telemetry.Logger.Error("arrow stream error", zap.Int("code", int(code)), zap.String("message", msg), zap.String("where", where))
	}
}

// run blocks the calling goroutine while executing stream logic.  run
// will return when the reader and writer are finished.  errors will be logged.
func (s *Stream) run(ctx context.Context, dc doneCancel, streamClient StreamClientFunc, grpcOptions []grpc.CallOption) {
	sc, method, err := streamClient(ctx, grpcOptions...)
	if err != nil {
		// Returning with stream.client == nil signals the
		// lack of an Arrow stream endpoint.  When all the
		// streams return with .client == nil, the ready
		// channel will be closed, which causes downgrade.
		//
		// Note: These are gRPC server internal errors and
		// will cause downgrade to standard OTLP.  These
		// cannot be simulated by connecting to a gRPC server
		// that does not support the ArrowStream service, with
		// or without the WaitForReady flag set.  In a real
		// gRPC server the first Unimplemented code is
		// generally delivered to the Recv() call below, so
		// this code path is not taken for an ordinary downgrade.
		s.telemetry.Logger.Error("cannot start arrow stream", zap.Error(err))
		return
	}
	// Setting .client != nil indicates that the endpoint was valid,
	// streaming may start.  When this stream finishes, it will be
	// restarted.
	s.method = method
	s.client = sc

	// ww is used to wait for the writer.  Since we wait for the writer,
	// the writer's goroutine is not added to exporter waitgroup (e.wg).
	var ww sync.WaitGroup

	var writeErr error
	ww.Go(func() {
		writeErr = s.write(ctx)
		if writeErr != nil {
			dc.cancel()
		}
	})

	// the result from read() is processed after cancel and wait,
	// so we can set s.client = nil in case of a delayed Unimplemented.
	err = s.read(ctx)

	// Wait for the writer to ensure that all waiters are known.
	dc.cancel()
	ww.Wait()

	if err != nil {
		// This branch is reached with an unimplemented status
		// with or without the WaitForReady flag.
		if status, ok := status.FromError(err); ok && status.Code() == codes.Unimplemented {
			// This (client == nil) signals the controller to
			// downgrade when all streams have returned in that
			// status.
			//
			// This is a special case because we reset s.client,
			// which sets up a downgrade after the streams return.
			s.client = nil
			s.telemetry.Logger.Info("arrow is not supported",
				zap.String("message", status.Message()),
			)
		} else {
			// All other cases, use the standard log handler.
			s.logStreamError("reader", err)
		}
	}
	if writeErr != nil {
		s.logStreamError("writer", writeErr)
	}

	s.workState.lock.Lock()
	defer s.workState.lock.Unlock()

	// The reader and writer have both finished; respond to any
	// outstanding waiters.
	for _, ch := range s.workState.waiters {
		// Note: the top-level OTLP exporter will retry.
		ch <- ErrStreamRestarting
	}

	s.workState.waiters = map[int64]chan<- error{}
}

// write repeatedly places this stream into the next-available queue, then
// performs a blocking send().  This returns when the data is in the write buffer,
// the caller waiting on its error channel.
func (s *Stream) write(ctx context.Context) (retErr error) {
	// always close the send channel when this function returns.
	defer func() { _ = s.client.CloseSend() }()

	// headers are encoding using hpack, reusing a buffer on each call.
	var hdrsBuf bytes.Buffer
	hdrsEnc := hpack.NewEncoder(&hdrsBuf)

	var timerCh <-chan time.Time
	if s.workState.maxStreamLifetime != 0 {
		timer := time.NewTimer(s.workState.maxStreamLifetime)
		timerCh = timer.C
		defer timer.Stop()
	}

	for {
		// this can block, and if the context is canceled we
		// wait for the reader to find this stream.
		var wri writeItem
		select {
		case <-timerCh:
			return nil
		case wri = <-s.workState.toWrite:
		case <-ctx.Done():
			return status.Errorf(codes.Canceled, "stream input: %v", ctx.Err())
		}

		err := s.encodeAndSend(wri, &hdrsBuf, hdrsEnc)
		if err != nil {
			// Note: For the return statement below, there is no potential
			// sender race because the stream is not available, as indicated by
			// the successful <-stream.toWrite above
			return err
		}
	}
}

func (s *Stream) encodeAndSend(wri writeItem, hdrsBuf *bytes.Buffer, hdrsEnc *hpack.Encoder) (retErr error) {
	ctx, span := s.tracer.Start(wri.producerCtx, "otel_arrow_stream_send")
	defer span.End()

	defer func() {
		// Set span status if an error is returned.
		if retErr != nil {
			span := trace.SpanFromContext(ctx)
			span.SetStatus(otelcodes.Error, retErr.Error())
		}
	}()
	// Get the global propagator, to inject context.  When there
	// are no fields, it's a no-op propagator implementation and
	// we can skip the allocations inside this block.
	prop := otel.GetTextMapPropagator()

	if len(prop.Fields()) > 0 {
		// When the incoming context carries nothing, the map
		// will be nil.  Allocate, if necessary.
		if wri.md == nil {
			wri.md = map[string]string{}
		}
		// Use the global propagator to inject trace context.  Note that
		// OpenTelemetry Collector will set a global propagator from the
		// service::telemetry::traces configuration.
		otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(wri.md))
	}

	batch, err := s.encode(wri.records)
	if err != nil {
		// This is some kind of internal error.  We will restart the
		// stream and mark this record as a permanent one.
		err = status.Errorf(codes.Internal, "encode: %v", err)
		wri.errCh <- err
		return err
	}

	// Optionally include outgoing metadata, if present.
	if len(wri.md) != 0 {
		hdrsBuf.Reset()
		for key, val := range wri.md {
			err := hdrsEnc.WriteField(hpack.HeaderField{
				Name:  key,
				Value: val,
			})
			if err != nil {
				// This case is like the encode-failure case
				// above, we will restart the stream but consider
				// this a permanent error.
				err = status.Errorf(codes.Internal, "hpack: %v", err)
				wri.errCh <- err
				return err
			}
		}
		batch.Headers = hdrsBuf.Bytes()
	}

	// Let the receiver knows what to look for.
	s.setBatchChannel(batch.BatchId, wri.errCh)

	// The netstats code knows that uncompressed size is
	// unreliable for arrow transport, so we instrument it
	// directly here.  Only the primary direction of transport
	// is instrumented this way.
	var sized netstats.SizesStruct
	sized.Method = s.method
	sized.Length = int64(wri.uncompSize)
	s.netReporter.CountSend(ctx, sized)

	if err := s.client.Send(batch); err != nil {
		// The error will be sent to errCh during cleanup for this stream.
		// Note: do not wrap this error, it may contain a Status.
		return err
	}

	return nil
}

// read repeatedly reads a batch status and releases the consumers waiting for
// a response.
func (s *Stream) read(_ context.Context) error {
	// Note we do not use the context, the stream context might
	// cancel a call to Recv() but the call to processBatchStatus
	// is non-blocking.
	for {
		// Note: if the client has called CloseSend() and is waiting for a response from the server.
		// And if the server fails for some reason, we will wait until some other condition, such as a context
		// timeout.  TODO: possibly, improve to wait for no outstanding requests and then stop reading.
		resp, err := s.client.Recv()
		if err != nil {
			// Note: do not wrap, contains a Status.
			return err
		}

		err = s.processBatchStatus(resp)
		if err != nil {
			return err
		}
	}
}

// getSenderChannel takes the stream lock and removes the corresponding
// sender channel.
func (sws *streamWorkState) getSenderChannel(bstat *arrowpb.BatchStatus) (chan<- error, error) {
	sws.lock.Lock()
	defer sws.lock.Unlock()

	ch, ok := sws.waiters[bstat.BatchId]
	if !ok {
		// Will break the stream.
		return nil, status.Errorf(codes.Internal, "unrecognized batch ID: %d", bstat.BatchId)
	}

	delete(sws.waiters, bstat.BatchId)
	return ch, nil
}

// processBatchStatus processes a single response from the server and unblocks the
// associated sender.
func (s *Stream) processBatchStatus(ss *arrowpb.BatchStatus) error {
	ch, ret := s.workState.getSenderChannel(ss)

	if ch == nil {
		// In case getSenderChannels encounters a problem, the
		// channel is nil.
		return ret
	}

	if ss.StatusCode == arrowpb.StatusCode_OK {
		ch <- nil
		return nil
	}
	// See ../../otelarrow.go's `shouldRetry()` method, the retry
	// behavior described here is achieved there by setting these
	// recognized codes.
	var err error
	switch ss.StatusCode {
	case arrowpb.StatusCode_UNAVAILABLE:
		// Retryable
		err = status.Errorf(codes.Unavailable, "destination unavailable: %d: %s", ss.BatchId, ss.StatusMessage)
	case arrowpb.StatusCode_INVALID_ARGUMENT:
		// Not retryable
		err = status.Errorf(codes.InvalidArgument, "invalid argument: %d: %s", ss.BatchId, ss.StatusMessage)
	case arrowpb.StatusCode_RESOURCE_EXHAUSTED:
		// Retry behavior is configurable
		err = status.Errorf(codes.ResourceExhausted, "resource exhausted: %d: %s", ss.BatchId, ss.StatusMessage)
	default:
		// Note: a Canceled StatusCode was once returned by receivers following
		// a CloseSend() from the exporter.  This is now handled using error
		// status codes.  If an exporter is upgraded before a receiver, the exporter
		// will log this error when the receiver closes streams.

		// Unrecognized status code.
		err = status.Errorf(codes.Internal, "unexpected stream response: %d: %s", ss.BatchId, ss.StatusMessage)

		// Will break the stream.
		ret = multierr.Append(ret, err)
	}
	ch <- err
	return ret
}

// encode produces the next batch of Arrow records.
func (s *Stream) encode(records any) (_ *arrowpb.BatchArrowRecords, retErr error) {
	// Defensively, protect against panics in the Arrow producer function.
	defer func() {
		if err := recover(); err != nil {
			// When this happens, the stacktrace is
			// important and lost if we don't capture it
			// here.
			s.telemetry.Logger.Debug("panic detail in otel-arrow-adapter",
				zap.Reflect("recovered", err),
				zap.Stack("stacktrace"),
			)
			retErr = status.Errorf(codes.Internal, "panic in otel-arrow-adapter: %v", err)
		}
	}()
	var batch *arrowpb.BatchArrowRecords
	var err error
	switch data := records.(type) {
	case ptrace.Traces:
		batch, err = s.producer.BatchArrowRecordsFromTraces(data)
	case plog.Logs:
		batch, err = s.producer.BatchArrowRecordsFromLogs(data)
	case pmetric.Metrics:
		batch, err = s.producer.BatchArrowRecordsFromMetrics(data)
	default:
		return nil, status.Errorf(codes.Unimplemented, "unsupported OTel-Arrow signal type: %T", records)
	}
	return batch, err
}
