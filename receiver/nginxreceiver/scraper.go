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
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nginxinc/nginx-prometheus-exporter/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nginxreceiver/internal/metadata"
)

type nginxScraper struct {
	httpClient *http.Client
	client     *client.NginxClient

	logger *zap.Logger
	cfg    *Config
}

func newNginxScraper(
	logger *zap.Logger,
	cfg *Config,
) *nginxScraper {
	return &nginxScraper{
		logger: logger,
		cfg:    cfg,
	}
}

func (r *nginxScraper) start(_ context.Context, host component.Host) error {
	httpClient, err := r.cfg.ToClient(host.GetExtensions())
	if err != nil {
		r.logger.Error("nginx", zap.Error(err), zap.String("status_code", codes.InvalidArgument.String()))
		return err
	}
	r.httpClient = httpClient

	return nil
}
func (r *nginxScraper) logGetErrors(err error) {
	if err != nil {
		switch {
		case strings.HasPrefix(err.Error(), "failed to get"):
			r.logger.Error("nginx", zap.Error(err), zap.String("status_code", codes.Unavailable.String()))
		case strings.HasPrefix(err.Error(), "expected"):
			r.logStatusCode(err)
		case strings.HasPrefix(err.Error(), "failed to read"):
			r.logger.Error("nginx", zap.Error(err), zap.String("status_code", codes.Internal.String()))
		case strings.HasPrefix(err.Error(), "failed to parse"):
			r.logger.Error("nginx", zap.Error(err), zap.String("status_code", codes.Internal.String()))
		default:
			r.logger.Error("nginx", zap.Error(err), zap.String("status_code", codes.Internal.String()))
		}
	}
}

func (r *nginxScraper) logStatusCode(err error) {
	if err != nil {
		switch err.Error() {
		case fmt.Sprintf("expected %v response, got %v", http.StatusOK, 400):
			r.logger.Error("nginx", zap.Error(err), zap.String("status_code", codes.Internal.String()))
		case fmt.Sprintf("expected %v response, got %v", http.StatusOK, 401):
			r.logger.Error("nginx", zap.Error(err), zap.String("status_code", codes.Unauthenticated.String()))
		case fmt.Sprintf("expected %v response, got %v", http.StatusOK, 403):
			r.logger.Error("nginx", zap.Error(err), zap.String("status_code", codes.PermissionDenied.String()))
		case fmt.Sprintf("expected %v response, got %v", http.StatusOK, 404):
			r.logger.Error("nginx", zap.Error(err), zap.String("status_code", codes.Unimplemented.String()))
		case fmt.Sprintf("expected %v response, got %v", http.StatusOK, 429):
			r.logger.Error("nginx", zap.Error(err), zap.String("status_code", codes.Unavailable.String()))
		case fmt.Sprintf("expected %v response, got %v", http.StatusOK, 502):
			r.logger.Error("nginx", zap.Error(err), zap.String("status_code", codes.Unavailable.String()))
		case fmt.Sprintf("expected %v response, got %v", http.StatusOK, 503):
			r.logger.Error("nginx", zap.Error(err), zap.String("status_code", codes.Unavailable.String()))
		case fmt.Sprintf("expected %v response, got %v", http.StatusOK, 504):
			r.logger.Error("nginx", zap.Error(err), zap.String("status_code", codes.Unavailable.String()))
		default:
			r.logger.Error("nginx", zap.Error(err), zap.String("status_code", codes.Unknown.String()))
		}
	}
}

func (r *nginxScraper) scrape(context.Context) (pdata.ResourceMetricsSlice, error) {
	// Init client in scrape method in case there are transient errors in the
	// constructor.
	if r.client == nil {
		var err error
		r.client, err = client.NewNginxClient(r.httpClient, r.cfg.HTTPClientSettings.Endpoint)
		if err != nil {
			r.logGetErrors(err)
			r.client = nil
			return pdata.ResourceMetricsSlice{}, err
		}
	}

	stats, err := r.client.GetStubStats()
	if err != nil {
		r.logGetErrors(err)
		r.logger.Error("Failed to fetch nginx stats", zap.Error(err))
		return pdata.ResourceMetricsSlice{}, err
	}

	now := pdata.TimestampFromTime(time.Now())
	metrics := pdata.NewMetrics()
	ilm := metrics.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty()
	ilm.InstrumentationLibrary().SetName("otelcol/nginx")

	addIntSum(ilm.Metrics(), metadata.M.NginxRequests.Init, now, stats.Requests)
	addIntSum(ilm.Metrics(), metadata.M.NginxConnectionsAccepted.Init, now, stats.Connections.Accepted)
	addIntSum(ilm.Metrics(), metadata.M.NginxConnectionsHandled.Init, now, stats.Connections.Handled)

	currConnMetric := ilm.Metrics().AppendEmpty()
	metadata.M.NginxConnectionsCurrent.Init(currConnMetric)
	dps := currConnMetric.Gauge().DataPoints()
	addCurrentConnectionDataPoint(dps, metadata.LabelState.Active, now, stats.Connections.Active)
	addCurrentConnectionDataPoint(dps, metadata.LabelState.Reading, now, stats.Connections.Reading)
	addCurrentConnectionDataPoint(dps, metadata.LabelState.Writing, now, stats.Connections.Writing)
	addCurrentConnectionDataPoint(dps, metadata.LabelState.Waiting, now, stats.Connections.Waiting)

	return metrics.ResourceMetrics(), nil
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
	dp.LabelsMap().Upsert(metadata.L.State, stateValue)
	dp.SetTimestamp(now)
	dp.SetIntVal(value)
}
