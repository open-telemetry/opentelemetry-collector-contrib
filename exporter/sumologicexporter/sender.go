// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter/internal/metadata"
)

var (
	metricsMarshaler = pmetric.ProtoMarshaler{}
	logsMarshaler    = plog.ProtoMarshaler{}
	tracesMarshaler  = ptrace.ProtoMarshaler{}
)

// metricPair represents information required to send one metric to the Sumo Logic
type metricPair struct {
	attributes pcommon.Map
	metric     pmetric.Metric
}

// countingReader keeps number of records related to reader
type countingReader struct {
	counter int64
	reader  io.Reader
}

// newCountingReader creates countingReader with given number of records
func newCountingReader(records int) *countingReader {
	return &countingReader{
		counter: int64(records),
	}
}

// withBytes sets up reader to read from bytes data
func (c *countingReader) withBytes(data []byte) *countingReader {
	c.reader = bytes.NewReader(data)
	return c
}

// withString sets up reader to read from string data
func (c *countingReader) withString(data string) *countingReader {
	c.reader = strings.NewReader(data)
	return c
}

// bodyBuilder keeps information about number of records related to data it keeps
type bodyBuilder struct {
	builder strings.Builder
	counter int
}

// newBodyBuilder returns empty bodyBuilder
func newBodyBuilder() bodyBuilder {
	return bodyBuilder{}
}

// Reset resets both counter and builder content
func (b *bodyBuilder) Reset() {
	b.counter = 0
	b.builder.Reset()
}

// addLine adds multiple lines to builder and increments counter
func (b *bodyBuilder) addLines(lines []string) {
	if len(lines) == 0 {
		return
	}

	// add the first line separately to avoid a conditional in the loop
	b.builder.WriteString(lines[0])

	for _, line := range lines[1:] {
		b.builder.WriteByte('\n')
		b.builder.WriteString(line) // WriteString can't actually return an error
	}
	b.counter += len(lines)
}

// addNewLine adds newline to builder
func (b *bodyBuilder) addNewLine() {
	b.builder.WriteByte('\n') // WriteByte can't actually return an error
}

// Len returns builder content length
func (b *bodyBuilder) Len() int {
	return b.builder.Len()
}

// toCountingReader converts bodyBuilder to countingReader
func (b *bodyBuilder) toCountingReader() *countingReader {
	return newCountingReader(b.counter).withString(b.builder.String())
}

type sender struct {
	logger                     *zap.Logger
	config                     *Config
	client                     *http.Client
	prometheusFormatter        prometheusFormatter
	dataURLMetrics             string
	dataURLLogs                string
	dataURLTraces              string
	stickySessionCookieFunc    func() string
	setStickySessionCookieFunc func(string)
	id                         component.ID
	telemetryBuilder           *metadata.TelemetryBuilder
}

const (
	headerContentType string = "Content-Type"
	headerClient      string = "X-Sumo-Client"
	headerHost        string = "X-Sumo-Host"
	headerName        string = "X-Sumo-Name"
	headerCategory    string = "X-Sumo-Category"
	headerFields      string = "X-Sumo-Fields"

	attributeKeySourceHost     = "_sourceHost"
	attributeKeySourceName     = "_sourceName"
	attributeKeySourceCategory = "_sourceCategory"

	contentTypeLogs       string = "application/x-www-form-urlencoded"
	contentTypePrometheus string = "application/vnd.sumologic.prometheus"
	contentTypeOTLP       string = "application/x-protobuf"
	stickySessionKey      string = "AWSALB"
)

func newSender(
	logger *zap.Logger,
	cfg *Config,
	cl *http.Client,
	pf prometheusFormatter,
	metricsURL string,
	logsURL string,
	tracesURL string,
	stickySessionCookieFunc func() string,
	setStickySessionCookieFunc func(string),
	id component.ID,
	telemetryBuilder *metadata.TelemetryBuilder,
) *sender {
	return &sender{
		logger:                     logger,
		config:                     cfg,
		client:                     cl,
		prometheusFormatter:        pf,
		dataURLMetrics:             metricsURL,
		dataURLLogs:                logsURL,
		dataURLTraces:              tracesURL,
		stickySessionCookieFunc:    stickySessionCookieFunc,
		setStickySessionCookieFunc: setStickySessionCookieFunc,
		id:                         id,
		telemetryBuilder:           telemetryBuilder,
	}
}

var errUnauthorized = errors.New("unauthorized")

// send sends data to sumologic
func (s *sender) send(ctx context.Context, pipeline PipelineType, reader *countingReader, flds fields) error {
	req, err := s.createRequest(ctx, pipeline, reader.reader)
	if err != nil {
		return err
	}

	if err = s.addRequestHeaders(req, pipeline, flds); err != nil {
		return err
	}

	if s.config.StickySessionEnabled {
		s.addStickySessionCookie(req)
	}

	s.logger.Debug("Sending data",
		zap.String("pipeline", string(pipeline)),
		zap.Any("headers", req.Header),
	)

	start := time.Now()
	resp, err := s.client.Do(req)
	if err != nil {
		s.recordMetrics(time.Since(start), reader.counter, req, nil, pipeline)
		return err
	}
	defer resp.Body.Close()

	s.recordMetrics(time.Since(start), reader.counter, req, resp, pipeline)

	return s.handleReceiverResponse(resp)
}

func (s *sender) handleReceiverResponse(resp *http.Response) error {
	if s.config.StickySessionEnabled {
		s.updateStickySessionCookie(resp)
	}

	// API responds with a 200 or 204 with ContentLength set to 0 when all data
	// has been successfully ingested.
	if resp.ContentLength == 0 && (resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent) {
		return nil
	}

	type ReceiverResponseCore struct {
		Status  int    `json:"status,omitempty"`
		ID      string `json:"id,omitempty"`
		Code    string `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
	}

	// API responds with a 200 or 204 with a JSON body describing what issues
	// were encountered when processing the sent data.
	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		if resp.ContentLength < 0 {
			s.logger.Warn("Unknown length of server response")
			return nil
		}

		var rResponse ReceiverResponseCore
		var (
			b  = bytes.NewBuffer(make([]byte, 0, resp.ContentLength))
			tr = io.TeeReader(resp.Body, b)
		)

		if err := json.NewDecoder(tr).Decode(&rResponse); err != nil {
			s.logger.Warn("Error decoding receiver response", zap.ByteString("body", b.Bytes()))
			return nil
		}

		l := s.logger.With(zap.String("status", resp.Status))
		if len(rResponse.ID) > 0 {
			l = l.With(zap.String("id", rResponse.ID))
		}
		if len(rResponse.Code) > 0 {
			l = l.With(zap.String("code", rResponse.Code))
		}
		if len(rResponse.Message) > 0 {
			l = l.With(zap.String("message", rResponse.Message))
		}
		l.Warn("There was an issue sending data")
		return nil

	case http.StatusUnauthorized:
		return errUnauthorized

	default:
		type ReceiverErrorResponse struct {
			ReceiverResponseCore
			Errors []struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"errors,omitempty"`
		}

		var rResponse ReceiverErrorResponse
		if resp.ContentLength > 0 {
			var (
				b  = bytes.NewBuffer(make([]byte, 0, resp.ContentLength))
				tr = io.TeeReader(resp.Body, b)
			)

			if err := json.NewDecoder(tr).Decode(&rResponse); err != nil {
				return fmt.Errorf("failed to decode API response (status: %s): %s",
					resp.Status, b.String(),
				)
			}
		}

		errMsgs := []string{
			fmt.Sprintf("status: %s", resp.Status),
		}

		if len(rResponse.ID) > 0 {
			errMsgs = append(errMsgs, fmt.Sprintf("id: %s", rResponse.ID))
		}
		if len(rResponse.Code) > 0 {
			errMsgs = append(errMsgs, fmt.Sprintf("code: %s", rResponse.Code))
		}
		if len(rResponse.Message) > 0 {
			errMsgs = append(errMsgs, fmt.Sprintf("message: %s", rResponse.Message))
		}
		if len(rResponse.Errors) > 0 {
			errMsgs = append(errMsgs, fmt.Sprintf("errors: %+v", rResponse.Errors))
		}

		err := fmt.Errorf("failed sending data: %s", strings.Join(errMsgs, ", "))

		if resp.StatusCode == http.StatusBadRequest {
			// Report the failure as permanent if the server thinks the request is malformed.
			return consumererror.NewPermanent(err)
		}

		return err
	}
}

func (s *sender) createRequest(ctx context.Context, pipeline PipelineType, data io.Reader) (*http.Request, error) {
	var url string

	switch pipeline {
	case MetricsPipeline:
		url = s.dataURLMetrics
	case LogsPipeline:
		url = s.dataURLLogs
	case TracesPipeline:
		url = s.dataURLTraces
	default:
		return nil, fmt.Errorf("unknown pipeline type: %s", pipeline)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, data)
	if err != nil {
		return req, err
	}

	return req, err
}

// logToText converts LogRecord to a plain text line, returns it and error eventually
func (s *sender) logToText(record plog.LogRecord) string {
	return record.Body().AsString()
}

// logToJSON converts LogRecord to a json line, returns it and error eventually
func (s *sender) logToJSON(record plog.LogRecord) (string, error) {
	recordCopy := plog.NewLogRecord()
	record.CopyTo(recordCopy)

	// Only append the body when it's not empty to prevent sending 'null' log.
	if body := recordCopy.Body(); !isEmptyAttributeValue(body) {
		body.CopyTo(recordCopy.Attributes().PutEmpty(DefaultLogKey))
	}

	nextLine := new(bytes.Buffer)
	enc := json.NewEncoder(nextLine)
	enc.SetEscapeHTML(false)
	err := enc.Encode(recordCopy.Attributes().AsRaw())
	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(nextLine.String(), "\n"), nil
}

func isEmptyAttributeValue(att pcommon.Value) bool {
	switch att.Type() {
	case pcommon.ValueTypeEmpty:
		return true
	case pcommon.ValueTypeStr:
		return len(att.Str()) == 0
	case pcommon.ValueTypeSlice:
		return att.Slice().Len() == 0
	case pcommon.ValueTypeMap:
		return att.Map().Len() == 0
	case pcommon.ValueTypeBytes:
		return att.Bytes().Len() == 0
	}

	return false
}

// sendNonOTLPLogs sends log records from the logBuffer formatted according
// to configured LogFormat and as the result of execution
// returns array of records which has not been sent correctly and error
func (s *sender) sendNonOTLPLogs(ctx context.Context, rl plog.ResourceLogs, flds fields) ([]plog.LogRecord, error) {
	if s.config.LogFormat == OTLPLogFormat {
		return nil, errors.New("attempting to send OTLP logs as non-OTLP data")
	}

	var (
		body           = newBodyBuilder()
		errs           []error
		droppedRecords []plog.LogRecord
		currentRecords []plog.LogRecord
	)

	slgs := rl.ScopeLogs()
	for i := 0; i < slgs.Len(); i++ {
		slg := slgs.At(i)
		for j := 0; j < slg.LogRecords().Len(); j++ {
			lr := slg.LogRecords().At(j)
			formattedLine, err := s.formatLogLine(lr)
			if err != nil {
				droppedRecords = append(droppedRecords, lr)
				errs = append(errs, err)
				continue
			}

			sent, err := s.appendAndMaybeSend(ctx, []string{formattedLine}, LogsPipeline, &body, flds)
			if err != nil {
				errs = append(errs, err)
				droppedRecords = append(droppedRecords, currentRecords...)
			}

			// If data was sent and either failed or succeeded, cleanup the currentRecords slice
			if sent {
				currentRecords = currentRecords[:0]
			}

			currentRecords = append(currentRecords, lr)
		}
	}

	if body.Len() > 0 {
		if err := s.send(ctx, LogsPipeline, body.toCountingReader(), flds); err != nil {
			errs = append(errs, err)
			droppedRecords = append(droppedRecords, currentRecords...)
		}
	}

	return droppedRecords, errors.Join(errs...)
}

func (s *sender) formatLogLine(lr plog.LogRecord) (string, error) {
	var formattedLine string
	var err error

	switch s.config.LogFormat {
	case TextFormat:
		formattedLine = s.logToText(lr)
	case JSONFormat:
		formattedLine, err = s.logToJSON(lr)
	default:
		err = errors.New("unexpected log format")
	}

	return formattedLine, err
}

// TODO: add support for HTTP limits
func (s *sender) sendOTLPLogs(ctx context.Context, ld plog.Logs) error {
	body, err := logsMarshaler.MarshalLogs(ld)
	if err != nil {
		return err
	}

	return s.send(ctx, LogsPipeline, newCountingReader(ld.LogRecordCount()).withBytes(body), fields{})
}

// sendNonOTLPMetrics sends metrics in right format basing on the s.config.MetricFormat
func (s *sender) sendNonOTLPMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, []error) {
	if s.config.MetricFormat == OTLPMetricFormat {
		return md, []error{errors.New("attempting to send OTLP metrics as non-OTLP data")}
	}

	var (
		body             = newBodyBuilder()
		errs             []error
		currentResources []pmetric.ResourceMetrics
		flds             fields
	)

	rms := md.ResourceMetrics()
	droppedMetrics := pmetric.NewMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		flds = newFields(rm.Resource().Attributes())
		sms := rm.ScopeMetrics()

		// generally speaking, it's fine to send multiple ResourceMetrics in a single request
		// the only exception is if the computed source headers are different, as those as unique per-request
		// so we check if the headers are different here and send what we have if they are
		if i > 0 {
			currentSourceHeaders := getSourcesHeaders(flds)
			previousFields := newFields(rms.At(i - 1).Resource().Attributes())
			previousSourceHeaders := getSourcesHeaders(previousFields)
			if !reflect.DeepEqual(previousSourceHeaders, currentSourceHeaders) && body.Len() > 0 {
				if err := s.send(ctx, MetricsPipeline, body.toCountingReader(), previousFields); err != nil {
					errs = append(errs, err)
					for _, resource := range currentResources {
						resource.CopyTo(droppedMetrics.ResourceMetrics().AppendEmpty())
					}
				}
				body.Reset()
				currentResources = currentResources[:0]
			}
		}

		// transform the metrics into formatted lines ready to be sent
		var formattedLines []string
		var err error
		for i := 0; i < sms.Len(); i++ {
			sm := sms.At(i)

			for j := 0; j < sm.Metrics().Len(); j++ {
				m := sm.Metrics().At(j)

				var formattedLine string

				switch s.config.MetricFormat {
				case PrometheusFormat:
					formattedLine = s.prometheusFormatter.metric2String(m, rm.Resource().Attributes())
				default:
					return md, []error{fmt.Errorf("unexpected metric format: %s", s.config.MetricFormat)}
				}

				formattedLines = append(formattedLines, formattedLine)
			}
		}

		sent, err := s.appendAndMaybeSend(ctx, formattedLines, MetricsPipeline, &body, flds)
		if err != nil {
			errs = append(errs, err)
			if sent {
				// failed at sending, add the resource to the dropped metrics
				// move instead of copy here to avoid duplicating data in memory on failure
				for _, resource := range currentResources {
					resource.CopyTo(droppedMetrics.ResourceMetrics().AppendEmpty())
				}
			}
		}

		// If data was sent, cleanup the currentResources slice
		if sent {
			currentResources = currentResources[:0]
		}

		currentResources = append(currentResources, rm)
	}

	if body.Len() > 0 {
		if err := s.send(ctx, MetricsPipeline, body.toCountingReader(), flds); err != nil {
			errs = append(errs, err)
			for _, resource := range currentResources {
				resource.CopyTo(droppedMetrics.ResourceMetrics().AppendEmpty())
			}
		}
	}

	return droppedMetrics, errs
}

func (s *sender) sendOTLPMetrics(ctx context.Context, md pmetric.Metrics) error {
	rms := md.ResourceMetrics()
	if rms.Len() == 0 {
		s.logger.Debug("there are no metrics to send, moving on")
		return nil
	}
	if s.config.DecomposeOtlpHistograms {
		md = decomposeHistograms(md)
	}

	body, err := metricsMarshaler.MarshalMetrics(md)
	if err != nil {
		return err
	}

	return s.send(ctx, MetricsPipeline, newCountingReader(md.DataPointCount()).withBytes(body), fields{})
}

// appendAndMaybeSend appends line to the request body that will be sent and sends
// the accumulated data if the internal logBuffer has been filled (with config.MaxRequestBodySize bytes).
// It returns a boolean indicating if the data was sent and an error
func (s *sender) appendAndMaybeSend(
	ctx context.Context,
	lines []string,
	pipeline PipelineType,
	body *bodyBuilder,
	flds fields,
) (sent bool, err error) {
	linesTotalLength := 0
	for _, line := range lines {
		linesTotalLength += len(line) + 1 // count the newline as well
	}

	if body.Len() > 0 && body.Len()+linesTotalLength >= s.config.MaxRequestBodySize {
		sent = true
		err = s.send(ctx, pipeline, body.toCountingReader(), flds)
		body.Reset()
	}

	if body.Len() > 0 {
		// Do not add newline if the body is empty
		body.addNewLine()
	}

	body.addLines(lines)

	return sent, err
}

// sendTraces sends traces in right format basing on the s.config.TraceFormat
func (s *sender) sendTraces(ctx context.Context, td ptrace.Traces) error {
	return s.sendOTLPTraces(ctx, td)
}

// sendOTLPTraces sends trace records in OTLP format
func (s *sender) sendOTLPTraces(ctx context.Context, td ptrace.Traces) error {
	if td.ResourceSpans().Len() == 0 {
		s.logger.Debug("there are no traces to send, moving on")
		return nil
	}

	capacity := td.SpanCount()

	body, err := tracesMarshaler.MarshalTraces(td)
	if err != nil {
		return err
	}
	if err := s.send(ctx, TracesPipeline, newCountingReader(capacity).withBytes(body), fields{}); err != nil {
		return err
	}
	return nil
}

func addSourcesHeaders(req *http.Request, flds fields) {
	sourceHeaderValues := getSourcesHeaders(flds)

	for headerName, headerValue := range sourceHeaderValues {
		req.Header.Add(headerName, headerValue)
	}
}

func getSourcesHeaders(flds fields) map[string]string {
	sourceHeaderValues := map[string]string{}
	if !flds.isInitialized() {
		return sourceHeaderValues
	}

	attrs := flds.orig

	if v, ok := attrs.Get(attributeKeySourceHost); ok {
		sourceHeaderValues[headerHost] = v.AsString()
	}

	if v, ok := attrs.Get(attributeKeySourceName); ok {
		sourceHeaderValues[headerName] = v.AsString()
	}

	if v, ok := attrs.Get(attributeKeySourceCategory); ok {
		sourceHeaderValues[headerCategory] = v.AsString()
	}
	return sourceHeaderValues
}

func addLogsHeaders(req *http.Request, lf LogFormatType, flds fields) {
	switch lf {
	case OTLPLogFormat:
		req.Header.Add(headerContentType, contentTypeOTLP)
	default:
		req.Header.Add(headerContentType, contentTypeLogs)
	}

	if fieldsStr := flds.string(); fieldsStr != "" {
		req.Header.Add(headerFields, fieldsStr)
	}
}

func addMetricsHeaders(req *http.Request, mf MetricFormatType) error {
	switch mf {
	case PrometheusFormat:
		req.Header.Add(headerContentType, contentTypePrometheus)
	case OTLPMetricFormat:
		req.Header.Add(headerContentType, contentTypeOTLP)
	default:
		return fmt.Errorf("unsupported metrics format: %s", mf)
	}
	return nil
}

func addTracesHeaders(req *http.Request) {
	req.Header.Add(headerContentType, contentTypeOTLP)
}

func (s *sender) addRequestHeaders(req *http.Request, pipeline PipelineType, flds fields) error {
	req.Header.Add(headerClient, s.config.Client)
	addSourcesHeaders(req, flds)

	switch pipeline {
	case LogsPipeline:
		addLogsHeaders(req, s.config.LogFormat, flds)
	case MetricsPipeline:
		if err := addMetricsHeaders(req, s.config.MetricFormat); err != nil {
			return err
		}
	case TracesPipeline:
		addTracesHeaders(req)
	default:
		return fmt.Errorf("unexpected pipeline: %v", pipeline)
	}
	return nil
}

func (s *sender) recordMetrics(duration time.Duration, count int64, req *http.Request, resp *http.Response, pipeline PipelineType) {
	statusCode := 0

	if resp != nil {
		statusCode = resp.StatusCode
	}

	id := s.id.String()

	attrs := attribute.NewSet(
		attribute.String("status_code", fmt.Sprint(statusCode)),
		attribute.String("endpoint", req.URL.String()),
		attribute.String("pipeline", string(pipeline)),
		attribute.String("exporter", id),
	)
	s.telemetryBuilder.ExporterRequestsDuration.Add(context.Background(), duration.Milliseconds(), metric.WithAttributeSet(attrs))
	s.telemetryBuilder.ExporterRequestsBytes.Add(context.Background(), req.ContentLength, metric.WithAttributeSet(attrs))
	s.telemetryBuilder.ExporterRequestsRecords.Add(context.Background(), count, metric.WithAttributeSet(attrs))
	s.telemetryBuilder.ExporterRequestsSent.Add(context.Background(), 1, metric.WithAttributeSet(attrs))
}

func (s *sender) addStickySessionCookie(req *http.Request) {
	currentCookieValue := s.stickySessionCookieFunc()
	if currentCookieValue != "" {
		cookie := &http.Cookie{
			Name:  stickySessionKey,
			Value: currentCookieValue,
		}
		req.AddCookie(cookie)
	}
}

func (s *sender) updateStickySessionCookie(resp *http.Response) {
	cookies := resp.Cookies()
	if len(cookies) > 0 {
		for _, cookie := range cookies {
			if cookie.Name == stickySessionKey {
				if cookie.Value != s.stickySessionCookieFunc() {
					s.setStickySessionCookieFunc(cookie.Value)
				}
				return
			}
		}
	}
}
