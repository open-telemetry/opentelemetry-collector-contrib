// Copyright 2022 The OpenTelemetry Authors
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

package host // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver/internal/host"

import (
	"context"
	"fmt"
	"net/url"
	"time"

	configutil "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	promcfg "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver/internal"
)

type hostScraper struct {
	internal.Scraper

	set  component.ReceiverCreateSettings
	next consumer.Metrics

	endpoint       string
	hosts          []internal.ScraperConfig
	scrapeInterval time.Duration

	wrapped component.MetricsReceiver
}

func NewScraper(ctx context.Context,
	set component.ReceiverCreateSettings,
	next consumer.Metrics,
	endpoint string,
	hosts []internal.ScraperConfig,
	scrapeInterval time.Duration,
) internal.Scraper {
	return &hostScraper{
		set:            set,
		next:           next,
		endpoint:       endpoint,
		hosts:          hosts,
		scrapeInterval: scrapeInterval,
	}
}

func (h *hostScraper) Start(ctx context.Context, host component.Host) error {
	fact := prometheusreceiver.NewFactory()

	promRecvCfg, err := h.ToPrometheusReceiverConfig(host, fact)
	if err != nil {
		return err
	}

	h.wrapped, err = fact.CreateMetricsReceiver(ctx, h.set, promRecvCfg, h.next)
	if err != nil {
		return err
	}

	err = h.wrapped.Start(ctx, host)
	if err != nil {
		return err
	}

	return nil
}

func (h *hostScraper) Shutdown(ctx context.Context) error {
	return h.wrapped.Shutdown(ctx)
}

func (h *hostScraper) ToPrometheusReceiverConfig(host component.Host, fact component.ReceiverFactory) (*prometheusreceiver.Config, error) {
	scrapeCfgs := []*promcfg.ScrapeConfig{}

	for _, arr := range h.hosts {
		u, err := url.Parse(h.endpoint)
		if err != nil {
			return nil, err
		}

		bearerToken, err := internal.RetrieveBearerToken(arr.Auth, host.GetExtensions())
		if err != nil {
			return nil, err
		}

		httpConfig := configutil.HTTPClientConfig{}
		httpConfig.BearerToken = configutil.Secret(bearerToken)

		scrapeConfig := &promcfg.ScrapeConfig{
			HTTPClientConfig: httpConfig,
			ScrapeInterval:   model.Duration(h.scrapeInterval),
			ScrapeTimeout:    model.Duration(h.scrapeInterval),
			JobName:          fmt.Sprintf("%s/%s/%s", "purefa", "hosts", arr.Address),
			HonorTimestamps:  true,
			Scheme:           u.Scheme,
			MetricsPath:      "/metrics/hosts",
			Params: url.Values{
				"endpoint": {arr.Address},
			},

			ServiceDiscoveryConfigs: discovery.Configs{
				&discovery.StaticConfig{
					{
						Targets: []model.LabelSet{
							{model.AddressLabel: model.LabelValue(u.Host)},
						},
					},
				},
			},
		}

		scrapeCfgs = append(scrapeCfgs, scrapeConfig)
	}

	promRecvCfg := fact.CreateDefaultConfig().(*prometheusreceiver.Config)
	promRecvCfg.PrometheusConfig = &promcfg.Config{ScrapeConfigs: scrapeCfgs}

	return promRecvCfg, nil
}
