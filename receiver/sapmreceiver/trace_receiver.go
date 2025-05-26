// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sapmreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver"

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	splunksapm "github.com/signalfx/sapm-proto/gen"
	"github.com/signalfx/sapm-proto/sapmprotocol"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/errorutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
)

var gzipWriterPool = &sync.Pool{
	New: func() any {
		return gzip.NewWriter(io.Discard)
	},
}

// sapmReceiver receives spans in the Splunk SAPM format over HTTP
type sapmReceiver struct {
	settings component.TelemetrySettings
	config   *Config

	server     *http.Server
	shutdownWG sync.WaitGroup

	nextConsumer consumer.Traces

	// defaultResponse is a placeholder. For now this receiver returns an empty sapm response.
	// This defaultResponse is an optimization so we don't have to proto.Marshal the response
	// for every request. At some point this may be removed when there is actual content to return.
	defaultResponse []byte

	obsrecv *receiverhelper.ObsReport
}

// handleRequest parses an http request containing sapm and passes the trace data to the next consumer
func (sr *sapmReceiver) handleRequest(req *http.Request) error {
	sapm, err := sapmprotocol.ParseTraceV2Request(req)
	// errors processing the request should return http.StatusBadRequest
	if err != nil {
		return err
	}

	ctx := sr.obsrecv.StartTracesOp(req.Context())

	td, err := jaeger.ProtoToTraces(sapm.Batches)
	if err != nil {
		return err
	}

	// pass the trace data to the next consumer
	err = sr.nextConsumer.ConsumeTraces(ctx, td)
	if err != nil {
		err = fmt.Errorf("error passing trace data to next consumer: %w", err)
	}

	sr.obsrecv.EndTracesOp(ctx, "protobuf", td.SpanCount(), err)
	return err
}

// HTTPHandlerFunc returns an http.HandlerFunc that handles SAPM requests
func (sr *sapmReceiver) HTTPHandlerFunc(rw http.ResponseWriter, req *http.Request) {
	// handle the request payload
	err := sr.handleRequest(req)
	if err != nil {
		errorutil.HTTPError(rw, err)
		return
	}

	// respBytes are bytes to write to the http.Response

	// build the response message
	// NOTE currently the response is an empty struct.  As an optimization this receiver will pass a
	// byte array that was generated in the receiver's constructor.  If this receiver needs to return
	// more than an empty struct, then the sapm.PostSpansResponse{} struct will need to be marshaled
	// and on error a http.StatusInternalServerError should be written to the http.ResponseWriter and
	// this function should immediately return.
	respBytes := sr.defaultResponse
	rw.Header().Set(sapmprotocol.ContentTypeHeaderName, sapmprotocol.ContentTypeHeaderValue)

	// write the response if client does not accept gzip encoding
	if req.Header.Get(sapmprotocol.AcceptEncodingHeaderName) != sapmprotocol.GZipEncodingHeaderValue {
		// write the response bytes
		_, err = rw.Write(respBytes)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
		}
		return
	}

	// gzip the response

	// get the gzip writer
	writer := gzipWriterPool.Get().(*gzip.Writer)
	defer gzipWriterPool.Put(writer)

	var gzipBuffer bytes.Buffer

	// reset the writer with the gzip buffer
	writer.Reset(&gzipBuffer)

	// gzip the responseBytes
	_, err = writer.Write(respBytes)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	// close the gzip writer and write gzip footer
	err = writer.Close()
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	// write the successfully gzipped payload
	rw.Header().Set(sapmprotocol.ContentEncodingHeaderName, sapmprotocol.GZipEncodingHeaderValue)
	_, err = rw.Write(gzipBuffer.Bytes())
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
	}
}

// Start starts the sapmReceiver's server.
func (sr *sapmReceiver) Start(ctx context.Context, host component.Host) error {
	// server.Handler will be nil on initial call, otherwise noop.
	if sr.server != nil && sr.server.Handler != nil {
		return nil
	}
	// set up the listener
	ln, err := sr.config.ToListener(ctx)
	if err != nil {
		return fmt.Errorf("failed to bind to address %s: %w", sr.config.Endpoint, err)
	}

	// use gorilla mux to create a router/handler
	nr := mux.NewRouter()
	nr.HandleFunc(sapmprotocol.TraceEndpointV2, sr.HTTPHandlerFunc)

	// create a server with the handler
	sr.server, err = sr.config.ToServer(ctx, host, sr.settings, nr)
	if err != nil {
		return err
	}

	sr.shutdownWG.Add(1)
	// run the server on a routine
	go func() {
		defer sr.shutdownWG.Done()
		if errHTTP := sr.server.Serve(ln); !errors.Is(errHTTP, http.ErrServerClosed) && errHTTP != nil {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(errHTTP))
		}
	}()
	return nil
}

// Shutdown stops the sapmReceiver's server.
func (sr *sapmReceiver) Shutdown(context.Context) error {
	if sr.server == nil {
		return nil
	}
	err := sr.server.Close()
	sr.shutdownWG.Wait()
	return err
}

// this validates at compile time that sapmReceiver implements the receiver.Traces interface
var _ receiver.Traces = (*sapmReceiver)(nil)

// newReceiver creates a sapmReceiver that receives SAPM over http
func newReceiver(
	params receiver.Settings,
	config *Config,
	nextConsumer consumer.Traces,
) (receiver.Traces, error) {
	// build the response message
	defaultResponse := &splunksapm.PostSpansResponse{}
	defaultResponseBytes, err := defaultResponse.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal default response body for %v receiver: %w", params.ID, err)
	}

	transport := "http"
	if config.TLSSetting != nil {
		transport = "https"
	}
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             params.ID,
		Transport:              transport,
		ReceiverCreateSettings: params,
	})
	if err != nil {
		return nil, err
	}
	return &sapmReceiver{
		settings:        params.TelemetrySettings,
		config:          config,
		nextConsumer:    nextConsumer,
		defaultResponse: defaultResponseBytes,
		obsrecv:         obsrecv,
	}, nil
}
