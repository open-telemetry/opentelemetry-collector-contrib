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

	"github.com/DataDog/datadog-go/statsd"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
)

type dogStatsDExporter struct {
	logger *zap.Logger
	cfg    *Config
	client *statsd.Client
}

func newDogStatsDExporter(logger *zap.Logger, cfg *Config) (*dogStatsDExporter, error) {

	options := []statsd.Option{
		statsd.WithNamespace(cfg.Metrics.Namespace),
		statsd.WithTags(cfg.TagsConfig.GetTags(true)), // true: add host tag
	}

	if !cfg.Metrics.DogStatsD.Telemetry {
		options = append(options, statsd.WithoutTelemetry())
	}

	client, err := statsd.New(
		cfg.Metrics.DogStatsD.Endpoint,
		options...,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to initialize DogStatsD client: %s", err)
	}

	return &dogStatsDExporter{logger, cfg, client}, nil
}

func (exp *dogStatsDExporter) PushMetricsData(_ context.Context, md pdata.Metrics) (int, error) {
	data := internaldata.MetricsToOC(md)
	series, droppedTimeSeries := MapMetrics(exp, data)

	for _, metric := range series.metrics {

		tags := metric.Tags

		// Send the hostname if it has not been overridden
		if exp.GetConfig().Hostname == "" && metric.GetHost() != "" {
			tags = append(tags, fmt.Sprintf("host:%s", metric.GetHost()))
		}

		var err error
		switch metric.GetType() {
		case Gauge:
			err = exp.client.Gauge(metric.GetMetric(), *metric.Points[0][1], tags, 1)
		}

		if err != nil {
			exp.GetLogger().Warn("could not send metric to statsd", zap.String("metric", *metric.Metric), zap.Error(err))
		}
	}

	return droppedTimeSeries, nil
}

func (exp *dogStatsDExporter) GetLogger() *zap.Logger {
	return exp.logger
}

func (exp *dogStatsDExporter) GetConfig() *Config {
	return exp.cfg
}

func (exp *dogStatsDExporter) GetQueueSettings() exporterhelper.QueueSettings {
	return exporterhelper.QueueSettings{Enabled: false}
}

func (exp *dogStatsDExporter) GetRetrySettings() exporterhelper.RetrySettings {
	return exporterhelper.RetrySettings{Enabled: false}
}
