// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opampextension

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestUnmarshalDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, component.UnmarshalConfig(confmap.New(), cfg))
	assert.Equal(t, factory.CreateDefaultConfig(), cfg)
}

func TestUnmarshalConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, component.UnmarshalConfig(cm, cfg))
	assert.Equal(t,
		&Config{
			Server: &OpAMPServer{
				WS: &OpAMPWebsocket{
					Endpoint: "wss://127.0.0.1:4320/v1/opamp",
				},
			},
			InstanceUID: "01BX5ZZKBKACTAV9WEVGEMMVRZ",
			Capabilities: Capabilities{
				ReportsEffectiveConfig: true,
			},
		}, cfg)
}

func TestConfigValidate(t *testing.T) {
	cfg := &Config{
		Server: &OpAMPServer{
			WS: &OpAMPWebsocket{},
		},
	}
	err := cfg.Validate()
	assert.Equal(t, "opamp server websocket endpoint must be provided", err.Error())
	cfg.Server.WS.Endpoint = "wss://127.0.0.1:4320/v1/opamp"
	assert.NoError(t, cfg.Validate())
	cfg.InstanceUID = "01BX5ZZKBKACTAV9WEVGEMMVRZFAIL"
	err = cfg.Validate()
	require.Error(t, err)
	assert.Equal(t, "opamp instance_uid is invalid", err.Error())
	cfg.InstanceUID = "01BX5ZZKBKACTAV9WEVGEMMVRZ"
	require.NoError(t, cfg.Validate())
}
