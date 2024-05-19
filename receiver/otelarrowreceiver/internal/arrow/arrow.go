// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package arrow // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver/internal/arrow"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	arrowpb "github.com/open-telemetry/otel-arrow/api/experimental/arrow/v1"
	"github.com/open-telemetry/otel-arrow/collector/netstats"
	arrowRecord "github.com/open-telemetry/otel-arrow/pkg/otel/arrow_record"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/extension/auth"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/otel"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"golang.org/x/net/http2/hpack"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	streamFormat        = "arrow"
	hpackMaxDynamicSize = 4096
	scopeName           = "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver"
)

var (
	ErrNoMetricsConsumer   = fmt.Errorf("no metrics consumer")
	ErrNoLogsConsumer      = fmt.Errorf("no logs consumer")
	ErrNoTracesConsumer    = fmt.Errorf("no traces consumer")
	ErrUnrecognizedPayload = fmt.Errorf("unrecognized OTLP payload")
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

	telemetry            component.TelemetrySettings
	tracer               trace.Tracer
	obsrecv              *receiverhelper.ObsReport
	gsettings            configgrpc.ServerConfig
	authServer           auth.Server
	newConsumer          func() arrowRecord.ConsumerAPI
	netReporter          netstats.Interface
	recvInFlightBytes    metric.Int64UpDownCounter
	recvInFlightItems    metric.Int64UpDownCounter
	recvInFlightRequests metric.Int64UpDownCounter
}

// New creates a new Receiver reference.
func New(
	cs Consumers,
	set receiver.CreateSettings,
	obsrecv *receiverhelper.ObsReport,
	gsettings configgrpc.ServerConfig,
	authServer auth.Server,
	newConsumer func() arrowRecord.ConsumerAPI,
	netReporter netstats.Interface,
) (*Receiver, error) {
	tracer := set.TelemetrySettings.TracerProvider.Tracer("otel-arrow-receiver")
	var errors, err error
	recv := &Receiver{
		Consumers:   cs,
		obsrecv:     obsrecv,
		telemetry:   set.TelemetrySettings,
		tracer:      tracer,
		authServer:  authServer,
		newConsumer: newConsumer,
		gsettings:   gsettings,
		netReporter: netReporter,
	}

	meter := recv.telemetry.MeterProvider.Meter(scopeName)
	recv.recvInFlightBytes, err = meter.Int64UpDownCounter(
		"otel_arrow_receiver_in_flight_bytes",
		metric.WithDescription("Number of bytes in flight"),
		metric.WithUnit("By"),
	)
	errors = multierr.Append(errors, err)

	recv.recvInFlightItems, err = meter.Int64UpDownCounter(
		"otel_arrow_receiver_in_flight_items",
		metric.WithDescription("Number of items in flight"),
	)
	errors = multierr.Append(errors, err)

	recv.recvInFlightRequests, err = meter.Int64UpDownCounter(
		"otel_arrow_receiver_in_flight_requests",
		metric.WithDescription("Number of requests in flight"),
	)
	errors = multierr.Append(errors, err)

	if errors != nil {
		return nil, errors
	}

	return recv, nil
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

func newHeaderReceiver(streamCtx context.Context, as auth.Server, includeMetadata bool) *headerReceiver {
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
		return ctx, nil, nil
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
func (r *Receiver) logStreamError(err error) {
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
		r.telemetry.Logger.Debug("arrow stream shutdown", zap.String("message", msg))
	} else {
		r.telemetry.Logger.Error("arrow stream error", zap.String("message", msg), zap.Int("code", int(code)))
	}
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

func (r *Receiver) anyStream(serverStream anyStreamServer, method string) (retErr error) {
	streamCtx := serverStream.Context()
	ac := r.newConsumer()
	hrcv := newHeaderReceiver(serverStream.Context(), r.authServer, r.gsettings.IncludeMetadata)

	defer func() {
		if err := recover(); err != nil {
			// When this happens, the stacktrace is
			// important and lost if we don't capture it
			// here.
			r.telemetry.Logger.Debug("panic detail in otel-arrow-adapter",
				zap.Reflect("recovered", err),
				zap.Stack("stacktrace"),
			)
			retErr = fmt.Errorf("panic in otel-arrow-adapter: %v", err)
		}
		if err := ac.Close(); err != nil {
			r.telemetry.Logger.Error("arrow stream close", zap.Error(err))
		}
	}()

	for {
		// Receive a batch corresponding with one ptrace.Traces, pmetric.Metrics,
		// or plog.Logs item.
		req, err := serverStream.Recv()
		if err != nil {
			// This includes the case where a client called CloseSend(), in
			// which case we see an EOF error here.
			r.logStreamError(err)

			if errors.Is(err, io.EOF) {
				return status.Error(codes.Canceled, "client stream shutdown")
			} else if errors.Is(err, context.Canceled) {
				return status.Error(codes.Canceled, "server stream shutdown")
			}
			return err
		}

		// Check for optional headers and set the incoming context.
		thisCtx, authHdrs, err := hrcv.combineHeaders(streamCtx, req.GetHeaders())
		if err != nil {
			// Failing to parse the incoming headers breaks the stream.
			r.telemetry.Logger.Error("arrow metadata error", zap.Error(err))
			return err
		}

		var authErr error
		if r.authServer != nil {
			var newCtx context.Context
			if newCtx, err = r.authServer.Authenticate(thisCtx, authHdrs); err != nil {
				authErr = err
			} else {
				thisCtx = newCtx
			}
		}

		if err := r.processAndConsume(thisCtx, method, ac, req, serverStream, authErr); err != nil {
			return err
		}
	}
}

func (r *Receiver) processAndConsume(ctx context.Context, method string, arrowConsumer arrowRecord.ConsumerAPI, req *arrowpb.BatchArrowRecords, serverStream anyStreamServer, authErr error) (retErr error) {
	var err error

	ctx, span := r.tracer.Start(ctx, "otel_arrow_stream_recv")
	defer span.End()

	r.recvInFlightRequests.Add(ctx, 1)
	defer func() {
		r.recvInFlightRequests.Add(ctx, -1)
		// Set span status if an error is returned.
		if retErr != nil {
			span := trace.SpanFromContext(ctx)
			span.SetStatus(otelcodes.Error, retErr.Error())
		}
	}()

	// Process records: an error in this code path does
	// not necessarily break the stream.
	if authErr != nil {
		err = authErr
	} else {
		err = r.processRecords(ctx, method, arrowConsumer, req)
	}

	// Note: Statuses can be batched, but we do not take
	// advantage of this feature.
	status := &arrowpb.BatchStatus{
		BatchId: req.GetBatchId(),
	}
	if err == nil {
		status.StatusCode = arrowpb.StatusCode_OK
	} else {
		status.StatusMessage = err.Error()
		switch {
		case errors.Is(err, arrowRecord.ErrConsumerMemoryLimit):
			r.telemetry.Logger.Error("arrow resource exhausted", zap.Error(err))
			status.StatusCode = arrowpb.StatusCode_RESOURCE_EXHAUSTED
		case consumererror.IsPermanent(err):
			r.telemetry.Logger.Error("arrow data error", zap.Error(err))
			status.StatusCode = arrowpb.StatusCode_INVALID_ARGUMENT
		default:
			r.telemetry.Logger.Debug("arrow consumer error", zap.Error(err))
			status.StatusCode = arrowpb.StatusCode_UNAVAILABLE
		}
	}

	err = serverStream.Send(status)
	if err != nil {
		r.logStreamError(err)
		return err
	}
	return nil
}

// processRecords returns an error and a boolean indicating whether
// the error (true) was from processing the data (i.e., invalid
// argument) or (false) from the consuming pipeline.  The boolean is
// not used when success (nil error) is returned.
func (r *Receiver) processRecords(ctx context.Context, method string, arrowConsumer arrowRecord.ConsumerAPI, records *arrowpb.BatchArrowRecords) error {
	payloads := records.GetArrowPayloads()
	if len(payloads) == 0 {
		return nil
	}
	var uncompSize int64
	if r.telemetry.MetricsLevel > configtelemetry.LevelNormal {
		defer func() {
			// The netstats code knows that uncompressed size is
			// unreliable for arrow transport, so we instrument it
			// directly here.  Only the primary direction of transport
			// is instrumented this way.
			var sized netstats.SizesStruct
			sized.Method = method
			if r.telemetry.MetricsLevel > configtelemetry.LevelNormal {
				sized.Length = uncompSize
			}
			r.netReporter.CountReceive(ctx, sized)
			r.netReporter.SetSpanSizeAttributes(ctx, sized)
		}()
	}

	switch payloads[0].Type {
	case arrowpb.ArrowPayloadType_UNIVARIATE_METRICS:
		if r.Metrics() == nil {
			return status.Error(codes.Unimplemented, "metrics service not available")
		}
		var sizer pmetric.ProtoMarshaler
		var numPts int

		ctx = r.obsrecv.StartMetricsOp(ctx)

		data, err := arrowConsumer.MetricsFrom(records)
		if err != nil {
			err = consumererror.NewPermanent(err)
		} else {
			for _, metrics := range data {
				items := metrics.DataPointCount()
				sz := int64(sizer.MetricsSize(metrics))

				r.recvInFlightBytes.Add(ctx, sz)
				r.recvInFlightItems.Add(ctx, int64(items))

				numPts += items
				uncompSize += sz
				err = multierr.Append(err,
					r.Metrics().ConsumeMetrics(ctx, metrics),
				)
			}
			// entire request has been processed, decrement counter.
			r.recvInFlightBytes.Add(ctx, -uncompSize)
			r.recvInFlightItems.Add(ctx, int64(-numPts))
		}
		r.obsrecv.EndMetricsOp(ctx, streamFormat, numPts, err)
		return err

	case arrowpb.ArrowPayloadType_LOGS:
		if r.Logs() == nil {
			return status.Error(codes.Unimplemented, "logs service not available")
		}
		var sizer plog.ProtoMarshaler
		var numLogs int
		ctx = r.obsrecv.StartLogsOp(ctx)

		data, err := arrowConsumer.LogsFrom(records)
		if err != nil {
			err = consumererror.NewPermanent(err)
		} else {
			for _, logs := range data {
				items := logs.LogRecordCount()
				sz := int64(sizer.LogsSize(logs))

				r.recvInFlightBytes.Add(ctx, sz)
				r.recvInFlightItems.Add(ctx, int64(items))
				numLogs += items
				uncompSize += sz
				err = multierr.Append(err,
					r.Logs().ConsumeLogs(ctx, logs),
				)
			}
			// entire request has been processed, decrement counter.
			r.recvInFlightBytes.Add(ctx, -uncompSize)
			r.recvInFlightItems.Add(ctx, int64(-numLogs))
		}
		r.obsrecv.EndLogsOp(ctx, streamFormat, numLogs, err)
		return err

	case arrowpb.ArrowPayloadType_SPANS:
		if r.Traces() == nil {
			return status.Error(codes.Unimplemented, "traces service not available")
		}
		var sizer ptrace.ProtoMarshaler
		var numSpans int
		ctx = r.obsrecv.StartTracesOp(ctx)

		data, err := arrowConsumer.TracesFrom(records)
		if err != nil {
			err = consumererror.NewPermanent(err)
		} else {
			for _, traces := range data {
				items := traces.SpanCount()
				sz := int64(sizer.TracesSize(traces))

				r.recvInFlightBytes.Add(ctx, sz)
				r.recvInFlightItems.Add(ctx, int64(items))

				numSpans += items
				uncompSize += sz
				err = multierr.Append(err,
					r.Traces().ConsumeTraces(ctx, traces),
				)
			}

			// entire request has been processed, decrement counter.
			r.recvInFlightBytes.Add(ctx, -uncompSize)
			r.recvInFlightItems.Add(ctx, int64(-numSpans))
		}
		r.obsrecv.EndTracesOp(ctx, streamFormat, numSpans, err)
		return err

	default:
		return ErrUnrecognizedPayload
	}
}
