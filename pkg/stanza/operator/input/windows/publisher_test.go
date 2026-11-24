// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windows

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublisherOpenPreexisting(t *testing.T) {
	publisher := Publisher{handle: 5}
	err := publisher.Open("provider_name_does_not_matter_for_this_test", nil)
	require.ErrorContains(t, err, "publisher handle is already open")
	require.True(t, publisher.Valid())
}

func TestPublisherOpenInvalidUTF8(t *testing.T) {
	publisher := NewPublisher()
	invalidUTF8 := "\u0000"
	err := publisher.Open(invalidUTF8, nil)
	require.ErrorContains(t, err, "failed to convert the provider name \"\\x00\" to utf16: invalid argument")
	require.False(t, publisher.Valid())
}

func TestPublisherOpenSyscallFailure(t *testing.T) {
	publisher := NewPublisher()
	provider := "provider"
	t.Cleanup(mockWithDeferredRestore(&evtOpenPublisherMetadata, func(uintptr, *uint16, *uint16, uint32, uint32) (uintptr, error) { return 0, ErrorNotSupported }))
	err := publisher.Open(provider, nil)
	require.ErrorContains(t, err, "failed to open the metadata for the \"provider\" provider: The request is not supported.")
	require.False(t, publisher.Valid())
}

func TestPublisherOpenSuccess(t *testing.T) {
	publisher := NewPublisher()
	provider := "provider"
	t.Cleanup(mockWithDeferredRestore(&evtOpenPublisherMetadata, func(uintptr, *uint16, *uint16, uint32, uint32) (uintptr, error) { return 5, nil }))
	err := publisher.Open(provider, nil)
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
	t.Cleanup(mockWithDeferredRestore(&evtClose, func(uintptr) error { return ErrorNotSupported }))
	err := publisher.Close()
	require.ErrorContains(t, err, "failed to close publisher")
	require.True(t, publisher.Valid())
}

func TestPublisherCloseSuccess(t *testing.T) {
	publisher := Publisher{handle: 5}
	t.Cleanup(mockWithDeferredRestore(&evtClose, func(uintptr) error { return nil }))
	err := publisher.Close()
	require.NoError(t, err)
	require.Equal(t, uintptr(0), publisher.handle)
	require.False(t, publisher.Valid())
}

// mockWithDeferredRestore swaps *target for mock and returns a func that puts the original back.
func mockWithDeferredRestore[T any](target *T, mock T) func() {
	original := *target
	*target = mock
	return func() {
		*target = original
	}
}
