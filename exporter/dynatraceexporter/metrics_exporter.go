// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dynatraceexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/apiconstants"
	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/dimensions"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter/internal/serialization"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/ttlmap"
)

const (
	cSweepIntervalSeconds = 300
	cMaxAgeSeconds        = 900
)

// newMetricsExporter exports to a Dynatrace Metrics v2 API
func newMetricsExporter(params exporter.CreateSettings, cfg *config.Config) *metricsExporter {
	var confDefaultDims []dimensions.Dimension
	for key, value := range cfg.DefaultDimensions {
		confDefaultDims = append(confDefaultDims, dimensions.NewDimension(key, value))
	}

	defaultDimensions := dimensions.MergeLists(
		dimensionsFromTags(cfg.Tags),
		dimensions.NewNormalizedDimensionList(confDefaultDims...),
	)

	staticDimensions := dimensions.NewNormalizedDimensionList(dimensions.NewDimension("dt.metrics.source", "opentelemetry"))

	prevPts := ttlmap.New(cSweepIntervalSeconds, cMaxAgeSeconds)
	prevPts.Start()

	return &metricsExporter{
		settings:          params.TelemetrySettings,
		cfg:               cfg,
		defaultDimensions: defaultDimensions,
		staticDimensions:  staticDimensions,
		prevPts:           prevPts,
	}
}

// metricsExporter forwards metrics to a Dynatrace agent
type metricsExporter struct {
	settings   component.TelemetrySettings
	cfg        *config.Config
	client     *http.Client
	isDisabled bool

	defaultDimensions dimensions.NormalizedDimensionList
	staticDimensions  dimensions.NormalizedDimensionList

	prevPts *ttlmap.TTLMap
}

// for backwards-compatibility with deprecated `Tags` config option
func dimensionsFromTags(tags []string) dimensions.NormalizedDimensionList {
	var dims []dimensions.Dimension
	for _, tag := range tags {
		parts := strings.SplitN(tag, "=", 2)
		if len(parts) == 2 {
			dims = append(dims, dimensions.NewDimension(parts[0], parts[1]))
		}
	}
	return dimensions.NewNormalizedDimensionList(dims...)
}

func (e *metricsExporter) PushMetricsData(ctx context.Context, md pmetric.Metrics) error {
	if e.isDisabled {
		return nil
	}

	lines := e.serializeMetrics(md)
	e.settings.Logger.Debug(
		"Serialization complete",
		zap.Int("data-point-count", md.DataPointCount()),
		zap.Int("lines", len(lines)),
	)

	// If request is empty string, there are no serializable metrics in the batch.
	// This can happen if all metric names are invalid
	if len(lines) == 0 {
		return nil
	}

	err := e.send(ctx, lines)

	if err != nil {
		return err
	}

	return nil
}

func (e *metricsExporter) serializeMetrics(md pmetric.Metrics) []string {
	var lines []string

	resourceMetrics := md.ResourceMetrics()

	for i := 0; i < resourceMetrics.Len(); i++ {
		resourceMetric := resourceMetrics.At(i)
		libraryMetrics := resourceMetric.ScopeMetrics()
		for j := 0; j < libraryMetrics.Len(); j++ {
			libraryMetric := libraryMetrics.At(j)
			metrics := libraryMetric.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)

				metricLines, err := serialization.SerializeMetric(e.settings.Logger, e.cfg.Prefix, metric, e.defaultDimensions, e.staticDimensions, e.prevPts)

				if err != nil {
					e.settings.Logger.Warn(
						"failed to serialize",
						zap.String("name", metric.Name()),
						zap.String("data-type", metric.Type().String()),
						zap.Error(err),
					)
				}

				if len(metricLines) > 0 {
					lines = append(lines, metricLines...)
				}
				e.settings.Logger.Debug(
					"Serialized metric data",
					zap.String("name", metric.Name()),
					zap.String("data-type", metric.Type().String()),
					zap.Int("data-len", len(metricLines)),
				)
			}
		}
	}

	return lines
}

var lastLog int64

// send sends a serialized metric batch to Dynatrace.
// An error indicates all lines were dropped regardless of the returned number.
func (e *metricsExporter) send(ctx context.Context, lines []string) error {
	e.settings.Logger.Debug("Exporting", zap.Int("lines", len(lines)))

	if now := time.Now().Unix(); len(lines) > apiconstants.GetPayloadLinesLimit() && now-lastLog > 60 {
		e.settings.Logger.Warn(
			fmt.Sprintf(
				"Batch too large. Sending in chunks of %[1]d metrics. If any chunk fails, previous chunks in the batch could be retried by the batch processor. Please set send_batch_max_size to %[1]d or less. Suppressing this log for 60 seconds.",
				apiconstants.GetPayloadLinesLimit(),
			),
		)
		lastLog = time.Now().Unix()
	}

	for i := 0; i < len(lines); i += apiconstants.GetPayloadLinesLimit() {
		end := i + apiconstants.GetPayloadLinesLimit()

		if end > len(lines) {
			end = len(lines)
		}

		err := e.sendBatch(ctx, lines[i:end])
		if err != nil {
			return err
		}
	}

	return nil
}

// send sends a serialized metric batch to Dynatrace.
// An error indicates all lines were dropped regardless of the returned number.
func (e *metricsExporter) sendBatch(ctx context.Context, lines []string) error {
	message := strings.Join(lines, "\n")
	e.settings.Logger.Debug(
		"sending a batch of metric lines",
		zap.Int("lines", len(lines)),
		zap.String("endpoint", e.cfg.Endpoint),
	)

	req, err := http.NewRequestWithContext(ctx, "POST", e.cfg.Endpoint, bytes.NewBufferString(message))

	if err != nil {
		return consumererror.NewPermanent(err)
	}

	resp, err := e.client.Do(req)

	if err != nil {
		e.settings.Logger.Error("failed to send request", zap.Error(err))
		return fmt.Errorf("sendBatch: %w", err)
	}

	defer resp.Body.Close()

	responseBody, rbUnmarshalErr := e.unmarshalResponseBody(resp)

	if resp.StatusCode == http.StatusRequestEntityTooLarge {
		// If a payload is too large, resending it will not help
		return consumererror.NewPermanent(fmt.Errorf("payload too large"))
	}

	if resp.StatusCode == http.StatusBadRequest {
		// At least some metrics were not accepted
		if rbUnmarshalErr != nil {
			return nil
		}

		e.settings.Logger.Warn(
			"Response from Dynatrace",
			zap.Int("accepted-lines", responseBody.Ok),
			zap.Int("rejected-lines", responseBody.Invalid),
			zap.String("error-message", responseBody.Error.Message),
			zap.String("status", resp.Status),
		)

		for _, line := range responseBody.Error.InvalidLines {
			// Enabled debug logging to see which lines were dropped
			if line.Line >= 0 && line.Line < len(lines) {
				e.settings.Logger.Debug(
					fmt.Sprintf("rejected line %3d: [%s] %s", line.Line, line.Error, lines[line.Line]),
				)
			}
		}

		return nil
	}

	if resp.StatusCode == http.StatusUnauthorized {
		// token is missing or wrong format
		e.isDisabled = true
		return consumererror.NewPermanent(fmt.Errorf("API token missing or invalid"))
	}

	if resp.StatusCode == http.StatusForbidden {
		return consumererror.NewPermanent(fmt.Errorf("API token missing the required scope (metrics.ingest)"))
	}

	if resp.StatusCode == http.StatusNotFound {
		return consumererror.NewPermanent(fmt.Errorf("metrics ingest v2 module not found - ensure module is enabled and endpoint is correct"))
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return consumererror.NewPermanent(
			fmt.Errorf("The server responded that too many requests have been sent. Please check your export interval and batch sizes and see https://www.dynatrace.com/support/help/dynatrace-api/basics/access-limit for more information"),
		)
	}

	if resp.StatusCode > http.StatusBadRequest { // '400 Bad Request' itself is handled above
		return consumererror.NewPermanent(fmt.Errorf(`Received error response status: "%v"`, resp.Status))
	}

	if rbUnmarshalErr == nil {
		e.settings.Logger.Debug(
			"Export successful. Response from Dynatrace:",
			zap.Int("accepted-lines", responseBody.Ok),
			zap.String("status", resp.Status),
		)
	}

	// No known errors
	return nil
}

// start starts the exporter
func (e *metricsExporter) start(_ context.Context, host component.Host) (err error) {
	client, err := e.cfg.HTTPClientSettings.ToClient(host, e.settings)
	if err != nil {
		e.settings.Logger.Error("Failed to construct HTTP client", zap.Error(err))
		return fmt.Errorf("start: %w", err)
	}

	e.client = client

	return nil
}

func (e *metricsExporter) unmarshalResponseBody(resp *http.Response) (metricsResponse, error) {
	bodyBytes, err := io.ReadAll(resp.Body)
	responseBody := metricsResponse{}
	if err != nil {
		// if the response cannot be read, do not retry the batch as it may have been successful
		e.settings.Logger.Error("Failed to read response from Dynatrace", zap.Error(err))
		return responseBody, fmt.Errorf("Failed to read response")
	}

	if err := json.Unmarshal(bodyBytes, &responseBody); err != nil {
		// if the response cannot be read, do not retry the batch as it may have been successful
		bodyStr := string(bodyBytes)
		bodyStr = truncateString(bodyStr, 1000)
		e.settings.Logger.Error("Failed to unmarshal response from Dynatrace", zap.Error(err), zap.String("body", bodyStr))
		return responseBody, fmt.Errorf("Failed to unmarshal response")
	}

	return responseBody, nil
}

func truncateString(str string, num int) string {
	truncated := str
	if len(str) > num {
		if num > 3 {
			num -= 3
		}
		truncated = str[0:num] + "..."
	}
	return truncated
}

// Response from Dynatrace is expected to be in JSON format
type metricsResponse struct {
	Ok      int                  `json:"linesOk"`
	Invalid int                  `json:"linesInvalid"`
	Error   metricsResponseError `json:"error"`
}

type metricsResponseError struct {
	Code         int                               `json:"code"`
	Message      string                            `json:"message"`
	InvalidLines []metricsResponseErrorInvalidLine `json:"invalidLines"`
}

type metricsResponseErrorInvalidLine struct {
	Line  int    `json:"line"`
	Error string `json:"error"`
}
