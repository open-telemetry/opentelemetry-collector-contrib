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
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter/internal/observability"
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
	logger              *zap.Logger
	logBuffer           []plog.LogRecord
	metricBuffer        []metricPair
	config              *Config
	client              *http.Client
	filter              filter
	sources             sourceFormats
	compressor          compressor
	prometheusFormatter prometheusFormatter
	dataURLMetrics      string
	dataURLLogs         string
	dataURLTraces       string
	graphiteFormatter   graphiteFormatter
	id                  component.ID
}

const (
	logKey string = "log"
	// maxBufferSize defines size of the logBuffer (maximum number of plog.LogRecord entries)
	maxBufferSize int = 1024 * 1024

	headerContentType     string = "Content-Type"
	headerContentEncoding string = "Content-Encoding"
	headerClient          string = "X-Sumo-Client"
	headerHost            string = "X-Sumo-Host"
	headerName            string = "X-Sumo-Name"
	headerCategory        string = "X-Sumo-Category"
	headerFields          string = "X-Sumo-Fields"

	attributeKeySourceHost     = "_sourceHost"
	attributeKeySourceName     = "_sourceName"
	attributeKeySourceCategory = "_sourceCategory"

	contentTypeLogs       string = "application/x-www-form-urlencoded"
	contentTypePrometheus string = "application/vnd.sumologic.prometheus"
	contentTypeCarbon2    string = "application/vnd.sumologic.carbon2"
	contentTypeGraphite   string = "application/vnd.sumologic.graphite"

	contentEncodingGzip    string = "gzip"
	contentEncodingDeflate string = "deflate"
)

func newSender(
	logger *zap.Logger,
	cfg *Config,
	cl *http.Client,
	f filter,
	s sourceFormats,
	c compressor,
	pf prometheusFormatter,
	metricsURL string,
	logsURL string,
	tracesURL string,
	gf graphiteFormatter,
	id component.ID,
) *sender {
	return &sender{
		logger:              logger,
		config:              cfg,
		client:              cl,
		filter:              f,
		sources:             s,
		compressor:          c,
		prometheusFormatter: pf,
		dataURLMetrics:      metricsURL,
		dataURLLogs:         logsURL,
		dataURLTraces:       tracesURL,
		graphiteFormatter:   gf,
		id:                  id,
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
	// API responds with a 200 or 204 with ConentLength set to 0 when all data
	// has been successfully ingested.
	if resp.ContentLength == 0 && (resp.StatusCode == 200 || resp.StatusCode == 204) {
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
	case 200, 204:
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

	case 401:
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
	default:
		return nil, fmt.Errorf("unknown pipeline type: %s", pipeline)
	}

	data, err := s.compressor.compress(data)
	if err != nil {
		return nil, err
	}

	return http.NewRequestWithContext(ctx, http.MethodPost, url, data)
}

// logToText converts LogRecord to a plain text line, returns it and error eventually
func (s *sender) logToText(record plog.LogRecord) string {
	return record.Body().AsString()
}

// logToJSON converts LogRecord to a json line, returns it and error eventually
func (s *sender) logToJSON(record plog.LogRecord) (string, error) {
	data := s.filter.filterOut(record.Attributes())
	record.Body().CopyTo(data.orig.PutEmpty(logKey))

	nextLine, err := json.Marshal(data.orig.AsRaw())
	if err != nil {
		return "", err
	}

	return bytes.NewBuffer(nextLine).String(), nil
}

// sendLogs sends log records from the logBuffer formatted according
// to configured LogFormat and as the result of execution
// returns array of records which has not been sent correctly and error
func (s *sender) sendLogs(ctx context.Context, flds fields) ([]plog.LogRecord, error) {
	var (
		body           = newBodyBuilder()
		errs           []error
		droppedRecords []plog.LogRecord
		currentRecords []plog.LogRecord
	)

	for _, record := range s.logBuffer {
		var formattedLine string
		var err error

		switch s.config.LogFormat {
		case TextFormat:
			formattedLine = s.logToText(record)
		case JSONFormat:
			formattedLine, err = s.logToJSON(record)
		default:
			err = errors.New("unexpected log format")
		}

		if err != nil {
			droppedRecords = append(droppedRecords, record)
			errs = append(errs, err)
			continue
		}

		sent, err := s.appendAndMaybeSend(ctx, []string{formattedLine}, LogsPipeline, &body, flds)
		if err != nil {
			errs = append(errs, err)
			droppedRecords = append(droppedRecords, currentRecords...)
		}

		// If data was sent, cleanup the currentTimeSeries counter
		if sent {
			currentRecords = currentRecords[:0]
		}

		currentRecords = append(currentRecords, record)
	}

	if body.Len() > 0 {
		if err := s.send(ctx, LogsPipeline, body.toCountingReader(), flds); err != nil {
			errs = append(errs, err)
			droppedRecords = append(droppedRecords, currentRecords...)
		}
	}

	return droppedRecords, errors.Join(errs...)
}

// sendMetrics sends metrics in right format basing on the s.config.MetricFormat
func (s *sender) sendMetrics(ctx context.Context, flds fields) ([]metricPair, error) {
	var (
		body           = newBodyBuilder()
		errs           []error
		droppedRecords []metricPair
		currentRecords []metricPair
	)

	for _, record := range s.metricBuffer {
		var formattedLine string
		var err error

		switch s.config.MetricFormat {
		case PrometheusFormat:
			formattedLine = s.prometheusFormatter.metric2String(record.metric, record.attributes)
		case Carbon2Format:
			formattedLine = carbon2Metric2String(record)
		case GraphiteFormat:
			formattedLine = s.graphiteFormatter.metric2String(record)
		default:
			err = fmt.Errorf("unexpected metric format: %s", s.config.MetricFormat)
		}

		if err != nil {
			droppedRecords = append(droppedRecords, record)
			errs = append(errs, err)
			continue
		}

		sent, err := s.appendAndMaybeSend(ctx, []string{formattedLine}, MetricsPipeline, &body, flds)
		if err != nil {
			errs = append(errs, err)
			if sent {
				droppedRecords = append(droppedRecords, currentRecords...)
			}
		}

		// If data was sent, cleanup the currentTimeSeries counter
		if sent {
			currentRecords = currentRecords[:0]
		}

		currentRecords = append(currentRecords, record)
	}

	if body.Len() > 0 {
		if err := s.send(ctx, MetricsPipeline, body.toCountingReader(), flds); err != nil {
			errs = append(errs, err)
			droppedRecords = append(droppedRecords, currentRecords...)
		}
	}

	return droppedRecords, errors.Join(errs...)
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

// cleanLogsBuffer zeroes logBuffer
func (s *sender) cleanLogsBuffer() {
	s.logBuffer = (s.logBuffer)[:0]
}

// batchLog adds log to the logBuffer and flushes them if logBuffer is full to avoid overflow
// returns list of log records which were not sent successfully
func (s *sender) batchLog(ctx context.Context, log plog.LogRecord, metadata fields) ([]plog.LogRecord, error) {
	s.logBuffer = append(s.logBuffer, log)

	if s.countLogs() >= maxBufferSize {
		dropped, err := s.sendLogs(ctx, metadata)
		s.cleanLogsBuffer()
		return dropped, err
	}

	return nil, nil
}

// countLogs returns number of logs in logBuffer
func (s *sender) countLogs() int {
	return len(s.logBuffer)
}

// cleanMetricBuffer zeroes metricBuffer
func (s *sender) cleanMetricBuffer() {
	s.metricBuffer = (s.metricBuffer)[:0]
}

// batchMetric adds metric to the metricBuffer and flushes them if metricBuffer is full to avoid overflow
// returns list of metric records which were not sent successfully
func (s *sender) batchMetric(ctx context.Context, metric metricPair, metadata fields) ([]metricPair, error) {
	s.metricBuffer = append(s.metricBuffer, metric)

	if s.countMetrics() >= maxBufferSize {
		dropped, err := s.sendMetrics(ctx, metadata)
		s.cleanMetricBuffer()
		return dropped, err
	}

	return nil, nil
}

// countMetrics returns number of metrics in metricBuffer
func (s *sender) countMetrics() int {
	return len(s.metricBuffer)
}

func (s *sender) addSourcesHeaders(req *http.Request, flds fields) {
	if s.sources.host.isSet() {
		req.Header.Add(headerHost, s.sources.host.format(flds))
	}

	if s.sources.name.isSet() {
		req.Header.Add(headerName, s.sources.name.format(flds))
	}

	if s.sources.category.isSet() {
		req.Header.Add(headerCategory, s.sources.category.format(flds))
	}
}

func addLogsHeaders(req *http.Request, _ LogFormatType, flds fields) {
	req.Header.Add(headerContentType, contentTypeLogs)

	if fieldsStr := flds.string(); fieldsStr != "" {
		req.Header.Add(headerFields, fieldsStr)
	}
}

func addMetricsHeaders(req *http.Request, mf MetricFormatType) error {
	switch mf {
	case PrometheusFormat:
		req.Header.Add(headerContentType, contentTypePrometheus)
	case Carbon2Format:
		req.Header.Add(headerContentType, contentTypeCarbon2)
	case GraphiteFormat:
		req.Header.Add(headerContentType, contentTypeGraphite)
	default:
		return fmt.Errorf("unsupported metrics format: %s", mf)
	}
	return nil
}

func (s *sender) addRequestHeaders(req *http.Request, pipeline PipelineType, flds fields) error {
	req.Header.Add(headerClient, s.config.Client)
	s.addSourcesHeaders(req, flds)

	// Add headers
	switch s.config.CompressEncoding {
	case GZIPCompression:
		req.Header.Set(headerContentEncoding, contentEncodingGzip)
	case DeflateCompression:
		req.Header.Set(headerContentEncoding, contentEncodingDeflate)
	case NoCompression:
	default:
		return fmt.Errorf("invalid content encoding: %s", s.config.CompressEncoding)
	}

	switch pipeline {
	case LogsPipeline:
		addLogsHeaders(req, s.config.LogFormat, flds)
	case MetricsPipeline:
		if err := addMetricsHeaders(req, s.config.MetricFormat); err != nil {
			return err
		}
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

	if err := observability.RecordRequestsDuration(duration, statusCode, req.URL.String(), string(pipeline), id); err != nil {
		s.logger.Debug("error for recording metric for request duration", zap.Error(err))
	}

	if err := observability.RecordRequestsBytes(req.ContentLength, statusCode, req.URL.String(), string(pipeline), id); err != nil {
		s.logger.Debug("error for recording metric for sent bytes", zap.Error(err))
	}

	if err := observability.RecordRequestsRecords(count, statusCode, req.URL.String(), string(pipeline), id); err != nil {
		s.logger.Debug("error for recording metric for sent records", zap.Error(err))
	}

	if err := observability.RecordRequestsSent(statusCode, req.URL.String(), string(pipeline), id); err != nil {
		s.logger.Debug("error for recording metric for sent request", zap.Error(err))
	}
}
