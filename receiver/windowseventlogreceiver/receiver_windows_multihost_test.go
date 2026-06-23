// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windowseventlogreceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/consumerretry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	stanzawindows "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver/internal/metadata"
)

func TestExpandMultipleHosts(t *testing.T) {
	tests := []struct {
		name     string
		remote   stanzawindows.RemoteConfig
		expected []stanzawindows.RemoteConfig
	}{
		{
			name: "single_group_shared_credentials",
			remote: stanzawindows.RemoteConfig{
				Username: "admin",
				Password: "secret",
				Domain:   "example.com",
				Hosts: []stanzawindows.HostGroupConfig{
					{Hosts: []string{"host1", "host2", "host3"}},
				},
			},
			expected: []stanzawindows.RemoteConfig{
				{Server: "host1", Username: "admin", Password: "secret", Domain: "example.com"},
				{Server: "host2", Username: "admin", Password: "secret", Domain: "example.com"},
				{Server: "host3", Username: "admin", Password: "secret", Domain: "example.com"},
			},
		},
		{
			name: "multiple_groups_with_override",
			remote: stanzawindows.RemoteConfig{
				Username: "admin",
				Password: "secret",
				Domain:   "example.com",
				Hosts: []stanzawindows.HostGroupConfig{
					{Hosts: []string{"host1", "host3"}},
					{Hosts: []string{"host2", "host4"}, Username: "root", Password: "rootsecret"},
				},
			},
			expected: []stanzawindows.RemoteConfig{
				{Server: "host1", Username: "admin", Password: "secret", Domain: "example.com"},
				{Server: "host3", Username: "admin", Password: "secret", Domain: "example.com"},
				{Server: "host2", Username: "root", Password: "rootsecret", Domain: "example.com"},
				{Server: "host4", Username: "root", Password: "rootsecret", Domain: "example.com"},
			},
		},
		{
			name: "partial_credential_override",
			remote: stanzawindows.RemoteConfig{
				Username: "admin",
				Password: "secret",
				Hosts: []stanzawindows.HostGroupConfig{
					{Hosts: []string{"host1"}, Password: "newpass"},
				},
			},
			expected: []stanzawindows.RemoteConfig{
				{Server: "host1", Username: "admin", Password: "newpass"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandMultipleHosts(tt.remote)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidate_MutualExclusion(t *testing.T) {
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ReceiverWindowseventlogMultipleRemoteHostsFeatureGate.ID(), true))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ReceiverWindowseventlogMultipleRemoteHostsFeatureGate.ID(), false))
	}()

	cfg := createTestConfig()
	cfg.InputConfig.Remote = stanzawindows.RemoteConfig{
		Server:   "host1",
		Username: "admin",
		Password: "secret",
		Hosts: []stanzawindows.HostGroupConfig{
			{Hosts: []string{"host2"}},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestValidate_FeatureGateRequired(t *testing.T) {
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ReceiverWindowseventlogMultipleRemoteHostsFeatureGate.ID(), false))

	cfg := createTestConfig()
	cfg.InputConfig.Remote = stanzawindows.RemoteConfig{
		Username: "admin",
		Password: "secret",
		Hosts: []stanzawindows.HostGroupConfig{
			{Hosts: []string{"host1"}},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "feature gate")
}

func TestValidate_EmptyHostGroup(t *testing.T) {
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ReceiverWindowseventlogMultipleRemoteHostsFeatureGate.ID(), true))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ReceiverWindowseventlogMultipleRemoteHostsFeatureGate.ID(), false))
	}()

	cfg := createTestConfig()
	cfg.InputConfig.Remote = stanzawindows.RemoteConfig{
		Username: "admin",
		Password: "secret",
		Hosts: []stanzawindows.HostGroupConfig{
			{Hosts: []string{}},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must contain at least one host")
}

func TestValidate_MissingCredentials(t *testing.T) {
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ReceiverWindowseventlogMultipleRemoteHostsFeatureGate.ID(), true))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ReceiverWindowseventlogMultipleRemoteHostsFeatureGate.ID(), false))
	}()

	cfg := createTestConfig()
	cfg.InputConfig.Remote = stanzawindows.RemoteConfig{
		Hosts: []stanzawindows.HostGroupConfig{
			{Hosts: []string{"host1"}},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-empty credentials")
}

func TestValidate_ValidMultiHostConfig(t *testing.T) {
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ReceiverWindowseventlogMultipleRemoteHostsFeatureGate.ID(), true))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ReceiverWindowseventlogMultipleRemoteHostsFeatureGate.ID(), false))
	}()

	cfg := createTestConfig()
	cfg.InputConfig.Remote = stanzawindows.RemoteConfig{
		Username: "admin",
		Password: "secret",
		Hosts: []stanzawindows.HostGroupConfig{
			{Hosts: []string{"host1", "host2"}},
			{Hosts: []string{"host3"}, Username: "other", Password: "pass"},
		},
	}

	err := cfg.Validate()
	require.NoError(t, err)
}

func TestCreateLogsReceiver_MultipleHosts(t *testing.T) {
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ReceiverWindowseventlogMultipleRemoteHostsFeatureGate.ID(), true))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ReceiverWindowseventlogMultipleRemoteHostsFeatureGate.ID(), false))
	}()

	cfg := createTestConfig()
	cfg.InputConfig.Remote = stanzawindows.RemoteConfig{
		Username: "admin",
		Password: "secret",
		Hosts: []stanzawindows.HostGroupConfig{
			{Hosts: []string{"host1", "host2"}},
			{Hosts: []string{"host3"}, Username: "root", Password: "rootpass"},
		},
	}
	sink := new(consumertest.LogsSink)

	rcvr, err := NewFactory().CreateLogs(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		sink,
	)
	require.NoError(t, err)

	multi, ok := rcvr.(*multiLogsReceiver)
	require.True(t, ok, "expected a *multiLogsReceiver when multiple hosts are configured")
	assert.Len(t, multi.receivers, 3)
}

func TestLoadConfigMultiHost(t *testing.T) {
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ReceiverWindowseventlogMultipleRemoteHostsFeatureGate.ID(), true))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ReceiverWindowseventlogMultipleRemoteHostsFeatureGate.ID(), false))
	}()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_multi_host.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	expected := &WindowsLogConfig{
		BaseConfig: adapter.BaseConfig{
			Operators:      []operator.Config{},
			RetryOnFailure: consumerretry.NewDefaultConfig(),
		},
		InputConfig: func() stanzawindows.Config {
			c := stanzawindows.NewConfig()
			c.Channel = "security"
			c.StartAt = "end"
			c.Remote = stanzawindows.RemoteConfig{
				Username: "admin",
				Password: "secret",
				Hosts: []stanzawindows.HostGroupConfig{
					{Hosts: []string{"host1", "host3"}},
					{Hosts: []string{"host2", "host4"}, Username: "root", Password: "rootsecret"},
				},
			}
			return *c
		}(),
	}
	assert.Equal(t, expected, cfg)
}
