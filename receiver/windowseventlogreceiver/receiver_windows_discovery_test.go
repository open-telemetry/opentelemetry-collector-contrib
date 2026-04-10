// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windowseventlogreceiver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	stanzawindows "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver/internal/metadata"
)

// mockLogsReceiver is a test double for receiver.Logs.
type mockLogsReceiver struct {
	startErr    error
	shutdownErr error
	started     bool
	shutdown    bool
}

func (m *mockLogsReceiver) Start(_ context.Context, _ component.Host) error {
	if m.startErr != nil {
		return m.startErr
	}
	m.started = true
	return nil
}

func (m *mockLogsReceiver) Shutdown(_ context.Context) error {
	m.shutdown = true
	return m.shutdownErr
}

var _ receiver.Logs = (*mockLogsReceiver)(nil)

func restoreDomainControllersFunc(t *testing.T) {
	t.Helper()
	original := getDomainControllersRemoteConfig
	t.Cleanup(func() {
		getDomainControllersRemoteConfig = original
	})
}

func TestCreateLogsReceiverWithDomainControllerDiscovery(t *testing.T) {
	restoreDomainControllersFunc(t)

	discoveredDCs := []stanzawindows.RemoteConfig{
		{Server: "dc1.example.com", Username: "user", Password: "pass", Domain: "example.com"},
		{Server: "dc2.example.com", Username: "user", Password: "pass", Domain: "example.com"},
	}
	getDomainControllersRemoteConfig = func(_ *zap.Logger, _, _ string) ([]stanzawindows.RemoteConfig, error) {
		return discoveredDCs, nil
	}

	_ = featuregate.GlobalRegistry().Set(metadata.DomainControllersAutodiscoveryFeatureGate.ID(), true)
	cfg := createTestConfig()
	cfg.DiscoverDomainControllers = true
	cfg.InputConfig.Remote = stanzawindows.RemoteConfig{
		Username: "user",
		Password: "pass",
	}
	sink := new(consumertest.LogsSink)

	rcvr, err := newFactoryAdapter().CreateLogs(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		sink,
	)
	require.NoError(t, err)

	multi, ok := rcvr.(*multiLogsReceiver)
	require.True(t, ok, "expected a *multiLogsReceiver when DiscoverDomainControllers is true")
	assert.Len(t, multi.receivers, len(discoveredDCs))
	_ = featuregate.GlobalRegistry().Set(metadata.DomainControllersAutodiscoveryFeatureGate.ID(), false)
}

func TestCreateLogsReceiverDomainControllerDiscoveryError(t *testing.T) {
	_ = featuregate.GlobalRegistry().Set(metadata.DomainControllersAutodiscoveryFeatureGate.ID(), true)
	restoreDomainControllersFunc(t)

	discoveryErr := errors.New("ldap connection refused")
	getDomainControllersRemoteConfig = func(_ *zap.Logger, _, _ string) ([]stanzawindows.RemoteConfig, error) {
		return nil, discoveryErr
	}

	cfg := createTestConfig()
	cfg.DiscoverDomainControllers = true
	sink := new(consumertest.LogsSink)

	_, err := newFactoryAdapter().CreateLogs(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		sink,
	)
	require.Error(t, err)
	assert.ErrorContains(t, err, "domain controller discovery failed")
	assert.ErrorIs(t, err, discoveryErr)
	_ = featuregate.GlobalRegistry().Set(metadata.DomainControllersAutodiscoveryFeatureGate.ID(), false)
}

func TestMultiLogsReceiverStartShutdown(t *testing.T) {
	r1 := &mockLogsReceiver{}
	r2 := &mockLogsReceiver{}
	multi := &multiLogsReceiver{receivers: []receiver.Logs{r1, r2}}

	require.NoError(t, multi.Start(t.Context(), componenttest.NewNopHost()))
	assert.True(t, r1.started)
	assert.True(t, r2.started)

	require.NoError(t, multi.Shutdown(t.Context()))
	assert.True(t, r1.shutdown)
	assert.True(t, r2.shutdown)
}

func TestMultiLogsReceiverStartFailureRollback(t *testing.T) {
	r1 := &mockLogsReceiver{}
	startErr := errors.New("start failed")
	r2 := &mockLogsReceiver{startErr: startErr}
	multi := &multiLogsReceiver{receivers: []receiver.Logs{r1, r2}}

	err := multi.Start(t.Context(), componenttest.NewNopHost())
	require.Error(t, err)
	assert.ErrorIs(t, err, startErr)

	// r1 was started successfully before r2 failed, so it must be shut down during rollback.
	assert.True(t, r1.started)
	assert.True(t, r1.shutdown)
	// r2 never started.
	assert.False(t, r2.started)
}

func TestMultiLogsReceiverShutdownAggregatesErrors(t *testing.T) {
	err1 := errors.New("shutdown error 1")
	err2 := errors.New("shutdown error 2")
	r1 := &mockLogsReceiver{shutdownErr: err1}
	r2 := &mockLogsReceiver{shutdownErr: err2}
	multi := &multiLogsReceiver{receivers: []receiver.Logs{r1, r2}}

	err := multi.Shutdown(t.Context())
	require.Error(t, err)
	assert.ErrorIs(t, err, err1)
	assert.ErrorIs(t, err, err2)
}
