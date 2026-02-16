// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package targetallocator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/targetallocator"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/goccy/go-yaml"
	commonconfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	promHTTP "github.com/prometheus/prometheus/discovery/http"
	"github.com/prometheus/prometheus/scrape"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

type Manager struct {
	settings             receiver.Settings
	shutdown             chan struct{}
	cfg                  *Config
	promCfg              *promconfig.Config
	initialScrapeConfigs []*promconfig.ScrapeConfig
	scrapeManager        *scrape.Manager
	discoveryManager     *discovery.Manager
	wg                   sync.WaitGroup

	// configUpdateCount tracks how many times the config has changed, for
	// testing.
	configUpdateCount *atomic.Int64
	// configUpdated is signaled after each config update, for testing.
	configUpdated chan struct{}
}

func NewManager(set receiver.Settings, cfg *Config, promCfg *promconfig.Config) *Manager {
	return &Manager{
		shutdown:             make(chan struct{}),
		settings:             set,
		cfg:                  cfg,
		promCfg:              promCfg,
		initialScrapeConfigs: promCfg.ScrapeConfigs,
		configUpdateCount:    &atomic.Int64{},
		configUpdated:        make(chan struct{}, 10),
	}
}

func (m *Manager) Start(ctx context.Context, host component.Host, sm *scrape.Manager, dm *discovery.Manager) error {
	m.scrapeManager = sm
	m.discoveryManager = dm
	err := m.applyCfg()
	if err != nil {
		m.settings.Logger.Error("Failed to apply new scrape configuration", zap.Error(err))
		return err
	}
	if m.cfg == nil {
		// the target allocator is disabled
		return nil
	}
	httpClient, err := m.cfg.ToClient(ctx, host.GetExtensions(), m.settings.TelemetrySettings)
	if err != nil {
		m.settings.Logger.Error("Failed to create http client", zap.Error(err))
		return err
	}
	m.settings.Logger.Info("Starting target allocator discovery")

	operation := func() (uint64, error) {
		savedHash, opErr := m.sync(uint64(0), httpClient)
		if opErr != nil {
			if errors.Is(opErr, syscall.ECONNREFUSED) {
				return 0, backoff.RetryAfter(1)
			}
			return 0, opErr
		}
		return savedHash, nil
	}
	// immediately sync jobs, not waiting for the first tick
	savedHash, err := backoff.Retry(ctx, operation, backoff.WithBackOff(backoff.NewExponentialBackOff()))
	if err != nil {
		return err
	}
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		targetAllocatorIntervalTicker := time.NewTicker(m.cfg.Interval)
		for {
			select {
			case <-targetAllocatorIntervalTicker.C:
				hash, newErr := m.sync(savedHash, httpClient)
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
	m.wg.Wait()
}

// sync request jobs from targetAllocator and update underlying receiver, if the response does not match the provided compareHash.
// baseDiscoveryCfg can be used to provide additional ScrapeConfigs which will be added to the retrieved jobs.
func (m *Manager) sync(compareHash uint64, httpClient *http.Client) (uint64, error) {
	m.settings.Logger.Debug("Syncing target allocator jobs")
	scrapeConfigsResponse, err := getScrapeConfigsResponse(httpClient, m.cfg.Endpoint)
	if err != nil {
		m.settings.Logger.Error("Failed to retrieve job list", zap.Error(err))
		return 0, err
	}

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

		if scrapeConfig.ScrapeFallbackProtocol == "" {
			scrapeConfig.ScrapeFallbackProtocol = promconfig.PrometheusText0_0_4
		}

		// Validate the scrape config and also fill in the defaults from the global config as needed.
		err = scrapeConfig.Validate(m.promCfg.GlobalConfig)
		if err != nil {
			m.settings.Logger.Error("Failed to validate the scrape configuration", zap.Error(err))
			return 0, err
		}

		m.promCfg.ScrapeConfigs = append(m.promCfg.ScrapeConfigs, scrapeConfig)
	}

	err = m.applyCfg()
	if err != nil {
		m.settings.Logger.Error("Failed to apply new scrape configuration", zap.Error(err))
		return 0, err
	}

	if m.configUpdateCount != nil {
		m.configUpdateCount.Add(1)
		select {
		case m.configUpdated <- struct{}{}:
		default:
		}
	}
	return hash, nil
}

func (m *Manager) applyCfg() error {
	scrapeConfigs, err := m.promCfg.GetScrapeConfigs()
	if err != nil {
		return fmt.Errorf("could not get scrape configs: %w", err)
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
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	jobToScrapeConfig := map[string]*promconfig.ScrapeConfig{}
	envReplacedBody := instantiateShard(body, os.LookupEnv)
	err = yaml.Unmarshal(envReplacedBody, &jobToScrapeConfig)
	if err != nil {
		return nil, err
	}
	return jobToScrapeConfig, nil
}

// instantiateShard inserts the SHARD environment variable in the returned configuration
func instantiateShard(body []byte, lookup func(string) (string, bool)) []byte {
	shard, ok := lookup("SHARD")

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
