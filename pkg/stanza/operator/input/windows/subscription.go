// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windows // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"

import (
	"errors"
	"fmt"
	"syscall"

	"golang.org/x/sys/windows"
)

// Subscription is a subscription to a windows eventlog channel.
type Subscription struct {
	handle        uintptr
	Server        string
	startAt       string
	sessionHandle uintptr
	channel       string
	query         *string
	bookmark      Bookmark
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
	defer func() {
		_ = windows.CloseHandle(signalEvent)
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
	subscriptionHandle, err := evtSubscribeFunc(sessionHandle, signalEvent, channelPtr, queryPtr, bookmark.handle, 0, 0, flags)
	if err != nil {
		return fmt.Errorf("failed to subscribe to %s channel: %w", channel, err)
	}

	s.handle = subscriptionHandle
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

	if err := evtClose(s.handle); err != nil {
		return fmt.Errorf("failed to close subscription handle: %w", err)
	}

	s.handle = 0
	return nil
}

var errSubscriptionHandleNotOpen = errors.New("subscription handle is not open")

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
func (s *Subscription) createFlags(startAt string, bookmark Bookmark) uint32 {
	if bookmark.handle != 0 {
		return EvtSubscribeStartAfterBookmark
	}

	if startAt == "beginning" {
		return EvtSubscribeStartAtOldestRecord
	}

	return EvtSubscribeToFutureEvents
}

// NewRemoteSubscription will create a new remote subscription with an empty handle.
func NewRemoteSubscription(server string) Subscription {
	return Subscription{
		Server: server,
		handle: 0,
	}
}

// NewLocalSubscription will create a new local subscription with an empty handle.
func NewLocalSubscription() Subscription {
	return Subscription{
		Server: "",
		handle: 0,
	}
}
