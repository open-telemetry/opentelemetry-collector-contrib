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
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
	"gopkg.in/zorkian/go-datadog-api.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/utils"
)

type metricsExporter struct {
	logger *zap.Logger
	cfg    *config.Config
	client *datadog.Client
}

func newMetricsExporter(params component.ExporterCreateParams, cfg *config.Config) (*metricsExporter, error) {
	client := utils.CreateClient(cfg.API.Key, cfg.Metrics.TCPAddr.Endpoint)
	client.ExtraHeader["User-Agent"] = utils.UserAgent(params.ApplicationStartInfo)
	client.HttpClient = utils.NewHTTPClient(10 * time.Second)

	utils.ValidateAPIKey(params.Logger, client)

	return &metricsExporter{params.Logger, cfg, client}, nil
}

func (exp *metricsExporter) processMetrics(ms []datadog.Metric) {
	addNamespace := exp.cfg.Metrics.Namespace != ""

	if addNamespace {
		metrics.AddNamespace(ms, exp.cfg.Metrics.Namespace)
	}

	metrics.AddHostname(ms, exp.logger, exp.cfg)
}

func (exp *metricsExporter) PushMetricsData(ctx context.Context, md pdata.Metrics) (int, error) {
	ms, droppedTimeSeries := MapMetrics(exp.logger, exp.cfg.Metrics, md)
	exp.processMetrics(ms)

	// The running metric is added after metrics are processed, as the running metric is already
	// processed in a special way in RunningMetric (eg. no namespace is added)
	pushTime := uint64(time.Now().UTC().UnixNano())
	ms = append(ms, metrics.RunningMetric("metrics", pushTime, exp.logger, exp.cfg)...)

	err := exp.client.PostMetrics(ms)
	return droppedTimeSeries, err
}
