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

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter/serialization"
)

// NewExporter exports to a Dynatrace MINT API
func newMetricsExporter(params component.ExporterCreateParams, cfg *config.Config) (*exporter, error) {
	client, err := cfg.HTTPClientSettings.ToClient()
	if err != nil {
		return nil, err
	}
	return &exporter{params.Logger, cfg, client}, nil
}

// exporter forwards metrics to a Dynatrace agent
type exporter struct {
	logger *zap.Logger
	cfg    *config.Config
	client *http.Client
}

const (
	maxMetricKeyLen = 250
)

// Export given CheckpointSet
func (e *exporter) PushMetricsData(ctx context.Context, md pdata.Metrics) (droppedTimeSeries int, err error) {
	output := ""
	dropped := 0

	resourceMetrics := md.ResourceMetrics()

	for i := 0; i < resourceMetrics.Len(); i++ {
		resourceMetric := resourceMetrics.At(i)
		if resourceMetric.IsNil() {
			continue
		}
		libraryMetrics := resourceMetric.InstrumentationLibraryMetrics()
		for j := 0; j < libraryMetrics.Len(); j++ {
			libraryMetric := libraryMetrics.At(j)
			if libraryMetric.IsNil() {
				continue
			}
			metrics := libraryMetric.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				if metric.IsNil() {
					continue
				}

				name, normalizationError := normalizeMetricName(e.cfg.Prefix, metric.Name())
				if normalizationError != nil {
					dropped++
					e.logger.Error(fmt.Sprintf("Failed to normalize metric name: %s", metric.Name()))
					continue
				}

				e.logger.Debug("Exporting type " + metric.DataType().String())

				switch metric.DataType() {
				case pdata.MetricDataTypeNone:
					continue
				case pdata.MetricDataTypeIntGauge:
					output += serialization.SerializeIntDataPoints(name, metric.IntGauge().DataPoints(), e.cfg.Tags)
				case pdata.MetricDataTypeDoubleGauge:
					output += serialization.SerializeDoubleDataPoints(name, metric.DoubleGauge().DataPoints(), e.cfg.Tags)
				case pdata.MetricDataTypeIntSum:
					output += serialization.SerializeIntDataPoints(name, metric.IntSum().DataPoints(), e.cfg.Tags)
				case pdata.MetricDataTypeDoubleSum:
					output += serialization.SerializeDoubleDataPoints(name, metric.DoubleSum().DataPoints(), e.cfg.Tags)
				case pdata.MetricDataTypeIntHistogram:
					output += serialization.SerializeIntHistogramMetrics(name, metric.IntHistogram().DataPoints(), e.cfg.Tags)
				case pdata.MetricDataTypeDoubleHistogram:
					output += serialization.SerializeDoubleHistogramMetrics(name, metric.DoubleHistogram().DataPoints(), e.cfg.Tags)
				}
			}
		}
	}

	if output != "" {
		err = e.send(output)
		if err != nil {
			return 0, fmt.Errorf("error processing data:, %s", err.Error())
		}
	}

	return dropped, nil
}

func (e *exporter) send(message string) error {
	e.logger.Debug("Sending lines to Dynatrace\n" + message)
	req, err := http.NewRequest("POST", e.cfg.Endpoint, bytes.NewBufferString(message))
	if err != nil {
		return fmt.Errorf("dynatrace error while creating HTTP request: %s", err.Error())
	}

	req.Header.Add("Content-Type", "text/plain; charset=UTF-8")
	req.Header.Add("Authorization", "Api-Token "+e.cfg.APIToken)
	req.Header.Add("User-Agent", "opentelemetry-collector")

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending HTTP request: %s", err.Error())
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error while receiving HTTP response: %s", err.Error())
	}

	responseBody := metricsResponse{}
	if err := json.Unmarshal(bodyBytes, &responseBody); err != nil {
		e.logger.Error(fmt.Sprintf("failed to unmarshal response: %s", err.Error()))
	} else {
		e.logger.Debug(fmt.Sprintf("Exported %d lines to Dynatrace", responseBody.Ok))

		if responseBody.Invalid > 0 {
			e.logger.Debug(fmt.Sprintf("Failed to export %d lines to Dynatrace", responseBody.Invalid))
		}

		if responseBody.Error != "" {
			e.logger.Error(fmt.Sprintf("Error from Dynatrace: %s", responseBody.Error))
		}
	}

	if !(resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted) {
		return fmt.Errorf("request failed with response code:, %d", resp.StatusCode)
	}

	return nil

}

// normalizeMetricName formats the custom namespace and view name to
// Metric naming Conventions
func normalizeMetricName(prefix, name string) (string, error) {
	name, err := serialization.NormalizeString(name, maxMetricKeyLen)

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
	Ok      int64  `json:"linesOk"`
	Invalid int64  `json:"linesInvalid"`
	Error   string `json:"error"`
}
