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

package prometheusreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-kit/log"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/scrape"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"
)

const (
	defaultGCInterval = 2 * time.Minute
	gcIntervalDelta   = 1 * time.Minute
)

// pReceiver is the type that provides Prometheus scraper/receiver functionality.
type pReceiver struct {
	cfg        *Config
	consumer   consumer.Metrics
	cancelFunc context.CancelFunc

	settings                      component.ReceiverCreateSettings
	scrapeManager                 *scrape.Manager
	discoveryManager              *discovery.Manager
	targetAllocatorIntervalTicker *time.Ticker
}

type LinkJSON struct {
	Link string `json:"_link"`
}

// New creates a new prometheus.Receiver reference.
func newPrometheusReceiver(set component.ReceiverCreateSettings, cfg *Config, next consumer.Metrics) *pReceiver {
	pr := &pReceiver{
		cfg:      cfg,
		consumer: next,
		settings: set,
	}
	return pr
}

// Start is the method that starts Prometheus scraping and it
// is controlled by having previously defined a Configuration using perhaps New.
func (r *pReceiver) Start(_ context.Context, host component.Host) error {
	discoveryCtx, cancel := context.WithCancel(context.Background())
	r.cancelFunc = cancel

	logger := internal.NewZapToGokitLogAdapter(r.settings.Logger)

	r.discoveryManager = discovery.NewManager(discoveryCtx, logger)

	baseDiscoveryCfg := make(map[string]discovery.Configs)

	// add scrape configs defined by the collector configs
	if r.cfg.PrometheusConfig != nil && r.cfg.PrometheusConfig.ScrapeConfigs != nil {
		for _, scrapeConfig := range r.cfg.PrometheusConfig.ScrapeConfigs {
			baseDiscoveryCfg[scrapeConfig.JobName] = scrapeConfig.ServiceDiscoveryConfigs
		}
	}

	err := r.initPrometheusComponents(host, logger)
	if err != nil {
		r.settings.Logger.Error("Failed to initPrometheusComponents Prometheus compnents", zap.Error(err))
		return err
	}

	err = r.applyCfg(baseDiscoveryCfg)
	if err != nil {
		r.settings.Logger.Error("Failed to apply new scrape configuration", zap.Error(err))
		return err
	}

	allocConf := r.cfg.TargetAllocator
	if allocConf != nil {
		r.targetAllocatorIntervalTicker = time.NewTicker(allocConf.Interval)

		go func() {
			savedHash := uint64(0)

			for {
				select {
				case <-r.targetAllocatorIntervalTicker.C:
					jobObject, err := getJobResponse(allocConf.Endpoint)
					if err != nil {
						r.settings.Logger.Error("Failed to retrieve job list", zap.Error(err))
						continue
					}

					hash, err := hashstructure.Hash(jobObject, hashstructure.FormatV2, nil)
					if err != nil {
						r.settings.Logger.Error("Failed to hash job list", zap.Error(err))
						continue
					}
					if hash == savedHash {
						// no update needed
						continue
					}

					discoveryCfg := baseDiscoveryCfg

					for key, linkJSON := range *jobObject {
						httpSD := *allocConf.HttpSDConfig
						httpSD.URL = fmt.Sprintf("%s%s?collector_id=%s", allocConf.Endpoint, linkJSON.Link, allocConf.CollectorID)
						discoveryCfg[key] = discovery.Configs{
							&httpSD,
						}
					}

					err = r.applyCfg(discoveryCfg)
					if err != nil {
						r.settings.Logger.Error("Failed to apply new scrape configuration", zap.Error(err))
						continue
					}

					savedHash = hash
				}
			}
		}()
	}

	return nil
}

func getJobResponse(baseURL string) (*map[string]LinkJSON, error) {
	jobURLString := fmt.Sprintf("%s/jobs", baseURL)
	_, err := url.Parse(jobURLString) // check if valid
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(jobURLString)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	jobObject := &map[string]LinkJSON{}
	err = json.NewDecoder(resp.Body).Decode(jobObject)
	if err != nil {
		return nil, err
	}
	return jobObject, nil
}

func (r *pReceiver) applyCfg(discoveryCfg map[string]discovery.Configs) error {
	if err := r.discoveryManager.ApplyConfig(discoveryCfg); err != nil {
		return err
	}
	return nil
}

func (r *pReceiver) initPrometheusComponents(host component.Host, logger log.Logger) error {
	go func() {
		if err := r.discoveryManager.Run(); err != nil {
			r.settings.Logger.Error("Discovery manager failed", zap.Error(err))
			host.ReportFatalError(err)
		}
	}()

	store := internal.NewAppendable(
		r.consumer,
		r.settings,
		gcInterval(r.cfg.PrometheusConfig),
		r.cfg.UseStartTimeMetric,
		r.cfg.StartTimeMetricRegex,
		r.cfg.ID(),
		r.cfg.PrometheusConfig.GlobalConfig.ExternalLabels,
	)
	r.scrapeManager = scrape.NewManager(&scrape.Options{PassMetadataInContext: true}, logger, store)
	go func() {
		if err := r.scrapeManager.Run(r.discoveryManager.SyncCh()); err != nil {
			r.settings.Logger.Error("Scrape manager failed", zap.Error(err))
			host.ReportFatalError(err)
		}
	}()
	if err := r.scrapeManager.ApplyConfig(r.cfg.PrometheusConfig); err != nil {
		return err
	}
	return nil
}

// gcInterval returns the longest scrape interval used by a scrape config,
// plus a delta to prevent race conditions.
// This ensures jobs are not garbage collected between scrapes.
func gcInterval(cfg *config.Config) time.Duration {
	gcInterval := defaultGCInterval
	if time.Duration(cfg.GlobalConfig.ScrapeInterval)+gcIntervalDelta > gcInterval {
		gcInterval = time.Duration(cfg.GlobalConfig.ScrapeInterval) + gcIntervalDelta
	}
	for _, scrapeConfig := range cfg.ScrapeConfigs {
		if time.Duration(scrapeConfig.ScrapeInterval)+gcIntervalDelta > gcInterval {
			gcInterval = time.Duration(scrapeConfig.ScrapeInterval) + gcIntervalDelta
		}
	}
	return gcInterval
}

// Shutdown stops and cancels the underlying Prometheus scrapers.
func (r *pReceiver) Shutdown(context.Context) error {
	r.cancelFunc()
	r.scrapeManager.Stop()
	if r.targetAllocatorIntervalTicker != nil {
		r.targetAllocatorIntervalTicker.Stop()
	}
	return nil
}
