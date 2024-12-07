package valkeyreceiver

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/valkeyreceiver/internal/metadata"
	"github.com/valkey-io/valkey-go"
)

type valkeyScraper struct {
	client     client
	settings   component.TelemetrySettings
	cfg        *Config
	mb         *metadata.MetricsBuilder
	configInfo configInfo
}

func newValkeyScraper(cfg *Config, settings receiver.Settings) (scraper.Metrics, error) {
	configInfo, err := newConfigInfo(cfg)
	if err != nil {
		return nil, err
	}

	vs := &valkeyScraper{
		cfg:        cfg,
		settings:   settings.TelemetrySettings,
		mb:         metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		configInfo: configInfo,
	}
	return scraper.NewMetrics(
		scraper.ScrapeMetricsFunc(vs.scrape),

		scraper.WithShutdown(vs.shutdown),
	)
}

func (vs *valkeyScraper) shutdown(context.Context) error {
	if vs.client != nil {
		return vs.client.close()
	}
	return nil
}

func (vs *valkeyScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	if vs.client == nil {
		opts := valkey.ClientOption{
			InitAddress: []string{vs.cfg.Endpoint},
			Username:    vs.cfg.Username,
			Password:    string(vs.cfg.Password),
		}

		var err error
		if opts.TLSConfig, err = vs.cfg.TLS.LoadTLSConfig(context.Background()); err != nil {
			return pmetric.Metrics{}, err
		}
		vs.client, err = newValkeyClient(opts)
		if err != nil {
			return pmetric.Metrics{}, err
		}
	}

	result, err := vs.client.retrieveInfo(ctx)
	if err != nil {
		return pmetric.Metrics{}, err
	}

	// connected_clients
	now := pcommon.NewTimestampFromTime(time.Now())
	vs.recordConnectionMetrics(now, result)

	fmt.Printf("%#v", result)
	return vs.mb.Emit(), nil
}

func (vs *valkeyScraper) recordConnectionMetrics(now pcommon.Timestamp, info map[string]string) {
	recordConnection := func(infoKey string, attribute metadata.AttributeValkeyClientConnectionState) {
		if val, ok := info[infoKey]; ok {
			if i, err := strconv.ParseInt(val, 10, 64); err == nil {
				vs.mb.RecordValkeyClientConnectionCountDataPoint(now, i, attribute)
			}
		}
	}

	recordConnection("connected_clients", metadata.AttributeValkeyClientConnectionStateUsed)
	recordConnection("blocked_clients", metadata.AttributeValkeyClientConnectionStateBlocked)
	recordConnection("tracking_clients", metadata.AttributeValkeyClientConnectionStateTracking)
}
