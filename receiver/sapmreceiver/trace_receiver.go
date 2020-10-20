// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sapmreceiver

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	splunksapm "github.com/signalfx/sapm-proto/gen"
	"github.com/signalfx/sapm-proto/sapmprotocol"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	jaegertranslator "go.opentelemetry.io/collector/translator/trace/jaeger"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
)

var gzipWriterPool = &sync.Pool{
	New: func() interface{} {
		return gzip.NewWriter(ioutil.Discard)
	},
}

// sapmReceiver receives spans in the Splunk SAPM format over HTTP
type sapmReceiver struct {
	// mu protects the fields of this type
	mu        sync.Mutex
	startOnce sync.Once
	stopOnce  sync.Once
	logger    *zap.Logger

	config *Config
	server *http.Server

	nextConsumer consumer.TracesConsumer

	// defaultResponse is a placeholder. For now this receiver returns an empty sapm response.
	// This defaultResponse is an optimization so we don't have to proto.Marshal the response
	// for every request. At some point this may be removed when there is actual content to return.
	defaultResponse []byte
}

// handleRequest parses an http request containing sapm and passes the trace data to the next consumer
func (sr *sapmReceiver) handleRequest(ctx context.Context, req *http.Request) error {
	sapm, err := sapmprotocol.ParseTraceV2Request(req)
	// errors processing the request should return http.StatusBadRequest
	if err != nil {
		return err
	}

	transport := "http"
	if sr.config.TLSSetting != nil {
		transport = "https"
	}
	ctx = obsreport.ReceiverContext(ctx, sr.config.Name(), transport, "")
	ctx = obsreport.StartTraceDataReceiveOp(ctx, sr.config.Name(), transport)

	td := jaegertranslator.ProtoBatchesToInternalTraces(sapm.Batches)

	if sr.config.AccessTokenPassthrough {
		if accessToken := req.Header.Get(splunk.SFxAccessTokenHeader); accessToken != "" {
			rSpans := td.ResourceSpans()
			for i := 0; i < rSpans.Len(); i++ {
				rSpan := rSpans.At(i)
				if !rSpan.IsNil() {
					attrs := rSpan.Resource().Attributes()
					attrs.UpsertString(splunk.SFxAccessTokenLabel, accessToken)
				}
			}
		}
	}

	// pass the trace data to the next consumer
	err = sr.nextConsumer.ConsumeTraces(ctx, td)
	if err != nil {
		err = fmt.Errorf("error passing trace data to next consumer: %v", err.Error())
	}

	obsreport.EndTraceDataReceiveOp(ctx, "protobuf", td.SpanCount(), err)
	return err
}

// HTTPHandlerFunction returns an http.HandlerFunc that handles SAPM requests
func (sr *sapmReceiver) HTTPHandlerFunc(rw http.ResponseWriter, req *http.Request) {
	// create context with the receiver name from the request context
	ctx := obsreport.ReceiverContext(req.Context(), sr.config.Name(), "http", "")

	// handle the request payload
	err := sr.handleRequest(ctx, req)
	if err != nil {
		// TODO account for this error (throttled logging or metrics)
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	// respBytes are bytes to write to the http.Response

	// build the response message
	// NOTE currently the response is an empty struct.  As an optimization this receiver will pass a
	// byte array that was generated in the receiver's constructor.  If this receiver needs to return
	// more than an empty struct, then the sapm.PostSpansResponse{} struct will need to be marshaled
	// and on error a http.StatusInternalServerError should be written to the http.ResponseWriter and
	// this function should immediately return.
	var respBytes = sr.defaultResponse
	rw.Header().Set(sapmprotocol.ContentTypeHeaderName, sapmprotocol.ContentTypeHeaderValue)

	// write the response if client does not accept gzip encoding
	if req.Header.Get(sapmprotocol.AcceptEncodingHeaderName) != sapmprotocol.GZipEncodingHeaderValue {
		// write the response bytes
		rw.Write(respBytes)
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
	rw.Write(gzipBuffer.Bytes())
}

// StartTraceReception starts the sapmReceiver's server
func (sr *sapmReceiver) Start(_ context.Context, host component.Host) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	var err = componenterror.ErrAlreadyStarted
	sr.startOnce.Do(func() {
		var ln net.Listener

		// set up the listener
		ln, err = sr.config.HTTPServerSettings.ToListener()
		if err != nil {
			err = fmt.Errorf("failed to bind to address %s: %w", sr.config.Endpoint, err)
			return
		}

		// use gorilla mux to create a router/handler
		nr := mux.NewRouter()
		nr.HandleFunc(sapmprotocol.TraceEndpointV2, sr.HTTPHandlerFunc)

		// create a server with the handler
		sr.server = sr.config.HTTPServerSettings.ToServer(nr)

		// run the server on a routine
		go func() {
			if errHTTP := sr.server.Serve(ln); errHTTP != nil {
				host.ReportFatalError(errHTTP)
			}
		}()
	})
	return err
}

// StopTraceRetention stops the the sapmReceiver's server
func (sr *sapmReceiver) Shutdown(context.Context) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	var err = componenterror.ErrAlreadyStopped
	sr.stopOnce.Do(func() {
		if sr.server != nil {
			err = sr.server.Close()
			sr.server = nil
		}
	})

	return err
}

// this validates at compile time that sapmReceiver implements the component.TraceReceiver interface
var _ component.TraceReceiver = (*sapmReceiver)(nil)

// New creates a sapmReceiver that receives SAPM over http
func New(
	ctx context.Context,
	params component.ReceiverCreateParams,
	config *Config,
	nextConsumer consumer.TracesConsumer,
) (component.TraceReceiver, error) {
	// build the response message
	defaultResponse := &splunksapm.PostSpansResponse{}
	defaultResponseBytes, err := defaultResponse.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal default response body for %s receiver: %v", config.Name(), err)
	}
	return &sapmReceiver{
		logger:          params.Logger,
		config:          config,
		nextConsumer:    nextConsumer,
		defaultResponse: defaultResponseBytes,
	}, nil
}
