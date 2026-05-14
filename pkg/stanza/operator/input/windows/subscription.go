// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windows // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"

import (
	"errors"
	"fmt"
	"syscall"

	"go.uber.org/multierr"
	"go.uber.org/zap"
	"golang.org/x/sys/windows"
)

// Subscription is a subscription to a windows eventlog channel.
type Subscription struct {
	handle        uintptr
	signalEvent   windows.Handle
	Server        string
	startAt       string
	sessionHandle uintptr
	channel       string
	query         *string
	bookmark      Bookmark
	logger        *zap.Logger
}

// Open will open the subscription handle.
// It returns an error if the subscription handle is already open or if any step in the process fails.
// If the remote server is not reachable, it returns an error indicating the failure.
func (s *Subscription) Open(startAt string, sessionHandle uintptr, channel string, query *string, bookmark Bookmark) error {
	if s.handle != 0 {
		return errors.New("subscription handle is already open")
	}

	signalEvent, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		return fmt.Errorf("failed to create signal handle: %w", err)
	}
	// Close the signal handle on any failure path below.
	closeSignalOnErr := true
	defer func() {
		if closeSignalOnErr {
			if closeErr := windows.CloseHandle(signalEvent); closeErr != nil && s.logger != nil {
				s.logger.Error("Failed to close signal event handle during subscription open failure", zap.Error(closeErr))
			}
		}
	}()

	if channel != "" && query != nil {
		return errors.New("can not supply both query and channel")
	}

	channelPtr, err := syscall.UTF16PtrFromString(channel)
	if err != nil {
		return fmt.Errorf("failed to convert channel to utf16: %w", err)
	}

	var queryPtr *uint16
	if query != nil {
		if queryPtr, err = syscall.UTF16PtrFromString(*query); err != nil {
			return fmt.Errorf("failed to convert XML query to utf16: %w", err)
		}
	}

	flags := s.createFlags(startAt, bookmark)
	subscriptionHandle, err := evtSubscribe(sessionHandle, signalEvent, channelPtr, queryPtr, bookmark.handle, 0, 0, flags)
	if err != nil {
		return fmt.Errorf("failed to subscribe to %s channel: %w", channel, err)
	}

	closeSignalOnErr = false // success — handle is now owned by the Subscription

	s.handle = subscriptionHandle
	s.signalEvent = signalEvent
	s.startAt = startAt
	s.sessionHandle = sessionHandle
	s.channel = channel
	s.bookmark = bookmark
	s.query = query
	return nil
}

// Close will close the subscription.
func (s *Subscription) Close() error {
	if s.handle == 0 {
		return nil
	}

	var errs error
	if err := evtClose(s.handle); err != nil {
		errs = multierr.Append(errs, fmt.Errorf("failed to close subscription handle: %w", err))
	} else {
		s.handle = 0
	}

	if s.signalEvent != 0 {
		if err := windows.CloseHandle(s.signalEvent); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("failed to close signal event handle: %w", err))
		} else {
			s.signalEvent = 0
		}
	}

	return errs
}

var errSubscriptionHandleNotOpen = errors.New("subscription handle is not open")

// Wait blocks until the subscription has new events, the cancel event is signaled, or the timeout
// elapses. Returns true if events may be available (subscription signal fired or timeout elapsed),
// false if the cancel event was signaled (caller should stop).
func (s *Subscription) Wait(cancelEvent windows.Handle, timeoutMs uint32) (bool, error) {
	if s.signalEvent == 0 {
		return false, errors.New("subscription signal handle is not open")
	}

	handles := []windows.Handle{s.signalEvent, cancelEvent}
	event, err := windows.WaitForMultipleObjects(handles, false, timeoutMs)
	if err != nil {
		return false, fmt.Errorf("failed to wait for subscription signal: %w", err)
	}

	// WAIT_OBJECT_0+1 means the cancel event (index 1) fired — caller should stop.
	if event == windows.WAIT_OBJECT_0+1 {
		return false, nil
	}

	// WAIT_OBJECT_0 (subscription signal) or WAIT_TIMEOUT (safety-net poll) — events may be available.
	return true, nil
}

func (s *Subscription) Read(maxReads int) ([]Event, int, error) {
	if s.handle == 0 {
		return nil, 0, errSubscriptionHandleNotOpen
	}

	if maxReads < 1 {
		return nil, 0, errors.New("max reads must be greater than 0")
	}

	events, actualMaxReads, err := s.readWithRetry(maxReads)
	if err != nil {
		return nil, 0, err
	}

	return events, actualMaxReads, nil
}

// readWithRetry will read events from the subscription with dynamic batch sizing if the RPC_S_INVALID_BOUND error occurs.
func (s *Subscription) readWithRetry(maxReads int) ([]Event, int, error) {
	eventHandles := make([]uintptr, maxReads)
	var eventsRead uint32

	err := evtNext(s.handle, uint32(maxReads), &eventHandles[0], 0, 0, &eventsRead)

	if errors.Is(err, ErrorInvalidOperation) && eventsRead == 0 {
		return nil, maxReads, nil
	}

	if errors.Is(err, windows.RPC_S_INVALID_BOUND) {
		// close current subscription
		if closeErr := s.Close(); closeErr != nil {
			return nil, maxReads, fmt.Errorf("failed to close subscription during recovery: %w", closeErr)
		}

		// reopen subscription with the same parameters
		if openErr := s.Open(s.startAt, s.sessionHandle, s.channel, s.query, s.bookmark); openErr != nil {
			return nil, maxReads, fmt.Errorf("failed to reopen subscription during recovery: %w", openErr)
		}

		// retry with half the batch size
		newMaxReads := max(maxReads/2, 1)
		return s.readWithRetry(newMaxReads)
	}

	if err != nil && !errors.Is(err, windows.ERROR_NO_MORE_ITEMS) {
		return nil, maxReads, err
	}

	events := make([]Event, 0, eventsRead)
	for _, eventHandle := range eventHandles[:eventsRead] {
		event := NewEvent(eventHandle)
		events = append(events, event)
	}

	return events, maxReads, nil
}

// createFlags will create the necessary subscription flags from the supplied arguments.
func (*Subscription) createFlags(startAt string, bookmark Bookmark) uint32 {
	if bookmark.handle != 0 {
		return EvtSubscribeStartAfterBookmark
	}

	if startAt == "beginning" {
		return EvtSubscribeStartAtOldestRecord
	}

	return EvtSubscribeToFutureEvents
}

// NewRemoteSubscription will create a new remote subscription with an empty handle.
func NewRemoteSubscription(server string, logger *zap.Logger) Subscription {
	return Subscription{
		Server: server,
		handle: 0,
		logger: logger,
	}
}

// NewLocalSubscription will create a new local subscription with an empty handle.
func NewLocalSubscription(logger *zap.Logger) Subscription {
	return Subscription{
		Server: "",
		handle: 0,
		logger: logger,
	}
}
