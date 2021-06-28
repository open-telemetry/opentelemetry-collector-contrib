// Copyright The OpenTelemetry Authors
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

package dynatraceexporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter/serialization"
)

// The maximum number of metrics that may be sent in a single request to the Dynatrace API
const maxChunkSize = 1000

// NewExporter exports to a Dynatrace Metrics v2 API
func newMetricsExporter(params component.ExporterCreateSettings, cfg *config.Config) *exporter {
	return &exporter{
		logger: params.Logger,
		cfg:    cfg,
	}
}

// exporter forwards metrics to a Dynatrace agent
type exporter struct {
	logger     *zap.Logger
	cfg        *config.Config
	client     *http.Client
	isDisabled bool
}

const (
	maxMetricKeyLen = 250
)

func (e *exporter) PushMetricsData(ctx context.Context, md pdata.Metrics) error {
	if e.isDisabled {
		return nil
	}

	lines, _ := e.serializeMetrics(md)

	// If request is empty string, there are no serializable metrics in the batch.
	// This can happen if all metric names are invalid
	if len(lines) == 0 {
		return nil
	}

	_, err := e.send(ctx, lines)

	if err != nil {
		return err
	}

	return nil
}

func (e *exporter) serializeMetrics(md pdata.Metrics) ([]string, int) {
	lines := make([]string, 0)
	dropped := 0

	resourceMetrics := md.ResourceMetrics()

	e.logger.Debug(fmt.Sprintf("res metric len: %d, e.cfg.Tags: %v\n", resourceMetrics.Len(), e.cfg.Tags))

	for i := 0; i < resourceMetrics.Len(); i++ {
		resourceMetric := resourceMetrics.At(i)
		libraryMetrics := resourceMetric.InstrumentationLibraryMetrics()
		for j := 0; j < libraryMetrics.Len(); j++ {
			libraryMetric := libraryMetrics.At(j)
			metrics := libraryMetric.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				name, normalizationError := normalizeMetricName(e.cfg.Prefix, metric.Name())
				if normalizationError != nil {
					dropped++
					e.logger.Error(fmt.Sprintf("Failed to normalize metric name: %s", metric.Name()))
					continue
				}

				var l []string
				switch metric.DataType() {
				case pdata.MetricDataTypeNone:
					continue
				case pdata.MetricDataTypeIntGauge:
					l = serialization.SerializeIntDataPoints(name, metric.IntGauge().DataPoints(), e.cfg.Tags)
				case pdata.MetricDataTypeDoubleGauge:
					l = serialization.SerializeDoubleDataPoints(name, metric.DoubleGauge().DataPoints(), e.cfg.Tags)
				case pdata.MetricDataTypeIntSum:
					l = serialization.SerializeIntDataPoints(name, metric.IntSum().DataPoints(), e.cfg.Tags)
				case pdata.MetricDataTypeDoubleSum:
					l = serialization.SerializeDoubleDataPoints(name, metric.DoubleSum().DataPoints(), e.cfg.Tags)
				case pdata.MetricDataTypeIntHistogram:
					l = serialization.SerializeIntHistogramMetrics(name, metric.IntHistogram().DataPoints(), e.cfg.Tags)
				case pdata.MetricDataTypeHistogram:
					l = serialization.SerializeHistogramMetrics(name, metric.Histogram().DataPoints(), e.cfg.Tags)
				}
				lines = append(lines, l...)
				e.logger.Debug(fmt.Sprintf("Exporting type %s, Name: %s, len: %d ", metric.DataType().String(), name, len(l)))
			}
		}
	}

	return lines, dropped
}

var lastLog int64

// send sends a serialized metric batch to Dynatrace.
// Returns the number of lines rejected by Dynatrace.
// An error indicates all lines were dropped regardless of the returned number.
func (e *exporter) send(ctx context.Context, lines []string) (int, error) {
	if now := time.Now().Unix(); len(lines) > maxChunkSize && now-lastLog > 60 {
		e.logger.Warn(fmt.Sprintf("Batch too large. Sending in chunks of %[1]d metrics. If any chunk fails, previous chunks in the batch could be retried by the batch processor. Please set send_batch_max_size to %[1]d or less. Suppressing this log for 60 seconds.", maxChunkSize))
		lastLog = time.Now().Unix()
	}

	rejected := 0
	for i := 0; i < len(lines); i += maxChunkSize {
		end := i + maxChunkSize

		if end > len(lines) {
			end = len(lines)
		}

		batchRejected, err := e.sendBatch(ctx, lines[i:end])
		rejected += batchRejected
		if err != nil {
			return rejected, err
		}
	}

	return rejected, nil
}

// send sends a serialized metric batch to Dynatrace.
// Returns the number of lines rejected by Dynatrace.
// An error indicates all lines were dropped regardless of the returned number.
func (e *exporter) sendBatch(ctx context.Context, lines []string) (int, error) {
	message := strings.Join(lines, "\n")
	e.logger.Debug(fmt.Sprintf("Sending lines to Dynatrace: %d", len(lines)))

	req, err := http.NewRequestWithContext(ctx, "POST", e.cfg.Endpoint, bytes.NewBufferString(message))
	if err != nil {
		return 0, consumererror.Permanent(err)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("error sending HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusRequestEntityTooLarge {
		// If a payload is too large, resending it will not help
		return 0, consumererror.Permanent(fmt.Errorf("payload too large"))
	}

	if resp.StatusCode == http.StatusBadRequest {
		// At least some metrics were not accepted
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			// if the response cannot be read, do not retry the batch as it may have been successful
			e.logger.Error(fmt.Sprintf("failed to read response: %s", err.Error()))
			return 0, nil
		}

		responseBody := metricsResponse{}
		if err := json.Unmarshal(bodyBytes, &responseBody); err != nil {
			// if the response cannot be read, do not retry the batch as it may have been successful
			e.logger.Error(fmt.Sprintf("failed to unmarshal response: %s", err.Error()))
			return 0, nil
		}

		e.logger.Debug(fmt.Sprintf("Accepted %d lines", responseBody.Ok))
		e.logger.Error(fmt.Sprintf("Rejected %d lines", responseBody.Invalid))

		if responseBody.Error.Message != "" {
			e.logger.Error(fmt.Sprintf("Error from Dynatrace: %s", responseBody.Error.Message))
		}

		for _, line := range responseBody.Error.InvalidLines {
			// Enabled debug logging to see which lines were dropped
			if line.Line >= 0 && line.Line < len(lines) {
				e.logger.Debug(fmt.Sprintf("rejected line %3d: [%s] %s", line.Line, line.Error, lines[line.Line]))
			}
		}

		return responseBody.Invalid, nil
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		// Unauthorized and Unauthenticated errors are permanent
		e.isDisabled = true
		return 0, consumererror.Permanent(fmt.Errorf(resp.Status))
	}

	if resp.StatusCode == http.StatusNotFound {
		e.isDisabled = true
		return 0, consumererror.Permanent(fmt.Errorf("dynatrace metrics ingest module is disabled"))
	}

	// No known errors
	return 0, nil
}

// start starts the exporter
func (e *exporter) start(_ context.Context, host component.Host) (err error) {
	client, err := e.cfg.HTTPClientSettings.ToClient(host.GetExtensions())
	if err != nil {
		return err
	}

	e.client = client

	return nil
}

// normalizeMetricName formats the custom namespace and view name to
// Metric naming Conventions
func normalizeMetricName(prefix, name string) (string, error) {
	normalizedLen := maxMetricKeyLen
	if l := len(prefix); l != 0 {
		normalizedLen -= l + 1
	}

	name, err := serialization.NormalizeString(name, normalizedLen)
	if err != nil {
		return "", err
	}

	if prefix != "" {
		name = prefix + "." + name
	}

	return name, nil
}

// Response from Dynatrace is expected to be in JSON format
type metricsResponse struct {
	Ok      int                  `json:"linesOk"`
	Invalid int                  `json:"linesInvalid"`
	Error   metricsResponseError `json:"error"`
}

type metricsResponseError struct {
	Code         string                            `json:"code"`
	Message      string                            `json:"message"`
	InvalidLines []metricsResponseErrorInvalidLine `json:"invalidLines"`
}

type metricsResponseErrorInvalidLine struct {
	Line  int    `json:"line"`
	Error string `json:"error"`
}
