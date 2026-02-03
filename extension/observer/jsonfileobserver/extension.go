// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonfileobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/jsonfileobserver"

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/endpointswatcher"
)

var _ extension.Extension = (*jsonFileObserver)(nil)

type jsonFileObserver struct {
	*endpointswatcher.EndpointsWatcher
	logger          *zap.Logger
	config          *Config
	handler         *handler
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	lastModTime     time.Time
	lastContentHash string
}

func newObserver(set extension.Settings, config *Config) (*jsonFileObserver, error) {
	h := &handler{
		idNamespace: set.ID.String(),
		endpoints:   &sync.Map{},
		logger:      set.Logger,
	}

	return &jsonFileObserver{
		EndpointsWatcher: endpointswatcher.New(h, time.Second, set.Logger),
		logger:           set.Logger,
		config:           config,
		handler:          h,
	}, nil
}

func (j *jsonFileObserver) Start(_ context.Context, _ component.Host) error {
	ctx, cancel := context.WithCancel(context.Background())
	j.cancel = cancel

	// Do an initial load
	if err := j.loadEndpoints(); err != nil {
		j.logger.Warn("Failed to load initial endpoints from JSON file", zap.Error(err))
	}

	// Start the file watcher goroutine
	j.wg.Add(1)
	go j.watchFile(ctx)

	return nil
}

func (j *jsonFileObserver) Shutdown(_ context.Context) error {
	if j.cancel != nil {
		j.cancel()
	}
	j.wg.Wait()
	return nil
}

func (j *jsonFileObserver) watchFile(ctx context.Context) {
	defer j.wg.Done()

	interval := j.config.RefreshInterval
	if interval <= 0 {
		interval = 10 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := j.checkAndReload(); err != nil {
				j.logger.Warn("Error checking JSON file for changes", zap.Error(err))
			}
		}
	}
}

func (j *jsonFileObserver) checkAndReload() error {
	info, err := os.Stat(j.config.Path)
	if err != nil {
		return err
	}

	// Check if file has been modified
	if info.ModTime().After(j.lastModTime) {
		j.logger.Debug("JSON file modified, reloading endpoints", zap.String("path", j.config.Path))
		if err := j.loadEndpoints(); err != nil {
			return err
		}
		j.lastModTime = info.ModTime()
	}

	return nil
}

func (j *jsonFileObserver) loadEndpoints() error {
	data, err := os.ReadFile(j.config.Path)
	if err != nil {
		return err
	}

	var endpointConfigs []EndpointConfig
	if err := json.Unmarshal(data, &endpointConfigs); err != nil {
		return err
	}

	// Build new endpoints map
	newEndpoints := make(map[observer.EndpointID]observer.Endpoint)
	for _, ec := range endpointConfigs {
		endpoint := convertEndpointConfig(j.handler.idNamespace, ec)
		newEndpoints[endpoint.ID] = endpoint
	}

	// Find removed endpoints
	j.handler.endpoints.Range(func(key, _ any) bool {
		id := key.(observer.EndpointID)
		if _, exists := newEndpoints[id]; !exists {
			j.handler.endpoints.Delete(id)
		}
		return true
	})

	// Add/update endpoints
	for id, endpoint := range newEndpoints {
		j.handler.endpoints.Store(id, endpoint)
	}

	j.logger.Info("Loaded endpoints from JSON file",
		zap.String("path", j.config.Path),
		zap.Int("count", len(endpointConfigs)))

	return nil
}
