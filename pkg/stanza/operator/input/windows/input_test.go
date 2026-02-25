// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windows // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"

import (
	"context"
	"errors"
	"math"
	"runtime"
	"testing"
	"time"
	"unsafe"

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

// TestInputStart_NoErrorIfIgnoreChannelErrorsEnabled ensures no error is thrown when ignore_channel_errors flag is enabled
// Other existing tests ensures the default behavior of error out when any error occurs while subscribing to the channel
func TestInputStart_NoErrorIfIgnoreChannelErrorEnabled(t *testing.T) {
	persister := testutil.NewMockPersister("")

	input := newTestInput()
	input.channel = "test-channel"
	input.startAt = "beginning"
	input.ignoreChannelErrors = true
	input.pollInterval = 1 * time.Second

	err := input.Start(persister)
	assert.NoError(t, err, "Expected no error when ignoreMissingChannel is true")
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

	originalEvtSubscribe := evtSubscribe
	defer func() { evtSubscribe = originalEvtSubscribe }()

	evtSubscribe = func(_ uintptr, _ windows.Handle, _, _ *uint16, _, _, _ uintptr, _ uint32) (uintptr, error) {
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

	originalEvtSubscribe := evtSubscribe
	defer func() { evtSubscribe = originalEvtSubscribe }()

	evtSubscribe = func(_ uintptr, _ windows.Handle, _, _ *uint16, _, _, _ uintptr, _ uint32) (uintptr, error) {
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

func TestInputStart_RemoteSessionWithDomain(t *testing.T) {
	persister := testutil.NewMockPersister("")

	// Mock EvtOpenSession to capture the login struct and verify Domain handling
	originalOpenSessionProc := openSessionProc
	var capturedDomain string
	var domainWasNil bool
	openSessionProc = MockProc{
		call: func(a ...uintptr) (uintptr, uintptr, error) {
			// a[0] = loginClass, a[1] = login pointer, a[2] = timeout, a[3] = flags
			if len(a) >= 4 && a[1] != 0 {
				capturedDomain = "remote-domain"
				domainWasNil = false
			} else {
				domainWasNil = true
			}
			return 1, 0, nil
		},
	}
	defer func() { openSessionProc = originalOpenSessionProc }()

	input := newTestInput()
	input.ignoreChannelErrors = true
	input.channel = "test-channel"
	input.startAt = "beginning"
	input.pollInterval = 1 * time.Second
	input.remote = RemoteConfig{
		Server:   "remote-server",
		Username: "test-user",
		Password: "test-pass",
		Domain:   "remote-domain",
	}

	err := input.Start(persister)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, input.Stop())
	})

	require.False(t, domainWasNil)
	require.Equal(t, "remote-domain", capturedDomain)
}

// TestInputRead_RPCInvalidBound tests that the Input handles RPC_S_INVALID_BOUND errors properly
func TestInputRead_RPCInvalidBound(t *testing.T) {
	// Save original procs and restore after test
	originalEvtNext := evtNext
	originalEvtClose := evtClose
	originalEvtSubscribe := evtSubscribe

	// Track calls to our mocked functions
	var nextCalls, closeCalls, subscribeCalls int

	// Mock the procs
	evtClose = func(_ uintptr) error {
		closeCalls++
		return nil
	}

	evtSubscribe = func(_ uintptr, _ windows.Handle, _, _ *uint16, _, _, _ uintptr, _ uint32) (uintptr, error) {
		subscribeCalls++
		return 42, nil
	}

	evtNext = func(_ uintptr, _ uint32, _ *uintptr, _, _ uint32, _ *uint32) error {
		nextCalls++
		if nextCalls == 1 {
			return windows.RPC_S_INVALID_BOUND
		}

		return nil
	}

	defer func() {
		evtNext = originalEvtNext
		evtClose = originalEvtClose
		evtSubscribe = originalEvtSubscribe
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
	input.read(t.Context())

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

	persister := testutil.NewMockPersister("")
	fake := testutil.NewFakeOutput(t)
	input.OutputOperators = []operator.Operator{fake}

	err := input.Start(persister)
	require.NoError(t, err)

	err = input.sendEvent(t.Context(), eventXML)
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
			"version":     uint8(0),
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

	persister := testutil.NewMockPersister("")
	fake := testutil.NewFakeOutput(t)
	input.OutputOperators = []operator.Operator{fake}

	err := input.Start(persister)
	require.NoError(t, err)

	err = input.sendEvent(t.Context(), eventXML)
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
			"version":     uint8(0),
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

// TestInputRead_Batching tests that the Input handles MaxEventsPerPoll and MaxReads correctly
func TestInputRead_Batching(t *testing.T) {
	originalEvtNext := evtNext
	originalEvtRender := evtRender
	originalEvtClose := evtClose
	originalEvtSubscribe := evtSubscribe
	originalCreateBookmarkProc := createBookmarkProc
	originalEvtUpdateBookmark := evtUpdateBookmark
	defer func() {
		evtNext = originalEvtNext
		evtRender = originalEvtRender
		evtClose = originalEvtClose
		evtSubscribe = originalEvtSubscribe
		createBookmarkProc = originalCreateBookmarkProc
		evtUpdateBookmark = originalEvtUpdateBookmark
	}()

	evtSubscribe = func(_ uintptr, _ windows.Handle, _, _ *uint16, _, _, _ uintptr, _ uint32) (uintptr, error) {
		return 42, nil
	}

	evtRender = func(_, _ uintptr, _, _ uint32, _ *byte) (*uint32, error) {
		bufferUsed := new(uint32)
		return bufferUsed, nil
	}

	evtClose = func(_ uintptr) error {
		return nil
	}

	createBookmarkProc = MockProc{
		call: func(_ ...uintptr) (uintptr, uintptr, error) {
			return 1, 0, nil
		},
	}

	evtUpdateBookmark = func(_, _ uintptr) error {
		return nil
	}

	var nextCalls, processedEvents, emittedEvents int

	maxEventsToEmit := -1

	mockBatch := make([]uintptr, 99)
	var pinner runtime.Pinner
	pinner.Pin(&mockBatch[0])
	defer pinner.Unpin()

	producedEvents := 0

	for i := range mockBatch {
		mockBatch[i] = uintptr(i)
	}

	evtNext = func(_ uintptr, eventsSize uint32, events *uintptr, _, _ uint32, returned *uint32) error {
		nextCalls++

		wantsToRead := int(eventsSize)
		producedEvents = min(len(mockBatch), wantsToRead)
		if maxEventsToEmit >= 0 {
			producedEvents = min(producedEvents, maxEventsToEmit-emittedEvents)
		}

		*returned = uint32(producedEvents)
		*events = uintptr(unsafe.Pointer(&mockBatch[0]))

		emittedEvents += producedEvents

		return nil
	}

	input := newTestInput()

	input.processEvent = func(_ context.Context, _ Event) error {
		processedEvents++
		return nil
	}

	input.buffer = NewBuffer()
	input.maxReads = len(mockBatch) - 10
	input.currentMaxReads = input.maxReads
	input.maxEventsPerPollCycle = 999

	input.subscription = Subscription{
		handle:        42,
		startAt:       "beginning",
		sessionHandle: 0,
		channel:       "test-channel",
	}

	input.read(t.Context())

	requiredNextCalls := int(math.Ceil(float64(input.maxEventsPerPollCycle) / float64(input.maxReads)))
	assert.Equal(t, input.maxEventsPerPollCycle, input.eventsReadInPollCycle)
	assert.Equal(t, requiredNextCalls, nextCalls)
	assert.Equal(t, input.maxEventsPerPollCycle, processedEvents)
	assert.Equal(t, input.currentMaxReads, input.maxReads)

	nextCalls = 0
	input.maxEventsPerPollCycle = 0
	input.eventsReadInPollCycle = 0
	emittedEvents = 0
	processedEvents = 0
	maxEventsToEmit = 420
	input.read(t.Context())

	// +1 is the 0 event stop call
	requiredNextCalls = int(math.Ceil(float64(maxEventsToEmit)/float64(input.maxReads))) + 1
	assert.Equal(t, requiredNextCalls, nextCalls)
	assert.Equal(t, maxEventsToEmit, processedEvents)
	assert.Equal(t, input.currentMaxReads, input.maxReads)
}
