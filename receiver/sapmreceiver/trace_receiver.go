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
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	splunksapm "github.com/signalfx/sapm-proto/gen"
	"github.com/signalfx/sapm-proto/sapmprotocol"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
)

var gzipWriterPool = &sync.Pool{
	New: func() interface{} {
		return gzip.NewWriter(ioutil.Discard)
	},
}

// sapmReceiver receives spans in the Splunk SAPM format over HTTP
type sapmReceiver struct {
	settings component.TelemetrySettings

	config *Config
	server *http.Server

	nextConsumer consumer.Traces

	// defaultResponse is a placeholder. For now this receiver returns an empty sapm response.
	// This defaultResponse is an optimization so we don't have to proto.Marshal the response
	// for every request. At some point this may be removed when there is actual content to return.
	defaultResponse []byte

	obsrecv *obsreport.Receiver
}

// handleRequest parses an http request containing sapm and passes the trace data to the next consumer
func (sr *sapmReceiver) handleRequest(req *http.Request) error {
	sapm, err := sapmprotocol.ParseTraceV2Request(req)
	// errors processing the request should return http.StatusBadRequest
	if err != nil {
		return err
	}

	ctx := sr.obsrecv.StartTracesOp(req.Context())

	td := jaeger.ProtoBatchesToInternalTraces(sapm.Batches)

	if sr.config.AccessTokenPassthrough {
		if accessToken := req.Header.Get(splunk.SFxAccessTokenHeader); accessToken != "" {
			rSpans := td.ResourceSpans()
			for i := 0; i < rSpans.Len(); i++ {
				rSpan := rSpans.At(i)
				attrs := rSpan.Resource().Attributes()
				attrs.UpsertString(splunk.SFxAccessTokenLabel, accessToken)
			}
		}
	}

	// pass the trace data to the next consumer
	err = sr.nextConsumer.ConsumeTraces(ctx, td)
	if err != nil {
		err = fmt.Errorf("error passing trace data to next consumer: %v", err.Error())
	}

	sr.obsrecv.EndTracesOp(ctx, "protobuf", td.SpanCount(), err)
	return err
}

// HTTPHandlerFunc returns an http.HandlerFunc that handles SAPM requests
func (sr *sapmReceiver) HTTPHandlerFunc(rw http.ResponseWriter, req *http.Request) {
	// handle the request payload
	err := sr.handleRequest(req)
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

// Start starts the sapmReceiver's server.
func (sr *sapmReceiver) Start(_ context.Context, host component.Host) error {
	// set up the listener
	ln, err := sr.config.HTTPServerSettings.ToListener()
	if err != nil {
		return fmt.Errorf("failed to bind to address %s: %w", sr.config.Endpoint, err)
	}

	// use gorilla mux to create a router/handler
	nr := mux.NewRouter()
	nr.HandleFunc(sapmprotocol.TraceEndpointV2, sr.HTTPHandlerFunc)

	// create a server with the handler
	sr.server = sr.config.HTTPServerSettings.ToServer(nr, sr.settings)

	// run the server on a routine
	go func() {
		if errHTTP := sr.server.Serve(ln); errHTTP != http.ErrServerClosed {
			host.ReportFatalError(errHTTP)
		}
	}()
	return nil
}

// Shutdown stops the the sapmReceiver's server.
func (sr *sapmReceiver) Shutdown(context.Context) error {
	return sr.server.Close()
}

// this validates at compile time that sapmReceiver implements the component.TracesReceiver interface
var _ component.TracesReceiver = (*sapmReceiver)(nil)

// newReceiver creates a sapmReceiver that receives SAPM over http
func newReceiver(
	params component.ReceiverCreateSettings,
	config *Config,
	nextConsumer consumer.Traces,
) (component.TracesReceiver, error) {
	// build the response message
	defaultResponse := &splunksapm.PostSpansResponse{}
	defaultResponseBytes, err := defaultResponse.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal default response body for %v receiver: %w", config.ID(), err)
	}
	transport := "http"
	if config.TLSSetting != nil {
		transport = "https"
	}
	return &sapmReceiver{
		settings:        params.TelemetrySettings,
		config:          config,
		nextConsumer:    nextConsumer,
		defaultResponse: defaultResponseBytes,
		obsrecv: obsreport.NewReceiver(obsreport.ReceiverSettings{
			ReceiverID:             config.ID(),
			Transport:              transport,
			ReceiverCreateSettings: params,
		}),
	}, nil
}
