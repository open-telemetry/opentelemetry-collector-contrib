// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package purefareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver"

import (
	"context"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver/internal"
)

var _ receiver.Metrics = (*purefaReceiver)(nil)

type purefaReceiver struct {
	cfg  *Config
	set  receiver.CreateSettings
	next consumer.Metrics

	wrapped receiver.Metrics
}

func newReceiver(cfg *Config, set receiver.CreateSettings, next consumer.Metrics) *purefaReceiver {
	return &purefaReceiver{
		cfg:  cfg,
		set:  set,
		next: next,
	}
}

func (r *purefaReceiver) Start(ctx context.Context, compHost component.Host) error {
	fact := prometheusreceiver.NewFactory()
	scrapeCfgs := []*config.ScrapeConfig{}

	commomLabel := model.LabelSet{
		"environment":   model.LabelValue(r.cfg.Env),
		"host":          model.LabelValue(r.cfg.Fa_array_name),
		"fa_array_name": model.LabelValue(r.cfg.Fa_array_name),
	}

	// Extracting environment & fa_array_name from commonLabel
	deploymentEnv := commomLabel["environment"]
	faarrayname := commomLabel["fa_array_name"]

	labelSet := model.LabelSet{
		"environment":   deploymentEnv,
		"fa_array_name": faarrayname,
	}

	arrScraper := internal.NewScraper(ctx, internal.ScraperTypeArray, r.cfg.Endpoint, r.cfg.Array, r.cfg.Settings.ReloadIntervals.Array, commomLabel)
	if scCfgs, err := arrScraper.ToPrometheusReceiverConfig(compHost, fact); err == nil {
		scrapeCfgs = append(scrapeCfgs, scCfgs...)
	} else {
		return err
	}

	hostScraper := internal.NewScraper(ctx, internal.ScraperTypeHosts, r.cfg.Endpoint, r.cfg.Hosts, r.cfg.Settings.ReloadIntervals.Hosts, labelSet)
	if scCfgs, err := hostScraper.ToPrometheusReceiverConfig(compHost, fact); err == nil {
		scrapeCfgs = append(scrapeCfgs, scCfgs...)
	} else {
		return err
	}

	directoriesScraper := internal.NewScraper(ctx, internal.ScraperTypeDirectories, r.cfg.Endpoint, r.cfg.Directories, r.cfg.Settings.ReloadIntervals.Directories, commomLabel)
	if scCfgs, err := directoriesScraper.ToPrometheusReceiverConfig(compHost, fact); err == nil {
		scrapeCfgs = append(scrapeCfgs, scCfgs...)
	} else {
		return err
	}

	podsScraper := internal.NewScraper(ctx, internal.ScraperTypePods, r.cfg.Endpoint, r.cfg.Pods, r.cfg.Settings.ReloadIntervals.Pods, commomLabel)
	if scCfgs, err := podsScraper.ToPrometheusReceiverConfig(compHost, fact); err == nil {
		scrapeCfgs = append(scrapeCfgs, scCfgs...)
	} else {
		return err
	}

	volumesScraper := internal.NewScraper(ctx, internal.ScraperTypeVolumes, r.cfg.Endpoint, r.cfg.Volumes, r.cfg.Settings.ReloadIntervals.Volumes, labelSet)
	if scCfgs, err := volumesScraper.ToPrometheusReceiverConfig(compHost, fact); err == nil {
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

func (r *purefaReceiver) Shutdown(ctx context.Context) error {
	if r.wrapped != nil {
		return r.wrapped.Shutdown(ctx)
	}
	return nil
}
