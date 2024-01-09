// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package windows

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetValidPublisher(t *testing.T) {
	publisherCache := newPublisherCache()
	defer func() {
		require.NoError(t, publisherCache.evictAll())
	}()

	// Provider "Application" exists in all Windows versions.
	publisher, openPublisherErr := publisherCache.get("Application")
	require.NoError(t, openPublisherErr)
	require.True(t, publisher.Valid())

	// Get the same publisher again.
	publisher, openPublisherErr = publisherCache.get("Application")
	require.NoError(t, openPublisherErr)
	require.True(t, publisher.Valid())
}

func TestGetInvalidPublisher(t *testing.T) {
	publisherCache := newPublisherCache()
	defer func() {
		require.NoError(t, publisherCache.evictAll())
	}()

	// Provider "InvalidProvider" does not exist in any Windows version.
	publisher, openPublisherErr := publisherCache.get("InvalidProvider")
	require.Error(t, openPublisherErr, "%v", publisherCache)
	require.False(t, publisher.Valid())

	// Get "InvalidProvider" publisher again.
	publisher, openPublisherErr = publisherCache.get("InvalidProvider")
	require.NoError(t, openPublisherErr) // It is cached, no error opening it.
	require.False(t, publisher.Valid())
}

func TestValidAndInvalidPublishers(t *testing.T) {
	publisherCache := newPublisherCache()
	defer func() {
		require.NoError(t, publisherCache.evictAll())
	}()

	// Provider "Application" exists in all Windows versions.
	publisher, openPublisherErr := publisherCache.get("Application")
	require.NoError(t, openPublisherErr)
	require.True(t, publisher.Valid())

	// Provider "InvalidProvider" does not exist in any Windows version.
	publisher, openPublisherErr = publisherCache.get("InvalidProvider")
	require.Error(t, openPublisherErr, "%v", publisherCache)
	require.False(t, publisher.Valid())

	// Get the existing publisher again.
	publisher, openPublisherErr = publisherCache.get("Application")
	require.NoError(t, openPublisherErr)
	require.True(t, publisher.Valid())

	// Get "InvalidProvider" publisher again.
	publisher, openPublisherErr = publisherCache.get("InvalidProvider")
	require.NoError(t, openPublisherErr) // It is cached, no error opening it.
	require.False(t, publisher.Valid())
}
