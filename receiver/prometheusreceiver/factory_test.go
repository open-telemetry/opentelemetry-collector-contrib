// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateReceiver(t *testing.T) {
	cfg := createDefaultConfig()

	// The default config does not provide scrape_config so we expect that metrics receiver
	// creation must also fail.
	creationSet := receivertest.NewNopSettings(metadata.Type)
	mReceiver, _ := createMetricsReceiver(context.Background(), creationSet, cfg, consumertest.NewNop())
	assert.NotNil(t, mReceiver)
	assert.NotNil(t, mReceiver.(*pReceiver).cfg.PrometheusConfig.GlobalConfig)
}

func TestFactoryCanParseServiceDiscoveryConfigs(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_sd.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	assert.NoError(t, sub.Unmarshal(cfg))
}

func TestMultipleCreateWithAPIServer(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.APIServer = &APIServer{
		Enabled: true,
		ServerConfig: confighttp.ServerConfig{
			Endpoint: "localhost:9090",
		},
	}
	set := receivertest.NewNopSettings(metadata.Type)
	firstRcvr, err := factory.CreateMetrics(context.Background(), set, cfg, consumertest.NewNop())
	require.NoError(t, err)
	host := componenttest.NewNopHost()
	require.NoError(t, err)
	require.NoError(t, firstRcvr.Start(context.Background(), host))
	require.NoError(t, firstRcvr.Shutdown(context.Background()))
	secondRcvr, err := factory.CreateMetrics(context.Background(), set, cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NoError(t, secondRcvr.Start(context.Background(), host))
	require.NoError(t, secondRcvr.Shutdown(context.Background()))
}

func TestMultipleCreate(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	set := receivertest.NewNopSettings(metadata.Type)
	firstRcvr, err := factory.CreateMetrics(context.Background(), set, cfg, consumertest.NewNop())
	require.NoError(t, err)
	host := componenttest.NewNopHost()
	require.NoError(t, err)
	require.NoError(t, firstRcvr.Start(context.Background(), host))
	require.NoError(t, firstRcvr.Shutdown(context.Background()))
	secondRcvr, err := factory.CreateMetrics(context.Background(), set, cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NoError(t, secondRcvr.Start(context.Background(), host))
	require.NoError(t, secondRcvr.Shutdown(context.Background()))
}
