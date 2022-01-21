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

package nginxreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nginxreceiver"

import (
	"context"
	"net/http"
	"time"

	"github.com/nginxinc/nginx-prometheus-exporter/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nginxreceiver/internal/metadata"
)

type nginxScraper struct {
	httpClient *http.Client
	client     *client.NginxClient

	settings component.TelemetrySettings
	cfg      *Config
}

func newNginxScraper(
	settings component.TelemetrySettings,
	cfg *Config,
) *nginxScraper {
	return &nginxScraper{
		settings: settings,
		cfg:      cfg,
	}
}

func (r *nginxScraper) start(_ context.Context, host component.Host) error {
	httpClient, err := r.cfg.ToClient(host.GetExtensions(), r.settings)
	if err != nil {
		return err
	}
	r.httpClient = httpClient

	return nil
}

func (r *nginxScraper) scrape(context.Context) (pdata.Metrics, error) {
	// Init client in scrape method in case there are transient errors in the
	// constructor.
	if r.client == nil {
		var err error
		r.client, err = client.NewNginxClient(r.httpClient, r.cfg.HTTPClientSettings.Endpoint)
		if err != nil {
			r.client = nil
			return pdata.Metrics{}, err
		}
	}

	stats, err := r.client.GetStubStats()
	if err != nil {
		r.settings.Logger.Error("Failed to fetch nginx stats", zap.Error(err))
		return pdata.Metrics{}, err
	}

	now := pdata.NewTimestampFromTime(time.Now())
	md := pdata.NewMetrics()
	ilm := md.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty()
	ilm.InstrumentationLibrary().SetName("otelcol/nginx")

	addIntSum(ilm.Metrics(), metadata.M.NginxRequests.Init, now, stats.Requests)
	addIntSum(ilm.Metrics(), metadata.M.NginxConnectionsAccepted.Init, now, stats.Connections.Accepted)
	addIntSum(ilm.Metrics(), metadata.M.NginxConnectionsHandled.Init, now, stats.Connections.Handled)

	currConnMetric := ilm.Metrics().AppendEmpty()
	metadata.M.NginxConnectionsCurrent.Init(currConnMetric)
	dps := currConnMetric.Gauge().DataPoints()
	addCurrentConnectionDataPoint(dps, metadata.AttributeState.Active, now, stats.Connections.Active)
	addCurrentConnectionDataPoint(dps, metadata.AttributeState.Reading, now, stats.Connections.Reading)
	addCurrentConnectionDataPoint(dps, metadata.AttributeState.Writing, now, stats.Connections.Writing)
	addCurrentConnectionDataPoint(dps, metadata.AttributeState.Waiting, now, stats.Connections.Waiting)

	return md, nil
}

func addIntSum(metrics pdata.MetricSlice, initFunc func(pdata.Metric), now pdata.Timestamp, value int64) {
	metric := metrics.AppendEmpty()
	initFunc(metric)
	dp := metric.Sum().DataPoints().AppendEmpty()
	dp.SetTimestamp(now)
	dp.SetIntVal(value)
}

func addCurrentConnectionDataPoint(dps pdata.NumberDataPointSlice, stateValue string, now pdata.Timestamp, value int64) {
	dp := dps.AppendEmpty()
	dp.Attributes().UpsertString(metadata.A.State, stateValue)
	dp.SetTimestamp(now)
	dp.SetIntVal(value)
}
