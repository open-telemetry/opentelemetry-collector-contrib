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

package datadogexporter

import (
	"context"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
	"gopkg.in/zorkian/go-datadog-api.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/metadata"
)

type metricsExporter struct {
	logger *zap.Logger
	cfg    *config.Config
	client *datadog.Client
	tags   []string
}

func validateAPIKey(logger *zap.Logger, client *datadog.Client) {
	logger.Info("Validating API key.")
	res, err := client.Validate()
	if err != nil {
		logger.Warn("Error while validating API key.", zap.Error(err))
	}

	if res {
		logger.Info("API key validation successful.")
	} else {
		logger.Warn("API key validation failed.")
	}
}

func newMetricsExporter(logger *zap.Logger, cfg *config.Config) (*metricsExporter, error) {
	client := datadog.NewClient(cfg.API.Key, "")
	client.SetBaseUrl(cfg.Metrics.TCPAddr.Endpoint)

	validateAPIKey(logger, client)

	// Calculate tags at startup
	tags := cfg.TagsConfig.GetTags(false)

	return &metricsExporter{logger, cfg, client, tags}, nil
}

func (exp *metricsExporter) processMetrics(metrics []datadog.Metric) {
	addNamespace := exp.cfg.Metrics.Namespace != ""
	overrideHostname := exp.cfg.Hostname != ""
	addTags := len(exp.tags) > 0

	for i := range metrics {
		if addNamespace {
			newName := exp.cfg.Metrics.Namespace + *metrics[i].Metric
			metrics[i].Metric = &newName
		}

		if overrideHostname || metrics[i].GetHost() == "" {
			metrics[i].Host = metadata.GetHost(exp.logger, exp.cfg)
		}

		if addTags {
			metrics[i].Tags = append(metrics[i].Tags, exp.tags...)
		}

	}
}

func (exp *metricsExporter) PushMetricsData(ctx context.Context, md pdata.Metrics) (int, error) {
	metrics, droppedTimeSeries := MapMetrics(exp.logger, exp.cfg.Metrics, md)
	exp.processMetrics(metrics)

	err := exp.client.PostMetrics(metrics)
	return droppedTimeSeries, err
}
