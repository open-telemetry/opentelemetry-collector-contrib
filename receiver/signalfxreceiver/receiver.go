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

package signalfxreceiver

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"
	"unsafe"

	"github.com/gorilla/mux"
	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"go.opencensus.io/trace"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

const (
	defaultServerTimeout = 20 * time.Second

	responseOK                      = "OK"
	responseInvalidMethod           = "Only \"POST\" method is supported"
	responseInvalidContentType      = "\"Content-Type\" must be \"application/x-protobuf\""
	responseInvalidEncoding         = "\"Content-Encoding\" must be \"gzip\" or empty"
	responseErrGzipReader           = "Error on gzip body"
	responseErrReadBody             = "Failed to read message body"
	responseErrUnmarshalBody        = "Failed to unmarshal message body"
	responseErrNextConsumer         = "Internal Server Error"
	responseErrLogsNotConfigured    = "Log pipeline has not been configured to handle events"
	responseErrMetricsNotConfigured = "Metric pipeline has not been configured to handle datapoints"

	// Centralizing some HTTP and related string constants.
	protobufContentType       = "application/x-protobuf"
	gzipEncoding              = "gzip"
	httpContentTypeHeader     = "Content-Type"
	httpContentEncodingHeader = "Content-Encoding"
)

var (
	errEmptyEndpoint = errors.New("empty endpoint")

	okRespBody               = initJSONResponse(responseOK)
	invalidMethodRespBody    = initJSONResponse(responseInvalidMethod)
	invalidContentRespBody   = initJSONResponse(responseInvalidContentType)
	invalidEncodingRespBody  = initJSONResponse(responseInvalidEncoding)
	errGzipReaderRespBody    = initJSONResponse(responseErrGzipReader)
	errReadBodyRespBody      = initJSONResponse(responseErrReadBody)
	errUnmarshalBodyRespBody = initJSONResponse(responseErrUnmarshalBody)
	errNextConsumerRespBody  = initJSONResponse(responseErrNextConsumer)
	errLogsNotConfigured     = initJSONResponse(responseErrLogsNotConfigured)
	errMetricsNotConfigured  = initJSONResponse(responseErrMetricsNotConfigured)
)

// sfxReceiver implements the component.MetricsReceiver for SignalFx metric protocol.
type sfxReceiver struct {
	sync.Mutex
	logger          *zap.Logger
	config          *Config
	metricsConsumer consumer.Metrics
	logsConsumer    consumer.Logs
	server          *http.Server
	obsrecv         *obsreport.Receiver
}

var _ component.MetricsReceiver = (*sfxReceiver)(nil)

// New creates the SignalFx receiver with the given configuration.
func newReceiver(
	logger *zap.Logger,
	config Config,
) *sfxReceiver {
	transport := "http"
	if config.TLSSetting != nil {
		transport = "https"
	}
	r := &sfxReceiver{
		logger:  logger,
		config:  &config,
		obsrecv: obsreport.NewReceiver(obsreport.ReceiverSettings{ReceiverID: config.ID(), Transport: transport}),
	}

	return r
}

func (r *sfxReceiver) RegisterMetricsConsumer(mc consumer.Metrics) {
	r.Lock()
	defer r.Unlock()

	r.metricsConsumer = mc
}

func (r *sfxReceiver) RegisterLogsConsumer(lc consumer.Logs) {
	r.Lock()
	defer r.Unlock()

	r.logsConsumer = lc
}

// Start tells the receiver to start its processing.
// By convention the consumer of the received data is set when the receiver
// instance is created.
func (r *sfxReceiver) Start(_ context.Context, host component.Host) error {
	r.Lock()
	defer r.Unlock()

	if r.metricsConsumer == nil && r.logsConsumer == nil {
		return componenterror.ErrNilNextConsumer
	}

	// set up the listener
	ln, err := r.config.HTTPServerSettings.ToListener()
	if err != nil {
		return fmt.Errorf("failed to bind to address %s: %w", r.config.Endpoint, err)
	}

	mx := mux.NewRouter()
	mx.HandleFunc("/v2/datapoint", r.handleDatapointReq)
	mx.HandleFunc("/v2/event", r.handleEventReq)

	r.server = r.config.HTTPServerSettings.ToServer(mx)

	// TODO: Evaluate what properties should be configurable, for now
	//		set some hard-coded values.
	r.server.ReadHeaderTimeout = defaultServerTimeout
	r.server.WriteTimeout = defaultServerTimeout

	go func() {
		if errHTTP := r.server.Serve(ln); errHTTP != http.ErrServerClosed {
			host.ReportFatalError(errHTTP)
		}
	}()
	return nil
}

// Shutdown tells the receiver that should stop reception,
// giving it a chance to perform any necessary clean-up.
func (r *sfxReceiver) Shutdown(context.Context) error {
	r.Lock()
	defer r.Unlock()

	return r.server.Close()
}

func (r *sfxReceiver) readBody(ctx context.Context, resp http.ResponseWriter, req *http.Request) ([]byte, bool) {
	if req.Method != http.MethodPost {
		r.failRequest(ctx, resp, http.StatusBadRequest, invalidMethodRespBody, nil)
		return nil, false
	}

	if req.Header.Get(httpContentTypeHeader) != protobufContentType {
		r.failRequest(ctx, resp, http.StatusUnsupportedMediaType, invalidContentRespBody, nil)
		return nil, false
	}

	encoding := req.Header.Get(httpContentEncodingHeader)
	if encoding != "" && encoding != gzipEncoding {
		r.failRequest(ctx, resp, http.StatusUnsupportedMediaType, invalidEncodingRespBody, nil)
		return nil, false
	}

	bodyReader := req.Body
	if encoding == gzipEncoding {
		var err error
		bodyReader, err = gzip.NewReader(bodyReader)
		if err != nil {
			r.failRequest(ctx, resp, http.StatusBadRequest, errGzipReaderRespBody, err)
			return nil, false
		}
	}

	body, err := ioutil.ReadAll(bodyReader)
	if err != nil {
		r.failRequest(ctx, resp, http.StatusBadRequest, errReadBodyRespBody, err)
		return nil, false
	}
	return body, true
}

func (r *sfxReceiver) writeResponse(ctx context.Context, resp http.ResponseWriter, err error) {
	if err != nil {
		r.failRequest(ctx, resp, http.StatusInternalServerError, errNextConsumerRespBody, err)
		return
	}

	resp.WriteHeader(http.StatusOK)
	resp.Write(okRespBody)
}

func (r *sfxReceiver) handleDatapointReq(resp http.ResponseWriter, req *http.Request) {
	ctx := r.obsrecv.StartMetricsOp(req.Context())

	if r.metricsConsumer == nil {
		r.failRequest(ctx, resp, http.StatusBadRequest, errMetricsNotConfigured, nil)
		return
	}

	body, ok := r.readBody(ctx, resp, req)
	if !ok {
		return
	}

	msg := &sfxpb.DataPointUploadMessage{}
	if err := msg.Unmarshal(body); err != nil {
		r.failRequest(ctx, resp, http.StatusBadRequest, errUnmarshalBodyRespBody, err)
		return
	}

	if len(msg.Datapoints) == 0 {
		r.obsrecv.EndMetricsOp(ctx, typeStr, 0, nil)
		resp.Write(okRespBody)
		return
	}

	md, _ := signalFxV2ToMetrics(r.logger, msg.Datapoints)

	if r.config.AccessTokenPassthrough {
		if accessToken := req.Header.Get(splunk.SFxAccessTokenHeader); accessToken != "" {
			for i := 0; i < md.ResourceMetrics().Len(); i++ {
				rm := md.ResourceMetrics().At(i)
				res := rm.Resource()
				res.Attributes().Insert(splunk.SFxAccessTokenLabel, pdata.NewAttributeValueString(accessToken))
			}
		}
	}

	err := r.metricsConsumer.ConsumeMetrics(ctx, md)
	r.obsrecv.EndMetricsOp(
		ctx,
		typeStr,
		len(msg.Datapoints),
		err)

	r.writeResponse(ctx, resp, err)
}

func (r *sfxReceiver) handleEventReq(resp http.ResponseWriter, req *http.Request) {
	ctx := r.obsrecv.StartMetricsOp(req.Context())

	if r.logsConsumer == nil {
		r.failRequest(ctx, resp, http.StatusBadRequest, errLogsNotConfigured, nil)
		return
	}

	body, ok := r.readBody(ctx, resp, req)
	if !ok {
		return
	}

	msg := &sfxpb.EventUploadMessage{}
	if err := msg.Unmarshal(body); err != nil {
		r.failRequest(ctx, resp, http.StatusBadRequest, errUnmarshalBodyRespBody, err)
		return
	}

	if len(msg.Events) == 0 {
		r.obsrecv.EndMetricsOp(ctx, typeStr, 0, nil)
		resp.Write(okRespBody)
		return
	}

	ld := pdata.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	ill := rl.InstrumentationLibraryLogs().AppendEmpty()
	signalFxV2EventsToLogRecords(msg.Events, ill.Logs())

	if r.config.AccessTokenPassthrough {
		if accessToken := req.Header.Get(splunk.SFxAccessTokenHeader); accessToken != "" {
			rl.Resource().Attributes().InsertString(splunk.SFxAccessTokenLabel, accessToken)
		}
	}

	err := r.logsConsumer.ConsumeLogs(ctx, ld)
	r.obsrecv.EndMetricsOp(
		ctx,
		typeStr,
		len(msg.Events),
		err)

	r.writeResponse(ctx, resp, err)
}

func (r *sfxReceiver) failRequest(
	ctx context.Context,
	resp http.ResponseWriter,
	httpStatusCode int,
	jsonResponse []byte,
	err error,
) {
	resp.WriteHeader(httpStatusCode)
	if len(jsonResponse) > 0 {
		// The response needs to be written as a JSON string.
		_, writeErr := resp.Write(jsonResponse)
		if writeErr != nil {
			r.logger.Warn(
				"Error writing HTTP response message",
				zap.Error(writeErr),
				zap.String("receiver", r.config.ID().String()))
		}
	}

	// Use the same pattern as strings.Builder String().
	msg := *(*string)(unsafe.Pointer(&jsonResponse))

	reqSpan := trace.FromContext(ctx)
	reqSpan.AddAttributes(
		trace.Int64Attribute(conventions.AttributeHTTPStatusCode, int64(httpStatusCode)),
		trace.StringAttribute(conventions.AttributeHTTPStatusText, msg))
	traceStatus := trace.Status{
		Code: trace.StatusCodeInvalidArgument,
	}
	if httpStatusCode == http.StatusInternalServerError {
		traceStatus.Code = trace.StatusCodeInternal
	}
	if err != nil {
		traceStatus.Message = err.Error()
	}
	reqSpan.SetStatus(traceStatus)
	reqSpan.End()

	r.logger.Debug(
		"SignalFx receiver request failed",
		zap.Int("http_status_code", httpStatusCode),
		zap.String("msg", msg),
		zap.Error(err), // It handles nil error
	)
}

func initJSONResponse(s string) []byte {
	respBody, err := json.Marshal(s)
	if err != nil {
		// This is to be used in initialization so panic here is fine.
		panic(err)
	}
	return respBody
}
