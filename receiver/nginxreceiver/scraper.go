// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nginxreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nginxreceiver"

import (
	"context"
	"time"

	"github.com/nginx/nginx-prometheus-exporter/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nginxreceiver/internal/metadata"
)

type nginxScraper struct {
	client   *client.NginxClient
	settings component.TelemetrySettings
	cfg      *Config
	mb       *metadata.MetricsBuilder
}

func newNginxScraper(
	settings receiver.Settings,
	cfg *Config,
) *nginxScraper {
	mb := metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings)
	return &nginxScraper{
		settings: settings.TelemetrySettings,
		cfg:      cfg,
		mb:       mb,
	}
}

func (r *nginxScraper) start(ctx context.Context, host component.Host) error {
	httpClient, err := r.cfg.ToClient(ctx, host, r.settings)
	if err != nil {
		return err
	}
	r.client = client.NewNginxClient(httpClient, r.cfg.Endpoint)
	return nil
}

func (r *nginxScraper) scrape(context.Context) (pmetric.Metrics, error) {
	stats, err := r.client.GetStubStats()
	if err != nil {
		r.settings.Logger.Error("Failed to fetch nginx stats", zap.Error(err))
		return pmetric.Metrics{}, err
	}

	now := pcommon.NewTimestampFromTime(time.Now())
	r.mb.RecordNginxRequestsDataPoint(now, stats.Requests)
	r.mb.RecordNginxConnectionsAcceptedDataPoint(now, stats.Connections.Accepted)
	r.mb.RecordNginxConnectionsHandledDataPoint(now, stats.Connections.Handled)
	r.mb.RecordNginxConnectionsCurrentDataPoint(now, stats.Connections.Active, metadata.AttributeStateActive)
	r.mb.RecordNginxConnectionsCurrentDataPoint(now, stats.Connections.Reading, metadata.AttributeStateReading)
	r.mb.RecordNginxConnectionsCurrentDataPoint(now, stats.Connections.Writing, metadata.AttributeStateWriting)
	r.mb.RecordNginxConnectionsCurrentDataPoint(now, stats.Connections.Waiting, metadata.AttributeStateWaiting)
	return r.mb.Emit(), nil
}
