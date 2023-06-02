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
		"deployment.environment": model.LabelValue(r.cfg.Env),
		"host.name":              model.LabelValue(r.cfg.Endpoint),
	}

	arrScraper := internal.NewScraper(ctx, internal.ScraperTypeArray, r.cfg.Endpoint, r.cfg.Array, r.cfg.Settings.ReloadIntervals.Array, commomLabel)
	if scCfgs, err := arrScraper.ToPrometheusReceiverConfig(compHost, fact); err == nil {
		scrapeCfgs = append(scrapeCfgs, scCfgs...)
	} else {
		return err
	}

	hostScraper := internal.NewScraper(ctx, internal.ScraperTypeHosts, r.cfg.Endpoint, r.cfg.Hosts, r.cfg.Settings.ReloadIntervals.Hosts, commomLabel)
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

	volumesScraper := internal.NewScraper(ctx, internal.ScraperTypeVolumes, r.cfg.Endpoint, r.cfg.Volumes, r.cfg.Settings.ReloadIntervals.Volumes, model.LabelSet{})
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
