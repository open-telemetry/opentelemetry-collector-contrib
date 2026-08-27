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
	"go.opentelemetry.io/collector/config/configopaque"
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
			name: "multiple_servers_shared_credentials",
			remote: stanzawindows.RemoteConfig{
				Servers:  []string{"host1", "host2", "host3"},
				Username: "admin",
				Password: "secret",
				Domain:   "example.com",
			},
			expected: []stanzawindows.RemoteConfig{
				{Server: "host1", Username: "admin", Password: "secret", Domain: "example.com"},
				{Server: "host2", Username: "admin", Password: "secret", Domain: "example.com"},
				{Server: "host3", Username: "admin", Password: "secret", Domain: "example.com"},
			},
		},
		{
			name: "single_server",
			remote: stanzawindows.RemoteConfig{
				Servers:  []string{"host1"},
				Username: "admin",
				Password: "secret",
			},
			expected: []stanzawindows.RemoteConfig{
				{Server: "host1", Username: "admin", Password: "secret"},
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
		Servers:  []string{"host2"},
		Username: "admin",
		Password: "secret",
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestValidate_FeatureGateRequired(t *testing.T) {
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ReceiverWindowseventlogMultipleRemoteHostsFeatureGate.ID(), false))

	cfg := createTestConfig()
	cfg.InputConfig.Remote = stanzawindows.RemoteConfig{
		Servers:  []string{"host1"},
		Username: "admin",
		Password: "secret",
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "feature gate")
}

func TestValidate_MissingCredentials(t *testing.T) {
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ReceiverWindowseventlogMultipleRemoteHostsFeatureGate.ID(), true))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ReceiverWindowseventlogMultipleRemoteHostsFeatureGate.ID(), false))
	}()

	cfg := createTestConfig()
	cfg.InputConfig.Remote = stanzawindows.RemoteConfig{
		Servers: []string{"host1"},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remote.username and remote.password are required")
}

func TestValidate_ValidMultiHostConfig(t *testing.T) {
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ReceiverWindowseventlogMultipleRemoteHostsFeatureGate.ID(), true))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ReceiverWindowseventlogMultipleRemoteHostsFeatureGate.ID(), false))
	}()

	cfg := createTestConfig()
	cfg.InputConfig.Remote = stanzawindows.RemoteConfig{
		Servers:  []string{"host1", "host2", "host3"},
		Username: "admin",
		Password: "secret",
		Domain:   "example.com",
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
		Servers:  []string{"host1", "host2", "host3"},
		Username: "admin",
		Password: "secret",
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
				Servers:  []string{"host1", "host2", "host3"},
				Username: "admin",
				Password: configopaque.String("secret"),
				Domain:   "example.com",
			}
			return *c
		}(),
	}
	assert.Equal(t, expected, cfg)
}
