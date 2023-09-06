// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package arrow // import "github.com/open-telemetry/otel-arrow/collector/exporter/otelarrowexporter/internal/arrow"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	arrowpb "github.com/open-telemetry/otel-arrow/api/experimental/arrow/v1"
	arrowRecord "github.com/open-telemetry/otel-arrow/pkg/otel/arrow_record"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"golang.org/x/net/http2/hpack"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// Stream is 1:1 with gRPC stream.
type Stream struct {
	// maxStreamLifetime is the max timeout before stream
	// should be closed on the client side. This ensures a
	// graceful shutdown before max_connection_age is reached
	// on the server side.
	maxStreamLifetime time.Duration

	// producer is exclusive to the holder of the stream.
	producer arrowRecord.ProducerAPI

	// prioritizer has a reference to the stream, this allows it to be severed.
	prioritizer *streamPrioritizer

	// perRPCCredentials from the auth extension, or nil.
	perRPCCredentials credentials.PerRPCCredentials

	// telemetry are a copy of the exporter's telemetry settings
	telemetry component.TelemetrySettings

	// client uses the exporter's grpc.ClientConn.  this is
	// initially nil only set when ArrowStream() calls meaning the
	// endpoint recognizes OTLP+Arrow.
	client arrowpb.ArrowStreamService_ArrowStreamClient

	// toWrite is passes a batch from the sender to the stream writer, which
	// includes a dedicated channel for the response.
	toWrite chan writeItem

	// lock protects waiters.
	lock sync.Mutex

	// waiters is the response channel for each active batch.
	waiters map[int64]chan error
}

// writeItem is passed from the sender (a pipeline consumer) to the
// stream writer, which is not bound by the sender's context.
type writeItem struct {
	// records is a ptrace.Traces, plog.Logs, or pmetric.Metrics
	records interface{}
	// md is the caller's metadata, derived from its context.
	md map[string]string
	// errCh is used by the stream reader to unblock the sender
	errCh chan error
}

// newStream constructs a stream
func newStream(
	producer arrowRecord.ProducerAPI,
	prioritizer *streamPrioritizer,
	telemetry component.TelemetrySettings,
	perRPCCredentials credentials.PerRPCCredentials,
) *Stream {
	return &Stream{
		producer:          producer,
		prioritizer:       prioritizer,
		perRPCCredentials: perRPCCredentials,
		telemetry:         telemetry,
		toWrite:           make(chan writeItem, 1),
		waiters:           map[int64]chan error{},
	}
}

// setBatchChannel places a waiting consumer's batchID into the waiters map, where
// the stream reader may find it.
func (s *Stream) setBatchChannel(batchID int64, errCh chan error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.waiters[batchID] = errCh
}

func (s *Stream) logStreamError(err error) {
	isEOF := errors.Is(err, io.EOF)
	isCanceled := errors.Is(err, context.Canceled)

	switch {
	case !isEOF && !isCanceled:
		s.telemetry.Logger.Error("arrow stream error", zap.Error(err))
	case isEOF:
		s.telemetry.Logger.Debug("arrow stream end")
	case isCanceled:
		s.telemetry.Logger.Debug("arrow stream canceled")
	}
}

// run blocks the calling goroutine while executing stream logic.  run
// will return when the reader and writer are finished.  errors will be logged.
func (s *Stream) run(bgctx context.Context, streamClient StreamClientFunc, grpcOptions []grpc.CallOption) {
	ctx, cancel := context.WithCancel(bgctx)
	defer cancel()

	sc, err := streamClient(ctx, grpcOptions...)
	if err != nil {
		// Returning with stream.client == nil signals the
		// lack of an Arrow stream endpoint.  When all the
		// streams return with .client == nil, the ready
		// channel will be closed.
		//
		// Note: These are gRPC server internal errors and
		// will cause downgrade to standard OTLP.  These
		// cannot be simulated by connecting to a gRPC server
		// that does not support the ArrowStream service, with
		// or without the WaitForReady flag set.  In a real
		// gRPC server the first Unimplemented code is
		// generally delivered to the Recv() call below, so
		// this code path is not taken for an ordinary downgrade.
		//
		// TODO: a more graceful recovery strategy?
		s.telemetry.Logger.Error("cannot start arrow stream", zap.Error(err))
		return
	}
	// Setting .client != nil indicates that the endpoint was valid,
	// streaming may start.  When this stream finishes, it will be
	// restarted.
	s.client = sc

	// ww is used to wait for the writer.  Since we wait for the writer,
	// the writer's goroutine is not added to exporter waitgroup (e.wg).
	var ww sync.WaitGroup

	var writeErr error
	ww.Add(1)
	go func() {
		defer ww.Done()
		writeErr = s.write(ctx)
		if writeErr != nil {
			cancel()
		}
	}()

	// the result from read() is processed after cancel and wait,
	// so we can set s.client = nil in case of a delayed Unimplemented.
	err = s.read(ctx)

	// Wait for the writer to ensure that all waiters are known.
	cancel()
	ww.Wait()

	if err != nil {
		// This branch is reached with an unimplemented status
		// with or without the WaitForReady flag.
		status, ok := status.FromError(err)

		if ok {
			switch status.Code() {
			case codes.Unimplemented:
				// This (client == nil) signals the controller
				// to downgrade when all streams have returned
				// in that status.
				//
				// TODO: Note there are partial failure modes
				// that will continue to function in a
				// degraded mode, such as when half of the
				// streams are successful and half of streams
				// take this return path.  Design a graceful
				// recovery mechanism?
				s.client = nil
				s.telemetry.Logger.Info("arrow is not supported",
					zap.String("message", status.Message()),
				)

			case codes.Unavailable, codes.Internal:
				// gRPC returns this when max connection age is reached.
				// The message string will contain NO_ERROR if it's a
				// graceful shutdown.
				//
				// Having seen:
				//
				//     arrow stream unknown {"kind": "exporter",
				//     "data_type": "traces", "name": "otlp/traces",
				//     "code": 13, "message": "stream terminated by
				//     RST_STREAM with error code: NO_ERROR"}
				//
				// from the default case below print `"code": 13`, this
				// branch is now used for both Unavailable (witnessed
				// in local testing) and Internal (witnessed in
				// production); in both cases "NO_ERROR" is the key
				// signifier.
				if strings.Contains(status.Message(), "NO_ERROR") {
					s.telemetry.Logger.Error("arrow stream reset (consider lowering max_stream_lifetime)",
						zap.String("message", status.Message()),
					)
				} else {
					s.telemetry.Logger.Error("arrow stream unavailable",
						zap.String("message", status.Message()),
					)
				}

			case codes.Canceled:
				// Note that when the writer encounters a local error (such
				// as a panic in the encoder) it will cancel the context and
				// writeErr will be set to an actual error, while the error
				// returned from read() will be the cancellation by the
				// writer. So if the reader's error is canceled and the
				// writer's error is non-nil, use it instead.
				if writeErr != nil {
					s.telemetry.Logger.Error("arrow stream internal error",
						zap.Error(writeErr),
					)
					// reset the writeErr so it doesn't print below.
					writeErr = nil
				} else {
					s.telemetry.Logger.Error("arrow stream canceled",
						zap.String("message", status.Message()),
					)
				}
			default:
				s.telemetry.Logger.Error("arrow stream unknown",
					zap.Uint32("code", uint32(status.Code())),
					zap.String("message", status.Message()),
				)
			}
		} else {
			s.logStreamError(err)
		}
	}
	if writeErr != nil {
		s.logStreamError(writeErr)
	}

	// The reader and writer have both finished; respond to any
	// outstanding waiters.
	for _, ch := range s.waiters {
		// Note: the top-level OTLP exporter will retry.
		ch <- ErrStreamRestarting
	}
}

// write repeatedly places this stream into the next-available queue, then
// performs a blocking send().  This returns when the data is in the write buffer,
// the caller waiting on its error channel.
func (s *Stream) write(ctx context.Context) error {
	// headers are encoding using hpack, reusing a buffer on each call.
	var hdrsBuf bytes.Buffer
	hdrsEnc := hpack.NewEncoder(&hdrsBuf)

	var timerCh <-chan time.Time
	if s.maxStreamLifetime != 0 {
		timer := time.NewTimer(s.maxStreamLifetime)
		timerCh = timer.C
		defer timer.Stop()
	}

	for {
		// Note: this can't block b/c stream has capacity &
		// individual streams shut down synchronously.
		s.prioritizer.setReady(s)

		// this can block, and if the context is canceled we
		// wait for the reader to find this stream.
		var wri writeItem
		var ok bool
		select {
		case <-timerCh:
			// If timerCh is nil, this will never happen.
			s.prioritizer.removeReady(s)
			return s.client.CloseSend()
		case wri, ok = <-s.toWrite:
			// channel is closed
			if !ok {
				return nil
			}
		case <-ctx.Done():
			// Because we did not <-stream.toWrite, there
			// is a potential sender race since the stream
			// is currently in the ready set.
			s.prioritizer.removeReady(s)
			return ctx.Err()
		}
		// Note: For the two return statements below there is no potential
		// sender race because the stream is not available, as indicated by
		// the successful <-stream.toWrite.

		batch, err := s.encode(wri.records)
		if err != nil {
			// This is some kind of internal error.  We will restart the
			// stream and mark this record as a permanent one.
			err = fmt.Errorf("encode: %w", err)
			wri.errCh <- consumererror.NewPermanent(err)
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
					// this a permenent error.
					err = fmt.Errorf("hpack: %w", err)
					wri.errCh <- consumererror.NewPermanent(err)
					return err
				}
			}
			batch.Headers = hdrsBuf.Bytes()
		}

		// Let the receiver knows what to look for.
		s.setBatchChannel(batch.BatchId, wri.errCh)

		if err := s.client.Send(batch); err != nil {
			// The error will be sent to errCh during cleanup for this stream.
			// Note: do not wrap this error, it may contain a Status.
			return err
		}
	}
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

		// This indicates the server received EOF from client shutdown.
		// This is not an error because this is an expected shutdown
		// initiated by the client by setting max_stream_lifetime.
		if resp.StatusCode == arrowpb.StatusCode_CANCELED {
			return nil
		}

		if err = s.processBatchStatus(resp); err != nil {
			return fmt.Errorf("process: %w", err)
		}
	}
}

// getSenderChannels takes the stream lock and removes the
// corresonding sender channel for each BatchId.  They are returned
// with the same index as the original status, for correlation.  Nil
// channels will be returned when there are errors locating the
// sender channel.
func (s *Stream) getSenderChannels(status *arrowpb.BatchStatus) (chan error, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	ch, ok := s.waiters[status.BatchId]
	if !ok {
		// Will break the stream.
		return nil, fmt.Errorf("unrecognized batch ID: %d", status.BatchId)
	}
	delete(s.waiters, status.BatchId)
	return ch, nil
}

// processBatchStatus processes a single response from the server and unblocks the
// associated sender.
func (s *Stream) processBatchStatus(ss *arrowpb.BatchStatus) error {
	ch, ret := s.getSenderChannels(ss)

	if ch == nil {
		// In case getSenderChannels encounters a problem, the
		// channel is nil.
		return ret
	}

	if ss.StatusCode == arrowpb.StatusCode_OK {
		ch <- nil
		return nil
	}
	// See ../../otlp.go's `shouldRetry()` method, the retry
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
		// Note: case arrowpb.StatusCode_CANCELED (a.k.a. codes.Canceled)
		// is handled before calling processBatchStatus().

		// Unrecognized status code.
		err = status.Errorf(codes.Internal, "unexpected stream response: %d: %s", ss.BatchId, ss.StatusMessage)

		// Will break the stream.
		ret = multierr.Append(ret, err)
	}
	ch <- err
	return ret
}

// SendAndWait submits a batch of records to be encoded and sent.  Meanwhile, this
// goroutine waits on the incoming context or for the asynchronous response to be
// received by the stream reader.
func (s *Stream) SendAndWait(ctx context.Context, records interface{}) error {
	errCh := make(chan error, 1)

	// Note that if the OTLP exporter's gRPC Headers field was
	// set, those (static) headers were used to establish the
	// stream.  The caller's context was returned by
	// baseExporter.enhanceContext() includes the static headers
	// plus optional client metadata.  Here, get whatever
	// headers that gRPC would have transmitted for a unary RPC
	// and convey them via the Arrow batch.

	// Note that the "uri" parameter to GetRequestMetadata is
	// not used by the headersetter extension and is not well
	// documented.  Since it's an optional list, we omit it.
	var md map[string]string
	if s.perRPCCredentials != nil {
		var err error
		md, err = s.perRPCCredentials.GetRequestMetadata(ctx)
		if err != nil {
			return err
		}
	}

	s.toWrite <- writeItem{
		records: records,
		md:      md,
		errCh:   errCh,
	}

	// Note this ensures the caller's timeout is respected.
	select {
	case <-ctx.Done():
		// This caller's context timed out.
		return ctx.Err()
	case err := <-errCh:
		// Note: includes err == nil and err != nil cases.
		return err
	}
}

// encode produces the next batch of Arrow records.
func (s *Stream) encode(records interface{}) (_ *arrowpb.BatchArrowRecords, retErr error) {
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
			retErr = fmt.Errorf("panic in otel-arrow-adapter: %v", err)
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
		return nil, fmt.Errorf("unsupported OTLP type: %T", records)
	}
	return batch, err
}
