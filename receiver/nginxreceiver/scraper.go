// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nginxreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nginxreceiver"

import (
	"context"
	"net/http"
	"time"

	"github.com/nginxinc/nginx-prometheus-exporter/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nginxreceiver/internal/metadata"
)

type nginxScraper struct {
	httpClient *http.Client
	client     *client.NginxClient

	settings component.TelemetrySettings
	cfg      *Config
	mb       *metadata.MetricsBuilder
}

func newNginxScraper(
	settings receiver.CreateSettings,
	cfg *Config,
) *nginxScraper {
	var mb *metadata.MetricsBuilder
	if connectorsAsSumGate.IsEnabled() {
		mb = metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings, metadata.WithCurrentConnectionsAsGaugeDisabled())
	} else {
		mb = metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings, metadata.WithCurrentConnectionsAsGauge())
	}
	return &nginxScraper{
		settings: settings.TelemetrySettings,
		cfg:      cfg,
		mb:       mb,
	}
}

func (r *nginxScraper) start(_ context.Context, host component.Host) error {
	httpClient, err := r.cfg.ToClient(host, r.settings)
	if err != nil {
		return err
	}
	r.httpClient = httpClient

	return nil
}

func (r *nginxScraper) scrape(context.Context) (pmetric.Metrics, error) {
	// Init client in scrape method in case there are transient errors in the constructor.
	if r.client == nil {
		var err error
		r.client, err = client.NewNginxClient(r.httpClient, r.cfg.HTTPClientSettings.Endpoint)
		if err != nil {
			r.client = nil
			return pmetric.Metrics{}, err
		}
	}

	stats, err := r.client.GetStubStats()
	if err != nil {
		r.settings.Logger.Error("Failed to fetch nginx stats", zap.Error(err))
		return pmetric.Metrics{}, err
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	r.mb.RecordNginxRequestsDataPoint(now, stats.Requests)
	r.mb.RecordNginxConnectionsAcceptedDataPoint(now, stats.Connections.Accepted)
	r.mb.RecordNginxConnectionsHandledDataPoint(now, stats.Connections.Handled)

	if connectorsAsSumGate.IsEnabled() {
		r.mb.RecordNginxConnectionsCurrentDataPoint(now, stats.Connections.Active, metadata.AttributeStateActive)
		r.mb.RecordNginxConnectionsCurrentDataPoint(now, stats.Connections.Reading, metadata.AttributeStateReading)
		r.mb.RecordNginxConnectionsCurrentDataPoint(now, stats.Connections.Writing, metadata.AttributeStateWriting)
		r.mb.RecordNginxConnectionsCurrentDataPoint(now, stats.Connections.Waiting, metadata.AttributeStateWaiting)
	} else {
		r.mb.RecordTempConnectionsCurrentDataPoint(now, stats.Connections.Active, metadata.AttributeStateActive)
		r.mb.RecordTempConnectionsCurrentDataPoint(now, stats.Connections.Reading, metadata.AttributeStateReading)
		r.mb.RecordTempConnectionsCurrentDataPoint(now, stats.Connections.Writing, metadata.AttributeStateWriting)
		r.mb.RecordTempConnectionsCurrentDataPoint(now, stats.Connections.Waiting, metadata.AttributeStateWaiting)
	}

	return r.mb.Emit(), nil
}
