// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver"

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/ackextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver/internal/metadata"
)

const (
	defaultServerTimeout = 20 * time.Second

	ackResponse                       = `{"acks": %s}`
	responseOK                        = `{"text": "Success", "code": 0}`
	responseOKWithAckID               = `{"text": "Success", "code": 0, "ackId": %d}`
	responseHecHealthy                = `{"text": "HEC is healthy", "code": 17}`
	responseInvalidMethodPostOnly     = `"Only \"POST\" method is supported"`
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
	responseErrDataChannelMissing     = `{"text": "Data channel is missing","code":10}`
	responseErrInvalidDataChannel     = `{"text": "Invalid data channel", "code": 11}`
	responseNoData                    = `{"text":"No data","code":5}`
	// Centralizing some HTTP and related string constants.
	gzipEncoding              = "gzip"
	httpContentEncodingHeader = "Content-Encoding"
	httpContentTypeHeader     = "Content-Type"
	httpJSONTypeHeader        = "application/json"
)

var (
	errEmptyEndpoint    = errors.New("empty endpoint")
	errInvalidMethod    = errors.New("invalid http method")
	errInvalidEncoding  = errors.New("invalid encoding")
	errExtensionMissing = errors.New("ack extension not found")

	okRespBody                    = []byte(responseOK)
	eventRequiredRespBody         = []byte(responseErrEventRequired)
	eventBlankRespBody            = []byte(responseErrEventBlank)
	requiredDataChannelHeader     = []byte(responseErrDataChannelMissing)
	invalidEncodingRespBody       = []byte(responseInvalidEncoding)
	invalidFormatRespBody         = []byte(responseInvalidDataFormat)
	invalidMethodRespBodyPostOnly = []byte(responseInvalidMethodPostOnly)
	errGzipReaderRespBody         = []byte(responseErrGzipReader)
	errUnmarshalBodyRespBody      = []byte(responseErrUnmarshalBody)
	errInternalServerError        = []byte(responseErrInternalServerError)
	errUnsupportedMetricEvent     = []byte(responseErrUnsupportedMetricEvent)
	errUnsupportedLogEvent        = []byte(responseErrUnsupportedLogEvent)
	noDataRespBody                = []byte(responseNoData)
)

// splunkReceiver implements the receiver.Metrics for Splunk HEC metric protocol.
type splunkReceiver struct {
	settings        receiver.Settings
	config          *Config
	logsConsumer    consumer.Logs
	metricsConsumer consumer.Metrics
	server          *http.Server
	shutdownWG      sync.WaitGroup
	obsrecv         *receiverhelper.ObsReport
	gzipReaderPool  *sync.Pool
	ackExt          ackextension.AckExtension
}

var (
	_ receiver.Metrics = (*splunkReceiver)(nil)
	_ receiver.Logs    = (*splunkReceiver)(nil)
)

// newReceiver creates the Splunk HEC receiver with the given configuration.
func newReceiver(settings receiver.Settings, config Config) (*splunkReceiver, error) {
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
		settings: settings,
		config:   &config,
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

// Start tells the receiver to start its processing.
// By convention the consumer of the received data is set when the receiver
// instance is created.
func (r *splunkReceiver) Start(ctx context.Context, host component.Host) error {
	// server.Handler will be nil on initial call, otherwise noop.
	if r.server != nil && r.server.Handler != nil {
		return nil
	}

	mx := mux.NewRouter()
	// set up the ack API handler if the ack extension is present
	if r.config.Ack.Extension != nil {
		ext, found := host.GetExtensions()[*r.config.Ack.Extension]
		if !found {
			return fmt.Errorf("specified ack extension with id %q could not be found", *r.config.Ack.Extension)
		}
		r.ackExt = ext.(ackextension.AckExtension)
		mx.NewRoute().Path(r.config.Ack.Path).HandlerFunc(r.handleAck)
	}

	mx.NewRoute().Path(r.config.HealthPath).HandlerFunc(r.handleHealthReq)
	mx.NewRoute().Path(r.config.HealthPath + "/1.0").HandlerFunc(r.handleHealthReq).Methods(http.MethodGet)
	if r.logsConsumer != nil {
		mx.NewRoute().Path(r.config.RawPath).HandlerFunc(r.handleRawReq)
	}
	mx.NewRoute().HandlerFunc(r.handleReq)
	// set up the listener
	ln, err := r.config.ServerConfig.ToListener(ctx)
	if err != nil {
		return fmt.Errorf("failed to bind to address %s: %w", r.config.Endpoint, err)
	}

	r.server, err = r.config.ServerConfig.ToServer(ctx, host, r.settings.TelemetrySettings, mx)
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
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(errHTTP))
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

func (r *splunkReceiver) processSuccessResponseWithAck(resp http.ResponseWriter, channelID string) error {
	if r.ackExt == nil {
		panic("writing response with ack when ack extension is not configured")
	}

	ackID := r.ackExt.ProcessEvent(channelID)
	r.ackExt.Ack(channelID, ackID)
	return r.processSuccessResponse(resp, []byte(fmt.Sprintf(responseOKWithAckID, ackID)))
}

func (r *splunkReceiver) processSuccessResponse(resp http.ResponseWriter, bodyContent []byte) error {
	resp.Header().Set(httpContentTypeHeader, httpJSONTypeHeader)
	resp.WriteHeader(http.StatusOK)
	_, err := resp.Write(bodyContent)
	return err
}

func (r *splunkReceiver) handleAck(resp http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		r.failRequest(resp, http.StatusBadRequest, invalidMethodRespBodyPostOnly, errInvalidMethod)
		return
	}

	// shouldn't run into this case since we only enable this handler IF ackExt exists. But we have this check just in case
	if r.ackExt == nil {
		r.failRequest(resp, http.StatusInternalServerError, errInternalServerError, errExtensionMissing)
		return
	}

	var channelID string
	var extracted bool
	channelID, extracted = r.extractChannel(req)
	if !extracted {
		r.failRequest(resp, http.StatusBadRequest, requiredDataChannelHeader, nil)
		return
	}
	if channelErr := r.validateChannelHeader(channelID); channelErr != nil {
		r.failRequest(resp, http.StatusBadRequest, []byte(channelErr.Error()), channelErr)
		return
	}

	dec := json.NewDecoder(req.Body)
	var ackRequest splunk.AckRequest

	err := dec.Decode(&ackRequest)
	if err != nil {
		r.failRequest(resp, http.StatusBadRequest, invalidFormatRespBody, err)
		return
	}

	if len(ackRequest.Acks) == 0 {
		r.failRequest(resp, http.StatusBadRequest, invalidFormatRespBody, errors.New("request body must include at least one ackID to be queried"))
		return
	}

	queriedAcks := r.ackExt.QueryAcks(channelID, ackRequest.Acks)
	ackString, _ := json.Marshal(queriedAcks)
	if err := r.processSuccessResponse(resp, []byte(fmt.Sprintf(ackResponse, ackString))); err != nil {
		r.failRequest(resp, http.StatusInternalServerError, errInternalServerError, err)
	}
}

func (r *splunkReceiver) handleRawReq(resp http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	ctx = r.obsrecv.StartLogsOp(ctx)

	if req.Method != http.MethodPost {
		r.failRequest(resp, http.StatusBadRequest, invalidMethodRespBodyPostOnly, errInvalidMethod)
		return
	}

	encoding := req.Header.Get(httpContentEncodingHeader)
	if encoding != "" && encoding != gzipEncoding {
		r.failRequest(resp, http.StatusUnsupportedMediaType, invalidEncodingRespBody, errInvalidEncoding)
		return
	}

	var channelID string
	var extracted bool
	if channelID, extracted = r.extractChannel(req); extracted {
		if channelErr := r.validateChannelHeader(channelID); channelErr != nil {
			r.failRequest(resp, http.StatusBadRequest, []byte(channelErr.Error()), channelErr)
			return
		}
	}

	if req.ContentLength == 0 {
		r.obsrecv.EndLogsOp(ctx, metadata.Type.String(), 0, nil)
		r.failRequest(resp, http.StatusBadRequest, noDataRespBody, nil)
		return
	}

	bodyReader := req.Body
	if encoding == gzipEncoding {
		reader := r.gzipReaderPool.Get().(*gzip.Reader)
		err := reader.Reset(bodyReader)
		if err != nil {
			r.failRequest(resp, http.StatusBadRequest, errGzipReaderRespBody, err)
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
			r.failRequest(resp, http.StatusBadRequest, invalidFormatRespBody, err)
			return
		}
		timestamp = pcommon.NewTimestampFromTime(time.Unix(t, 0))
	}

	ld, slLen, err := splunkHecRawToLogData(bodyReader, query, resourceCustomizer, r.config, timestamp)
	if err != nil {
		r.failRequest(resp, http.StatusInternalServerError, errInternalServerError, err)
		return
	}
	consumerErr := r.logsConsumer.ConsumeLogs(ctx, ld)

	_ = bodyReader.Close()

	if consumerErr != nil {
		r.failRequest(resp, http.StatusInternalServerError, errInternalServerError, consumerErr)
	} else {
		var ackErr error
		if len(channelID) > 0 && r.ackExt != nil {
			ackErr = r.processSuccessResponseWithAck(resp, channelID)
		} else {
			ackErr = r.processSuccessResponse(resp, okRespBody)
		}
		if ackErr != nil {
			r.failRequest(resp, http.StatusInternalServerError, errInternalServerError, err)
		} else {
			r.obsrecv.EndLogsOp(ctx, metadata.Type.String(), slLen, nil)
		}
	}
}

func (r *splunkReceiver) extractChannel(req *http.Request) (string, bool) {
	// check header
	for k, v := range req.Header {
		if strings.EqualFold(k, splunk.HTTPSplunkChannelHeader) {
			return strings.ToUpper(v[0]), true
		}
	}
	// check query param
	for k, v := range req.URL.Query() {
		if strings.EqualFold(k, "channel") {
			return strings.ToUpper(v[0]), true
		}
	}

	return "", false
}

func (r *splunkReceiver) validateChannelHeader(channelID string) error {
	if len(channelID) == 0 {
		return errors.New(responseErrDataChannelMissing)
	}

	// channel id must be a valid uuid
	// https://docs.splunk.com/Documentation/Splunk/9.2.1/Data/AboutHECIDXAck#:~:text=close%20the%20file.-,About%20channels%20and%20sending%20data,-Sending%20events%20to
	_, err := uuid.Parse(channelID)
	if err != nil {
		return errors.New(responseErrInvalidDataChannel)
	}

	return nil
}

func (r *splunkReceiver) handleReq(resp http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	if req.Method != http.MethodPost {
		r.failRequest(resp, http.StatusBadRequest, invalidMethodRespBodyPostOnly, errInvalidMethod)
		return
	}

	encoding := req.Header.Get(httpContentEncodingHeader)
	if encoding != "" && encoding != gzipEncoding {
		r.failRequest(resp, http.StatusUnsupportedMediaType, invalidEncodingRespBody, errInvalidEncoding)
		return
	}

	channelID, extracted := r.extractChannel(req)
	if extracted {
		if channelErr := r.validateChannelHeader(channelID); channelErr != nil {
			r.failRequest(resp, http.StatusBadRequest, []byte(channelErr.Error()), channelErr)
			return
		}
	}

	bodyReader := req.Body
	if encoding == gzipEncoding {
		reader := r.gzipReaderPool.Get().(*gzip.Reader)
		err := reader.Reset(bodyReader)
		if err != nil {
			r.failRequest(resp, http.StatusBadRequest, errGzipReaderRespBody, err)
			return
		}
		bodyReader = reader
		defer r.gzipReaderPool.Put(reader)
	}

	if req.ContentLength == 0 {
		r.failRequest(resp, http.StatusBadRequest, noDataRespBody, nil)
		return
	}

	dec := jsoniter.NewDecoder(bodyReader)

	var events []*splunk.Event
	var metricEvents []*splunk.Event

	for dec.More() {
		var msg splunk.Event
		err := dec.Decode(&msg)
		if err != nil {
			r.failRequest(resp, http.StatusBadRequest, invalidFormatRespBody, err)
			return
		}

		for _, v := range msg.Fields {
			if !isFlatJSONField(v) {
				r.failRequest(resp, http.StatusBadRequest, []byte(fmt.Sprintf(responseErrHandlingIndexedFields, len(events)+len(metricEvents))), nil)
				return
			}
		}

		if msg.IsMetric() {
			if r.metricsConsumer == nil {
				r.failRequest(resp, http.StatusBadRequest, errUnsupportedMetricEvent, err)
				return
			}
			metricEvents = append(metricEvents, &msg)
		} else {
			if msg.Event == nil {
				r.failRequest(resp, http.StatusBadRequest, eventRequiredRespBody, nil)
				return
			}

			if msg.Event == "" {
				r.failRequest(resp, http.StatusBadRequest, eventBlankRespBody, nil)
				return
			}

			if r.logsConsumer == nil {
				r.failRequest(resp, http.StatusBadRequest, errUnsupportedLogEvent, err)
				return
			}
			events = append(events, &msg)
		}
	}
	resourceCustomizer := r.createResourceCustomizer(req)
	if r.logsConsumer != nil && len(events) > 0 {
		ld, err := splunkHecToLogData(r.settings.Logger, events, resourceCustomizer, r.config)
		if err != nil {
			r.failRequest(resp, http.StatusBadRequest, errUnmarshalBodyRespBody, err)
			return
		}
		ctx = r.obsrecv.StartLogsOp(ctx)
		decodeErr := r.logsConsumer.ConsumeLogs(ctx, ld)
		r.obsrecv.EndLogsOp(ctx, metadata.Type.String(), len(events), nil)
		if decodeErr != nil {
			r.failRequest(resp, http.StatusInternalServerError, errInternalServerError, decodeErr)
			return
		}
	}
	if r.metricsConsumer != nil && len(metricEvents) > 0 {
		md, _ := splunkHecToMetricsData(r.settings.Logger, metricEvents, resourceCustomizer, r.config)
		ctx = r.obsrecv.StartMetricsOp(ctx)
		decodeErr := r.metricsConsumer.ConsumeMetrics(ctx, md)
		r.obsrecv.EndMetricsOp(ctx, metadata.Type.String(), len(metricEvents), nil)
		if decodeErr != nil {
			r.failRequest(resp, http.StatusInternalServerError, errInternalServerError, decodeErr)
			return
		}
	}

	var ackErr error
	if len(channelID) > 0 && r.ackExt != nil {
		ackErr = r.processSuccessResponseWithAck(resp, channelID)
	} else {
		ackErr = r.processSuccessResponse(resp, okRespBody)
	}
	if ackErr != nil {
		r.failRequest(resp, http.StatusInternalServerError, errInternalServerError, ackErr)
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
	resp http.ResponseWriter,
	httpStatusCode int,
	jsonResponse []byte,
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
