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
	require.True(t, publisher.Valid())
}

func TestPublisherOpenInvalidUTF8(t *testing.T) {
	publisher := NewPublisher()
	invalidUTF8 := "\u0000"
	err := publisher.Open(invalidUTF8)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to convert the provider name \"\\x00\" to utf16: invalid argument")
	require.False(t, publisher.Valid())
}

func TestPublisherOpenSyscallFailure(t *testing.T) {
	publisher := NewPublisher()
	provider := "provider"
	defer mockWithDeferredRestore(&openPublisherMetadataProc, SimpleMockProc(0, 0, ErrorNotSupported))()
	err := publisher.Open(provider)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to open the metadata for the \"provider\" provider: The request is not supported.")
	require.False(t, publisher.Valid())
}

func TestPublisherOpenSuccess(t *testing.T) {
	publisher := NewPublisher()
	provider := "provider"
	defer mockWithDeferredRestore(&openPublisherMetadataProc, SimpleMockProc(5, 0, ErrorSuccess))()
	err := publisher.Open(provider)
	require.NoError(t, err)
	require.Equal(t, uintptr(5), publisher.handle)
	require.True(t, publisher.Valid())
}

func TestPublisherCloseWhenAlreadyClosed(t *testing.T) {
	publisher := NewPublisher()
	err := publisher.Close()
	require.NoError(t, err)
	require.False(t, publisher.Valid())
}

func TestPublisherCloseSyscallFailure(t *testing.T) {
	publisher := Publisher{handle: 5}
	defer mockWithDeferredRestore(&closeProc, SimpleMockProc(0, 0, ErrorNotSupported))()
	err := publisher.Close()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to close publisher")
	require.True(t, publisher.Valid())
}

func TestPublisherCloseSuccess(t *testing.T) {
	publisher := Publisher{handle: 5}
	originalCloseProc := closeProc
	closeProc = SimpleMockProc(1, 0, ErrorSuccess)
	defer func() { closeProc = originalCloseProc }()
	err := publisher.Close()
	require.NoError(t, err)
	require.Equal(t, uintptr(0), publisher.handle)
	require.False(t, publisher.Valid())
}

func mockWithDeferredRestore(call *SyscallProc, mockCall SyscallProc) func() {
	original := *call
	*call = mockCall
	return func() {
		*call = original
	}
}
