// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sharedpromconfig // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/sharedpromconfig"

import (
	"sync"

	promconfig "github.com/prometheus/prometheus/config"
)

// Config provides thread-safe access to a shared Prometheus configuration.
// The target allocator's sync() replaces ScrapeConfigs on every cycle,
// while the API server concurrently reads the config to serve
// /api/v1/status/config and /api/v1/targets.
// This wraps the Prometheus config with a mutex tied to it.
type Config struct {
	mu  sync.RWMutex
	cfg *promconfig.Config
}

// NewConfig wraps an existing Prometheus config for safe concurrent access.
func NewConfig(cfg *promconfig.Config) *Config {
	return &Config{cfg: cfg}
}

// Get returns a shallow copy of the current config under a read lock.
// Callers must not mutate the returned value.
func (s *Config) Get() promconfig.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return *s.cfg
}

// Mutate calls fn while holding the write lock, allowing safe in-place
// modification of the underlying config. The function should only replace
// top-level fields (e.g. cfg.ScrapeConfigs = newConfigs), not mutate
// elements within existing slices or maps.
func (s *Config) Mutate(fn func(cfg *promconfig.Config)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(s.cfg)
}
