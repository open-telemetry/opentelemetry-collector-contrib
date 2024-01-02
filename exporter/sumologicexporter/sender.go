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

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type appendResponse struct {
	// sent gives information if the data was sent or not
	sent bool
	// appended keeps state of appending new log line to the body
	appended bool
}

// metricPair represents information required to send one metric to the Sumo Logic
type metricPair struct {
	attributes pcommon.Map
	metric     pmetric.Metric
}

type sender struct {
	logBuffer           []plog.LogRecord
	metricBuffer        []metricPair
	config              *Config
	client              *http.Client
	filter              filter
	sources             sourceFormats
	compressor          compressor
	prometheusFormatter prometheusFormatter
	graphiteFormatter   graphiteFormatter
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

	contentTypeLogs       string = "application/x-www-form-urlencoded"
	contentTypePrometheus string = "application/vnd.sumologic.prometheus"
	contentTypeCarbon2    string = "application/vnd.sumologic.carbon2"
	contentTypeGraphite   string = "application/vnd.sumologic.graphite"

	contentEncodingGzip    string = "gzip"
	contentEncodingDeflate string = "deflate"
)

func newAppendResponse() appendResponse {
	return appendResponse{
		appended: true,
	}
}

func newSender(
	cfg *Config,
	cl *http.Client,
	f filter,
	s sourceFormats,
	c compressor,
	pf prometheusFormatter,
	gf graphiteFormatter,
) *sender {
	return &sender{
		config:              cfg,
		client:              cl,
		filter:              f,
		sources:             s,
		compressor:          c,
		prometheusFormatter: pf,
		graphiteFormatter:   gf,
	}
}

// send sends data to sumologic
func (s *sender) send(ctx context.Context, pipeline PipelineType, body io.Reader, flds fields) error {
	data, err := s.compressor.compress(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.config.HTTPClientSettings.Endpoint, data)
	if err != nil {
		return err
	}

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

	req.Header.Add(headerClient, s.config.Client)

	if s.sources.host.isSet() {
		req.Header.Add(headerHost, s.sources.host.format(flds))
	}

	if s.sources.name.isSet() {
		req.Header.Add(headerName, s.sources.name.format(flds))
	}

	if s.sources.category.isSet() {
		req.Header.Add(headerCategory, s.sources.category.format(flds))
	}

	switch pipeline {
	case LogsPipeline:
		req.Header.Add(headerContentType, contentTypeLogs)
		req.Header.Add(headerFields, flds.string())
	case MetricsPipeline:
		switch s.config.MetricFormat {
		case PrometheusFormat:
			req.Header.Add(headerContentType, contentTypePrometheus)
		case Carbon2Format:
			req.Header.Add(headerContentType, contentTypeCarbon2)
		case GraphiteFormat:
			req.Header.Add(headerContentType, contentTypeGraphite)
		default:
			return fmt.Errorf("unsupported metrics format: %s", s.config.MetricFormat)
		}
	default:
		return errors.New("unexpected pipeline")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("error during sending data: %s", resp.Status)
	}
	return nil
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
		body           strings.Builder
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

		ar, err := s.appendAndSend(ctx, formattedLine, LogsPipeline, &body, flds)
		if err != nil {
			errs = append(errs, err)
			if ar.sent {
				droppedRecords = append(droppedRecords, currentRecords...)
			}

			if !ar.appended {
				droppedRecords = append(droppedRecords, record)
			}
		}

		// If data was sent, cleanup the currentTimeSeries counter
		if ar.sent {
			currentRecords = currentRecords[:0]
		}

		// If log has been appended to body, increment the currentTimeSeries
		if ar.appended {
			currentRecords = append(currentRecords, record)
		}
	}

	if body.Len() > 0 {
		if err := s.send(ctx, LogsPipeline, strings.NewReader(body.String()), flds); err != nil {
			errs = append(errs, err)
			droppedRecords = append(droppedRecords, currentRecords...)
		}
	}

	return droppedRecords, errors.Join(errs...)
}

// sendMetrics sends metrics in right format basing on the s.config.MetricFormat
func (s *sender) sendMetrics(ctx context.Context, flds fields) ([]metricPair, error) {
	var (
		body           strings.Builder
		errs           []error
		droppedRecords []metricPair
		currentRecords []metricPair
	)

	for _, record := range s.metricBuffer {
		var formattedLine string
		var err error

		switch s.config.MetricFormat {
		case PrometheusFormat:
			formattedLine = s.prometheusFormatter.metric2String(record)
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

		ar, err := s.appendAndSend(ctx, formattedLine, MetricsPipeline, &body, flds)
		if err != nil {
			errs = append(errs, err)
			if ar.sent {
				droppedRecords = append(droppedRecords, currentRecords...)
			}

			if !ar.appended {
				droppedRecords = append(droppedRecords, record)
			}
		}

		// If data was sent, cleanup the currentTimeSeries counter
		if ar.sent {
			currentRecords = currentRecords[:0]
		}

		// If log has been appended to body, increment the currentTimeSeries
		if ar.appended {
			currentRecords = append(currentRecords, record)
		}
	}

	if body.Len() > 0 {
		if err := s.send(ctx, MetricsPipeline, strings.NewReader(body.String()), flds); err != nil {
			errs = append(errs, err)
			droppedRecords = append(droppedRecords, currentRecords...)
		}
	}

	return droppedRecords, errors.Join(errs...)
}

// appendAndSend appends line to the request body that will be sent and sends
// the accumulated data if the internal logBuffer has been filled (with maxBufferSize elements).
// It returns appendResponse
func (s *sender) appendAndSend(
	ctx context.Context,
	line string,
	pipeline PipelineType,
	body *strings.Builder,
	flds fields,
) (appendResponse, error) {
	var errs []error
	ar := newAppendResponse()

	if body.Len() > 0 && body.Len()+len(line) >= s.config.MaxRequestBodySize {
		ar.sent = true
		errs = append(errs, s.send(ctx, pipeline, strings.NewReader(body.String()), flds))
		body.Reset()
	}

	if body.Len() > 0 {
		// Do not add newline if the body is empty
		if _, err := body.WriteString("\n"); err != nil {
			errs = append(errs, err)
			ar.appended = false
		}
	}

	if ar.appended {
		// Do not append new line if separator was not appended
		if _, err := body.WriteString(line); err != nil {
			errs = append(errs, err)
			ar.appended = false
		}
	}

	return ar, errors.Join(errs...)
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
