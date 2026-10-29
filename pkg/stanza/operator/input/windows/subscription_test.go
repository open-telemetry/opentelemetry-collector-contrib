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
