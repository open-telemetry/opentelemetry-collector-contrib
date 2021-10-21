// Copyright 2020, OpenTelemetry Authors
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

package splunkhecreceiver

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

	responseOK                        = "OK"
	responseInvalidMethod             = `Only "POST" method is supported`
	responseInvalidEncoding           = `"Content-Encoding" must be "gzip" or empty`
	responseErrGzipReader             = "Error on gzip body"
	responseErrUnmarshalBody          = "Failed to unmarshal message body"
	responseErrInternalServerError    = "Internal Server Error"
	responseErrUnsupportedMetricEvent = "Unsupported metric event"
	responseErrUnsupportedLogEvent    = "Unsupported log event"

	// Centralizing some HTTP and related string constants.
	gzipEncoding              = "gzip"
	httpContentEncodingHeader = "Content-Encoding"
)

var (
	errNilNextMetricsConsumer = errors.New("nil metricsConsumer")
	errNilNextLogsConsumer    = errors.New("nil logsConsumer")
	errEmptyEndpoint          = errors.New("empty endpoint")
	errInvalidMethod          = errors.New("invalid http method")
	errInvalidEncoding        = errors.New("invalid encoding")

	okRespBody                = initJSONResponse(responseOK)
	invalidMethodRespBody     = initJSONResponse(responseInvalidMethod)
	invalidEncodingRespBody   = initJSONResponse(responseInvalidEncoding)
	errGzipReaderRespBody     = initJSONResponse(responseErrGzipReader)
	errUnmarshalBodyRespBody  = initJSONResponse(responseErrUnmarshalBody)
	errInternalServerError    = initJSONResponse(responseErrInternalServerError)
	errUnsupportedMetricEvent = initJSONResponse(responseErrUnsupportedMetricEvent)
	errUnsupportedLogEvent    = initJSONResponse(responseErrUnsupportedLogEvent)
)

// splunkReceiver implements the component.MetricsReceiver for Splunk HEC metric protocol.
type splunkReceiver struct {
	settings        component.ReceiverCreateSettings
	config          *Config
	logsConsumer    consumer.Logs
	metricsConsumer consumer.Metrics
	server          *http.Server
	obsrecv         *obsreport.Receiver
	gzipReaderPool  *sync.Pool
}

var _ component.MetricsReceiver = (*splunkReceiver)(nil)

// newMetricsReceiver creates the Splunk HEC receiver with the given configuration.
func newMetricsReceiver(
	settings component.ReceiverCreateSettings,
	config Config,
	nextConsumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	if nextConsumer == nil {
		return nil, errNilNextMetricsConsumer
	}

	if config.Endpoint == "" {
		return nil, errEmptyEndpoint
	}

	transport := "http"
	if config.TLSSetting != nil {
		transport = "https"
	}

	r := &splunkReceiver{
		settings:        settings,
		config:          &config,
		metricsConsumer: nextConsumer,
		server: &http.Server{
			Addr: config.Endpoint,
			// TODO: Evaluate what properties should be configurable, for now
			//		set some hard-coded values.
			ReadHeaderTimeout: defaultServerTimeout,
			WriteTimeout:      defaultServerTimeout,
		},
		obsrecv: obsreport.NewReceiver(obsreport.ReceiverSettings{
			ReceiverID:             config.ID(),
			Transport:              transport,
			ReceiverCreateSettings: settings,
		}),
		gzipReaderPool: &sync.Pool{New: func() interface{} { return new(gzip.Reader) }},
	}

	return r, nil
}

// newLogsReceiver creates the Splunk HEC receiver with the given configuration.
func newLogsReceiver(
	settings component.ReceiverCreateSettings,
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
		settings:     settings,
		config:       &config,
		logsConsumer: nextConsumer,
		server: &http.Server{
			Addr: config.Endpoint,
			// TODO: Evaluate what properties should be configurable, for now
			//		set some hard-coded values.
			ReadHeaderTimeout: defaultServerTimeout,
			WriteTimeout:      defaultServerTimeout,
		},
		gzipReaderPool: &sync.Pool{New: func() interface{} { return new(gzip.Reader) }},
		obsrecv: obsreport.NewReceiver(obsreport.ReceiverSettings{
			ReceiverID:             config.ID(),
			Transport:              transport,
			ReceiverCreateSettings: settings,
		}),
	}

	return r, nil
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
	if r.logsConsumer != nil {
		mx.NewRoute().Path(r.config.RawPath).HandlerFunc(r.handleRawReq)
	}
	mx.NewRoute().HandlerFunc(r.handleReq)

	r.server = r.config.HTTPServerSettings.ToServer(mx, r.settings.TelemetrySettings)

	// TODO: Evaluate what properties should be configurable, for now
	//		set some hard-coded values.
	r.server.ReadHeaderTimeout = defaultServerTimeout
	r.server.WriteTimeout = defaultServerTimeout

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

func (r *splunkReceiver) handleRawReq(resp http.ResponseWriter, req *http.Request) {
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

		if err != nil {
			r.failRequest(ctx, resp, http.StatusBadRequest, errGzipReaderRespBody, 0, err)
			_, _ = ioutil.ReadAll(req.Body)
			_ = req.Body.Close()
			return
		}
		bodyReader = reader
		defer r.gzipReaderPool.Put(reader)
	}

	sc := bufio.NewScanner(bodyReader)

	ld := pdata.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	resourceCustomizer := r.createResourceCustomizer(req)
	if resourceCustomizer != nil {
		resourceCustomizer(rl.Resource())
	}
	ill := rl.InstrumentationLibraryLogs().AppendEmpty()

	for sc.Scan() {
		logRecord := ill.Logs().AppendEmpty()
		logLine := sc.Text()
		logRecord.Body().SetStringVal(logLine)
	}
	consumerErr := r.logsConsumer.ConsumeLogs(ctx, ld)

	_ = bodyReader.Close()

	if consumerErr != nil {
		r.failRequest(ctx, resp, http.StatusInternalServerError, errInternalServerError, ill.Logs().Len(), consumerErr)
	} else {
		resp.WriteHeader(http.StatusAccepted)
		r.obsrecv.EndLogsOp(ctx, typeStr, ill.Logs().Len(), nil)
	}
}

func (r *splunkReceiver) handleReq(resp http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	if r.logsConsumer == nil {
		ctx = r.obsrecv.StartMetricsOp(ctx)
	} else {
		ctx = r.obsrecv.StartLogsOp(ctx)
	}

	if req.Method != http.MethodPost {
		r.failRequest(ctx, resp, http.StatusBadRequest, invalidMethodRespBody, 0, errInvalidMethod)
		return
	}

	encoding := req.Header.Get(httpContentEncodingHeader)
	if encoding != "" && encoding != gzipEncoding {
		r.failRequest(ctx, resp, http.StatusUnsupportedMediaType, invalidEncodingRespBody, 0, errInvalidEncoding)
		return
	}

	bodyReader := req.Body
	if encoding == gzipEncoding {
		reader := r.gzipReaderPool.Get().(*gzip.Reader)
		err := reader.Reset(bodyReader)
		if err != nil {
			r.failRequest(ctx, resp, http.StatusBadRequest, errGzipReaderRespBody, 0, err)
			return
		}
		bodyReader = reader
		defer r.gzipReaderPool.Put(reader)
	}

	if req.ContentLength == 0 {
		resp.Write(okRespBody)
		return
	}

	dec := json.NewDecoder(bodyReader)

	var events []*splunk.Event

	for dec.More() {
		var msg splunk.Event
		err := dec.Decode(&msg)
		if err != nil {
			r.failRequest(ctx, resp, http.StatusBadRequest, errUnmarshalBodyRespBody, len(events), err)
			return
		}
		if msg.IsMetric() {
			if r.metricsConsumer == nil {
				r.failRequest(ctx, resp, http.StatusBadRequest, errUnsupportedMetricEvent, len(events), err)
				return
			}
		} else if r.logsConsumer == nil {
			r.failRequest(ctx, resp, http.StatusBadRequest, errUnsupportedLogEvent, len(events), err)
			return
		}

		events = append(events, &msg)
	}
	if r.logsConsumer != nil {
		r.consumeLogs(ctx, events, resp, req)
	} else {
		r.consumeMetrics(ctx, events, resp, req)
	}
}

func (r *splunkReceiver) consumeMetrics(ctx context.Context, events []*splunk.Event, resp http.ResponseWriter, req *http.Request) {
	resourceCustomizer := r.createResourceCustomizer(req)
	md, _ := splunkHecToMetricsData(r.settings.Logger, events, resourceCustomizer, r.config)

	decodeErr := r.metricsConsumer.ConsumeMetrics(ctx, md)
	r.obsrecv.EndMetricsOp(ctx, typeStr, len(events), decodeErr)

	if decodeErr != nil {
		r.failRequest(ctx, resp, http.StatusInternalServerError, errInternalServerError, len(events), decodeErr)
	} else {
		resp.WriteHeader(http.StatusAccepted)
		resp.Write(okRespBody)
	}
}

func (r *splunkReceiver) consumeLogs(ctx context.Context, events []*splunk.Event, resp http.ResponseWriter, req *http.Request) {
	resourceCustomizer := r.createResourceCustomizer(req)
	ld, err := splunkHecToLogData(r.settings.Logger, events, resourceCustomizer, r.config)
	if err != nil {
		r.failRequest(ctx, resp, http.StatusBadRequest, errUnmarshalBodyRespBody, len(events), err)
		return
	}

	decodeErr := r.logsConsumer.ConsumeLogs(ctx, ld)
	r.obsrecv.EndLogsOp(ctx, typeStr, len(events), decodeErr)
	if decodeErr != nil {
		r.failRequest(ctx, resp, http.StatusInternalServerError, errInternalServerError, len(events), decodeErr)
	} else {
		resp.WriteHeader(http.StatusAccepted)
		resp.Write(okRespBody)
	}
}

func (r *splunkReceiver) createResourceCustomizer(req *http.Request) func(resource pdata.Resource) {
	if r.config.AccessTokenPassthrough {
		accessToken := req.Header.Get(splunk.HECTokenHeader)
		if accessToken != "" {
			return func(resource pdata.Resource) {
				resource.Attributes().InsertString(splunk.HecTokenLabel, accessToken)
			}
		}
	}
	return nil
}

func (r *splunkReceiver) failRequest(
	ctx context.Context,
	resp http.ResponseWriter,
	httpStatusCode int,
	jsonResponse []byte,
	numRecordsReceived int,
	err error,
) {
	resp.WriteHeader(httpStatusCode)
	if len(jsonResponse) > 0 {
		// The response needs to be written as a JSON string.
		resp.Header().Add("Content-Type", "application/json")
		_, writeErr := resp.Write(jsonResponse)
		if writeErr != nil {
			r.settings.Logger.Warn("Error writing HTTP response message", zap.Error(writeErr))
		}
	}

	if r.metricsConsumer == nil {
		r.obsrecv.EndLogsOp(ctx, typeStr, numRecordsReceived, err)
	} else {
		r.obsrecv.EndMetricsOp(ctx, typeStr, numRecordsReceived, err)
	}

	if r.settings.Logger.Core().Enabled(zap.DebugLevel) {
		msg := string(jsonResponse)
		r.settings.Logger.Debug(
			"Splunk HEC receiver request failed",
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
