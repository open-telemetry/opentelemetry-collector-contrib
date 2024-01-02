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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver/internal/metadata"
)

const (
	defaultServerTimeout = 20 * time.Second

	responseOK                        = `{"text": "Success", "code": 0}`
	responseHecHealthy                = `{"text": "HEC is healthy", "code": 17}`
	responseInvalidMethod             = `"Only \"POST\" method is supported"`
	responseInvalidEncoding           = `"\"Content-Encoding\" must be \"gzip\" or empty"`
	responseInvalidDataFormat         = `{"text":"Invalid data format","code":6}`
	responseErrEventRequired          = `{"text":"Event field is required","code":12}`
	responseErrEventBlank             = `{"text":"Event field cannot be blank","code":13}`
	responseErrGzipReader             = `"Error on gzip body"`
	responseErrUnmarshalBody          = `"Failed to unmarshal message body"`
	responseErrInternalServerError    = `"Internal Server Error"`
	responseErrUnsupportedMetricEvent = `"Unsupported metric event"`
	responseErrUnsupportedLogEvent    = `"Unsupported log event"`
	responseErrHandlingIndexedFields  = `{"text":"Error in handling indexed fields","code":15,"invalid-event-number":%d}`
	responseNoData                    = `{"text":"No data","code":5}`
	// Centralizing some HTTP and related string constants.
	gzipEncoding              = "gzip"
	httpContentEncodingHeader = "Content-Encoding"
	httpContentTypeHeader     = "Content-Type"
	httpJSONTypeHeader        = "application/json"
)

var (
	errNilNextMetricsConsumer = errors.New("nil metricsConsumer")
	errNilNextLogsConsumer    = errors.New("nil logsConsumer")
	errEmptyEndpoint          = errors.New("empty endpoint")
	errInvalidMethod          = errors.New("invalid http method")
	errInvalidEncoding        = errors.New("invalid encoding")

	okRespBody                = []byte(responseOK)
	eventRequiredRespBody     = []byte(responseErrEventRequired)
	eventBlankRespBody        = []byte(responseErrEventBlank)
	invalidEncodingRespBody   = []byte(responseInvalidEncoding)
	invalidFormatRespBody     = []byte(responseInvalidDataFormat)
	invalidMethodRespBody     = []byte(responseInvalidMethod)
	errGzipReaderRespBody     = []byte(responseErrGzipReader)
	errUnmarshalBodyRespBody  = []byte(responseErrUnmarshalBody)
	errInternalServerError    = []byte(responseErrInternalServerError)
	errUnsupportedMetricEvent = []byte(responseErrUnsupportedMetricEvent)
	errUnsupportedLogEvent    = []byte(responseErrUnsupportedLogEvent)
	noDataRespBody            = []byte(responseNoData)
)

// splunkReceiver implements the receiver.Metrics for Splunk HEC metric protocol.
type splunkReceiver struct {
	settings        receiver.CreateSettings
	config          *Config
	logsConsumer    consumer.Logs
	metricsConsumer consumer.Metrics
	server          *http.Server
	shutdownWG      sync.WaitGroup
	obsrecv         *receiverhelper.ObsReport
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

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
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
		gzipReaderPool: &sync.Pool{New: func() any { return new(gzip.Reader) }},
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

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
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
		gzipReaderPool: &sync.Pool{New: func() any { return new(gzip.Reader) }},
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

func (r *splunkReceiver) writeSuccessResponse(ctx context.Context, resp http.ResponseWriter, eventCount int) {
	resp.Header().Set(httpContentTypeHeader, httpJSONTypeHeader)
	resp.WriteHeader(http.StatusOK)
	if _, err := resp.Write(okRespBody); err != nil {
		r.failRequest(ctx, resp, http.StatusInternalServerError, errInternalServerError, eventCount, err)
	}
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
	query := req.URL.Query()
	var timestamp pcommon.Timestamp
	if query.Has(queryTime) {
		t, err := strconv.ParseInt(query.Get(queryTime), 10, 64)
		if t < 0 {
			err = errors.New("time cannot be less than 0")
		}
		if err != nil {
			r.failRequest(ctx, resp, http.StatusBadRequest, invalidFormatRespBody, 0, err)
			return
		}
		timestamp = pcommon.NewTimestampFromTime(time.Unix(t, 0))
	}

	ld, slLen, err := splunkHecRawToLogData(bodyReader, query, resourceCustomizer, r.config, timestamp)
	if err != nil {
		r.failRequest(ctx, resp, http.StatusInternalServerError, errInternalServerError, slLen, err)
		return
	}
	consumerErr := r.logsConsumer.ConsumeLogs(ctx, ld)

	_ = bodyReader.Close()

	if consumerErr != nil {
		r.failRequest(ctx, resp, http.StatusInternalServerError, errInternalServerError, slLen, consumerErr)
	} else {
		r.writeSuccessResponse(ctx, resp, ld.LogRecordCount())
		r.obsrecv.EndLogsOp(ctx, metadata.Type, slLen, nil)
	}
}

func (r *splunkReceiver) handleReq(resp http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	if r.logsConsumer != nil {
		ctx = r.obsrecv.StartLogsOp(ctx)
	}
	if r.metricsConsumer != nil {
		ctx = r.obsrecv.StartMetricsOp(ctx)
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
	var metricEvents []*splunk.Event

	for dec.More() {
		var msg splunk.Event
		err := dec.Decode(&msg)
		if err != nil {
			r.failRequest(ctx, resp, http.StatusBadRequest, invalidFormatRespBody, len(events)+len(metricEvents), err)
			return
		}

		if msg.Event == nil {
			r.failRequest(ctx, resp, http.StatusBadRequest, eventRequiredRespBody, len(events)+len(metricEvents), nil)
			return
		}

		if msg.Event == "" {
			r.failRequest(ctx, resp, http.StatusBadRequest, eventBlankRespBody, len(events)+len(metricEvents), nil)
			return
		}

		for _, v := range msg.Fields {
			if !isFlatJSONField(v) {
				r.failRequest(ctx, resp, http.StatusBadRequest, []byte(fmt.Sprintf(responseErrHandlingIndexedFields, len(events)+len(metricEvents))), len(events)+len(metricEvents), nil)
				return
			}
		}
		if msg.IsMetric() {
			if r.metricsConsumer == nil {
				r.failRequest(ctx, resp, http.StatusBadRequest, errUnsupportedMetricEvent, len(metricEvents), err)
				return
			}
			metricEvents = append(metricEvents, &msg)
		} else {
			if r.logsConsumer == nil {
				r.failRequest(ctx, resp, http.StatusBadRequest, errUnsupportedLogEvent, len(events), err)
				return
			}
			events = append(events, &msg)
		}

	}
	resourceCustomizer := r.createResourceCustomizer(req)
	if r.logsConsumer != nil && len(events) > 0 {
		ld, err := splunkHecToLogData(r.settings.Logger, events, resourceCustomizer, r.config)
		if err != nil {
			r.failRequest(ctx, resp, http.StatusBadRequest, errUnmarshalBodyRespBody, len(events), err)
			return
		}
		decodeErr := r.logsConsumer.ConsumeLogs(ctx, ld)
		r.obsrecv.EndLogsOp(ctx, metadata.Type, len(events), decodeErr)
		if decodeErr != nil {
			r.failRequest(ctx, resp, http.StatusInternalServerError, errInternalServerError, len(events), decodeErr)
			return
		}

	}
	if r.metricsConsumer != nil && len(metricEvents) > 0 {
		md, _ := splunkHecToMetricsData(r.settings.Logger, metricEvents, resourceCustomizer, r.config)

		decodeErr := r.metricsConsumer.ConsumeMetrics(ctx, md)
		r.obsrecv.EndMetricsOp(ctx, metadata.Type, len(metricEvents), decodeErr)
		if decodeErr != nil {
			r.failRequest(ctx, resp, http.StatusInternalServerError, errInternalServerError, len(metricEvents), decodeErr)
			return
		}
	}

	r.writeSuccessResponse(ctx, resp, len(events)+len(metricEvents))
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

func isFlatJSONField(field any) bool {
	switch value := field.(type) {
	case map[string]any:
		return false
	case []any:
		for _, v := range value {
			switch v.(type) {
			case map[string]any, []any:
				return false
			}
		}
	}
	return true
}
