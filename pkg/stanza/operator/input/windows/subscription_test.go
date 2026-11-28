// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windows

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// TestSubscriptionOpen_TolerateQueryErrors ensures a query subscription is retried with
// EvtSubscribeTolerateQueryErrors only when requested, and only after the strict subscription failed.
func TestSubscriptionOpen_TolerateQueryErrors(t *testing.T) {
	originalEvtSubscribe := evtSubscribe
	defer func() { evtSubscribe = originalEvtSubscribe }()

	var gotFlags []uint32
	evtSubscribe = func(_ uintptr, _ windows.Handle, _, _ *uint16, _, _, _ uintptr, flags uint32) (uintptr, error) {
		gotFlags = append(gotFlags, flags)
		if flags&EvtSubscribeTolerateQueryErrors == 0 {
			return 0, windows.ERROR_EVT_CHANNEL_NOT_FOUND
		}
		return 42, nil
	}

	query := `<QueryList><Query Id="0"><Select Path="Application">*</Select><Select Path="no-such-channel">*</Select></Query></QueryList>`
	for _, tc := range []struct {
		name     string
		channel  string
		query    *string
		startAt  string
		bookmark Bookmark
		tolerate bool
		expected []uint32
	}{
		{name: "strict query", query: &query, expected: []uint32{EvtSubscribeToFutureEvents}},
		{name: "tolerant channel", channel: "no-such-channel", tolerate: true, expected: []uint32{EvtSubscribeToFutureEvents}},
		{name: "tolerant query", query: &query, tolerate: true, expected: []uint32{EvtSubscribeToFutureEvents, EvtSubscribeToFutureEvents | EvtSubscribeTolerateQueryErrors}},
		{name: "tolerant query from beginning", query: &query, startAt: "beginning", tolerate: true, expected: []uint32{EvtSubscribeStartAtOldestRecord, EvtSubscribeStartAtOldestRecord | EvtSubscribeTolerateQueryErrors}},
		{name: "tolerant query after bookmark", query: &query, bookmark: Bookmark{handle: 1}, tolerate: true, expected: []uint32{EvtSubscribeStartAfterBookmark, EvtSubscribeStartAfterBookmark | EvtSubscribeTolerateQueryErrors}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotFlags = nil
			sub := NewLocalSubscription(nil)
			err := sub.Open(tc.startAt, 0, tc.channel, tc.query, nil, tc.bookmark, tc.tolerate)
			assert.Equal(t, tc.expected, gotFlags)
			if len(tc.expected) == 1 {
				require.ErrorIs(t, err, windows.ERROR_EVT_CHANNEL_NOT_FOUND)
				return
			}
			require.NoError(t, err)

			// The mocked handle must not reach the real EvtClose.
			sub.handle = 0
			require.NoError(t, windows.CloseHandle(sub.signalEvent))
		})
	}
}

// TestSubscriptionRead_SurfacesRPCInvalidBound ensures Subscription.Read returns
// RPC_S_INVALID_BOUND directly; recovery is the caller's responsibility.
func TestSubscriptionRead_SurfacesRPCInvalidBound(t *testing.T) {
	originalEvtNext := evtNext
	defer func() { evtNext = originalEvtNext }()

	evtNext = func(_ uintptr, _ uint32, _ *uintptr, _, _ uint32, _ *uint32) error {
		return windows.RPC_S_INVALID_BOUND
	}

	sub := Subscription{handle: 42}
	_, err := sub.Read(10)
	require.ErrorIs(t, err, windows.RPC_S_INVALID_BOUND)
}

// TestSubscriptionRead_TreatsErrorNoMoreItemsAsEOF ensures Subscription.Read treats
// ERROR_NO_MORE_ITEMS as a normal end-of-batch signal, returning any events that were
// read before the error without propagating the error itself.
func TestSubscriptionRead_TreatsErrorNoMoreItemsAsEOF(t *testing.T) {
	originalEvtNext := evtNext
	defer func() { evtNext = originalEvtNext }()

	evtNext = func(_ uintptr, _ uint32, events *uintptr, _, _ uint32, eventsRead *uint32) error {
		*eventsRead = 2
		*events = 1 // non-zero dummy handle
		return windows.ERROR_NO_MORE_ITEMS
	}

	sub := Subscription{handle: 42}
	events, err := sub.Read(10)
	require.NoError(t, err)
	assert.Len(t, events, 2)
}

// TestSubscriptionRead_ReturnsNilOnErrorInvalidOperation ensures Subscription.Read returns
// nil when evtNext reports no operation is in progress (no events available).
func TestSubscriptionRead_ReturnsNilOnErrorInvalidOperation(t *testing.T) {
	originalEvtNext := evtNext
	defer func() { evtNext = originalEvtNext }()

	evtNext = func(_ uintptr, _ uint32, _ *uintptr, _, _ uint32, eventsRead *uint32) error {
		*eventsRead = 0
		return ErrorInvalidOperation
	}

	sub := Subscription{handle: 42}
	events, err := sub.Read(10)
	require.NoError(t, err)
	assert.Empty(t, events)
}
