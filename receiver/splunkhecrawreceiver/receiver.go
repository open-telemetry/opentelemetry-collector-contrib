// Copyright 2021, OpenTelemetry Authors
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

package splunkhecrawreceiver

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

const (
	defaultServerTimeout = 20 * time.Second

	responseInvalidMethod          = `Only "POST" method is supported`
	responseInvalidEncoding        = `"Content-Encoding" must be "gzip" or empty`
	responseErrGzipReader          = "Error on gzip body"
	responseErrInternalServerError = "Internal Server Error"

	// Centralizing some HTTP and related string constants.
	gzipEncoding              = "gzip"
	httpContentEncodingHeader = "Content-Encoding"
)

var (
	errNilNextLogsConsumer = errors.New("nil logsConsumer")
	errEmptyEndpoint       = errors.New("empty endpoint")
	errInvalidMethod       = errors.New("invalid http method")
	errInvalidEncoding     = errors.New("invalid encoding")

	invalidMethodRespBody   = initJSONResponse(responseInvalidMethod)
	invalidEncodingRespBody = initJSONResponse(responseInvalidEncoding)
	errGzipReaderRespBody   = initJSONResponse(responseErrGzipReader)
	errInternalServerError  = initJSONResponse(responseErrInternalServerError)
)

// newLogsReceiver creates the Splunk HEC receiver with the given configuration.
func newLogsReceiver(
	logger *zap.Logger,
	config Config,
	nextConsumer consumer.Logs,
) (component.LogsReceiver, error) {
	if nextConsumer == nil {
		return nil, errNilNextLogsConsumer
	}

	if config.Endpoint == "" {
		return nil, errEmptyEndpoint
	}

	transport := "http"
	if config.TLSSetting != nil {
		transport = "https"
	}

	r := &splunkReceiver{
		logger:       logger,
		config:       &config,
		logsConsumer: nextConsumer,
		server: &http.Server{
			Addr: config.Endpoint,
			// TODO: Evaluate what properties should be configurable, for now
			//		set some hard-coded values.
			ReadHeaderTimeout: defaultServerTimeout,
			WriteTimeout:      defaultServerTimeout,
		},
		obsrecv: obsreport.NewReceiver(obsreport.ReceiverSettings{ReceiverID: config.ID(), Transport: transport}),
	}

	return r, nil
}

// splunkReceiver implements the component.LogsReceiver for Splunk HEC raw protocol.
type splunkReceiver struct {
	logger             *zap.Logger
	config             *Config
	logsConsumer       consumer.Logs
	server             *http.Server
	obsrecv            *obsreport.Receiver
	resourceCustomizer func(*http.Request, pdata.Resource)
	gzipReaderPool     sync.Pool
}

// Start tells the receiver to start its processing.
// By convention the consumer of the received data is set when the receiver
// instance is created.
func (r *splunkReceiver) Start(_ context.Context, host component.Host) error {
	var ln net.Listener
	// set up the listener
	ln, err := r.config.HTTPServerSettings.ToListener()
	if err != nil {
		return fmt.Errorf("failed to bind to address %s: %w", r.config.Endpoint, err)
	}

	mx := mux.NewRouter()
	mx.NewRoute().HandlerFunc(r.handleReq)

	r.server = r.config.HTTPServerSettings.ToServer(mx)

	// TODO: Evaluate what properties should be configurable, for now
	//		set some hard-coded values.
	r.server.ReadHeaderTimeout = defaultServerTimeout
	r.server.WriteTimeout = defaultServerTimeout
	r.resourceCustomizer = r.createResourceCustomizer()
	r.gzipReaderPool = sync.Pool{New: func() interface{} { return new(gzip.Reader) }}

	go func() {
		if errHTTP := r.server.Serve(ln); errHTTP != http.ErrServerClosed {
			host.ReportFatalError(errHTTP)
		}
	}()

	return err
}

// Shutdown tells the receiver that should stop reception,
// giving it a chance to perform any necessary clean-up.
func (r *splunkReceiver) Shutdown(context.Context) error {
	return r.server.Close()
}

func (r *splunkReceiver) handleReq(resp http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	ctx = r.obsrecv.StartLogsOp(ctx)

	if req.Method != http.MethodPost {
		r.failRequest(ctx, resp, http.StatusBadRequest, invalidMethodRespBody, 0, errInvalidMethod)
		return
	}

	encoding := req.Header.Get(httpContentEncodingHeader)
	if encoding != "" && encoding != gzipEncoding {
		r.failRequest(ctx, resp, http.StatusUnsupportedMediaType, invalidEncodingRespBody, 0, errInvalidEncoding)
		return
	}

	if req.ContentLength == 0 {
		r.obsrecv.EndLogsOp(ctx, typeStr, 0, nil)
		return
	}

	bodyReader := req.Body
	if encoding == gzipEncoding {
		reader := r.gzipReaderPool.Get().(*gzip.Reader)
		err := reader.Reset(bodyReader)
		bodyReader = reader

		if err != nil {
			r.failRequest(ctx, resp, http.StatusBadRequest, errGzipReaderRespBody, 0, err)
			_, _ = ioutil.ReadAll(req.Body)
			_ = req.Body.Close()
			return
		}
	}

	sc := bufio.NewScanner(bodyReader)

	ld := pdata.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	r.resourceCustomizer(req, rl.Resource())
	ill := rl.InstrumentationLibraryLogs().AppendEmpty()

	for sc.Scan() {
		logRecord := ill.Logs().AppendEmpty()
		logLine := sc.Text()
		logRecord.Body().SetStringVal(logLine)
	}
	decodeErr := r.logsConsumer.ConsumeLogs(ctx, ld)

	_ = bodyReader.Close()

	if decodeErr != nil {
		r.failRequest(ctx, resp, http.StatusInternalServerError, errInternalServerError, ill.Logs().Len(), decodeErr)
	} else {
		resp.WriteHeader(http.StatusAccepted)
		r.obsrecv.EndLogsOp(ctx, typeStr, ill.Logs().Len(), nil)
	}
}

func (r *splunkReceiver) failRequest(
	ctx context.Context,
	resp http.ResponseWriter,
	httpStatusCode int,
	jsonResponse []byte,
	numLogsReceived int,
	err error,
) {
	resp.WriteHeader(httpStatusCode)
	if len(jsonResponse) > 0 {
		// The response needs to be written as a JSON string.
		resp.Header().Add("Content-Type", "application/json")
		_, writeErr := resp.Write(jsonResponse)
		if writeErr != nil {
			r.logger.Warn("Error writing HTTP response message", zap.Error(writeErr))
		}
	}

	r.obsrecv.EndLogsOp(ctx, typeStr, numLogsReceived, err)

	if r.logger.Core().Enabled(zap.DebugLevel) {
		msg := string(jsonResponse)
		r.logger.Debug(
			"Splunk HEC raw receiver request failed",
			zap.Int("http_status_code", httpStatusCode),
			zap.String("msg", msg),
			zap.Error(err), // It handles nil error
		)
	}
}

func initJSONResponse(s string) []byte {
	respBody, err := json.Marshal(s)
	if err != nil {
		// This is to be used in initialization so panic here is fine.
		panic(err)
	}
	return respBody
}

func (r *splunkReceiver) createResourceCustomizer() func(*http.Request, pdata.Resource) {
	if r.config.AccessTokenPassthrough {
		return func(req *http.Request, resource pdata.Resource) {
			if accessToken := req.Header.Get(splunk.HECTokenHeader); accessToken != "" {
				resource.Attributes().InsertString(splunk.HecTokenLabel, accessToken)
			}
		}
	}
	return func(req *http.Request, resource pdata.Resource) {}
}
