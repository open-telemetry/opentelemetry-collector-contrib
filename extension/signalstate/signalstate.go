// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signalstate // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/signalstate"

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/signalstate/internal/metadata"
)

// Window is one named adaptive-capture window. While now < ExpiresAt, boost
// behavior is active.
type Window struct {
	OpenedAt  time.Time
	ExpiresAt time.Time
}

// Config is empty for v1: the store is pure runtime state. Future versions
// may add periodic persistence via a storage extension.
type Config struct{}

var _ component.Config = (*Config)(nil)

func (c *Config) Validate() error { return nil }

// SignalState is the shared cross-signal state store: processors in the
// metrics, logs, and trace pipelines all read and write the same named
// windows through this extension. Reads never fail: an unknown window is
// simply closed, which is calm-state behavior (fail toward visibility).
type SignalState struct {
	cfg *Config
	log *zap.Logger

	mu      sync.Mutex
	windows map[string]*Window

	tel         *metadata.TelemetryBuilder
	observedNow func() time.Time
}

func newSignalState(_ context.Context, cfg *Config, set extension.Settings) (*SignalState, error) {
	tb, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	s := &SignalState{
		cfg:         cfg,
		log:         set.Logger,
		windows:     make(map[string]*Window),
		tel:         tb,
		observedNow: time.Now,
	}
	return s, nil
}

// Open (re)opens a named window: open until now+ttl; re-Open extends it.
func (s *SignalState) Open(_ context.Context, name string, ttl time.Duration) {
	now := s.observedNow()
	s.mu.Lock()
	defer s.mu.Unlock()
	if w, ok := s.windows[name]; ok {
		w.ExpiresAt = now.Add(ttl)
		return
	}
	s.windows[name] = &Window{OpenedAt: now, ExpiresAt: now.Add(ttl)}
	s.tel.OtelcolSignalstateWindows.Record(context.Background(), int64(len(s.windows)),
		metric.WithAttributes())
	s.log.Debug("signalstate: window opened", zap.String("window", name), zap.Duration("ttl", ttl))
}

// IsOpen reports whether the window is currently in its boost period.
func (s *SignalState) IsOpen(_ context.Context, name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, ok := s.windows[name]
	if !ok {
		return false
	}
	if s.observedNow().Before(w.ExpiresAt) {
		return true
	}
	delete(s.windows, name) // opportunistic GC of expired windows
	return false
}

// ExpiresAt returns when the named window expires (zero for unknown window).
func (s *SignalState) ExpiresAt(_ context.Context, name string) time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	if w, ok := s.windows[name]; ok {
		return w.ExpiresAt
	}
	return time.Time{}
}

func (s *SignalState) Start(_ context.Context, _ component.Host) error { return nil }
func (s *SignalState) Shutdown(_ context.Context) error                { return nil }

var _ extension.Extension = (*SignalState)(nil)
