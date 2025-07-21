// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tinybirdexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tinybirdexporter"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tinybirdexporter/internal"
)

const (
	headerRetryAfter  = "Retry-After"
	contentTypeNDJSON = "application/x-ndjson"
)

type tinybirdExporter struct {
	config    *Config
	client    *http.Client
	logger    *zap.Logger
	settings  component.TelemetrySettings
	userAgent string
}

func newExporter(cfg component.Config, set exporter.Settings) *tinybirdExporter {
	oCfg := cfg.(*Config)

	userAgent := fmt.Sprintf("%s/%s (%s/%s)",
		set.BuildInfo.Description, set.BuildInfo.Version, runtime.GOOS, runtime.GOARCH)

	return &tinybirdExporter{
		config:    oCfg,
		logger:    set.Logger,
		userAgent: userAgent,
		settings:  set.TelemetrySettings,
	}
}

func (e *tinybirdExporter) start(ctx context.Context, host component.Host) error {
	var err error
	e.client, err = e.config.ClientConfig.ToClient(ctx, host, e.settings)
	return err
}

func (e *tinybirdExporter) pushTraces(ctx context.Context, td ptrace.Traces) error {
	buffer := bytes.NewBuffer(nil)
	encoder := json.NewEncoder(buffer)
	err := internal.ConvertTraces(td, encoder)
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	if buffer.Len() > 0 {
		return e.export(ctx, e.config.Traces.Datasource, buffer)
	}
	return nil
}

func (e *tinybirdExporter) pushMetrics(ctx context.Context, md pmetric.Metrics) error {
	sumBuffer := bytes.NewBuffer(nil)
	sumEncoder := json.NewEncoder(sumBuffer)

	gaugeBuffer := bytes.NewBuffer(nil)
	gaugeEncoder := json.NewEncoder(gaugeBuffer)

	histogramBuffer := bytes.NewBuffer(nil)
	histogramEncoder := json.NewEncoder(histogramBuffer)

	exponentialHistogramBuffer := bytes.NewBuffer(nil)
	exponentialHistogramEncoder := json.NewEncoder(exponentialHistogramBuffer)

	err := internal.ConvertMetrics(md, sumEncoder, gaugeEncoder, histogramEncoder, exponentialHistogramEncoder)
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	// TODO: perform the exports in parallel to improve the operation latency
	if sumBuffer.Len() > 0 {
		err = errors.Join(err, e.export(ctx, e.config.Metrics.MetricsSum.Datasource, sumBuffer))
	}
	if gaugeBuffer.Len() > 0 {
		err = errors.Join(err, e.export(ctx, e.config.Metrics.MetricsGauge.Datasource, gaugeBuffer))
	}
	if histogramBuffer.Len() > 0 {
		err = errors.Join(err, e.export(ctx, e.config.Metrics.MetricsHistogram.Datasource, histogramBuffer))
	}
	if exponentialHistogramBuffer.Len() > 0 {
		err = errors.Join(err, e.export(ctx, e.config.Metrics.MetricsExponentialHistogram.Datasource, exponentialHistogramBuffer))
	}
	return err
}

func (e *tinybirdExporter) pushLogs(ctx context.Context, ld plog.Logs) error {
	buffer := bytes.NewBuffer(nil)
	encoder := json.NewEncoder(buffer)
	err := internal.ConvertLogs(ld, encoder)
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	if buffer.Len() > 0 {
		return e.export(ctx, e.config.Logs.Datasource, buffer)
	}
	return nil
}

func (e *tinybirdExporter) export(ctx context.Context, dataSource string, buffer *bytes.Buffer) error {
	// Create request and add query parameters
	url := e.config.ClientConfig.Endpoint + "/v0/events"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, buffer)
	if err != nil {
		return consumererror.NewPermanent(err)
	}
	q := req.URL.Query()
	q.Set("name", dataSource)
	if e.config.Wait {
		q.Set("wait", "true")
	}
	req.URL.RawQuery = q.Encode()

	// Set headers
	req.Header.Set("Content-Type", contentTypeNDJSON)
	req.Header.Set("Authorization", "Bearer "+string(e.config.Token))
	req.Header.Set("User-Agent", e.userAgent)

	// Send request
	resp, err := e.client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		// Drain the response body to avoid leaking resources.
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	// Check if the request was successful.
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	// Read error response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	formattedErr := fmt.Errorf("error exporting items, request to %s responded with HTTP Status Code %d, Message=%s",
		url, resp.StatusCode, string(respBody))

	// If the status code is not retryable, return a permanent error.
	if !isRetryableStatusCode(resp.StatusCode) {
		return consumererror.NewPermanent(formattedErr)
	}

	// Check if the server is overwhelmed.
	isThrottleError := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable
	if isThrottleError {
		values := resp.Header.Values(headerRetryAfter)
		if len(values) == 0 {
			return formattedErr
		}
		// The value of Retry-After field can be either an HTTP-date or a number of
		// seconds to delay after the response is received. See https://datatracker.ietf.org/doc/html/rfc7231#section-7.1.3
		//
		// Tinybird Events API returns the delay-seconds in the Retry-After header.
		// https://www.tinybird.co/docs/forward/get-data-in/events-api#rate-limit-headers
		if seconds, err := strconv.Atoi(values[0]); err == nil {
			return exporterhelper.NewThrottleRetry(formattedErr, time.Duration(seconds)*time.Second)
		}
	}

	return formattedErr
}

// Determine if the status code is retryable according to Tinybird Events API.
// See https://www.tinybird.co/docs/api-reference/events-api#return-http-status-codes
func isRetryableStatusCode(code int) bool {
	switch code {
	case http.StatusTooManyRequests:
		return true
	case http.StatusInternalServerError:
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
