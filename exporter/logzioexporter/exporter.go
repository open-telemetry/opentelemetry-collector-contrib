// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logzioexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/pkg/cache"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
)

const (
	loggerName               = "logzio-exporter"
	headerRetryAfter         = "Retry-After"
	maxHTTPResponseReadBytes = 64 * 1024
)

// logzioExporter implements an OpenTelemetry trace exporter that exports all spans to Logz.io
type logzioExporter struct {
	config       *Config
	client       *http.Client
	logger       hclog.Logger
	settings     component.TelemetrySettings
	serviceCache cache.Cache
}

func newLogzioExporter(cfg *Config, params exporter.CreateSettings) (*logzioExporter, error) {
	logger := hclog2ZapLogger{
		Zap:  params.Logger,
		name: loggerName,
	}
	if cfg == nil {
		return nil, errors.New("exporter config can't be null")
	}
	return &logzioExporter{
		config:   cfg,
		logger:   &logger,
		settings: params.TelemetrySettings,
		serviceCache: cache.NewLRUWithOptions(
			100000,
			&cache.Options{
				TTL: 24 * time.Hour,
			},
		),
	}, nil
}

func newLogzioTracesExporter(config *Config, set exporter.CreateSettings) (exporter.Traces, error) {
	exporter, err := newLogzioExporter(config, set)
	if err != nil {
		return nil, err
	}
	exporter.config.HTTPClientSettings.Endpoint, err = generateEndpoint(config)
	if err != nil {
		return nil, err
	}
	config.checkAndWarnDeprecatedOptions(exporter.logger)
	return exporterhelper.NewTracesExporter(
		context.TODO(),
		set,
		config,
		exporter.pushTraceData,
		exporterhelper.WithStart(exporter.start),
		// disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithQueue(config.QueueSettings),
		exporterhelper.WithRetry(config.RetrySettings),
	)
}
func newLogzioLogsExporter(config *Config, set exporter.CreateSettings) (exporter.Logs, error) {
	exporter, err := newLogzioExporter(config, set)
	if err != nil {
		return nil, err
	}
	exporter.config.HTTPClientSettings.Endpoint, err = generateEndpoint(config)
	if err != nil {
		return nil, err
	}
	config.checkAndWarnDeprecatedOptions(exporter.logger)
	return exporterhelper.NewLogsExporter(
		context.TODO(),
		set,
		config,
		exporter.pushLogData,
		exporterhelper.WithStart(exporter.start),
		// disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithQueue(config.QueueSettings),
		exporterhelper.WithRetry(config.RetrySettings),
	)
}

func (exporter *logzioExporter) start(_ context.Context, host component.Host) error {
	client, err := exporter.config.HTTPClientSettings.ToClient(host, exporter.settings)
	if err != nil {
		return err
	}
	exporter.client = client
	return nil
}

func (exporter *logzioExporter) pushLogData(ctx context.Context, ld plog.Logs) error {
	var dataBuffer bytes.Buffer
	resourceLogs := ld.ResourceLogs()
	for i := 0; i < resourceLogs.Len(); i++ {
		resource := resourceLogs.At(i).Resource()
		scopeLogs := resourceLogs.At(i).ScopeLogs()
		for j := 0; j < scopeLogs.Len(); j++ {
			logRecords := scopeLogs.At(j).LogRecords()
			scope := scopeLogs.At(j).Scope()
			for k := 0; k < logRecords.Len(); k++ {
				log := logRecords.At(k)
				jsonLog := convertLogRecordToJSON(log, scope.Name(), resource)
				logzioLog, err := json.Marshal(jsonLog)
				if err != nil {
					return err
				}
				_, err = dataBuffer.Write(append(logzioLog, '\n'))
				if err != nil {
					return err
				}
			}
		}
	}
	err := exporter.export(ctx, exporter.config.HTTPClientSettings.Endpoint, dataBuffer.Bytes())
	// reset the data buffer after each export to prevent duplicated data
	dataBuffer.Reset()
	return err
}

func (exporter *logzioExporter) pushTraceData(ctx context.Context, traces ptrace.Traces) error {
	// a buffer to store logzio span and services bytes
	var dataBuffer bytes.Buffer
	batches, err := jaeger.ProtoFromTraces(traces)
	if err != nil {
		return err
	}
	for _, batch := range batches {
		for _, span := range batch.Spans {
			span.Process = batch.Process
			span.Tags = exporter.dropEmptyTags(span.Tags)
			span.Process.Tags = exporter.dropEmptyTags(span.Process.Tags)
			logzioSpan, transformErr := transformToLogzioSpanBytes(span)
			if transformErr != nil {
				return transformErr
			}
			_, err = dataBuffer.Write(append(logzioSpan, '\n'))
			if err != nil {
				return err
			}
			// Create logzio service
			// if the service hash already exists in cache: skip
			// else: store service in cache and send to logz.io
			// this prevents sending duplicate logzio services
			service := newLogzioService(span)
			serviceHash, hashErr := service.HashCode()
			if exporter.serviceCache.Get(serviceHash) == nil || hashErr != nil {
				if hashErr == nil {
					exporter.serviceCache.Put(serviceHash, serviceHash)
				}
				serviceBytes, marshalErr := json.Marshal(service)
				if marshalErr != nil {
					return marshalErr
				}
				_, err = dataBuffer.Write(append(serviceBytes, '\n'))
				if err != nil {
					return err
				}
			}
		}
	}
	err = exporter.export(ctx, exporter.config.HTTPClientSettings.Endpoint, dataBuffer.Bytes())
	// reset the data buffer after each export to prevent duplicated data
	dataBuffer.Reset()
	return err
}

// export is similar to otlphttp export method with changes in log messages + Permanent error for `StatusUnauthorized` and `StatusForbidden`
// https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/otlphttpexporter/otlp.go#L127
func (exporter *logzioExporter) export(ctx context.Context, url string, request []byte) error {
	exporter.logger.Debug(fmt.Sprintf("Preparing to make HTTP request with %d bytes", len(request)))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(request))
	if err != nil {
		return consumererror.NewPermanent(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := exporter.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make an HTTP request: %w", err)
	}

	defer func() {
		// Discard any remaining response body when we are done reading.
		_, _ = io.CopyN(io.Discard, resp.Body, maxHTTPResponseReadBytes)
		resp.Body.Close()
	}()
	exporter.logger.Debug(fmt.Sprintf("Response status code: %d", resp.StatusCode))
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		// Request is successful.
		return nil
	}
	respStatus := readResponse(resp)
	// Format the error message. Use the status if it is present in the response.
	var formattedErr error
	if respStatus != nil {
		formattedErr = fmt.Errorf(
			"error exporting items, request to %s responded with HTTP Status Code %d, Message=%s, Details=%v",
			url, resp.StatusCode, respStatus.Message, respStatus.Details)
	} else {
		formattedErr = fmt.Errorf(
			"error exporting items, request to %s responded with HTTP Status Code %d",
			url, resp.StatusCode)
	}

	// Check if the server is overwhelmed.
	// See spec https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/protocol/otlp.md#throttling-1
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		// Fallback to 0 if the Retry-After header is not present. This will trigger the
		// default backoff policy by our caller (retry handler).
		retryAfter := 0
		if val := resp.Header.Get(headerRetryAfter); val != "" {
			if seconds, err2 := strconv.Atoi(val); err2 == nil {
				retryAfter = seconds
			}
		}
		// Indicate to our caller to pause for the specified number of seconds.
		return exporterhelper.NewThrottleRetry(formattedErr, time.Duration(retryAfter)*time.Second)
	}

	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return consumererror.NewPermanent(formattedErr)
	}
	// All other errors are retryable, so don't wrap them in consumererror.NewPermanent().
	return formattedErr
}

// Read the response and decode the status.Status from the body.
// Returns nil if the response is empty or cannot be decoded.
func readResponse(resp *http.Response) *status.Status {
	var respStatus *status.Status
	if resp.StatusCode >= 400 && resp.StatusCode <= 599 {
		// Request failed. Read the body. OTLP spec says:
		// "Response body for all HTTP 4xx and HTTP 5xx responses MUST be a
		// Protobuf-encoded Status message that describes the problem."
		maxRead := resp.ContentLength
		if maxRead == -1 || maxRead > maxHTTPResponseReadBytes {
			maxRead = maxHTTPResponseReadBytes
		}
		respBytes := make([]byte, maxRead)
		n, err := io.ReadFull(resp.Body, respBytes)
		if err == nil && n > 0 {
			// Decode it as Status struct. See https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/protocol/otlp.md#failures
			respStatus = &status.Status{}
			err = proto.Unmarshal(respBytes, respStatus)
			if err != nil {
				respStatus = nil
			}
		}
	}

	return respStatus
}

func (exporter *logzioExporter) dropEmptyTags(tags []model.KeyValue) []model.KeyValue {
	for i, tag := range tags {
		if tag.Key == "" {
			tags[i] = tags[len(tags)-1] // Copy last element to index i.
			tags = tags[:len(tags)-1]   // Truncate slice.
			exporter.logger.Warn(fmt.Sprintf("Found tag empty key: %s, dropping tag..", tag.String()))
		}
	}
	return tags
}
