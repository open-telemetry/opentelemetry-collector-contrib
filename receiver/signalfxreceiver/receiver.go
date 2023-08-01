// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signalfxreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver"

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
	"unsafe"

	"github.com/gorilla/mux"
	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"go.opencensus.io/trace"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/signalfx"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver/internal/metadata"
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

	translator = &signalfx.ToTranslator{}
)

// sfxReceiver implements the receiver.Metrics for SignalFx metric protocol.
type sfxReceiver struct {
	settings        receiver.CreateSettings
	config          *Config
	metricsConsumer consumer.Metrics
	logsConsumer    consumer.Logs
	server          *http.Server
	shutdownWG      sync.WaitGroup
	obsrecv         *obsreport.Receiver
}

var _ receiver.Metrics = (*sfxReceiver)(nil)

// New creates the SignalFx receiver with the given configuration.
func newReceiver(
	settings receiver.CreateSettings,
	config Config,
) (*sfxReceiver, error) {
	transport := "http"
	if config.TLSSetting != nil {
		transport = "https"
	}
	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             settings.ID,
		Transport:              transport,
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}
	r := &sfxReceiver{
		settings: settings,
		config:   &config,
		obsrecv:  obsrecv,
	}

	return r, nil
}

func (r *sfxReceiver) RegisterMetricsConsumer(mc consumer.Metrics) {
	r.metricsConsumer = mc
}

func (r *sfxReceiver) RegisterLogsConsumer(lc consumer.Logs) {
	r.logsConsumer = lc
}

// Start tells the receiver to start its processing.
// By convention the consumer of the received data is set when the receiver
// instance is created.
func (r *sfxReceiver) Start(_ context.Context, host component.Host) error {
	if r.metricsConsumer == nil && r.logsConsumer == nil {
		return component.ErrNilNextConsumer
	}

	if r.server != nil {
		return nil
	}

	// set up the listener
	ln, err := r.config.HTTPServerSettings.ToListener()
	if err != nil {
		return fmt.Errorf("failed to bind to address %s: %w", r.config.Endpoint, err)
	}

	mx := mux.NewRouter()
	mx.HandleFunc("/v2/datapoint", r.handleDatapointReq)
	mx.HandleFunc("/v2/event", r.handleEventReq)

	r.server, err = r.config.HTTPServerSettings.ToServer(host, r.settings.TelemetrySettings, mx)
	if err != nil {
		return err
	}

	// TODO: Evaluate what properties should be configurable, for now
	//		set some hard-coded values.
	r.server.ReadHeaderTimeout = defaultServerTimeout
	r.server.WriteTimeout = defaultServerTimeout

	r.shutdownWG.Add(1)
	go func() {
		defer r.shutdownWG.Done()
		if errHTTP := r.server.Serve(ln); !errors.Is(errHTTP, http.ErrServerClosed) && errHTTP != nil {
			host.ReportFatalError(errHTTP)
		}
	}()
	return nil
}

// Shutdown tells the receiver that should stop reception,
// giving it a chance to perform any necessary clean-up.
func (r *sfxReceiver) Shutdown(context.Context) error {
	if r.server == nil {
		return nil
	}
	err := r.server.Close()
	r.shutdownWG.Wait()
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

	body, err := io.ReadAll(bodyReader)
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
	_, err = resp.Write(okRespBody)
	if err != nil {
		r.failRequest(ctx, resp, http.StatusInternalServerError, errNextConsumerRespBody, err)
	}
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
		r.obsrecv.EndMetricsOp(ctx, metadata.Type, 0, nil)
		_, _ = resp.Write(okRespBody)
		return
	}

	md, err := translator.ToMetrics(msg.Datapoints)
	if err != nil {
		r.settings.Logger.Debug("SignalFx conversion error", zap.Error(err))
	}

	if r.config.AccessTokenPassthrough {
		if accessToken := req.Header.Get(splunk.SFxAccessTokenHeader); accessToken != "" {
			for i := 0; i < md.ResourceMetrics().Len(); i++ {
				rm := md.ResourceMetrics().At(i)
				res := rm.Resource()
				res.Attributes().PutStr(splunk.SFxAccessTokenLabel, accessToken)
			}
		}
	}

	err = r.metricsConsumer.ConsumeMetrics(ctx, md)
	r.obsrecv.EndMetricsOp(ctx, metadata.Type, len(msg.Datapoints), err)

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
		r.obsrecv.EndMetricsOp(ctx, metadata.Type, 0, nil)
		_, _ = resp.Write(okRespBody)
		return
	}

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	signalFxV2EventsToLogRecords(msg.Events, sl.LogRecords())

	if r.config.AccessTokenPassthrough {
		if accessToken := req.Header.Get(splunk.SFxAccessTokenHeader); accessToken != "" {
			rl.Resource().Attributes().PutStr(splunk.SFxAccessTokenLabel, accessToken)
		}
	}

	err := r.logsConsumer.ConsumeLogs(ctx, ld)
	r.obsrecv.EndMetricsOp(
		ctx,
		metadata.Type,
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
			r.settings.Logger.Warn(
				"Error writing HTTP response message",
				zap.Error(writeErr),
				zap.String("receiver", r.settings.ID.String()))
		}
	}

	// Use the same pattern as strings.Builder String().
	msg := *(*string)(unsafe.Pointer(&jsonResponse))

	reqSpan := trace.FromContext(ctx)
	reqSpan.AddAttributes(
		trace.Int64Attribute(conventions.AttributeHTTPStatusCode, int64(httpStatusCode)),
		trace.StringAttribute("http.status_text", msg))
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

	r.settings.Logger.Debug(
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
