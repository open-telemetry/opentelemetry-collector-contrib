// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmptrapreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmptrapreceiver/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg)
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	assert.Equal(t, "0.0.0.0:1620", cfg.(*Config).ListenAddress)
}

func TestCreateReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	set := receivertest.NewNopSettings(metadata.Type)
	recv, err := factory.CreateLogs(t.Context(), set, cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, recv)
}

func TestConfigValidate(t *testing.T) {
	cfg := &Config{}
	require.Error(t, cfg.Validate())
	cfg.ListenAddress = "127.0.0.1:0"
	require.NoError(t, cfg.Validate())
	cfg.V3 = &V3Config{}
	require.Error(t, cfg.Validate())
	cfg.V3.User = "trapuser"
	require.NoError(t, cfg.Validate())
}
