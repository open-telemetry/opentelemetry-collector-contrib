// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator/internal/metadata"
)

func Test_loadAndCreateMetricsRuntimeReceiver(t *testing.T) {
	logCore, logs := observer.New(zap.DebugLevel)
	logger := zap.New(logCore).With(zap.String("name", "receiver_creator"))
	rcs := receivertest.NewNopSettings(metadata.Type)
	rcs.Logger = logger
	run := &receiverRunner{params: rcs, idNamespace: component.NewIDWithName(metadata.Type, "1")}
	exampleFactory := &nopWithEndpointFactory{}
	template, err := newReceiverTemplate("nop/1", nil)
	require.NoError(t, err)

	loadedConfig, endpoint, err := run.loadRuntimeReceiverConfig(exampleFactory, template.receiverConfig, userConfigMap{
		tmpSetEndpointConfigKey: struct{}{},
		endpointConfigKey:       "localhost:12345",
	})
	require.NoError(t, err)
	assert.Equal(t, "localhost:12345", endpoint)
	require.NotNil(t, loadedConfig)
	nopConfig := loadedConfig.(*nopWithEndpointConfig)
	// Verify that the overridden endpoint is used instead of the one in the config file.
	assert.Equal(t, "localhost:12345", nopConfig.Endpoint)
	expectedID := `nop/1/receiver_creator/1{endpoint="localhost:12345"}/endpoint.id`

	// Test that metric receiver can be created from loaded config and it logs its id for the "name" field.
	t.Run("test create receiver from loaded config", func(t *testing.T) {
		recvr, err := run.createMetricsRuntimeReceiver(
			exampleFactory,
			component.MustNewIDWithName("nop", "1/receiver_creator/1{endpoint=\"localhost:12345\"}/endpoint.id"),
			loadedConfig,
			nil)
		require.NoError(t, err)
		assert.NotNil(t, recvr)
		assert.IsType(t, &nopWithEndpointReceiver{}, recvr)
		recvr.(*nopWithEndpointReceiver).Logger.Warn("test message")
		assert.True(t, func() bool {
			var found bool
			for _, entry := range logs.All() {
				if name, ok := entry.ContextMap()["name"]; ok {
					found = true
					assert.Equal(t, expectedID, name)
				}
			}
			return found
		}())
	})
}

func TestValidateSetEndpointFromConfig(t *testing.T) {
	type configWithEndpoint struct {
		Endpoint any `mapstructure:"endpoint"`
	}

	receiverWithEndpoint := receiver.NewFactory(component.MustNewType("with_endpoint"), func() component.Config {
		return &configWithEndpoint{}
	})

	type configWithoutEndpoint struct {
		NotEndpoint any `mapstructure:"not.endpoint"`
	}

	receiverWithoutEndpoint := receiver.NewFactory(component.MustNewType("without_endpoint"), func() component.Config {
		return &configWithoutEndpoint{}
	})

	setEndpointConfMap, setEndpoint, setErr := mergeTemplatedAndDiscoveredConfigs(
		receiverWithEndpoint, nil, map[string]any{
			tmpSetEndpointConfigKey: struct{}{},
			endpointConfigKey:       "an.endpoint",
		},
	)
	require.Equal(t, map[string]any{endpointConfigKey: "an.endpoint"}, setEndpointConfMap.ToStringMap())
	require.Equal(t, "an.endpoint", setEndpoint)
	require.NoError(t, setErr)

	inheritedEndpointConfMap, inheritedEndpoint, inheritedErr := mergeTemplatedAndDiscoveredConfigs(
		receiverWithEndpoint, map[string]any{
			endpointConfigKey: "an.endpoint",
		}, map[string]any{},
	)
	require.Equal(t, map[string]any{endpointConfigKey: "an.endpoint"}, inheritedEndpointConfMap.ToStringMap())
	require.Equal(t, "an.endpoint", inheritedEndpoint)
	require.NoError(t, inheritedErr)

	setEndpointConfMap, setEndpoint, setErr = mergeTemplatedAndDiscoveredConfigs(
		receiverWithoutEndpoint, nil, map[string]any{
			tmpSetEndpointConfigKey: struct{}{},
			endpointConfigKey:       "an.endpoint",
		},
	)
	require.Equal(t, map[string]any{}, setEndpointConfMap.ToStringMap())
	require.Equal(t, "an.endpoint", setEndpoint)
	require.NoError(t, setErr)

	inheritedEndpointConfMap, inheritedEndpoint, inheritedErr = mergeTemplatedAndDiscoveredConfigs(
		receiverWithoutEndpoint, map[string]any{
			endpointConfigKey: "an.endpoint",
		}, map[string]any{},
	)
	require.Equal(t, map[string]any{endpointConfigKey: "an.endpoint"}, inheritedEndpointConfMap.ToStringMap())
	require.Equal(t, "an.endpoint", inheritedEndpoint)
	require.NoError(t, inheritedErr)
}
