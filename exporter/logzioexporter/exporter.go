// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logzioexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	st "google.golang.org/grpc/status"
	"io"
	"net/http"
	"regexp"
	"runtime"
	"strconv"
	"time"

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
	headerRetryAfter         = "Retry-After"
	maxHTTPResponseReadBytes = 64 * 1024
	jsonContentType          = "application/json"
	protobufContentType      = "application/x-protobuf"
)

type partialSuccessHandler func(bytes []byte, contentType string) error

// logzioExporter implements an OpenTelemetry trace exporter that exports all spans to Logz.io
type logzioExporter struct {
	config       *Config
	client       *http.Client
	logger       *zap.Logger
	settings     component.TelemetrySettings
	serviceCache cache.Cache
}

func newLogzioExporter(cfg *Config, params exporter.Settings) (*logzioExporter, error) {
	logger := params.Logger
	if cfg == nil {
		return nil, errors.New("exporter config can't be null")
	}
	return &logzioExporter{
		config:   cfg,
		logger:   logger,
		settings: params.TelemetrySettings,
		serviceCache: cache.NewLRUWithOptions(
			100000,
			&cache.Options{
				TTL: 24 * time.Hour,
			},
		),
	}, nil
}

func newLogzioTracesExporter(config *Config, set exporter.Settings) (exporter.Traces, error) {
	exporter, err := newLogzioExporter(config, set)
	if err != nil {
		return nil, err
	}
	exporter.config.ClientConfig.Endpoint, err = generateTracesEndpoint(config)
	config.checkAndWarnDeprecatedOptions(exporter.logger)
	userAgent := fmt.Sprintf("otel-collector-logzio-traces-exporter-%s-%s-%s", set.BuildInfo.Version, runtime.GOOS, runtime.GOARCH)
	exporter.config.ClientConfig.Headers = map[string]configopaque.String{
		"Content-Type": jsonContentType,
		"User-Agent":   configopaque.String(userAgent),
	}
	return exporterhelper.NewTraces(
		context.Background(),
		set,
		config,
		exporter.pushTraceData,
		exporterhelper.WithStart(exporter.start),
		// disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}),
		exporterhelper.WithQueue(config.QueueSettings),
		exporterhelper.WithRetry(config.BackOffConfig),
	)
}
func newLogzioLogsExporter(config *Config, set exporter.Settings) (exporter.Logs, error) {
	exporter, err := newLogzioExporter(config, set)
	if err != nil {
		return nil, err
	}
	exporter.config.ClientConfig.Endpoint, err = generateLogsEndpoint(config)
	userAgent := fmt.Sprintf("otel-collector-logzio-logs-exporter-%s-%s-%s", set.BuildInfo.Version, runtime.GOOS, runtime.GOARCH)
	exporter.config.ClientConfig.Headers = map[string]configopaque.String{
		"Authorization": "Bearer " + config.Token,
		"Content-Type":  protobufContentType,
		"User-Agent":    configopaque.String(userAgent),
	}
	config.checkAndWarnDeprecatedOptions(exporter.logger)
	return exporterhelper.NewLogs(
		context.Background(),
		set,
		config,
		exporter.pushLogData,
		exporterhelper.WithStart(exporter.start),
		// disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}),
		exporterhelper.WithQueue(config.QueueSettings),
		exporterhelper.WithRetry(config.BackOffConfig),
	)
}

func (exporter *logzioExporter) start(ctx context.Context, host component.Host) error {
	client, err := exporter.config.ClientConfig.ToClient(ctx, host, exporter.settings)
	if err != nil {
		return err
	}
	exporter.client = client
	return nil
}

func (exporter *logzioExporter) pushLogData(ctx context.Context, ld plog.Logs) error {
	tr := plogotlp.NewExportRequestFromLogs(ld)
	var err error
	var request []byte
	request, err = tr.MarshalProto()
	if err != nil {
		return consumererror.NewPermanent(err)
	}
	return exporter.export(ctx, exporter.config.ClientConfig.Endpoint, request, exporter.logsPartialSuccessHandler)
}

func (exporter *logzioExporter) pushTraceData(ctx context.Context, traces ptrace.Traces) error {
	// a buffer to store logzio span and services bytes
	var dataBuffer bytes.Buffer
	batches := jaeger.ProtoFromTraces(traces)
	for _, batch := range batches {
		for _, span := range batch.Spans {
			span.Process = batch.Process
			span.Tags = exporter.dropEmptyTags(span.Tags)
			span.Process.Tags = exporter.dropEmptyTags(span.Process.Tags)
			logzioSpan, transformErr := transformToLogzioSpanBytes(span)
			if transformErr != nil {
				return transformErr
			}
			_, err := dataBuffer.Write(append(logzioSpan, '\n'))
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
	err := exporter.export(ctx, exporter.config.ClientConfig.Endpoint, dataBuffer.Bytes(), exporter.tracesPartialSuccessHandler)
	// reset the data buffer after each export to prevent duplicated data
	dataBuffer.Reset()
	return err
}

func (exporter *logzioExporter) export(ctx context.Context, url string, request []byte, partialSuccessHandler partialSuccessHandler) error {
	maskedURL := regexp.MustCompile(`(token=)[^&]+`).ReplaceAllString(url, `$1****`)
	exporter.logger.Debug("Preparing to make HTTP request", zap.String("url", maskedURL), zap.Int("request_size", len(request)))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(request))
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	resp, err := exporter.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make an HTTP request: %w", err)
	}

	defer func() {
		// Discard any remaining response body when we are done reading.
		io.CopyN(io.Discard, resp.Body, maxHTTPResponseReadBytes) // nolint:errcheck
		resp.Body.Close()
	}()
	exporter.logger.Debug(fmt.Sprintf("Response status code: %d", resp.StatusCode))

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		return handlePartialSuccessResponse(resp, partialSuccessHandler)
	}

	respStatus := readResponseStatus(resp)

	// Format the error message. Use the status if it is present in the response.
	var errString string
	var formattedErr error
	if respStatus != nil {
		errString = fmt.Sprintf(
			"error exporting items, request to %s responded with HTTP Status Code %d, Message=%s, Details=%v",
			url, resp.StatusCode, respStatus.Message, respStatus.Details)
	} else {
		errString = fmt.Sprintf(
			"error exporting items, request to %s responded with HTTP Status Code %d",
			url, resp.StatusCode)
	}
	formattedErr = NewStatusFromMsgAndHTTPCode(errString, resp.StatusCode).Err()

	if isRetryableStatusCode(resp.StatusCode) {
		// A retry duration of 0 seconds will trigger the default backoff policy
		// of our caller (retry handler).
		retryAfter := 0

		// Check if the server is overwhelmed.
		// See spec https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/protocol/otlp.md#otlphttp-throttling
		isThrottleError := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable
		if val := resp.Header.Get(headerRetryAfter); isThrottleError && val != "" {
			if seconds, err2 := strconv.Atoi(val); err2 == nil {
				retryAfter = seconds
			}
		}

		return exporterhelper.NewThrottleRetry(formattedErr, time.Duration(retryAfter)*time.Second)
	}

	return consumererror.NewPermanent(formattedErr)
}

func readResponseBody(resp *http.Response) ([]byte, error) {
	if resp.ContentLength == 0 {
		return nil, nil
	}

	maxRead := resp.ContentLength

	// if maxRead == -1, the ContentLength header has not been sent, so read up to
	// the maximum permitted body size. If it is larger than the permitted body
	// size, still try to read from the body in case the value is an error. If the
	// body is larger than the maximum size, proto unmarshaling will likely fail.
	if maxRead == -1 || maxRead > maxHTTPResponseReadBytes {
		maxRead = maxHTTPResponseReadBytes
	}
	protoBytes := make([]byte, maxRead)
	n, err := io.ReadFull(resp.Body, protoBytes)

	// No bytes read and an EOF error indicates there is no body to read.
	if n == 0 && (err == nil || errors.Is(err, io.EOF)) {
		return nil, nil
	}

	// io.ReadFull will return io.ErrorUnexpectedEOF if the Content-Length header
	// wasn't set, since we will try to read past the length of the body. If this
	// is the case, the body will still have the full message in it, so we want to
	// ignore the error and parse the message.
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}

	return protoBytes[:n], nil
}

// Read the response and decode the status.Status from the body.
// Returns nil if the response is empty or cannot be decoded.
func readResponseStatus(resp *http.Response) *status.Status {
	var respStatus *status.Status
	if resp.StatusCode >= 400 && resp.StatusCode <= 599 {
		// Request failed. Read the body. OTLP spec says:
		// "Response body for all HTTP 4xx and HTTP 5xx responses MUST be a
		// Protobuf-encoded Status message that describes the problem."
		respBytes, err := readResponseBody(resp)
		if err != nil {
			return nil
		}

		// Decode it as Status struct. See https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/protocol/otlp.md#failures
		respStatus = &status.Status{}
		err = proto.Unmarshal(respBytes, respStatus)
		if err != nil {
			return nil
		}
	}

	return respStatus
}

func (exporter *logzioExporter) tracesPartialSuccessHandler(protoBytes []byte, contentType string) error {
	if protoBytes == nil {
		return nil
	}
	exportResponse := ptraceotlp.NewExportResponse()
	switch contentType {
	case protobufContentType:
		err := exportResponse.UnmarshalProto(protoBytes)
		if err != nil {
			return fmt.Errorf("error parsing protobuf response: %w", err)
		}
	case jsonContentType:
		err := exportResponse.UnmarshalJSON(protoBytes)
		if err != nil {
			return fmt.Errorf("error parsing json response: %w", err)
		}
	default:
		return nil
	}

	partialSuccess := exportResponse.PartialSuccess()
	if !(partialSuccess.ErrorMessage() == "" && partialSuccess.RejectedSpans() == 0) {
		exporter.logger.Warn("Partial success response",
			zap.String("message", exportResponse.PartialSuccess().ErrorMessage()),
			zap.Int64("dropped_spans", exportResponse.PartialSuccess().RejectedSpans()),
		)
	}
	return nil
}

func (exporter *logzioExporter) logsPartialSuccessHandler(protoBytes []byte, contentType string) error {
	if protoBytes == nil {
		return nil
	}
	exportResponse := plogotlp.NewExportResponse()
	switch contentType {
	case protobufContentType:
		err := exportResponse.UnmarshalProto(protoBytes)
		if err != nil {
			return fmt.Errorf("error parsing protobuf response: %w", err)
		}
	case jsonContentType:
		err := exportResponse.UnmarshalJSON(protoBytes)
		if err != nil {
			return fmt.Errorf("error parsing json response: %w", err)
		}
	default:
		return nil
	}
	partialSuccess := exportResponse.PartialSuccess()
	if !(partialSuccess.ErrorMessage() == "" && partialSuccess.RejectedLogRecords() == 0) {
		exporter.logger.Warn("Partial success response",
			zap.String("message", exportResponse.PartialSuccess().ErrorMessage()),
			zap.Int64("dropped_log_records", exportResponse.PartialSuccess().RejectedLogRecords()),
		)
	}
	return nil
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

func handlePartialSuccessResponse(resp *http.Response, partialSuccessHandler partialSuccessHandler) error {
	bodyBytes, err := readResponseBody(resp)
	if err != nil {
		return err
	}

	return partialSuccessHandler(bodyBytes, resp.Header.Get("Content-Type"))
}

// Determine if the status code is retryable according to the specification.
// For more, see https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/protocol/otlp.md#failures-1
func isRetryableStatusCode(code int) bool {
	switch code {
	case http.StatusTooManyRequests:
		return true
	case http.StatusBadGateway:
		return true
	case http.StatusServiceUnavailable:
		return true
	case http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// NewStatusFromMsgAndHTTPCode returns a gRPC status based on an error message string and a http status code.
// This function is shared between the http receiver and http exporter for error propagation.
func NewStatusFromMsgAndHTTPCode(errMsg string, statusCode int) *st.Status {
	var c codes.Code
	// Mapping based on https://github.com/grpc/grpc/blob/master/doc/http-grpc-status-mapping.md
	// 429 mapping to ResourceExhausted and 400 mapping to StatusBadRequest are exceptions.
	switch statusCode {
	case http.StatusBadRequest:
		c = codes.InvalidArgument
	case http.StatusUnauthorized:
		c = codes.Unauthenticated
	case http.StatusForbidden:
		c = codes.PermissionDenied
	case http.StatusNotFound:
		c = codes.Unimplemented
	case http.StatusTooManyRequests:
		c = codes.ResourceExhausted
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		c = codes.Unavailable
	default:
		c = codes.Unknown
	}
	return st.New(c, errMsg)
}
