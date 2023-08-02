// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package windows

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublisherOpenPreexisting(t *testing.T) {
	publisher := Publisher{handle: 5}
	err := publisher.Open("")
	require.Error(t, err)
	require.Contains(t, err.Error(), "publisher handle is already open")
}

func TestPublisherOpenInvalidUTF8(t *testing.T) {
	publisher := NewPublisher()
	invalidUTF8 := "\u0000"
	err := publisher.Open(invalidUTF8)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to convert provider to utf16")
}

func TestPublisherOpenSyscallFailure(t *testing.T) {
	publisher := NewPublisher()
	provider := "provider"
	openPublisherMetadataProc = SimpleMockProc(0, 0, ErrorNotSupported)
	err := publisher.Open(provider)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to open publisher handle")
}

func TestPublisherOpenSuccess(t *testing.T) {
	publisher := NewPublisher()
	provider := "provider"
	openPublisherMetadataProc = SimpleMockProc(5, 0, ErrorSuccess)
	err := publisher.Open(provider)
	require.NoError(t, err)
	require.Equal(t, uintptr(5), publisher.handle)
}

func TestPublisherCloseWhenAlreadyClosed(t *testing.T) {
	publisher := NewPublisher()
	err := publisher.Close()
	require.NoError(t, err)
}

func TestPublisherCloseSyscallFailure(t *testing.T) {
	publisher := Publisher{handle: 5}
	closeProc = SimpleMockProc(0, 0, ErrorNotSupported)
	err := publisher.Close()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to close publisher")
}

func TestPublisherCloseSuccess(t *testing.T) {
	publisher := Publisher{handle: 5}
	closeProc = SimpleMockProc(1, 0, ErrorSuccess)
	err := publisher.Close()
	require.NoError(t, err)
	require.Equal(t, uintptr(0), publisher.handle)
}
