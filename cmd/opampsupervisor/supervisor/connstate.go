// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// ConnectionState represents the current state of the OpAMP server connection.
type ConnectionState int

const (
	// ConnectionStateWaiting indicates waiting for the initial connection.
	ConnectionStateWaiting ConnectionState = iota
	// ConnectionStateConnected indicates successfully connected to the OpAMP server.
	ConnectionStateConnected
	// ConnectionStateDisconnected indicates disconnected from the OpAMP server after being connected.
	ConnectionStateDisconnected
)

func (s ConnectionState) String() string {
	switch s {
	case ConnectionStateWaiting:
		return "waiting"
	case ConnectionStateConnected:
		return "connected"
	case ConnectionStateDisconnected:
		return "disconnected"
	default:
		return "unknown"
	}
}

// ConnectionStateEvent represents a significant connection state change that
// may require action from the supervisor.
type ConnectionStateEvent int

const (
	// ConnectionStateEventConnected indicates the connection was established.
	// This may trigger switching from fallback config back to remote config.
	ConnectionStateEventConnected ConnectionStateEvent = iota
	// ConnectionStateEventFallbackTriggered indicates the fallback timeout has expired
	// and the supervisor should switch to the fallback configuration.
	ConnectionStateEventFallbackTriggered
)

func (e ConnectionStateEvent) String() string {
	switch e {
	case ConnectionStateEventConnected:
		return "connected"
	case ConnectionStateEventFallbackTriggered:
		return "fallback_triggered"
	default:
		return "unknown"
	}
}

// ConnectionStateTrackerConfig contains the configuration for the ConnectionStateTracker.
type ConnectionStateTrackerConfig struct {
	// StartupTimeout is the duration to wait for the initial connection before
	// triggering fallback. If zero, startup fallback is disabled.
	StartupTimeout time.Duration
	// RuntimeTimeout is the duration to wait after disconnection before
	// triggering fallback. If zero, runtime fallback is disabled.
	RuntimeTimeout time.Duration
	// Logger is the logger to use for logging state changes.
	Logger *zap.Logger
}

// ConnectionStateTracker tracks the connection state to the OpAMP server and
// triggers fallback events when appropriate timeouts expire.
//
// This struct is designed to be extensible for future trigger conditions such as
// "failed X times in Y period".
type ConnectionStateTracker struct {
	mu     sync.Mutex
	config ConnectionStateTrackerConfig
	logger *zap.Logger

	// Current connection state
	state ConnectionState

	// Whether we're currently using fallback configuration
	usingFallback bool

	// Timer for fallback timeout
	fallbackTimer *time.Timer

	// Channel to send events to the supervisor
	eventCh chan ConnectionStateEvent

	// stopped indicates the tracker has been stopped
	stopped bool
}

// NewConnectionStateTracker creates a new ConnectionStateTracker with the given configuration.
func NewConnectionStateTracker(config ConnectionStateTrackerConfig) *ConnectionStateTracker {
	logger := config.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	tracker := &ConnectionStateTracker{
		config:        config,
		logger:        logger,
		state:         ConnectionStateWaiting,
		usingFallback: false,
		eventCh:       make(chan ConnectionStateEvent, 10),
	}

	return tracker
}

// Start starts the connection state tracker. If startup fallback is enabled,
// it will start the startup timeout timer.
func (t *ConnectionStateTracker) Start() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.stopped {
		return
	}

	if t.config.StartupTimeout <= 0 {
		t.logger.Debug("Starting connection state tracker without startup timeout")
		return
	}
	t.logger.Debug(
		"Starting connection state tracker with startup timeout",
		zap.Duration("timeout", t.config.StartupTimeout),
	)
	t.startFallbackTimer(t.config.StartupTimeout)
}

// Stop stops the connection state tracker and cleans up resources.
func (t *ConnectionStateTracker) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stopped = true
	t.stopFallbackTimer()
	close(t.eventCh)
}

// Events returns a channel that receives connection state events.
// The supervisor should listen on this channel and handle events appropriately.
func (t *ConnectionStateTracker) Events() <-chan ConnectionStateEvent {
	return t.eventCh
}

// OnConnect should be called when the OpAMP client successfully connects to the server.
func (t *ConnectionStateTracker) OnConnect() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.stopped {
		return
	}

	t.state = ConnectionStateConnected

	// Stop any pending fallback timer
	t.stopFallbackTimer()

	// If we were using fallback, notify the supervisor to switch back to remote config
	if t.usingFallback {
		t.usingFallback = false
		t.sendEvent(ConnectionStateEventConnected)
	}
}

// OnConnectFailed should be called when the OpAMP client fails to connect to the server.
func (t *ConnectionStateTracker) OnConnectFailed() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.stopped {
		return
	}

	// Only transition to disconnected if we were previously connected.
	// Otherwise, we stay in waiting state.
	switch t.state {
	case ConnectionStateConnected:
		t.state = ConnectionStateDisconnected

		// Start runtime fallback timer if configured and not already using fallback
		if t.config.RuntimeTimeout > 0 && !t.usingFallback {
			t.logger.Debug(
				"Starting runtime fallback timer",
				zap.Duration("timeout", t.config.RuntimeTimeout),
			)
			t.startFallbackTimer(t.config.RuntimeTimeout)
		}
	case ConnectionStateWaiting:
		t.logger.Debug(
			"OpAMP connection attempt failed while waiting for initial connection",
			zap.Bool("using_fallback", t.usingFallback),
		)
		// Timer was already started in Start() if startup timeout is configured
	}
}

// IsUsingFallback returns true if the tracker has triggered fallback mode.
func (t *ConnectionStateTracker) IsUsingFallback() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.usingFallback
}

// State returns a copy of the current connection state.
func (t *ConnectionStateTracker) State() ConnectionState {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state
}

// startFallbackTimer starts a timer that will trigger fallback when it expires.
// Must be called with the mutex held.
func (t *ConnectionStateTracker) startFallbackTimer(timeout time.Duration) {
	t.stopFallbackTimer()

	t.fallbackTimer = time.AfterFunc(timeout, func() {
		t.onFallbackTimeout()
	})
}

// stopFallbackTimer stops the current fallback timer if one is running.
// Must be called with the mutex held.
func (t *ConnectionStateTracker) stopFallbackTimer() {
	if t.fallbackTimer != nil {
		t.fallbackTimer.Stop()
		t.fallbackTimer = nil
	}
}

// onFallbackTimeout is called when the fallback timer expires.
func (t *ConnectionStateTracker) onFallbackTimeout() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.stopped {
		return
	}

	// Only trigger fallback if we're not already connected and not already using fallback
	if t.state != ConnectionStateConnected && !t.usingFallback {
		t.logger.Warn(
			"Fallback timeout expired, switching to fallback configuration",
			zap.String("state", t.state.String()),
		)
		t.usingFallback = true
		t.sendEvent(ConnectionStateEventFallbackTriggered)
	}
}

// sendEvent sends an event to the event channel without blocking.
// Must be called with the mutex held.
func (t *ConnectionStateTracker) sendEvent(event ConnectionStateEvent) {
	select {
	case t.eventCh <- event:
		t.logger.Debug("Sent connection state event", zap.String("event", event.String()))
	default:
		t.logger.Warn("Connection state event channel full, dropping event", zap.String("event", event.String()))
	}
}
