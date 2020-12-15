// Copyright 2020, OpenTelemetry Authors
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

package sumologicexporter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer/pdata"
)

type appendResponse struct {
	// sent gives information if the data was sent or not
	sent bool
	// appended keeps state of appending new log line to the body
	appended bool
}

type sender struct {
	buffer     []pdata.LogRecord
	config     *Config
	client     *http.Client
	filter     filter
	ctx        context.Context
	sources    sourceFormats
	compressor compressor
}

const (
	logKey string = "log"
	// maxBufferSize defines size of the buffer (maximum number of pdata.LogRecord entries)
	maxBufferSize int = 1024 * 1024
)

func newAppendResponse() appendResponse {
	return appendResponse{
		appended: true,
	}
}

func newSender(
	ctx context.Context,
	cfg *Config,
	cl *http.Client,
	f filter,
	s sourceFormats,
	c compressor,
) *sender {
	return &sender{
		config:     cfg,
		client:     cl,
		filter:     f,
		ctx:        ctx,
		sources:    s,
		compressor: c,
	}
}

// send sends data to sumologic
func (s *sender) send(pipeline PipelineType, body io.Reader, flds fields) error {
	data, err := s.compressor.compress(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(s.ctx, http.MethodPost, s.config.HTTPClientSettings.Endpoint, data)
	if err != nil {
		return err
	}

	// Add headers
	switch s.config.CompressEncoding {
	case GZIPCompression:
		req.Header.Set("Content-Encoding", "gzip")
	case DeflateCompression:
		req.Header.Set("Content-Encoding", "deflate")
	case NoCompression:
	default:
		return fmt.Errorf("invalid content encoding: %s", s.config.CompressEncoding)
	}

	req.Header.Add("X-Sumo-Client", s.config.Client)

	if s.sources.host.isSet() {
		req.Header.Add("X-Sumo-Host", s.sources.host.format(flds))
	}

	if s.sources.name.isSet() {
		req.Header.Add("X-Sumo-Name", s.sources.name.format(flds))
	}

	if s.sources.category.isSet() {
		req.Header.Add("X-Sumo-Category", s.sources.category.format(flds))
	}

	switch pipeline {
	case LogsPipeline:
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Add("X-Sumo-Fields", flds.string())
	case MetricsPipeline:
		// ToDo: Implement metrics pipeline
		return errors.New("current sender version doesn't support metrics")
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
func (s *sender) logToText(record pdata.LogRecord) (string, error) {
	return record.Body().StringVal(), nil
}

// logToText converts LogRecord to a json line, returns it and error eventually
func (s *sender) logToJSON(record pdata.LogRecord) (string, error) {
	data := s.filter.filterOut(record.Attributes())
	data[logKey] = record.Body().StringVal()

	nextLine, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	return bytes.NewBuffer(nextLine).String(), nil
}

// sendLogs sends log records from the buffer formatted according
// to configured LogFormat and as the result of execution
// returns array of records which has not been sent correctly and error
func (s *sender) sendLogs(flds fields) ([]pdata.LogRecord, error) {
	var (
		body           strings.Builder
		errs           []error
		droppedRecords []pdata.LogRecord
		currentRecords []pdata.LogRecord
	)

	for _, record := range s.buffer {
		var (
			formattedLine string
			err           error
		)

		switch s.config.LogFormat {
		case TextFormat:
			formattedLine, err = s.logToText(record)
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

		ar, err := s.appendAndSend(formattedLine, LogsPipeline, &body, flds)
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

	if err := s.send(LogsPipeline, strings.NewReader(body.String()), flds); err != nil {
		errs = append(errs, err)
		droppedRecords = append(droppedRecords, currentRecords...)
	}

	if len(errs) > 0 {
		return droppedRecords, componenterror.CombineErrors(errs)
	}
	return droppedRecords, nil
}

// appendAndSend appends line to the request body that will be sent and sends
// the accumulated data if the internal buffer has been filled (with maxBufferSize elements).
// It returns appendResponse
func (s *sender) appendAndSend(
	line string,
	pipeline PipelineType,
	body *strings.Builder,
	flds fields,
) (appendResponse, error) {
	var errors []error
	ar := newAppendResponse()

	if body.Len() > 0 && body.Len()+len(line) >= s.config.MaxRequestBodySize {
		ar.sent = true
		if err := s.send(pipeline, strings.NewReader(body.String()), flds); err != nil {
			errors = append(errors, err)
		}
		body.Reset()
	}

	if body.Len() > 0 {
		// Do not add newline if the body is empty
		if _, err := body.WriteString("\n"); err != nil {
			errors = append(errors, err)
			ar.appended = false
		}
	}

	if ar.appended {
		// Do not append new line if separator was not appended
		if _, err := body.WriteString(line); err != nil {
			errors = append(errors, err)
			ar.appended = false
		}
	}

	if len(errors) > 0 {
		return ar, componenterror.CombineErrors(errors)
	}
	return ar, nil
}

// cleanBuffer zeroes buffer
func (s *sender) cleanBuffer() {
	s.buffer = (s.buffer)[:0]
}

// batch adds log to the buffer and flushes them if buffer is full to avoid overflow
// returns list of log records which were not sent successfully
func (s *sender) batch(log pdata.LogRecord, metadata fields) ([]pdata.LogRecord, error) {
	s.buffer = append(s.buffer, log)

	if s.count() >= maxBufferSize {
		dropped, err := s.sendLogs(metadata)
		s.cleanBuffer()
		return dropped, err
	}

	return nil, nil
}

// count returns number of logs in buffer
func (s *sender) count() int {
	return len(s.buffer)
}
