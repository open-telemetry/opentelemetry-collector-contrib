// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logzioexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/hashicorp/go-hclog"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/protobuf/proto"
)

const (
	loggerName               = "logzio-exporter"
	headerRetryAfter         = "Retry-After"
	headerAuthorization      = "Authorization"
	maxHTTPResponseReadBytes = 64 * 1024
)

// logzioExporter implements an OpenTelemetry trace exporter that exports all spans to Logz.io
type logzioExporter struct {
	config   *Config
	client   *http.Client
	logger   hclog.Logger
	settings component.TelemetrySettings
}

func newLogzioExporter(cfg *Config, params exporter.Settings) (*logzioExporter, error) {
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
	}, nil
}

func newLogzioTracesExporter(config *Config, set exporter.Settings) (exporter.Traces, error) {
	exporter, err := newLogzioExporter(config, set)
	if err != nil {
		return nil, err
	}
	exporter.config.Endpoint, err = generateEndpoint(config, "traces")
	if err != nil {
		return nil, err
	}
	config.checkAndWarnDeprecatedOptions(exporter.logger)
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
	exporter.config.Endpoint, err = generateEndpoint(config, "logs")
	if err != nil {
		return nil, err
	}
	config.checkAndWarnDeprecatedOptions(exporter.logger)
	return exporterhelper.NewLogs(
		context.TODO(),
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
	client, err := exporter.config.ToClient(ctx, host, exporter.settings)
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
	return exporter.export(ctx, exporter.config.Endpoint, request)
}

func mergeMapEntries(maps ...pcommon.Map) pcommon.Map {
	res := map[string]any{}
	for _, m := range maps {
		for key, val := range m.AsRaw() {
			// Check if the key was already added
			if resMapValue, keyExists := res[key]; keyExists {
				rt := reflect.TypeOf(resMapValue)
				switch rt.Kind() {
				case reflect.Slice:
					res[key] = append(resMapValue.([]any), val)
				default:
					// Create a new slice and append values if the key exists:
					valslice := []any{}
					res[key] = append(valslice, resMapValue, val)
				}
			} else {
				res[key] = val
			}
		}
	}
	pcommonRes := pcommon.NewMap()
	err := pcommonRes.FromRaw(res)
	if err != nil {
		return pcommon.Map{}
	}
	return pcommonRes
}

func (exporter *logzioExporter) pushTraceData(ctx context.Context, traces ptrace.Traces) error {
	tr := ptraceotlp.NewExportRequestFromTraces(traces)
	var err error
	var request []byte
	request, err = tr.MarshalProto()
	if err != nil {
		return consumererror.NewPermanent(err)
	}
	return exporter.export(ctx, exporter.config.Endpoint, request)
}

// export is similar to otlphttp export method with changes in log messages + Permanent error for `StatusUnauthorized` and `StatusForbidden`
// https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/otlphttpexporter/otlp.go#L127
func (exporter *logzioExporter) export(ctx context.Context, url string, request []byte) error {
	exporter.logger.Debug(fmt.Sprintf("Preparing to make HTTP request with %d bytes", len(request)))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(request))
	if err != nil {
		return consumererror.NewPermanent(err)
	}
	req.Header.Set(headerAuthorization, fmt.Sprintf("Bearer %s", string(exporter.config.Token)))
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
