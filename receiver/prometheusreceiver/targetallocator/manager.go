// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package targetallocator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/targetallocator"

import (
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"time"

	"github.com/fsnotify/fsnotify"
	commonconfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	promHTTP "github.com/prometheus/prometheus/discovery/http"
	"github.com/prometheus/prometheus/scrape"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

type Manager struct {
	settings               receiver.Settings
	shutdown               chan struct{}
	cfg                    *Config
	promCfg                *promconfig.Config
	initialScrapeConfigs   []*promconfig.ScrapeConfig
	scrapeManager          *scrape.Manager
	discoveryManager       *discovery.Manager
	enableNativeHistograms bool
	watcher                *fsnotify.Watcher
	host                   component.Host
	httpClient             *http.Client
}

func NewManager(set receiver.Settings, cfg *Config, promCfg *promconfig.Config, enableNativeHistograms bool) *Manager {
	return &Manager{
		shutdown:               make(chan struct{}),
		settings:               set,
		cfg:                    cfg,
		promCfg:                promCfg,
		initialScrapeConfigs:   promCfg.ScrapeConfigs,
		enableNativeHistograms: enableNativeHistograms,
	}
}

func (m *Manager) Start(ctx context.Context, host component.Host, sm *scrape.Manager, dm *discovery.Manager) error {
	m.scrapeManager = sm
	m.discoveryManager = dm
	m.host = host

	err := m.applyCfg()
	if err != nil {
		m.settings.Logger.Error("Failed to apply new scrape configuration", zap.Error(err))
		return err
	}
	if m.cfg == nil {
		// the target allocator is disabled
		return nil
	}
	if err = m.setHTTPClient(ctx); err != nil {
		return err
	}
	m.settings.Logger.Info("Starting target allocator discovery")
	// immediately sync jobs, not waiting for the first tick
	savedHash, err := m.sync(uint64(0))
	if err != nil {
		m.settings.Logger.Error("Failed to sync target allocator", zap.Error(err))
	}

	// Setup fsnotify watchers for TLS files
	if err := m.setupTLSWatchers(ctx); err != nil {
		m.settings.Logger.Error("Error setting up TLS watchers", zap.Error(err))
	}

	go func() {
		targetAllocatorIntervalTicker := time.NewTicker(m.cfg.Interval)
		for {
			select {
			case <-targetAllocatorIntervalTicker.C:
				hash, newErr := m.sync(savedHash)
				if newErr != nil {
					m.settings.Logger.Error(newErr.Error())
					continue
				}
				savedHash = hash
			case <-m.shutdown:
				targetAllocatorIntervalTicker.Stop()
				m.settings.Logger.Info("Stopping target allocator")
				return
			}
		}
	}()
	return nil
}

func (m *Manager) Shutdown() {
	close(m.shutdown)
	if m.watcher != nil {
		if err := m.watcher.Close(); err != nil {
			m.settings.Logger.Warn("Error closing fsnotify watcher", zap.Error(err))
		}
	}
}

// setupTLSWatchers creates one fsnotify watcher and adds CAFile, CertFile, KeyFile
// so that we automatically re‐sync whenever they change.
func (m *Manager) setupTLSWatchers(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}
	m.watcher = watcher

	addFile := func(path string) {
		if path == "" {
			return
		}
		if err := watcher.Add(path); err != nil {
			m.settings.Logger.Error("Failed to watch TLS file", zap.String("file", path), zap.Error(err))
		}
	}

	addFile(m.cfg.TLSSetting.CAFile)
	addFile(m.cfg.TLSSetting.CertFile)
	addFile(m.cfg.TLSSetting.KeyFile)

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) != 0 {
					m.settings.Logger.Info("TLS file changed; re-syncing",
						zap.String("file", event.Name), zap.String("op", event.Op.String()))
					if err := m.setHTTPClient(ctx); err != nil {
						m.settings.Logger.Error("Failed to sync after TLS file change", zap.Error(err))
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				m.settings.Logger.Error("fsnotify watcher error", zap.Error(err))
			case <-m.shutdown:
				return
			}
		}
	}()

	return nil
}

func (m *Manager) setHTTPClient(ctx context.Context) error {
	var err error
	m.httpClient, err = m.cfg.ToClient(ctx, m.host, m.settings.TelemetrySettings)
	if err != nil {
		m.settings.Logger.Error("Failed to create http client", zap.Error(err))
		return err
	}
	return nil
}

// sync request jobs from targetAllocator and update underlying receiver, if the response does not match the provided compareHash.
// baseDiscoveryCfg can be used to provide additional ScrapeConfigs which will be added to the retrieved jobs.
func (m *Manager) sync(compareHash uint64) (uint64, error) {
	m.settings.Logger.Debug("Syncing target allocator jobs")
	m.settings.Logger.Debug("endpoint", zap.String("endpoint", m.cfg.Endpoint))

	scrapeConfigsResponse, err := getScrapeConfigsResponse(m.httpClient, m.cfg.Endpoint)
	if err != nil {
		m.settings.Logger.Error("Failed to retrieve job list", zap.Error(err))
		return 0, err
	}

	m.settings.Logger.Debug("Scrape configs response", zap.Reflect("scrape_configs", scrapeConfigsResponse))
	hash, err := getScrapeConfigHash(scrapeConfigsResponse)
	if err != nil {
		m.settings.Logger.Error("Failed to hash job list", zap.Error(err))
		return 0, err
	}
	if hash == compareHash {
		// no update needed
		return hash, nil
	}

	// Copy initial scrape configurations
	initialConfig := make([]*promconfig.ScrapeConfig, len(m.initialScrapeConfigs))
	copy(initialConfig, m.initialScrapeConfigs)

	m.promCfg.ScrapeConfigs = initialConfig

	for jobName, scrapeConfig := range scrapeConfigsResponse {
		var httpSD promHTTP.SDConfig
		if m.cfg.HTTPSDConfig == nil {
			httpSD = promHTTP.SDConfig{
				RefreshInterval: model.Duration(30 * time.Second),
			}
		} else {
			httpSD = promHTTP.SDConfig(*m.cfg.HTTPSDConfig)
		}
		escapedJob := url.QueryEscape(jobName)
		httpSD.URL = fmt.Sprintf("%s/jobs/%s/targets?collector_id=%s", m.cfg.Endpoint, escapedJob, m.cfg.CollectorID)

		err = configureSDHTTPClientConfigFromTA(&httpSD, m.cfg)
		if err != nil {
			m.settings.Logger.Error("Failed to configure http client config", zap.Error(err))
			return 0, err
		}

		httpSD.HTTPClientConfig.FollowRedirects = false
		scrapeConfig.ServiceDiscoveryConfigs = discovery.Configs{
			&httpSD,
		}

		if m.cfg.HTTPScrapeConfig != nil {
			scrapeConfig.HTTPClientConfig = commonconfig.HTTPClientConfig(*m.cfg.HTTPScrapeConfig)
		}

		m.promCfg.ScrapeConfigs = append(m.promCfg.ScrapeConfigs, scrapeConfig)
	}

	err = m.applyCfg()
	if err != nil {
		m.settings.Logger.Error("Failed to apply new scrape configuration", zap.Error(err))
		return 0, err
	}

	return hash, nil
}

func (m *Manager) applyCfg() error {
	scrapeConfigs, err := m.promCfg.GetScrapeConfigs()
	if err != nil {
		return fmt.Errorf("could not get scrape configs: %w", err)
	}
	if !m.enableNativeHistograms {
		// Enforce scraping classic histograms to avoid dropping them.
		for _, scrapeConfig := range m.promCfg.ScrapeConfigs {
			scrapeConfig.AlwaysScrapeClassicHistograms = true
		}
	}

	if err := m.scrapeManager.ApplyConfig(m.promCfg); err != nil {
		return err
	}

	discoveryCfg := make(map[string]discovery.Configs)
	for _, scrapeConfig := range scrapeConfigs {
		discoveryCfg[scrapeConfig.JobName] = scrapeConfig.ServiceDiscoveryConfigs
		m.settings.Logger.Info("Scrape job added", zap.String("jobName", scrapeConfig.JobName))
	}
	return m.discoveryManager.ApplyConfig(discoveryCfg)
}

func getScrapeConfigsResponse(httpClient *http.Client, baseURL string) (map[string]*promconfig.ScrapeConfig, error) {
	scrapeConfigsURL := baseURL + "/scrape_configs"
	_, err := url.Parse(scrapeConfigsURL) // check if valid
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Get(scrapeConfigsURL)
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	jobToScrapeConfig := map[string]*promconfig.ScrapeConfig{}
	envReplacedBody := instantiateShard(body)
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

// instantiateShard inserts the SHARD environment variable in the returned configuration
func instantiateShard(body []byte) []byte {
	shard, ok := os.LookupEnv("SHARD")
	if !ok {
		shard = "0"
	}
	return bytes.ReplaceAll(body, []byte("$(SHARD)"), []byte(shard))
}

// Calculate a hash for a scrape config map.
// This is done by marshaling to YAML because it's the most straightforward and doesn't run into problems with unexported fields.
func getScrapeConfigHash(jobToScrapeConfig map[string]*promconfig.ScrapeConfig) (uint64, error) {
	var err error
	hash := fnv.New64()
	yamlEncoder := yaml.NewEncoder(hash)

	jobKeys := make([]string, 0, len(jobToScrapeConfig))
	for jobName := range jobToScrapeConfig {
		jobKeys = append(jobKeys, jobName)
	}
	sort.Strings(jobKeys)

	for _, jobName := range jobKeys {
		_, err = hash.Write([]byte(jobName))
		if err != nil {
			return 0, err
		}
		err = yamlEncoder.Encode(jobToScrapeConfig[jobName])
		if err != nil {
			return 0, err
		}
	}
	yamlEncoder.Close()
	return hash.Sum64(), err
}
