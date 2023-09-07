// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package windows

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEventCloseWhenAlreadyClosed(t *testing.T) {
	event := NewEvent(0)
	err := event.Close()
	require.NoError(t, err)
}

func TestEventCloseSyscallFailure(t *testing.T) {
	event := NewEvent(5)
	closeProc = SimpleMockProc(0, 0, ErrorNotSupported)
	err := event.Close()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to close event handle")
}

func TestEventCloseSuccess(t *testing.T) {
	event := NewEvent(5)
	closeProc = SimpleMockProc(1, 0, ErrorSuccess)
	err := event.Close()
	require.NoError(t, err)
	require.Equal(t, uintptr(0), event.handle)
}
