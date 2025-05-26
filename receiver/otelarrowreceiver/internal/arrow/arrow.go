// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package arrow // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver/internal/arrow"

import (
	"context"
	"errors"
	"io"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	arrowpb "github.com/open-telemetry/otel-arrow/api/experimental/arrow/v1"
	arrowRecord "github.com/open-telemetry/otel-arrow/pkg/otel/arrow_record"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/otel"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"golang.org/x/net/http2/hpack"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/grpcutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/admission2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/netstats"
)

const (
	streamFormat        = "arrow"
	hpackMaxDynamicSize = 4096
)

var (
	ErrNoMetricsConsumer   = errors.New("no metrics consumer")
	ErrNoLogsConsumer      = errors.New("no logs consumer")
	ErrNoTracesConsumer    = errors.New("no traces consumer")
	ErrUnrecognizedPayload = consumererror.NewPermanent(errors.New("unrecognized OTel-Arrow payload"))
)

type Consumers interface {
	Traces() consumer.Traces
	Metrics() consumer.Metrics
	Logs() consumer.Logs
}

type Receiver struct {
	Consumers

	arrowpb.UnsafeArrowTracesServiceServer
	arrowpb.UnsafeArrowLogsServiceServer
	arrowpb.UnsafeArrowMetricsServiceServer

	telemetry    component.TelemetrySettings
	tracer       trace.Tracer
	obsrecv      *receiverhelper.ObsReport
	gsettings    configgrpc.ServerConfig
	authServer   extensionauth.Server
	newConsumer  func() arrowRecord.ConsumerAPI
	netReporter  netstats.Interface
	boundedQueue admission2.Queue
}

// receiverStream holds the inFlightWG for a single stream.
type receiverStream struct {
	*Receiver
	inFlightWG sync.WaitGroup
}

// New creates a new Receiver reference.
func New(
	cs Consumers,
	set receiver.Settings,
	obsrecv *receiverhelper.ObsReport,
	gsettings configgrpc.ServerConfig,
	authServer extensionauth.Server,
	newConsumer func() arrowRecord.ConsumerAPI,
	bq admission2.Queue,
	netReporter netstats.Interface,
) (*Receiver, error) {
	tracer := set.TracerProvider.Tracer("otel-arrow-receiver")
	return &Receiver{
		Consumers:    cs,
		obsrecv:      obsrecv,
		telemetry:    set.TelemetrySettings,
		tracer:       tracer,
		authServer:   authServer,
		newConsumer:  newConsumer,
		gsettings:    gsettings,
		netReporter:  netReporter,
		boundedQueue: bq,
	}, nil
}

// headerReceiver contains the state necessary to decode per-request metadata
// from an arrow stream.
type headerReceiver struct {
	// decoder maintains state across the stream.
	decoder *hpack.Decoder

	// includeMetadata as configured by gRPC settings.
	includeMetadata bool

	// hasAuthServer indicates that headers must be produced
	// independent of includeMetadata.
	hasAuthServer bool

	// client connection info from the stream context, (optionally
	// if includeMetadata) to be extended with per-request metadata.
	connInfo client.Info

	// streamHdrs was translated from the incoming context, will be
	// merged with per-request metadata.  Note that the contents of
	// this map are equivalent to connInfo.Metadata, however that
	// library does not let us iterate over the map so we recalculate
	// this from the gRPC incoming stream context.
	streamHdrs map[string][]string

	// tmpHdrs is used by the decoder's emit function during Write.
	tmpHdrs map[string][]string
}

func newHeaderReceiver(streamCtx context.Context, as extensionauth.Server, includeMetadata bool) *headerReceiver {
	hr := &headerReceiver{
		includeMetadata: includeMetadata,
		hasAuthServer:   as != nil,
		connInfo:        client.FromContext(streamCtx),
	}

	// Note that we capture the incoming context if there is an
	// Auth plugin configured or includeMetadata is set.
	if hr.includeMetadata || hr.hasAuthServer {
		if smd, ok := metadata.FromIncomingContext(streamCtx); ok {
			hr.streamHdrs = smd
		}
	}

	// Note the hpack decoder supports additional protections,
	// such as SetMaxStringLength(), but as we already have limits
	// on stream request size, this seems unnecessary.
	hr.decoder = hpack.NewDecoder(hpackMaxDynamicSize, hr.tmpHdrsAppend)

	return hr
}

// combineHeaders calculates per-request Metadata by combining the stream's
// client.Info with additional key:values associated with the arrow batch.
func (h *headerReceiver) combineHeaders(ctx context.Context, hdrsBytes []byte) (context.Context, map[string][]string, error) {
	if len(hdrsBytes) == 0 && len(h.streamHdrs) == 0 {
		// Note: call newContext in this case to ensure that
		// connInfo is added to the context, for Auth.
		return h.newContext(ctx, nil), nil, nil
	}

	if len(hdrsBytes) == 0 {
		return h.newContext(ctx, h.streamHdrs), h.streamHdrs, nil
	}

	// Note that we will parse the headers even if they are not
	// used, to check for validity and/or trace context.  Also
	// note this code was once optimized to avoid the following
	// map allocation in cases where the return value would not be
	// used.  This logic was "is metadata present" or "is auth
	// server used".  Then we added to this, "is trace propagation
	// in use" and simplified this function to always store the
	// headers into a temporary map.
	h.tmpHdrs = map[string][]string{}

	// Write calls the emitFunc, appending directly into `tmpHdrs`.
	if _, err := h.decoder.Write(hdrsBytes); err != nil {
		return ctx, nil, err
	}

	// Get the global propagator, to extract context.  When there
	// are no fields, it's a no-op propagator implementation and
	// we can skip the allocations inside this block.
	carrier := otel.GetTextMapPropagator()
	if len(carrier.Fields()) != 0 {
		// When there are no fields, it's a no-op
		// implementation and we can skip the allocations.
		flat := map[string]string{}
		for _, key := range carrier.Fields() {
			have := h.tmpHdrs[key]
			if len(have) > 0 {
				flat[key] = have[0]
				delete(h.tmpHdrs, key)
			}
		}

		ctx = carrier.Extract(ctx, propagation.MapCarrier(flat))
	}

	// Add streamHdrs that were not carried in the per-request headers.
	for k, v := range h.streamHdrs {
		// Note: This is done after the per-request metadata is defined
		// in recognition of a potential for duplicated values stemming
		// from the Arrow exporter's independent call to the Auth
		// extension's GetRequestMetadata().  This paired with the
		// headersetter's return of empty-string values means, we would
		// end up with an empty-string element for any headersetter
		// `from_context` rules b/c the stream uses background context.
		// This allows static headers through.
		//
		// See https://github.com/open-telemetry/opentelemetry-collector/issues/6965
		lk := strings.ToLower(k)
		if _, ok := h.tmpHdrs[lk]; !ok {
			h.tmpHdrs[lk] = v
		}
	}

	// Release the temporary copy used in emitFunc().
	newHdrs := h.tmpHdrs
	h.tmpHdrs = nil

	// Note: newHdrs is passed to the Auth plugin.  Whether
	// newHdrs is set in the context depends on h.includeMetadata.
	return h.newContext(ctx, newHdrs), newHdrs, nil
}

// tmpHdrsAppend appends to tmpHdrs, from decoder's emit function.
func (h *headerReceiver) tmpHdrsAppend(hf hpack.HeaderField) {
	if h.tmpHdrs != nil {
		// We force strings.ToLower to ensure consistency.  gRPC itself
		// does this and would do the same.
		hn := strings.ToLower(hf.Name)
		h.tmpHdrs[hn] = append(h.tmpHdrs[hn], hf.Value)
	}
}

func (h *headerReceiver) newContext(ctx context.Context, hdrs map[string][]string) context.Context {
	// Retain the Addr/Auth of the stream connection, update the
	// per-request metadata from the Arrow batch.
	var md client.Metadata
	if h.includeMetadata && hdrs != nil {
		md = client.NewMetadata(hdrs)
	}
	return client.NewContext(ctx, client.Info{
		Addr:     h.connInfo.Addr,
		Auth:     h.connInfo.Auth,
		Metadata: md,
	})
}

// logStreamError decides how to log an error.
func (r *Receiver) logStreamError(err error, where string) (occode otelcodes.Code, msg string) {
	var code codes.Code
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
		occode = otelcodes.Unset
		r.telemetry.Logger.Debug("arrow stream shutdown", zap.String("message", msg), zap.String("where", where))
	} else {
		occode = otelcodes.Error
		r.telemetry.Logger.Error("arrow stream error", zap.Int("code", int(code)), zap.String("message", msg), zap.String("where", where))
	}

	return occode, msg
}

func gRPCName(desc grpc.ServiceDesc) string {
	return netstats.GRPCStreamMethodName(desc, desc.Streams[0])
}

var (
	arrowTracesMethod  = gRPCName(arrowpb.ArrowTracesService_ServiceDesc)
	arrowMetricsMethod = gRPCName(arrowpb.ArrowMetricsService_ServiceDesc)
	arrowLogsMethod    = gRPCName(arrowpb.ArrowLogsService_ServiceDesc)
)

func (r *Receiver) ArrowTraces(serverStream arrowpb.ArrowTracesService_ArrowTracesServer) error {
	return r.anyStream(serverStream, arrowTracesMethod)
}

func (r *Receiver) ArrowLogs(serverStream arrowpb.ArrowLogsService_ArrowLogsServer) error {
	return r.anyStream(serverStream, arrowLogsMethod)
}

func (r *Receiver) ArrowMetrics(serverStream arrowpb.ArrowMetricsService_ArrowMetricsServer) error {
	return r.anyStream(serverStream, arrowMetricsMethod)
}

type anyStreamServer interface {
	Send(*arrowpb.BatchStatus) error
	Recv() (*arrowpb.BatchArrowRecords, error)
	grpc.ServerStream
}

type batchResp struct {
	id  int64
	err error
}

func (r *Receiver) recoverErr(retErr *error) {
	if err := recover(); err != nil {
		// When this happens, the stacktrace is
		// important and lost if we don't capture it
		// here.
		r.telemetry.Logger.Error("panic detail in otel-arrow-adapter",
			zap.Reflect("recovered", err),
			zap.Stack("stacktrace"),
		)
		*retErr = status.Errorf(codes.Internal, "panic in otel-arrow-adapter: %v", err)
	}
}

func (r *Receiver) anyStream(serverStream anyStreamServer, method string) (retErr error) {
	streamCtx := serverStream.Context()
	ac := r.newConsumer()

	defer func() {
		if err := ac.Close(); err != nil {
			r.telemetry.Logger.Error("arrow stream close", zap.Error(err))
		}
	}()
	defer r.recoverErr(&retErr)

	// doneCancel allows an error in the sender/receiver to
	// interrupt the corresponding thread.
	doneCtx, doneCancel := context.WithCancel(streamCtx)
	defer doneCancel()

	recvErrCh := make(chan error, 1)
	sendErrCh := make(chan error, 1)
	pendingCh := make(chan batchResp, runtime.NumCPU())

	// wg is used to ensure this thread returns after both
	// sender and receiver threads return.
	var sendWG sync.WaitGroup
	var recvWG sync.WaitGroup
	sendWG.Add(1)
	recvWG.Add(1)

	// flushCtx controls the start of flushing.  when this is canceled
	// after the receiver finishes, the flush operation begins.
	flushCtx, flushCancel := context.WithCancel(doneCtx)
	defer flushCancel()

	rstream := &receiverStream{
		Receiver: r,
	}

	go func() {
		var err error
		defer recvWG.Done()
		defer r.recoverErr(&err)
		err = rstream.srvReceiveLoop(doneCtx, serverStream, pendingCh, method, ac)
		recvErrCh <- err
	}()

	go func() {
		var err error
		defer sendWG.Done()
		defer r.recoverErr(&err)
		// the sender receives flushCtx, which is canceled after the
		// receiver returns (success or no).
		err = rstream.srvSendLoop(flushCtx, serverStream, &recvWG, pendingCh)
		sendErrCh <- err
	}()

	// Wait for sender/receiver threads to return before returning.
	defer recvWG.Wait()
	defer sendWG.Wait()

	for {
		select {
		case <-doneCtx.Done():
			return status.Error(codes.Canceled, "server stream shutdown")
		case err := <-recvErrCh:
			flushCancel()
			if errors.Is(err, io.EOF) {
				// the receiver returned EOF, next we
				// expect the sender to finish.
				continue
			}
			return err
		case err := <-sendErrCh:
			// explicit cancel here, in case the sender fails before
			// the receiver does. break the receiver loop here:
			doneCancel()
			return err
		}
	}
}

func (r *receiverStream) newInFlightData(ctx context.Context, method string, batchID int64, pendingCh chan<- batchResp) *inFlightData {
	_, span := r.tracer.Start(ctx, "otel_arrow_stream_inflight")

	r.inFlightWG.Add(1)
	id := &inFlightData{
		receiverStream: r,
		method:         method,
		batchID:        batchID,
		pendingCh:      pendingCh,
		span:           span,
	}
	id.refs.Add(1)
	return id
}

// inFlightData is responsible for storing the resources held by one request.
type inFlightData struct {
	// Receiver is the owner of the resources held by this object.
	*receiverStream

	method    string
	batchID   int64
	pendingCh chan<- batchResp
	span      trace.Span

	// refs counts the number of goroutines holding this object.
	// initially the recvOne() body, on success the
	// consumeAndRespond() function.
	refs atomic.Int32

	numItems   int   // how many items
	uncompSize int64 // uncompressed data size == how many bytes held in the semaphore
	releaser   admission2.ReleaseFunc
}

func (id *inFlightData) recvDone(ctx context.Context, recvErrPtr *error) {
	retErr := *recvErrPtr

	if retErr != nil {
		// logStreamError because this response will break the stream.
		occode, msg := id.logStreamError(retErr, "recv")
		id.span.SetStatus(occode, msg)
	}

	id.anyDone(ctx)
}

func (id *inFlightData) consumeDone(ctx context.Context, consumeErrPtr *error) {
	retErr := *consumeErrPtr

	if retErr != nil {
		// debug-level because the error was external from the pipeline.
		id.telemetry.Logger.Debug("otel-arrow consume", zap.Error(retErr))
		id.span.SetStatus(otelcodes.Error, retErr.Error())
	}

	id.replyToCaller(ctx, retErr)
	id.anyDone(ctx)
}

func (id *inFlightData) replyToCaller(ctx context.Context, callerErr error) {
	select {
	case id.pendingCh <- batchResp{
		id:  id.batchID,
		err: callerErr,
	}:
		// OK: Responded.
	case <-ctx.Done():
		// OK: Never responded due to cancelation.
	}
}

func (id *inFlightData) anyDone(ctx context.Context) {
	// check if there are still refs, in which case leave the in-flight
	// counts where they are.
	if id.refs.Add(-1) != 0 {
		return
	}

	id.span.End()

	if id.releaser != nil {
		id.releaser()
	}

	// The netstats code knows that uncompressed size is
	// unreliable for arrow transport, so we instrument it
	// directly here.  Only the primary direction of transport
	// is instrumented this way.
	var sized netstats.SizesStruct
	sized.Method = id.method
	sized.Length = id.uncompSize
	id.netReporter.CountReceive(ctx, sized)

	id.inFlightWG.Done()
}

// recvOne begins processing a single Arrow batch.
//
// If an error is encountered before Arrow data is successfully consumed,
// the stream will break and the error will be returned immediately.
//
// If the error is due to authorization, the stream remains unbroken
// and the request fails.
//
// If not enough resources are available, the stream will block (if
// waiting permitted) or break (insufficient waiters).
//
// Assuming success, a new goroutine is created to handle consuming the
// data.
//
// This handles constructing an inFlightData object, which itself
// tracks everything that needs to be used by instrumentation when the
// batch finishes.
func (r *receiverStream) recvOne(streamCtx context.Context, serverStream anyStreamServer, hrcv *headerReceiver, pendingCh chan<- batchResp, method string, ac arrowRecord.ConsumerAPI) (retErr error) {
	// Receive a batch corresponding with one ptrace.Traces, pmetric.Metrics,
	// or plog.Logs item.
	req, recvErr := serverStream.Recv()

	// the incoming stream context is the parent of the in-flight context, which
	// carries a span covering sequential stream-processing work.  the context
	// is severed at this point, with flight.span a contextless child that will be
	// finished in recvDone().
	flight := r.newInFlightData(streamCtx, method, req.GetBatchId(), pendingCh)

	// inflightCtx is carried through into consumeAndProcess on the success path.
	// this inherits the stream context so that its auth headers are present
	// when the per-data Auth call is made.
	inflightCtx := streamCtx
	defer flight.recvDone(inflightCtx, &retErr)

	if recvErr != nil {
		if errors.Is(recvErr, io.EOF) {
			return recvErr
		} else if errors.Is(recvErr, context.Canceled) {
			// This is a special case to avoid introducing a span error
			// for a canceled operation.
			return io.EOF
		} else if status, ok := status.FromError(recvErr); ok && status.Code() == codes.Canceled {
			// This is a special case to avoid introducing a span error
			// for a canceled operation.
			return io.EOF
		}
		// Note: err is directly from gRPC, should already have status.
		return recvErr
	}

	// Check for optional headers and set the incoming context.
	inflightCtx, authHdrs, hdrErr := hrcv.combineHeaders(inflightCtx, req.GetHeaders())
	if hdrErr != nil {
		// Failing to parse the incoming headers breaks the stream.
		return status.Errorf(codes.Internal, "arrow metadata error: %v", hdrErr)
	}

	// start this span after hrcv.combineHeaders returns extracted context. This will allow this span
	// to be a part of the data path trace instead of only being included as a child of the stream inflight trace.
	inflightCtx, span := r.tracer.Start(inflightCtx, "otel_arrow_stream_recv")
	defer span.End()

	// Authorize the request, if configured, prior to acquiring resources.
	if r.authServer != nil {
		var authErr error
		inflightCtx, authErr = r.authServer.Authenticate(inflightCtx, authHdrs)
		if authErr != nil {
			flight.replyToCaller(inflightCtx, status.Error(codes.Unauthenticated, authErr.Error()))
			return nil
		}
	}

	var callerCancel context.CancelFunc
	if encodedTimeout, has := authHdrs["grpc-timeout"]; has && len(encodedTimeout) == 1 {
		if timeout, decodeErr := grpcutil.DecodeTimeout(encodedTimeout[0]); decodeErr != nil {
			r.telemetry.Logger.Debug("grpc-timeout parse error", zap.Error(decodeErr))
		} else {
			// timeout parsed successfully
			inflightCtx, callerCancel = context.WithTimeout(inflightCtx, timeout)

			// if we return before the new goroutine is started below
			// cancel the context.  callerCancel will be non-nil until
			// the new goroutine is created at the end of this function.
			defer func() {
				if callerCancel != nil {
					callerCancel()
				}
			}()
		}
	}

	data, numItems, uncompSize, consumeErr := r.consumeBatch(ac, req)

	if consumeErr != nil {
		if errors.Is(consumeErr, arrowRecord.ErrConsumerMemoryLimit) {
			return status.Errorf(codes.ResourceExhausted, "otel-arrow decode: %v", consumeErr)
		}
		return status.Errorf(codes.Internal, "otel-arrow decode: %v", consumeErr)
	}

	// Use the bounded queue to memory limit based on incoming
	// uncompressed request size and waiters.  Acquire will fail
	// immediately if there are too many waiters, or will
	// otherwise block until timeout or enough memory becomes
	// available.
	releaser, acquireErr := r.boundedQueue.Acquire(inflightCtx, uint64(uncompSize))
	if acquireErr != nil {
		return acquireErr
	}
	flight.uncompSize = uncompSize
	flight.numItems = numItems
	flight.releaser = releaser

	// Recognize that the request is still in-flight via consumeAndRespond()
	flight.refs.Add(1)

	// consumeAndRespond consumes the data and returns control to the sender loop.
	go func(callerCancel context.CancelFunc) {
		if callerCancel != nil {
			defer callerCancel()
		}
		r.consumeAndRespond(inflightCtx, streamCtx, data, flight)
	}(callerCancel)

	// Reset callerCancel so the deferred function above does not call it here.
	callerCancel = nil

	return nil
}

// consumeAndRespond finishes the span started in recvOne and logs the
// result after invoking the pipeline to consume the data.
func (r *Receiver) consumeAndRespond(ctx, streamCtx context.Context, data any, flight *inFlightData) {
	var err error
	defer flight.consumeDone(streamCtx, &err)

	// recoverErr is a special function because it recovers panics, so we
	// keep it in a separate defer than the processing above, which will
	// run after the panic is recovered into an ordinary error.
	defer r.recoverErr(&err)

	err = r.consumeData(ctx, data, flight)
}

// srvReceiveLoop repeatedly receives one batch of data.
func (r *receiverStream) srvReceiveLoop(ctx context.Context, serverStream anyStreamServer, pendingCh chan<- batchResp, method string, ac arrowRecord.ConsumerAPI) (retErr error) {
	hrcv := newHeaderReceiver(ctx, r.authServer, r.gsettings.IncludeMetadata)
	for {
		select {
		case <-ctx.Done():
			return status.Error(codes.Canceled, "server stream shutdown")
		default:
			if err := r.recvOne(ctx, serverStream, hrcv, pendingCh, method, ac); err != nil {
				return err
			}
		}
	}
}

// srvReceiveLoop repeatedly sends one batch data response.
func (r *receiverStream) sendOne(serverStream anyStreamServer, resp batchResp) error {
	// Note: Statuses can be batched, but we do not take
	// advantage of this feature.
	bs := &arrowpb.BatchStatus{
		BatchId: resp.id,
	}
	if resp.err == nil {
		bs.StatusCode = arrowpb.StatusCode_OK
	} else {
		// Generally, code in the receiver should use
		// status.Errorf(codes.XXX, ...)  so that we take the
		// first branch.
		if gsc, ok := status.FromError(resp.err); ok {
			bs.StatusCode = arrowpb.StatusCode(gsc.Code())
			bs.StatusMessage = gsc.Message()
		} else {
			// Ideally, we don't take this branch because all code uses
			// gRPC status constructors and we've taken the branch above.
			//
			// This is a fallback for several broad categories of error.
			bs.StatusMessage = resp.err.Error()

			switch {
			case consumererror.IsPermanent(resp.err):
				// Some kind of pipeline error, somewhere downstream.
				r.telemetry.Logger.Error("arrow data error", zap.Error(resp.err))
				bs.StatusCode = arrowpb.StatusCode_INVALID_ARGUMENT
			default:
				// Probably a pipeline error, retryable.
				r.telemetry.Logger.Debug("arrow consumer error", zap.Error(resp.err))
				bs.StatusCode = arrowpb.StatusCode_UNAVAILABLE
			}
		}
	}

	if err := serverStream.Send(bs); err != nil {
		// logStreamError because this response will break the stream.
		_, _ = r.logStreamError(err, "send")
		return err
	}

	return nil
}

func (r *receiverStream) flushSender(serverStream anyStreamServer, recvWG *sync.WaitGroup, pendingCh <-chan batchResp) error {
	// wait to ensure no more items are accepted
	recvWG.Wait()

	// wait for all responses to be sent
	r.inFlightWG.Wait()

	for {
		select {
		case resp := <-pendingCh:
			if err := r.sendOne(serverStream, resp); err != nil {
				return err
			}
		default:
			// Currently nothing left in pendingCh.
			return nil
		}
	}
}

func (r *receiverStream) srvSendLoop(ctx context.Context, serverStream anyStreamServer, recvWG *sync.WaitGroup, pendingCh <-chan batchResp) error {
	for {
		select {
		case <-ctx.Done():
			return r.flushSender(serverStream, recvWG, pendingCh)
		case resp := <-pendingCh:
			if err := r.sendOne(serverStream, resp); err != nil {
				return err
			}
		}
	}
}

// consumeBatch applies the batch to the Arrow Consumer, returns a
// slice of pdata objects of the corresponding data type as `any`.
// along with the number of items and true uncompressed size.
func (r *Receiver) consumeBatch(arrowConsumer arrowRecord.ConsumerAPI, records *arrowpb.BatchArrowRecords) (retData any, numItems int, uncompSize int64, retErr error) {
	payloads := records.GetArrowPayloads()
	if len(payloads) == 0 {
		return nil, 0, 0, nil
	}

	switch payloads[0].Type {
	case arrowpb.ArrowPayloadType_UNIVARIATE_METRICS:
		if r.Metrics() == nil {
			return nil, 0, 0, status.Error(codes.Unimplemented, "metrics service not available")
		}
		var sizer pmetric.ProtoMarshaler

		data, err := arrowConsumer.MetricsFrom(records)
		if err == nil {
			for _, metrics := range data {
				numItems += metrics.DataPointCount()
				uncompSize += int64(sizer.MetricsSize(metrics))
			}
		}
		retData = data
		retErr = err

	case arrowpb.ArrowPayloadType_LOGS:
		if r.Logs() == nil {
			return nil, 0, 0, status.Error(codes.Unimplemented, "logs service not available")
		}
		var sizer plog.ProtoMarshaler

		data, err := arrowConsumer.LogsFrom(records)
		if err == nil {
			for _, logs := range data {
				numItems += logs.LogRecordCount()
				uncompSize += int64(sizer.LogsSize(logs))
			}
		}
		retData = data
		retErr = err

	case arrowpb.ArrowPayloadType_SPANS:
		if r.Traces() == nil {
			return nil, 0, 0, status.Error(codes.Unimplemented, "traces service not available")
		}
		var sizer ptrace.ProtoMarshaler

		data, err := arrowConsumer.TracesFrom(records)
		if err == nil {
			for _, traces := range data {
				numItems += traces.SpanCount()
				uncompSize += int64(sizer.TracesSize(traces))
			}
		}
		retData = data
		retErr = err

	default:
		retErr = ErrUnrecognizedPayload
	}

	return retData, numItems, uncompSize, retErr
}

// consumeData invokes the next pipeline consumer for a received batch of data.
// it uses the standard OTel collector instrumentation (receiverhelper.ObsReport).
//
// if any errors are permanent, returns a permanent error.
func (r *Receiver) consumeData(ctx context.Context, data any, flight *inFlightData) (retErr error) {
	oneOp := func(err error) {
		retErr = multierr.Append(retErr, err)
	}
	var final func(context.Context, string, int, error)

	switch items := data.(type) {
	case []pmetric.Metrics:
		ctx = r.obsrecv.StartMetricsOp(ctx)
		for _, metrics := range items {
			oneOp(r.Metrics().ConsumeMetrics(ctx, metrics))
		}
		final = r.obsrecv.EndMetricsOp

	case []plog.Logs:
		ctx = r.obsrecv.StartLogsOp(ctx)
		for _, logs := range items {
			oneOp(r.Logs().ConsumeLogs(ctx, logs))
		}
		final = r.obsrecv.EndLogsOp

	case []ptrace.Traces:
		ctx = r.obsrecv.StartTracesOp(ctx)
		for _, traces := range items {
			oneOp(r.Traces().ConsumeTraces(ctx, traces))
		}
		final = r.obsrecv.EndTracesOp

	default:
		retErr = ErrUnrecognizedPayload
	}
	if final != nil {
		final(ctx, streamFormat, flight.numItems, retErr)
	}
	return retErr
}
