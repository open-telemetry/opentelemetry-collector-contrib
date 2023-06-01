// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver"

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver/internal/metadata"
)

const (
	defaultServerTimeout = 20 * time.Second

	responseOK                        = "OK"
	responseHecHealthy                = `{"text": "HEC is healthy", "code": 17}`
	responseInvalidMethod             = `Only "POST" method is supported`
	responseInvalidEncoding           = `"Content-Encoding" must be "gzip" or empty`
	responseInvalidDataFormat         = `{"text":"Invalid data format","code":6}`
	responseErrEventRequired          = `{"text":"Event field is required","code":12}`
	responseErrEventBlank             = `{"text":"Event field cannot be blank","code":13}`
	responseErrGzipReader             = "Error on gzip body"
	responseErrUnmarshalBody          = "Failed to unmarshal message body"
	responseErrInternalServerError    = "Internal Server Error"
	responseErrUnsupportedMetricEvent = "Unsupported metric event"
	responseErrUnsupportedLogEvent    = "Unsupported log event"
	responseErrHandlingIndexedFields  = `{"text":"Error in handling indexed fields","code":15,"invalid-event-number":%d}`
	responseNoData                    = `{"text":"No data","code":5}`
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
	eventRequiredRespBody     = initJSONResponse(responseErrEventRequired)
	eventBlankRespBody        = initJSONResponse(responseErrEventBlank)
	invalidEncodingRespBody   = initJSONResponse(responseInvalidEncoding)
	invalidFormatRespBody     = initJSONResponse(responseInvalidDataFormat)
	invalidMethodRespBody     = initJSONResponse(responseInvalidMethod)
	errGzipReaderRespBody     = initJSONResponse(responseErrGzipReader)
	errUnmarshalBodyRespBody  = initJSONResponse(responseErrUnmarshalBody)
	errInternalServerError    = initJSONResponse(responseErrInternalServerError)
	errUnsupportedMetricEvent = initJSONResponse(responseErrUnsupportedMetricEvent)
	errUnsupportedLogEvent    = initJSONResponse(responseErrUnsupportedLogEvent)
	noDataRespBody            = initJSONResponse(responseNoData)
)

// splunkReceiver implements the receiver.Metrics for Splunk HEC metric protocol.
type splunkReceiver struct {
	settings        receiver.CreateSettings
	config          *Config
	logsConsumer    consumer.Logs
	metricsConsumer consumer.Metrics
	server          *http.Server
	shutdownWG      sync.WaitGroup
	obsrecv         *obsreport.Receiver
	gzipReaderPool  *sync.Pool
}

var _ receiver.Metrics = (*splunkReceiver)(nil)

// newMetricsReceiver creates the Splunk HEC receiver with the given configuration.
func newMetricsReceiver(
	settings receiver.CreateSettings,
	config Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
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

	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             settings.ID,
		Transport:              transport,
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
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
		obsrecv:        obsrecv,
		gzipReaderPool: &sync.Pool{New: func() interface{} { return new(gzip.Reader) }},
	}

	return r, nil
}

// newLogsReceiver creates the Splunk HEC receiver with the given configuration.
func newLogsReceiver(
	settings receiver.CreateSettings,
	config Config,
	nextConsumer consumer.Logs,
) (receiver.Logs, error) {
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

	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             settings.ID,
		Transport:              transport,
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
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
		obsrecv:        obsrecv,
	}

	return r, nil
}

// Start tells the receiver to start its processing.
// By convention the consumer of the received data is set when the receiver
// instance is created.
func (r *splunkReceiver) Start(_ context.Context, host component.Host) error {
	// server.Handler will be nil on initial call, otherwise noop.
	if r.server != nil && r.server.Handler != nil {
		return nil
	}

	var ln net.Listener
	// set up the listener
	ln, err := r.config.HTTPServerSettings.ToListener()
	if err != nil {
		return fmt.Errorf("failed to bind to address %s: %w", r.config.Endpoint, err)
	}

	mx := mux.NewRouter()
	mx.NewRoute().Path(r.config.HealthPath).HandlerFunc(r.handleHealthReq)
	mx.NewRoute().Path(r.config.HealthPath + "/1.0").HandlerFunc(r.handleHealthReq).Methods("GET")
	if r.logsConsumer != nil {
		mx.NewRoute().Path(r.config.RawPath).HandlerFunc(r.handleRawReq)
	}
	mx.NewRoute().HandlerFunc(r.handleReq)

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

	return err
}

// Shutdown tells the receiver that should stop reception,
// giving it a chance to perform any necessary clean-up.
func (r *splunkReceiver) Shutdown(context.Context) error {
	err := r.server.Close()
	r.shutdownWG.Wait()
	return err
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
		r.obsrecv.EndLogsOp(ctx, metadata.Type, 0, nil)
		r.failRequest(ctx, resp, http.StatusBadRequest, noDataRespBody, 0, nil)
		return
	}

	bodyReader := req.Body
	if encoding == gzipEncoding {
		reader := r.gzipReaderPool.Get().(*gzip.Reader)
		err := reader.Reset(bodyReader)

		if err != nil {
			r.failRequest(ctx, resp, http.StatusBadRequest, errGzipReaderRespBody, 0, err)
			_, _ = io.ReadAll(req.Body)
			_ = req.Body.Close()
			return
		}
		bodyReader = reader
		defer r.gzipReaderPool.Put(reader)
	}

	resourceCustomizer := r.createResourceCustomizer(req)
	ld, slLen, err := splunkHecRawToLogData(bodyReader, req.URL.Query(), resourceCustomizer, r.config)
	if err != nil {
		r.failRequest(ctx, resp, http.StatusInternalServerError, errInternalServerError, slLen, err)
		return
	}
	consumerErr := r.logsConsumer.ConsumeLogs(ctx, ld)

	_ = bodyReader.Close()

	if consumerErr != nil {
		r.failRequest(ctx, resp, http.StatusInternalServerError, errInternalServerError, slLen, consumerErr)
	} else {
		resp.WriteHeader(http.StatusOK)
		r.obsrecv.EndLogsOp(ctx, metadata.Type, slLen, nil)
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
		r.failRequest(ctx, resp, http.StatusBadRequest, noDataRespBody, 0, nil)
		return
	}

	dec := jsoniter.NewDecoder(bodyReader)

	var events []*splunk.Event

	for dec.More() {
		var msg splunk.Event
		err := dec.Decode(&msg)
		if err != nil {
			r.failRequest(ctx, resp, http.StatusBadRequest, invalidFormatRespBody, len(events), err)
			return
		}

		if msg.Event == nil {
			r.failRequest(ctx, resp, http.StatusBadRequest, eventRequiredRespBody, len(events), nil)
			return
		}

		if msg.Event == "" {
			r.failRequest(ctx, resp, http.StatusBadRequest, eventBlankRespBody, len(events), nil)
			return
		}

		for _, v := range msg.Fields {
			if !isFlatJSONField(v) {
				r.failRequest(ctx, resp, http.StatusBadRequest, []byte(fmt.Sprintf(responseErrHandlingIndexedFields, len(events))), len(events), nil)
				return
			}
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
	r.obsrecv.EndMetricsOp(ctx, metadata.Type, len(events), decodeErr)

	if decodeErr != nil {
		r.failRequest(ctx, resp, http.StatusInternalServerError, errInternalServerError, len(events), decodeErr)
	} else {
		resp.WriteHeader(http.StatusOK)
		_, err := resp.Write(okRespBody)
		if err != nil {
			r.failRequest(ctx, resp, http.StatusInternalServerError, errInternalServerError, len(events), err)
		}
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
	r.obsrecv.EndLogsOp(ctx, metadata.Type, len(events), decodeErr)
	if decodeErr != nil {
		r.failRequest(ctx, resp, http.StatusInternalServerError, errInternalServerError, len(events), decodeErr)
	} else {
		resp.WriteHeader(http.StatusOK)
		if _, err := resp.Write(okRespBody); err != nil {
			r.failRequest(ctx, resp, http.StatusInternalServerError, errInternalServerError, len(events), err)
		}
	}
}

func (r *splunkReceiver) createResourceCustomizer(req *http.Request) func(resource pcommon.Resource) {
	if r.config.AccessTokenPassthrough {
		accessToken := req.Header.Get("Authorization")
		if strings.HasPrefix(accessToken, splunk.HECTokenHeader+" ") {
			accessTokenValue := accessToken[len(splunk.HECTokenHeader)+1:]
			return func(resource pcommon.Resource) {
				resource.Attributes().PutStr(splunk.HecTokenLabel, accessTokenValue)
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
		r.obsrecv.EndLogsOp(ctx, metadata.Type, numRecordsReceived, err)
	} else {
		r.obsrecv.EndMetricsOp(ctx, metadata.Type, numRecordsReceived, err)
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

func (r *splunkReceiver) handleHealthReq(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Add("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write([]byte(responseHecHealthy))
}

func initJSONResponse(s string) []byte {
	respBody, err := jsoniter.Marshal(s)
	if err != nil {
		// This is to be used in initialization so panic here is fine.
		panic(err)
	}
	return respBody
}

func isFlatJSONField(field interface{}) bool {
	switch value := field.(type) {
	case map[string]interface{}:
		return false
	case []interface{}:
		for _, v := range value {
			switch v.(type) {
			case map[string]interface{}, []interface{}:
				return false
			}
		}
	}
	return true
}
