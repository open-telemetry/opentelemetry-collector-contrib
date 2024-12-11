// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windows // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"golang.org/x/sys/windows"

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

	originalEvtSubscribeFunc := evtSubscribeFunc
	defer func() { evtSubscribeFunc = originalEvtSubscribeFunc }()

	evtSubscribeFunc = func(_ uintptr, _ windows.Handle, _ *uint16, _ *uint16, _ uintptr, _ uintptr, _ uintptr, _ uint32) (uintptr, error) {
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

	originalEvtSubscribeFunc := evtSubscribeFunc
	defer func() { evtSubscribeFunc = originalEvtSubscribeFunc }()

	evtSubscribeFunc = func(_ uintptr, _ windows.Handle, _ *uint16, _ *uint16, _ uintptr, _ uintptr, _ uintptr, _ uint32) (uintptr, error) {
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
