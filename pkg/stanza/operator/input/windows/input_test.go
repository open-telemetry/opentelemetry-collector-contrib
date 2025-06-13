// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windows // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"golang.org/x/sys/windows"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func newTestInput() *Input {
	return newInput(component.TelemetrySettings{
		Logger: zap.NewNop(),
	})
}

// TestInputCreate_Stop ensures the input correctly shuts down even if it was never started.
func TestInputCreate_Stop(t *testing.T) {
	input := newTestInput()
	assert.NoError(t, input.Stop())
}

// TestInputStart_LocalSubscriptionError ensures the input correctly handles local subscription errors.
func TestInputStart_LocalSubscriptionError(t *testing.T) {
	persister := testutil.NewMockPersister("")

	input := newTestInput()
	input.channel = "test-channel"
	input.startAt = "beginning"
	input.pollInterval = 1 * time.Second

	err := input.Start(persister)
	assert.ErrorContains(t, err, "The specified channel could not be found")
}

// TestInputStart_RemoteSubscriptionError ensures the input correctly handles remote subscription errors.
func TestInputStart_RemoteSubscriptionError(t *testing.T) {
	persister := testutil.NewMockPersister("")

	input := newTestInput()
	input.startRemoteSession = func() error { return nil }
	input.channel = "test-channel"
	input.startAt = "beginning"
	input.pollInterval = 1 * time.Second
	input.remote = RemoteConfig{
		Server: "remote-server",
	}

	err := input.Start(persister)
	assert.ErrorContains(t, err, "The specified channel could not be found")
}

// TestInputStart_RemoteSessionError ensures the input correctly handles remote session errors.
func TestInputStart_RemoteSessionError(t *testing.T) {
	persister := testutil.NewMockPersister("")

	input := newTestInput()
	input.startRemoteSession = func() error {
		return errors.New("remote session error")
	}
	input.channel = "test-channel"
	input.startAt = "beginning"
	input.pollInterval = 1 * time.Second
	input.remote = RemoteConfig{
		Server: "remote-server",
	}

	err := input.Start(persister)
	assert.ErrorContains(t, err, "failed to start remote session for server remote-server: remote session error")
}

// TestInputStart_RemoteAccessDeniedError ensures the input correctly handles remote access denied errors.
func TestInputStart_RemoteAccessDeniedError(t *testing.T) {
	persister := testutil.NewMockPersister("")

	originalEvtSubscribeFunc := evtSubscribeFunc
	defer func() { evtSubscribeFunc = originalEvtSubscribeFunc }()

	evtSubscribeFunc = func(_ uintptr, _ windows.Handle, _ *uint16, _ *uint16, _ uintptr, _ uintptr, _ uintptr, _ uint32) (uintptr, error) {
		return 0, windows.ERROR_ACCESS_DENIED
	}

	input := newTestInput()
	input.startRemoteSession = func() error { return nil }
	input.channel = "test-channel"
	input.startAt = "beginning"
	input.pollInterval = 1 * time.Second
	input.remote = RemoteConfig{
		Server: "remote-server",
	}

	err := input.Start(persister)
	assert.ErrorContains(t, err, "failed to open subscription for remote server")
	assert.ErrorContains(t, err, "Access is denied")
}

// TestInputStart_BadChannelName ensures the input correctly handles bad channel names.
func TestInputStart_BadChannelName(t *testing.T) {
	persister := testutil.NewMockPersister("")

	originalEvtSubscribeFunc := evtSubscribeFunc
	defer func() { evtSubscribeFunc = originalEvtSubscribeFunc }()

	evtSubscribeFunc = func(_ uintptr, _ windows.Handle, _ *uint16, _ *uint16, _ uintptr, _ uintptr, _ uintptr, _ uint32) (uintptr, error) {
		return 0, windows.ERROR_EVT_CHANNEL_NOT_FOUND
	}

	input := newTestInput()
	input.startRemoteSession = func() error { return nil }
	input.channel = "bad-channel"
	input.startAt = "beginning"
	input.pollInterval = 1 * time.Second
	input.remote = RemoteConfig{
		Server: "remote-server",
	}

	err := input.Start(persister)
	assert.ErrorContains(t, err, "failed to open subscription for remote server")
	assert.ErrorContains(t, err, "The specified channel could not be found")
}

// TestInputRead_RPCInvalidBound tests that the Input handles RPC_S_INVALID_BOUND errors properly
func TestInputRead_RPCInvalidBound(t *testing.T) {
	// Save original procs and restore after test
	originalNextProc := nextProc
	originalCloseProc := closeProc
	originalSubscribeProc := subscribeProc

	// Track calls to our mocked functions
	var nextCalls, closeCalls, subscribeCalls int

	// Mock the procs
	closeProc = MockProc{
		call: func(_ ...uintptr) (uintptr, uintptr, error) {
			closeCalls++
			return 1, 0, nil
		},
	}

	subscribeProc = MockProc{
		call: func(_ ...uintptr) (uintptr, uintptr, error) {
			subscribeCalls++
			return 42, 0, nil
		},
	}

	nextProc = MockProc{
		call: func(_ ...uintptr) (uintptr, uintptr, error) {
			nextCalls++
			if nextCalls == 1 {
				return 0, 0, windows.RPC_S_INVALID_BOUND
			}

			return 1, 0, nil
		},
	}

	defer func() {
		nextProc = originalNextProc
		closeProc = originalCloseProc
		subscribeProc = originalSubscribeProc
	}()

	// Create a logger with an observer for testing log output
	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)

	// Create input instance with mocked dependencies
	input := newInput(component.TelemetrySettings{
		Logger: logger,
	})

	// Set up test values
	input.maxReads = 100
	input.currentMaxReads = 100

	// Set up subscription with valid handle and enough info to reopen
	input.subscription = Subscription{
		handle:        42, // Dummy handle
		startAt:       "beginning",
		sessionHandle: 0,
		channel:       "test-channel",
	}

	// Call the method under test
	ctx := context.Background()
	input.read(ctx)

	// Verify the correct number of calls to each mock
	assert.Equal(t, 2, nextCalls, "nextProc should be called twice (initial failure and retry)")
	assert.Equal(t, 1, closeCalls, "closeProc should be called once to close subscription")
	assert.Equal(t, 1, subscribeCalls, "subscribeProc should be called once to reopen subscription")

	// Verify that batch size was reduced
	assert.Equal(t, 50, input.currentMaxReads)

	// Verify that a warning log was generated
	require.Equal(t, 1, logs.Len())
	assert.Contains(t, logs.All()[0].Message, "Encountered RPC_S_INVALID_BOUND")
}
