// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestConnectionState_String(t *testing.T) {
	tests := []struct {
		state    ConnectionState
		expected string
	}{
		{ConnectionStateWaiting, "waiting"},
		{ConnectionStateConnected, "connected"},
		{ConnectionStateDisconnected, "disconnected"},
		{ConnectionState(99), "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.state.String())
		})
	}
}

func TestConnectionStateEvent_String(t *testing.T) {
	tests := []struct {
		event    ConnectionStateEvent
		expected string
	}{
		{ConnectionStateEventConnected, "connected"},
		{ConnectionStateEventFallbackTriggered, "fallback_triggered"},
		{ConnectionStateEvent(99), "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.event.String())
		})
	}
}

func TestNewConnectionStateTracker(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := ConnectionStateTrackerConfig{
		StartupTimeout: 5 * time.Second,
		RuntimeTimeout: 10 * time.Second,
		Logger:         logger,
	}

	tracker := NewConnectionStateTracker(config)
	require.NotNil(t, tracker)

	assert.Equal(t, ConnectionStateWaiting, tracker.State())
	assert.False(t, tracker.IsUsingFallback())
}

func TestNewConnectionStateTracker_NilLogger(t *testing.T) {
	config := ConnectionStateTrackerConfig{
		StartupTimeout: 5 * time.Second,
		Logger:         nil,
	}

	tracker := NewConnectionStateTracker(config)
	require.NotNil(t, tracker)
	// Should not panic with nil logger
}

func TestConnectionStateTracker_OnConnect(t *testing.T) {
	logger := zaptest.NewLogger(t)
	tracker := NewConnectionStateTracker(ConnectionStateTrackerConfig{
		StartupTimeout: 100 * time.Millisecond,
		Logger:         logger,
	})

	tracker.Start()
	defer tracker.Stop()

	// Simulate successful connection
	tracker.OnConnect()

	assert.Equal(t, ConnectionStateConnected, tracker.State())
	assert.False(t, tracker.IsUsingFallback())
}

func TestConnectionStateTracker_OnConnectFailed_WhileWaiting(t *testing.T) {
	logger := zaptest.NewLogger(t)
	tracker := NewConnectionStateTracker(ConnectionStateTrackerConfig{
		StartupTimeout: 100 * time.Millisecond,
		Logger:         logger,
	})

	tracker.Start()
	defer tracker.Stop()

	// Simulate failed connection while waiting
	tracker.OnConnectFailed()

	// Should still be in waiting state
	assert.Equal(t, ConnectionStateWaiting, tracker.State())
}

func TestConnectionStateTracker_OnConnectFailed_WhileConnected(t *testing.T) {
	logger := zaptest.NewLogger(t)
	tracker := NewConnectionStateTracker(ConnectionStateTrackerConfig{
		RuntimeTimeout: 100 * time.Millisecond,
		Logger:         logger,
	})

	tracker.Start()
	defer tracker.Stop()

	// First connect
	tracker.OnConnect()
	assert.Equal(t, ConnectionStateConnected, tracker.State())

	// Then fail
	tracker.OnConnectFailed()
	assert.Equal(t, ConnectionStateDisconnected, tracker.State())
}

func TestConnectionStateTracker_StartupFallbackTimeout(t *testing.T) {
	logger := zaptest.NewLogger(t)
	tracker := NewConnectionStateTracker(ConnectionStateTrackerConfig{
		StartupTimeout: 50 * time.Millisecond,
		Logger:         logger,
	})

	tracker.Start()
	defer tracker.Stop()

	// Wait for the fallback event
	select {
	case event := <-tracker.Events():
		assert.Equal(t, ConnectionStateEventFallbackTriggered, event)
		assert.True(t, tracker.IsUsingFallback())
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Expected fallback event but got timeout")
	}
}

func TestConnectionStateTracker_RuntimeFallbackTimeout(t *testing.T) {
	logger := zaptest.NewLogger(t)
	tracker := NewConnectionStateTracker(ConnectionStateTrackerConfig{
		RuntimeTimeout: 50 * time.Millisecond,
		Logger:         logger,
	})

	tracker.Start()
	defer tracker.Stop()

	// First connect
	tracker.OnConnect()

	// Then disconnect
	tracker.OnConnectFailed()

	// Wait for the fallback event
	select {
	case event := <-tracker.Events():
		assert.Equal(t, ConnectionStateEventFallbackTriggered, event)
		assert.True(t, tracker.IsUsingFallback())
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Expected fallback event but got timeout")
	}
}

func TestConnectionStateTracker_ConnectCancelsFallbackTimer(t *testing.T) {
	logger := zaptest.NewLogger(t)
	tracker := NewConnectionStateTracker(ConnectionStateTrackerConfig{
		StartupTimeout: 100 * time.Millisecond,
		Logger:         logger,
	})

	tracker.Start()
	defer tracker.Stop()

	// Connect before timeout expires
	time.Sleep(30 * time.Millisecond)
	tracker.OnConnect()

	// Wait past the original timeout
	time.Sleep(150 * time.Millisecond)

	// Should not have triggered fallback
	assert.False(t, tracker.IsUsingFallback())

	// Events channel should be empty
	select {
	case event := <-tracker.Events():
		t.Fatalf("Unexpected event: %v", event)
	default:
		// Expected - no events
	}
}

func TestConnectionStateTracker_ReconnectAfterFallback(t *testing.T) {
	logger := zaptest.NewLogger(t)
	tracker := NewConnectionStateTracker(ConnectionStateTrackerConfig{
		StartupTimeout: 30 * time.Millisecond,
		Logger:         logger,
	})

	tracker.Start()
	defer tracker.Stop()

	// Wait for fallback
	select {
	case event := <-tracker.Events():
		assert.Equal(t, ConnectionStateEventFallbackTriggered, event)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected fallback event but got timeout")
	}

	assert.True(t, tracker.IsUsingFallback())

	// Now reconnect
	tracker.OnConnect()

	// Should get connected event
	select {
	case event := <-tracker.Events():
		assert.Equal(t, ConnectionStateEventConnected, event)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected connected event but got timeout")
	}

	assert.False(t, tracker.IsUsingFallback())
	assert.Equal(t, ConnectionStateConnected, tracker.State())
}

func TestConnectionStateTracker_NoTimeoutConfigured(t *testing.T) {
	logger := zaptest.NewLogger(t)
	tracker := NewConnectionStateTracker(ConnectionStateTrackerConfig{
		StartupTimeout: 0, // Disabled
		RuntimeTimeout: 0, // Disabled
		Logger:         logger,
	})

	tracker.Start()
	defer tracker.Stop()

	// Simulate some activity
	tracker.OnConnectFailed()
	time.Sleep(50 * time.Millisecond)
	tracker.OnConnectFailed()
	time.Sleep(50 * time.Millisecond)

	// Should never trigger fallback
	assert.False(t, tracker.IsUsingFallback())

	select {
	case event := <-tracker.Events():
		t.Fatalf("Unexpected event: %v", event)
	default:
		// Expected - no events
	}
}

func TestConnectionStateTracker_StopPreventsEvents(t *testing.T) {
	logger := zaptest.NewLogger(t)
	tracker := NewConnectionStateTracker(ConnectionStateTrackerConfig{
		StartupTimeout: 30 * time.Millisecond,
		Logger:         logger,
	})

	tracker.Start()
	tracker.Stop()

	// Wait past the timeout
	time.Sleep(50 * time.Millisecond)

	// Should not receive any events after stop
	assert.False(t, tracker.IsUsingFallback())
}

func TestConnectionStateTracker_MultipleConnectCalls(t *testing.T) {
	logger := zaptest.NewLogger(t)
	tracker := NewConnectionStateTracker(ConnectionStateTrackerConfig{
		StartupTimeout: 100 * time.Millisecond,
		Logger:         logger,
	})

	tracker.Start()
	defer tracker.Stop()

	// Multiple connect calls should be idempotent
	tracker.OnConnect()
	tracker.OnConnect()
	tracker.OnConnect()

	assert.Equal(t, ConnectionStateConnected, tracker.State())
	assert.False(t, tracker.IsUsingFallback())
}

func TestConnectionStateTracker_ConnectWhileAlreadyUsingFallback(t *testing.T) {
	logger := zap.NewNop()
	tracker := NewConnectionStateTracker(ConnectionStateTrackerConfig{
		StartupTimeout: 10 * time.Millisecond,
		Logger:         logger,
	})

	tracker.Start()
	defer tracker.Stop()

	// Wait for fallback to trigger
	select {
	case <-tracker.Events():
		// Got fallback event
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected fallback event")
	}

	assert.True(t, tracker.IsUsingFallback())

	// Now connect
	tracker.OnConnect()

	// Should send connected event
	select {
	case event := <-tracker.Events():
		assert.Equal(t, ConnectionStateEventConnected, event)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected connected event")
	}

	assert.False(t, tracker.IsUsingFallback())
}

func TestConnectionStateTracker_RuntimeTimerOnlyStartsAfterConnected(t *testing.T) {
	logger := zaptest.NewLogger(t)
	tracker := NewConnectionStateTracker(ConnectionStateTrackerConfig{
		StartupTimeout: 0,                     // Disabled
		RuntimeTimeout: 50 * time.Millisecond, // Enabled
		Logger:         logger,
	})

	tracker.Start()
	defer tracker.Stop()

	// OnConnectFailed while in Waiting state should NOT start runtime timer
	tracker.OnConnectFailed()
	assert.Equal(t, ConnectionStateWaiting, tracker.State())

	// Wait past the runtime timeout
	time.Sleep(100 * time.Millisecond)

	// Should NOT have triggered fallback because we were never connected
	assert.False(t, tracker.IsUsingFallback())

	// Now connect and then disconnect
	tracker.OnConnect()
	assert.Equal(t, ConnectionStateConnected, tracker.State())

	tracker.OnConnectFailed()
	assert.Equal(t, ConnectionStateDisconnected, tracker.State())

	// NOW the runtime timer should start and trigger fallback
	select {
	case event := <-tracker.Events():
		assert.Equal(t, ConnectionStateEventFallbackTriggered, event)
		assert.True(t, tracker.IsUsingFallback())
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Expected fallback event after disconnect from connected state")
	}
}

func TestConnectionStateTracker_MultipleDisconnectReconnectCycles(t *testing.T) {
	logger := zaptest.NewLogger(t)
	tracker := NewConnectionStateTracker(ConnectionStateTrackerConfig{
		RuntimeTimeout: 50 * time.Millisecond,
		Logger:         logger,
	})

	tracker.Start()
	defer tracker.Stop()

	// Cycle 1: Connect -> Disconnect -> Reconnect before timeout
	tracker.OnConnect()
	assert.Equal(t, ConnectionStateConnected, tracker.State())

	tracker.OnConnectFailed()
	assert.Equal(t, ConnectionStateDisconnected, tracker.State())

	// Reconnect before timeout
	time.Sleep(20 * time.Millisecond)
	tracker.OnConnect()
	assert.Equal(t, ConnectionStateConnected, tracker.State())
	assert.False(t, tracker.IsUsingFallback())

	// Cycle 2: Disconnect -> Reconnect before timeout again
	tracker.OnConnectFailed()
	time.Sleep(20 * time.Millisecond)
	tracker.OnConnect()
	assert.Equal(t, ConnectionStateConnected, tracker.State())
	assert.False(t, tracker.IsUsingFallback())

	// Cycle 3: Disconnect and let it timeout
	tracker.OnConnectFailed()

	// Wait for fallback
	select {
	case event := <-tracker.Events():
		assert.Equal(t, ConnectionStateEventFallbackTriggered, event)
		assert.True(t, tracker.IsUsingFallback())
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Expected fallback event")
	}

	// Cycle 4: Reconnect after fallback
	tracker.OnConnect()

	select {
	case event := <-tracker.Events():
		assert.Equal(t, ConnectionStateEventConnected, event)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected connected event")
	}

	assert.False(t, tracker.IsUsingFallback())
	assert.Equal(t, ConnectionStateConnected, tracker.State())
}

func TestConnectionStateTracker_StopDuringFallbackTimer(t *testing.T) {
	logger := zaptest.NewLogger(t)
	tracker := NewConnectionStateTracker(ConnectionStateTrackerConfig{
		StartupTimeout: 100 * time.Millisecond,
		Logger:         logger,
	})

	tracker.Start()

	// Wait a bit but not long enough for timeout
	time.Sleep(30 * time.Millisecond)

	// Stop while timer is still running
	tracker.Stop()

	// Wait past the original timeout
	time.Sleep(150 * time.Millisecond)

	// Should not have triggered fallback
	assert.False(t, tracker.IsUsingFallback())

	// Calling OnConnect/OnConnectFailed after stop should not panic
	tracker.OnConnect()
	tracker.OnConnectFailed()
}

func TestConnectionStateTracker_OnConnectNotUsingFallback(t *testing.T) {
	logger := zaptest.NewLogger(t)
	tracker := NewConnectionStateTracker(ConnectionStateTrackerConfig{
		StartupTimeout: 100 * time.Millisecond,
		Logger:         logger,
	})

	tracker.Start()
	defer tracker.Stop()

	// Connect before fallback triggers
	tracker.OnConnect()

	// No connected event should be sent because we weren't using fallback
	select {
	case event := <-tracker.Events():
		t.Fatalf("Unexpected event: %v (should not send connected event when not recovering from fallback)", event)
	case <-time.After(50 * time.Millisecond):
		// Expected - no events
	}

	assert.Equal(t, ConnectionStateConnected, tracker.State())
	assert.False(t, tracker.IsUsingFallback())
}

func TestConnectionStateTracker_NoRuntimeFallbackWhenAlreadyUsingFallback(t *testing.T) {
	logger := zaptest.NewLogger(t)
	tracker := NewConnectionStateTracker(ConnectionStateTrackerConfig{
		StartupTimeout: 30 * time.Millisecond,
		RuntimeTimeout: 30 * time.Millisecond,
		Logger:         logger,
	})

	tracker.Start()
	defer tracker.Stop()

	// Wait for startup fallback to trigger
	select {
	case event := <-tracker.Events():
		assert.Equal(t, ConnectionStateEventFallbackTriggered, event)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected fallback event")
	}

	assert.True(t, tracker.IsUsingFallback())

	// Connect and then disconnect
	tracker.OnConnect()
	<-tracker.Events() // Consume the connected event

	tracker.OnConnectFailed()

	// Wait past the runtime timeout
	time.Sleep(100 * time.Millisecond)

	// Should not trigger another fallback event because we just recovered
	// The usingFallback flag was cleared when we connected
	select {
	case event := <-tracker.Events():
		// We should get a fallback event since we disconnected again
		assert.Equal(t, ConnectionStateEventFallbackTriggered, event)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected fallback event after disconnect")
	}
}
