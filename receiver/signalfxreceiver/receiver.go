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
	"net"
	"net/http"
	"sync"
	"time"
	"unsafe"

	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/gorilla/mux"
	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"go.opencensus.io/trace"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
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
	errNilNextConsumer = errors.New("nil nextConsumer")
	errEmptyEndpoint   = errors.New("empty endpoint")

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
	metricsConsumer consumer.MetricsConsumer
	logsConsumer    consumer.LogsConsumer
	server          *http.Server

	startOnce sync.Once
	stopOnce  sync.Once
}

var _ component.MetricsReceiver = (*sfxReceiver)(nil)

// New creates the SignalFx receiver with the given configuration.
func newReceiver(
	logger *zap.Logger,
	config Config,
) *sfxReceiver {
	r := &sfxReceiver{
		logger: logger,
		config: &config,
	}

	return r
}

func (r *sfxReceiver) RegisterMetricsConsumer(mc consumer.MetricsConsumer) {
	r.Lock()
	defer r.Unlock()

	r.metricsConsumer = mc
}

func (r *sfxReceiver) RegisterLogsConsumer(lc consumer.LogsConsumer) {
	r.Lock()
	defer r.Unlock()

	r.logsConsumer = lc
}

// StartMetricsReception tells the receiver to start its processing.
// By convention the consumer of the received data is set when the receiver
// instance is created.
func (r *sfxReceiver) Start(_ context.Context, host component.Host) error {
	r.Lock()
	defer r.Unlock()

	if r.metricsConsumer == nil && r.logsConsumer == nil {
		return errNilNextConsumer
	}

	err := componenterror.ErrAlreadyStarted
	r.startOnce.Do(func() {
		err = nil

		var ln net.Listener
		// set up the listener
		ln, err = r.config.HTTPServerSettings.ToListener()
		if err != nil {
			err = fmt.Errorf("failed to bind to address %s: %w", r.config.Endpoint, err)
			return
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
			if errHTTP := r.server.Serve(ln); errHTTP != nil {
				host.ReportFatalError(errHTTP)
			}
		}()
	})

	return err
}

// StopMetricsReception tells the receiver that should stop reception,
// giving it a chance to perform any necessary clean-up.
func (r *sfxReceiver) Shutdown(context.Context) error {
	r.Lock()
	defer r.Unlock()

	err := componenterror.ErrAlreadyStopped
	r.stopOnce.Do(func() {
		err = r.server.Close()
	})
	return err
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

	resp.WriteHeader(http.StatusAccepted)
	resp.Write(okRespBody)
}

func (r *sfxReceiver) handleDatapointReq(resp http.ResponseWriter, req *http.Request) {
	transport := "http"
	if r.config.TLSSetting != nil {
		transport = "https"
	}

	ctx := obsreport.ReceiverContext(req.Context(), r.config.Name(), transport, r.config.Name())
	ctx = obsreport.StartMetricsReceiveOp(ctx, r.config.Name(), transport)

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
		obsreport.EndMetricsReceiveOp(ctx, typeStr, 0, 0, nil)
		resp.Write(okRespBody)
		return
	}

	md, _ := signalFxV2ToMetricsData(r.logger, msg.Datapoints)

	if r.config.AccessTokenPassthrough {
		if accessToken := req.Header.Get(splunk.SFxAccessTokenHeader); accessToken != "" {
			if md.Resource == nil {
				md.Resource = &resourcepb.Resource{}
			}
			if md.Resource.Labels == nil {
				md.Resource.Labels = make(map[string]string, 1)
			}
			md.Resource.Labels[splunk.SFxAccessTokenLabel] = accessToken
		}
	}

	err := r.metricsConsumer.ConsumeMetrics(ctx, internaldata.OCToMetrics(md))
	obsreport.EndMetricsReceiveOp(
		ctx,
		typeStr,
		len(msg.Datapoints),
		len(msg.Datapoints),
		err)

	r.writeResponse(ctx, resp, err)
}

func (r *sfxReceiver) handleEventReq(resp http.ResponseWriter, req *http.Request) {
	transport := "http"
	if r.config.TLSSetting != nil {
		transport = "https"
	}

	ctx := obsreport.ReceiverContext(req.Context(), r.config.Name(), transport, r.config.Name())
	ctx = obsreport.StartMetricsReceiveOp(ctx, r.config.Name(), transport)

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
		obsreport.EndMetricsReceiveOp(ctx, typeStr, 0, 0, nil)
		resp.Write(okRespBody)
		return
	}

	logSlice := signalFxV2EventsToLogRecords(r.logger, msg.Events)

	ld := pdata.NewLogs()
	rls := ld.ResourceLogs()
	rls.Resize(1)
	rl := rls.At(0)
	resource := rl.Resource()

	ills := rl.InstrumentationLibraryLogs()
	ills.Resize(1)
	ill := ills.At(0)

	logSlice.MoveAndAppendTo(ill.Logs())

	if r.config.AccessTokenPassthrough {
		if accessToken := req.Header.Get(splunk.SFxAccessTokenHeader); accessToken != "" {
			resource.InitEmpty()
			resource.Attributes().InsertString(splunk.SFxAccessTokenLabel, accessToken)
		}
	}

	err := r.logsConsumer.ConsumeLogs(ctx, ld)
	obsreport.EndMetricsReceiveOp(
		ctx,
		typeStr,
		len(msg.Events),
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
				zap.String("receiver", r.config.Name()))
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
		zap.String("receiver", r.config.Name()))
}

func initJSONResponse(s string) []byte {
	respBody, err := json.Marshal(s)
	if err != nil {
		// This is to be used in initialization so panic here is fine.
		panic(err)
	}
	return respBody
}
