// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package purefbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefbreceiver"

import (
	"context"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefbreceiver/internal"
)

var _ receiver.Metrics = (*purefbMetricsReceiver)(nil)

type purefbMetricsReceiver struct {
	cfg  *Config
	set  receiver.CreateSettings
	next consumer.Metrics

	wrapped receiver.Metrics
}

func newReceiver(cfg *Config, set receiver.CreateSettings, next consumer.Metrics) *purefbMetricsReceiver {
	return &purefbMetricsReceiver{
		cfg:  cfg,
		set:  set,
		next: next,
	}
}

func (r *purefbMetricsReceiver) Start(ctx context.Context, compHost component.Host) error {
	fact := prometheusreceiver.NewFactory()
	scrapeCfgs := []*config.ScrapeConfig{}

	commomLabel := model.LabelSet{
		"env":           model.LabelValue(r.cfg.Env),
		"fb_array_name": model.LabelValue(r.cfg.Endpoint),
		"host":          model.LabelValue(r.cfg.Endpoint),
	}

	arrScraper := internal.NewScraper(ctx, internal.ScraperTypeArray, r.cfg.Endpoint, r.cfg.Arrays, r.cfg.Settings.ReloadIntervals.Array, commomLabel)
	if scCfgs, err := arrScraper.ToPrometheusReceiverConfig(compHost, fact); err == nil {
		scrapeCfgs = append(scrapeCfgs, scCfgs...)
	} else {
		return err
	}

	clientsScraper := internal.NewScraper(ctx, internal.ScraperTypeClients, r.cfg.Endpoint, r.cfg.Clients, r.cfg.Settings.ReloadIntervals.Clients, commomLabel)
	if scCfgs, err := clientsScraper.ToPrometheusReceiverConfig(compHost, fact); err == nil {
		scrapeCfgs = append(scrapeCfgs, scCfgs...)
	} else {
		return err
	}

	usageScraper := internal.NewScraper(ctx, internal.ScraperTypeUsage, r.cfg.Endpoint, r.cfg.Usage, r.cfg.Settings.ReloadIntervals.Usage, commomLabel)
	if scCfgs, err := usageScraper.ToPrometheusReceiverConfig(compHost, fact); err == nil {
		scrapeCfgs = append(scrapeCfgs, scCfgs...)
	} else {
		return err
	}
	promRecvCfg := fact.CreateDefaultConfig().(*prometheusreceiver.Config)
	promRecvCfg.PrometheusConfig = &config.Config{ScrapeConfigs: scrapeCfgs}

	wrapped, err := fact.CreateMetricsReceiver(ctx, r.set, promRecvCfg, r.next)
	if err != nil {
		return err
	}
	r.wrapped = wrapped

	err = r.wrapped.Start(ctx, compHost)
	if err != nil {
		return err
	}

	return nil
}

func (r *purefbMetricsReceiver) Shutdown(ctx context.Context) error {
	if r.wrapped != nil {
		return r.wrapped.Shutdown(ctx)
	}
	return nil
}
