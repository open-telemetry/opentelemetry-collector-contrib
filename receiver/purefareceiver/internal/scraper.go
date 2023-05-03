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

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver/internal"

import (
	"context"
	"fmt"
	"net/url"
	"time"

	configutil "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
)

type Scraper interface {
	ToPrometheusReceiverConfig(host component.Host, fact receiver.Factory) ([]*config.ScrapeConfig, error)
}

type ScraperType string

const (
	ScraperTypeArray       ScraperType = "array"
	ScraperTypeHosts       ScraperType = "hosts"
	ScraperTypeDirectories ScraperType = "directories"
	ScraperTypePods        ScraperType = "pods"
	ScraperTypeVolumes     ScraperType = "volumes"
)

type scraper struct {
	scraperType    ScraperType
	endpoint       string
	configs        []ScraperConfig
	scrapeInterval time.Duration
	labels         model.LabelSet
}

func NewScraper(ctx context.Context,
	scraperType ScraperType,
	endpoint string,
	configs []ScraperConfig,
	scrapeInterval time.Duration,
	labels model.LabelSet,
) Scraper {
	return &scraper{
		scraperType:    scraperType,
		endpoint:       endpoint,
		configs:        configs,
		scrapeInterval: scrapeInterval,
		labels:         labels,
	}
}

func (h *scraper) ToPrometheusReceiverConfig(host component.Host, fact receiver.Factory) ([]*config.ScrapeConfig, error) {
	scrapeCfgs := []*config.ScrapeConfig{}

	for _, arr := range h.configs {
		u, err := url.Parse(h.endpoint)
		if err != nil {
			return nil, err
		}

		bearerToken, err := RetrieveBearerToken(arr.Auth, host.GetExtensions())
		if err != nil {
			return nil, err
		}

		httpConfig := configutil.HTTPClientConfig{}
		httpConfig.BearerToken = configutil.Secret(bearerToken)

		labels := h.labels
		labels["fa_array_name"] = model.LabelValue(arr.Address)

		scrapeConfig := &config.ScrapeConfig{
			HTTPClientConfig: httpConfig,
			ScrapeInterval:   model.Duration(h.scrapeInterval),
			ScrapeTimeout:    model.Duration(h.scrapeInterval),
			JobName:          fmt.Sprintf("%s/%s/%s", "purefa", h.scraperType, arr.Address),
			HonorTimestamps:  true,
			Scheme:           u.Scheme,
			MetricsPath:      fmt.Sprintf("/metrics/%s", h.scraperType),
			Params: url.Values{
				"endpoint": {arr.Address},
			},

			ServiceDiscoveryConfigs: discovery.Configs{
				&discovery.StaticConfig{
					{
						Targets: []model.LabelSet{
							{model.AddressLabel: model.LabelValue(u.Host)},
						},
						Labels: labels,
					},
				},
			},
		}

		scrapeCfgs = append(scrapeCfgs, scrapeConfig)
	}

	return scrapeCfgs, nil
}
