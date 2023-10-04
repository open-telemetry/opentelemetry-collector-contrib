// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lokireceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

func TestCreateDefaultConfig(t *testing.T) {
	defer testutil.SetFeatureGateForTest(t, component.UseLocalHostAsDefaultHostfeatureGate, false)()
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	lokiCfg, ok := cfg.(*Config)
	require.True(t, ok)
	assert.Equal(t, lokiCfg.GRPC.NetAddr.Endpoint, "0.0.0.0:3600")
	assert.Equal(t, lokiCfg.HTTP.Endpoint, "0.0.0.0:3500")
}

func TestCreateDefaultConfigLocalhost(t *testing.T) {
	defer testutil.SetFeatureGateForTest(t, component.UseLocalHostAsDefaultHostfeatureGate, true)()
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	lokiCfg, ok := cfg.(*Config)
	require.True(t, ok)
	assert.Equal(t, lokiCfg.GRPC.NetAddr.Endpoint, "localhost:3600")
	assert.Equal(t, lokiCfg.HTTP.Endpoint, "localhost:3500")
}

func TestCreateReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).Protocols.GRPC = &configgrpc.GRPCServerSettings{
		NetAddr: confignet.NetAddr{
			Endpoint:  "localhost:3600",
			Transport: "tcp",
		},
	}
	set := receivertest.NewNopCreateSettings()
	receiver, err := factory.CreateLogsReceiver(context.Background(), set, cfg, consumertest.NewNop())
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, receiver, "receiver creation failed")
}
