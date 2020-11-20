// Copyright 2020, OpenTelemetry Authors
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

package nginxreceiver

import (
	"context"
	"time"

	"github.com/nginxinc/nginx-prometheus-exporter/client"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/consumer/simple"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nginxreceiver/internal/metadata"
)

type nginxScraper struct {
	client *client.NginxClient

	logger *zap.Logger
	config *config
}

func newNginxScraper(
	logger *zap.Logger,
	config *config,
) *nginxScraper {
	return &nginxScraper{
		logger: logger,
		config: config,
	}
}

func (r *nginxScraper) scrape(ctx context.Context) (pdata.ResourceMetricsSlice, error) {
	// Init client in scrape method in case there are transient errors in the
	// constructor.
	if r.client == nil {
		httpClient, err := r.config.ToClient()
		if err != nil {
			return pdata.ResourceMetricsSlice{}, err
		}

		r.client, err = client.NewNginxClient(httpClient, r.config.HTTPClientSettings.Endpoint)
		if err != nil {
			r.client = nil
			return pdata.ResourceMetricsSlice{}, err
		}
	}

	metrics := simple.Metrics{
		Metrics:                    pdata.NewMetrics(),
		Timestamp:                  time.Now(),
		MetricFactoriesByName:      metadata.M.FactoriesByName(),
		InstrumentationLibraryName: "otelcol/nginx",
	}

	stats, err := r.client.GetStubStats()
	if err != nil {
		r.logger.Error("Failed to fetch nginx stats", zap.Error(err))
		return pdata.ResourceMetricsSlice{}, err
	}

	metrics.AddSumDataPoint(metadata.M.NginxRequests.Name(), stats.Requests)
	metrics.AddGaugeDataPoint(metadata.M.NginxConnectionsActive.Name(), stats.Connections.Active)
	metrics.AddSumDataPoint(metadata.M.NginxConnectionsAccepted.Name(), stats.Connections.Accepted)
	metrics.AddSumDataPoint(metadata.M.NginxConnectionsHandled.Name(), stats.Connections.Handled)
	metrics.AddGaugeDataPoint(metadata.M.NginxConnectionsReading.Name(), stats.Connections.Reading)
	metrics.AddGaugeDataPoint(metadata.M.NginxConnectionsWriting.Name(), stats.Connections.Writing)
	metrics.AddGaugeDataPoint(metadata.M.NginxConnectionsWaiting.Name(), stats.Connections.Waiting)

	return metrics.Metrics.ResourceMetrics(), nil
}
