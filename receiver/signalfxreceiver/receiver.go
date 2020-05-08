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
	"io/ioutil"
	"net/http"
	"sync"
	"time"
	"unsafe"

	"github.com/golang/protobuf/proto"
	"github.com/gorilla/mux"
	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/component/componenterror"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"github.com/open-telemetry/opentelemetry-collector/obsreport"
	"github.com/open-telemetry/opentelemetry-collector/translator/conventions"
	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
)

const (
	defaultServerTimeout = 20 * time.Second

	responseOK                 = "OK"
	responseInvalidMethod      = "Only \"POST\" method is supported"
	responseInvalidContentType = "\"Content-Type\" must be \"application/x-protobuf\""
	responseInvalidEncoding    = "\"Content-Encoding\" must be \"gzip\" or empty"
	responseErrGzipReader      = "Error on gzip body"
	responseErrReadBody        = "Failed to read message body"
	responseErrUnmarshalBody   = "Failed to unmarshal message body"
	responseErrNextConsumer    = "Internal Server Error"

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
)

// sfxReceiver implements the component.MetricsReceiver for SignalFx metric protocol.
type sfxReceiver struct {
	sync.Mutex
	logger       *zap.Logger
	config       *Config
	nextConsumer consumer.MetricsConsumerOld
	server       *http.Server

	startOnce sync.Once
	stopOnce  sync.Once
}

var _ component.MetricsReceiver = (*sfxReceiver)(nil)

// New creates the SignalFx receiver with the given configuration.
func New(
	logger *zap.Logger,
	config Config,
	nextConsumer consumer.MetricsConsumerOld,
) (component.MetricsReceiver, error) {

	if nextConsumer == nil {
		return nil, errNilNextConsumer
	}

	if config.Endpoint == "" {
		return nil, errEmptyEndpoint
	}

	r := &sfxReceiver{
		logger:       logger,
		config:       &config,
		nextConsumer: nextConsumer,
		server: &http.Server{
			Addr: config.Endpoint,
			// TODO: Evaluate what properties should be configurable, for now
			//		set some hard-coded values.
			ReadHeaderTimeout: defaultServerTimeout,
			WriteTimeout:      defaultServerTimeout,
		},
	}

	mx := mux.NewRouter()
	mx.HandleFunc("/v2/datapoint", r.handleReq)
	r.server.Handler = mx

	return r, nil
}

// StartMetricsReception tells the receiver to start its processing.
// By convention the consumer of the received data is set when the receiver
// instance is created.
func (r *sfxReceiver) Start(_ context.Context, host component.Host) error {
	r.Lock()
	defer r.Unlock()

	err := componenterror.ErrAlreadyStarted
	r.startOnce.Do(func() {
		err = nil

		go func() {
			if err = r.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				host.ReportFatalError(err)
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

func (r *sfxReceiver) handleReq(resp http.ResponseWriter, req *http.Request) {
	ctx := obsreport.ReceiverContext(req.Context(), r.config.Name(), "http", r.config.Name())
	ctx = obsreport.StartMetricsReceiveOp(ctx, r.config.Name(), "http")

	if req.Method != http.MethodPost {
		r.failRequest(ctx, resp, http.StatusBadRequest, invalidMethodRespBody, nil)
		return
	}

	if req.Header.Get(httpContentTypeHeader) != protobufContentType {
		r.failRequest(ctx, resp, http.StatusUnsupportedMediaType, invalidContentRespBody, nil)
		return
	}

	encoding := req.Header.Get(httpContentEncodingHeader)
	if encoding != "" && encoding != gzipEncoding {
		r.failRequest(ctx, resp, http.StatusUnsupportedMediaType, invalidEncodingRespBody, nil)
		return
	}

	bodyReader := req.Body
	if encoding == gzipEncoding {
		var err error
		bodyReader, err = gzip.NewReader(bodyReader)
		if err != nil {
			r.failRequest(ctx, resp, http.StatusBadRequest, errGzipReaderRespBody, err)
			return
		}
	}

	body, err := ioutil.ReadAll(bodyReader)
	if err != nil {
		r.failRequest(ctx, resp, http.StatusBadRequest, errReadBodyRespBody, err)
		return
	}

	msg := &sfxpb.DataPointUploadMessage{}
	if err = proto.Unmarshal(body, msg); err != nil {
		r.failRequest(ctx, resp, http.StatusBadRequest, errUnmarshalBodyRespBody, err)
		return
	}

	if len(msg.Datapoints) == 0 {
		obsreport.EndMetricsReceiveOp(ctx, typeStr, 0, 0, nil)
		resp.Write(okRespBody)
		return
	}

	md, _ := SignalFxV2ToMetricsData(r.logger, msg.Datapoints)

	err = r.nextConsumer.ConsumeMetricsData(ctx, *md)
	obsreport.EndMetricsReceiveOp(
		ctx,
		typeStr,
		len(msg.Datapoints),
		len(msg.Datapoints),
		err)
	if err != nil {
		r.failRequest(ctx, resp, http.StatusInternalServerError, errNextConsumerRespBody, err)
		return
	}

	resp.WriteHeader(http.StatusAccepted)
	resp.Write(okRespBody)
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
