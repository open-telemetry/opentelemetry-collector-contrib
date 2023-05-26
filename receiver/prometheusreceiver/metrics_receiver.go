// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/mitchellh/hashstructure/v2"
	commonconfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	promHTTP "github.com/prometheus/prometheus/discovery/http"
	"github.com/prometheus/prometheus/scrape"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"
)

const (
	defaultGCInterval = 2 * time.Minute
	gcIntervalDelta   = 1 * time.Minute
)

// pReceiver is the type that provides Prometheus scraper/receiver functionality.
type pReceiver struct {
	cfg                 *Config
	consumer            consumer.Metrics
	cancelFunc          context.CancelFunc
	targetAllocatorStop chan struct{}
	configLoaded        chan struct{}
	loadConfigOnce      sync.Once

	settings         receiver.CreateSettings
	registry         *featuregate.Registry
	scrapeManager    *scrape.Manager
	discoveryManager *discovery.Manager
}

// New creates a new prometheus.Receiver reference.
func newPrometheusReceiver(set receiver.CreateSettings, cfg *Config, next consumer.Metrics, registry *featuregate.Registry) *pReceiver {
	pr := &pReceiver{
		cfg:                 cfg,
		consumer:            next,
		settings:            set,
		configLoaded:        make(chan struct{}),
		targetAllocatorStop: make(chan struct{}),
		registry:            registry,
	}
	return pr
}

// Start is the method that starts Prometheus scraping. It
// is controlled by having previously defined a Configuration using perhaps New.
func (r *pReceiver) Start(_ context.Context, host component.Host) error {
	discoveryCtx, cancel := context.WithCancel(context.Background())
	r.cancelFunc = cancel

	logger := internal.NewZapToGokitLogAdapter(r.settings.Logger)

	// add scrape configs defined by the collector configs
	baseCfg := r.cfg.PrometheusConfig

	err := r.initPrometheusComponents(discoveryCtx, host, logger)
	if err != nil {
		r.settings.Logger.Error("Failed to initPrometheusComponents Prometheus components", zap.Error(err))
		return err
	}

	err = r.applyCfg(baseCfg)
	if err != nil {
		r.settings.Logger.Error("Failed to apply new scrape configuration", zap.Error(err))
		return err
	}

	allocConf := r.cfg.TargetAllocator
	if allocConf != nil {
		err = r.startTargetAllocator(allocConf, baseCfg)
		if err != nil {
			return err
		}
	}

	r.loadConfigOnce.Do(func() {
		close(r.configLoaded)
	})

	return nil
}

func (r *pReceiver) startTargetAllocator(allocConf *targetAllocator, baseCfg *config.Config) error {
	r.settings.Logger.Info("Starting target allocator discovery")
	// immediately sync jobs, not waiting for the first tick
	savedHash, err := r.syncTargetAllocator(uint64(0), allocConf, baseCfg)
	if err != nil {
		return err
	}
	go func() {
		targetAllocatorIntervalTicker := time.NewTicker(allocConf.Interval)
		for {
			select {
			case <-targetAllocatorIntervalTicker.C:
				hash, newErr := r.syncTargetAllocator(savedHash, allocConf, baseCfg)
				if newErr != nil {
					r.settings.Logger.Error(newErr.Error())
					continue
				}
				savedHash = hash
			case <-r.targetAllocatorStop:
				targetAllocatorIntervalTicker.Stop()
				r.settings.Logger.Info("Stopping target allocator")
				return
			}
		}
	}()
	return nil
}

// syncTargetAllocator request jobs from targetAllocator and update underlying receiver, if the response does not match the provided compareHash.
// baseDiscoveryCfg can be used to provide additional ScrapeConfigs which will be added to the retrieved jobs.
func (r *pReceiver) syncTargetAllocator(compareHash uint64, allocConf *targetAllocator, baseCfg *config.Config) (uint64, error) {
	r.settings.Logger.Debug("Syncing target allocator jobs")
	scrapeConfigsResponse, err := r.getScrapeConfigsResponse(allocConf.Endpoint)
	if err != nil {
		r.settings.Logger.Error("Failed to retrieve job list", zap.Error(err))
		return 0, err
	}

	hash, err := hashstructure.Hash(scrapeConfigsResponse, hashstructure.FormatV2, nil)
	if err != nil {
		r.settings.Logger.Error("Failed to hash job list", zap.Error(err))
		return 0, err
	}
	if hash == compareHash {
		// no update needed
		return hash, nil
	}

	// Clear out the current configurations
	baseCfg.ScrapeConfigs = []*config.ScrapeConfig{}

	for jobName, scrapeConfig := range scrapeConfigsResponse {
		var httpSD promHTTP.SDConfig
		if allocConf.HTTPSDConfig == nil {
			httpSD = promHTTP.SDConfig{
				RefreshInterval: model.Duration(30 * time.Second),
			}
		} else {
			httpSD = *allocConf.HTTPSDConfig
		}
		escapedJob := url.QueryEscape(jobName)
		httpSD.URL = fmt.Sprintf("%s/jobs/%s/targets?collector_id=%s", allocConf.Endpoint, escapedJob, allocConf.CollectorID)
		httpSD.HTTPClientConfig.FollowRedirects = false
		scrapeConfig.ServiceDiscoveryConfigs = discovery.Configs{
			&httpSD,
		}

		baseCfg.ScrapeConfigs = append(baseCfg.ScrapeConfigs, scrapeConfig)
	}

	err = r.applyCfg(baseCfg)
	if err != nil {
		r.settings.Logger.Error("Failed to apply new scrape configuration", zap.Error(err))
		return 0, err
	}

	return hash, nil
}

// instantiateShard inserts the SHARD environment variable in the returned configuration
func (r *pReceiver) instantiateShard(body []byte) []byte {
	shard, ok := os.LookupEnv("SHARD")
	if !ok {
		shard = "0"
	}
	return bytes.ReplaceAll(body, []byte("$(SHARD)"), []byte(shard))
}

func (r *pReceiver) getScrapeConfigsResponse(baseURL string) (map[string]*config.ScrapeConfig, error) {
	scrapeConfigsURL := fmt.Sprintf("%s/scrape_configs", baseURL)
	_, err := url.Parse(scrapeConfigsURL) // check if valid
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(scrapeConfigsURL) //nolint
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	jobToScrapeConfig := map[string]*config.ScrapeConfig{}
	envReplacedBody := r.instantiateShard(body)
	err = yaml.Unmarshal(envReplacedBody, &jobToScrapeConfig)
	if err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return jobToScrapeConfig, nil
}

func (r *pReceiver) applyCfg(cfg *config.Config) error {
	if err := r.scrapeManager.ApplyConfig(cfg); err != nil {
		return err
	}

	discoveryCfg := make(map[string]discovery.Configs)
	for _, scrapeConfig := range cfg.ScrapeConfigs {
		discoveryCfg[scrapeConfig.JobName] = scrapeConfig.ServiceDiscoveryConfigs
		r.settings.Logger.Info("Scrape job added", zap.String("jobName", scrapeConfig.JobName))
	}
	if err := r.discoveryManager.ApplyConfig(discoveryCfg); err != nil {
		return err
	}
	return nil
}

func (r *pReceiver) initPrometheusComponents(ctx context.Context, host component.Host, logger log.Logger) error {
	r.discoveryManager = discovery.NewManager(ctx, logger)

	go func() {
		r.settings.Logger.Info("Starting discovery manager")
		if err := r.discoveryManager.Run(); err != nil {
			r.settings.Logger.Error("Discovery manager failed", zap.Error(err))
			host.ReportFatalError(err)
		}
	}()

	var startTimeMetricRegex *regexp.Regexp
	if r.cfg.StartTimeMetricRegex != "" {
		var err error
		startTimeMetricRegex, err = regexp.Compile(r.cfg.StartTimeMetricRegex)
		if err != nil {
			return err
		}
	}

	store, err := internal.NewAppendable(
		r.consumer,
		r.settings,
		gcInterval(r.cfg.PrometheusConfig),
		r.cfg.UseStartTimeMetric,
		startTimeMetricRegex,
		useCreatedMetricGate.IsEnabled(),
		r.cfg.PrometheusConfig.GlobalConfig.ExternalLabels,
		r.registry,
	)
	if err != nil {
		return err
	}

	r.scrapeManager = scrape.NewManager(&scrape.Options{
		PassMetadataInContext: true,
		ExtraMetrics: r.cfg.ReportExtraMetrics,
		HTTPClientOptions: []commonconfig.HTTPClientOption{
			commonconfig.WithUserAgent(r.settings.BuildInfo.Command + "/" + r.settings.BuildInfo.Version),
		},
	}, logger, store)

	go func() {
		// The scrape manager needs to wait for the configuration to be loaded before beginning
		<-r.configLoaded
		r.settings.Logger.Info("Starting scrape manager")
		if err := r.scrapeManager.Run(r.discoveryManager.SyncCh()); err != nil {
			r.settings.Logger.Error("Scrape manager failed", zap.Error(err))
			host.ReportFatalError(err)
		}
	}()
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
	if r.cancelFunc != nil {
		r.cancelFunc()
	}
	if r.scrapeManager != nil {
		r.scrapeManager.Stop()
	}
	close(r.targetAllocatorStop)
	return nil
}
