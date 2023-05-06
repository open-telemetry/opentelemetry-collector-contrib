// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
