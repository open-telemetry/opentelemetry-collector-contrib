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
	"fmt"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
	"gopkg.in/zorkian/go-datadog-api.v2"
)

type metricsAPIExporter struct {
	logger *zap.Logger
	cfg    *Config
	client *datadog.Client
	tags   []string
}

func newMetricsAPIExporter(logger *zap.Logger, cfg *Config) (*metricsAPIExporter, error) {
	client := datadog.NewClient(cfg.API.Key, "")
	client.SetBaseUrl(cfg.Metrics.Agentless.TCPAddr.Endpoint)

	if ok, err := client.Validate(); err != nil {
		logger.Warn("Error when validating API key", zap.Error(err))
	} else if ok {
		logger.Info("Provided Datadog API key is valid")
	} else {
		return nil, fmt.Errorf("provided Datadog API key is invalid: %s", cfg.API.GetCensoredKey())
	}

	// Calculate tags at startup
	tags := cfg.TagsConfig.GetTags(false)

	return &metricsAPIExporter{logger, cfg, client, tags}, nil

}

func (exp *metricsAPIExporter) PushMetricsData(ctx context.Context, md pdata.Metrics) (int, error) {
	data := internaldata.MetricsToOC(md)
	series, droppedTimeSeries := MapMetrics(exp, data)

	addNamespace := exp.GetConfig().Metrics.Namespace != ""
	overrideHostname := exp.GetConfig().Hostname != ""
	addTags := len(exp.tags) > 0

	for i := range series.metrics {
		if addNamespace {
			newName := exp.GetConfig().Metrics.Namespace + *series.metrics[i].Metric
			series.metrics[i].Metric = &newName
		}

		if overrideHostname || series.metrics[i].GetHost() == "" {
			series.metrics[i].Host = GetHost(exp.GetConfig())
		}

		if addTags {
			series.metrics[i].Tags = append(series.metrics[i].Tags, exp.tags...)
		}

	}

	err := exp.client.PostMetrics(series.metrics)
	return droppedTimeSeries, err
}

func (exp *metricsAPIExporter) GetLogger() *zap.Logger {
	return exp.logger
}

func (exp *metricsAPIExporter) GetConfig() *Config {
	return exp.cfg
}

func (exp *metricsAPIExporter) GetQueueSettings() exporterhelper.QueueSettings {
	return exporterhelper.CreateDefaultQueueSettings()
}

func (exp *metricsAPIExporter) GetRetrySettings() exporterhelper.RetrySettings {
	return exporterhelper.CreateDefaultRetrySettings()
}
