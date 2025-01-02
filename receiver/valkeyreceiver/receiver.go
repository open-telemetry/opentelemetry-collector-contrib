// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package valkeyreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/valkeyreceiver"

import (
	"context"
	"strconv"
	"time"

	"github.com/valkey-io/valkey-go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/valkeyreceiver/internal/metadata"
)

type valkeyScraper struct {
	client     client
	settings   component.TelemetrySettings
	cfg        *Config
	mb         *metadata.MetricsBuilder
	configInfo configInfo
}

func newValkeyScraper(cfg *Config, settings receiver.Settings) (*valkeyScraper, error) {
	configInfo, err := newConfigInfo(cfg)
	if err != nil {
		return nil, err
	}

	return &valkeyScraper{
		cfg:        cfg,
		settings:   settings.TelemetrySettings,
		mb:         metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		configInfo: configInfo,
	}, nil
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

	rb := vs.mb.NewResourceBuilder()
	rb.SetValkeyVersion(getValkeyVersion(result))
	rb.SetServerAddress(vs.configInfo.Address)
	rb.SetServerPort(vs.configInfo.Port)
	return vs.mb.Emit(metadata.WithResource(rb.Emit())), nil
}

// getValkeyVersion retrieves version string from 'redis_version' Valkey info key-value pairs
// e.g. "redis_version:5.0.7"
func getValkeyVersion(info map[string]string) string {
	if str, ok := info["redis_version"]; ok {
		return str
	}
	return "unknown"
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
