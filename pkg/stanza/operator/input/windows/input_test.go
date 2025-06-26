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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
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

// TestInputIncludeLogRecordOriginal tests that the log.record.original attribute is added when include_log_record_original is true
func TestInputIncludeLogRecordOriginal(t *testing.T) {
	input := newTestInput()
	input.includeLogRecordOriginal = true
	input.pollInterval = time.Second
	input.buffer = NewBuffer() // Initialize buffer

	// Create a mock event XML
	eventXML := &EventXML{
		Original: "<Event><System><Provider Name='TestProvider'/><EventID>1</EventID></System></Event>",
		TimeCreated: TimeCreated{
			SystemTime: "2024-01-01T00:00:00Z",
		},
	}

	ctx := context.Background()
	persister := testutil.NewMockPersister("")
	fake := testutil.NewFakeOutput(t)
	input.OutputOperators = []operator.Operator{fake}

	err := input.Start(persister)
	require.NoError(t, err)

	err = input.sendEvent(ctx, eventXML)
	require.NoError(t, err)

	expectedEntry := &entry.Entry{
		Timestamp: time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC),
		Body: map[string]any{
			"channel":    "",
			"computer":   "",
			"event_data": map[string]any{},
			"event_id": map[string]any{
				"id":         uint32(0),
				"qualifiers": uint16(0),
			},
			"keywords": []string(nil),
			"level":    "",
			"message":  "",
			"opcode":   "",
			"provider": map[string]any{
				"event_source": "",
				"guid":         "",
				"name":         "",
			},
			"record_id":   uint64(0),
			"system_time": "2024-01-01T00:00:00Z",
			"task":        "",
		},
		Attributes: map[string]any{
			"log.record.original": eventXML.Original,
		},
	}

	select {
	case actualEntry := <-fake.Received:
		actualEntry.ObservedTimestamp = time.Time{}
		assert.Equal(t, expectedEntry, actualEntry)
	case <-time.After(time.Second):
		require.FailNow(t, "Timed out waiting for entry")
	}

	err = input.Stop()
	require.NoError(t, err)
}

// TestInputIncludeLogRecordOriginalFalse tests that the log.record.original attribute is not added when include_log_record_original is false
func TestInputIncludeLogRecordOriginalFalse(t *testing.T) {
	input := newTestInput()
	input.includeLogRecordOriginal = false
	input.pollInterval = time.Second
	input.buffer = NewBuffer() // Initialize buffer

	// Create a mock event XML
	eventXML := &EventXML{
		Original: "<Event><System><Provider Name='TestProvider'/><EventID>1</EventID></System></Event>",
		TimeCreated: TimeCreated{
			SystemTime: "2024-01-01T00:00:00Z",
		},
	}

	ctx := context.Background()
	persister := testutil.NewMockPersister("")
	fake := testutil.NewFakeOutput(t)
	input.OutputOperators = []operator.Operator{fake}

	err := input.Start(persister)
	require.NoError(t, err)

	err = input.sendEvent(ctx, eventXML)
	require.NoError(t, err)

	expectedEntry := &entry.Entry{
		Timestamp: time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC),
		Body: map[string]any{
			"channel":    "",
			"computer":   "",
			"event_data": map[string]any{},
			"event_id": map[string]any{
				"id":         uint32(0),
				"qualifiers": uint16(0),
			},
			"keywords": []string(nil),
			"level":    "",
			"message":  "",
			"opcode":   "",
			"provider": map[string]any{
				"event_source": "",
				"guid":         "",
				"name":         "",
			},
			"record_id":   uint64(0),
			"system_time": "2024-01-01T00:00:00Z",
			"task":        "",
		},
		Attributes: nil,
	}

	// Verify that log.record.original attribute does not exist
	select {
	case actualEntry := <-fake.Received:
		actualEntry.ObservedTimestamp = time.Time{}
		assert.Equal(t, expectedEntry, actualEntry)
	case <-time.After(time.Second):
		require.FailNow(t, "Timed out waiting for entry")
	}

	err = input.Stop()
	require.NoError(t, err)
}
