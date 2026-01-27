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
		endpointConfigKey: "localhost:12345",
	}, true) // endpointAutoSet=true since endpoint was auto-discovered
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

	// Test: endpoint auto-set + receiver supports endpoint field -> endpoint is kept
	setEndpointConfMap, setEndpoint, setErr := mergeTemplatedAndDiscoveredConfigs(
		receiverWithEndpoint, nil, map[string]any{
			endpointConfigKey: "an.endpoint",
		}, true, // endpointAutoSet=true
	)
	require.Equal(t, map[string]any{endpointConfigKey: "an.endpoint"}, setEndpointConfMap.ToStringMap())
	require.Equal(t, "an.endpoint", setEndpoint)
	require.NoError(t, setErr)

	// Test: endpoint user-specified (in templated config) -> endpoint is preserved
	inheritedEndpointConfMap, inheritedEndpoint, inheritedErr := mergeTemplatedAndDiscoveredConfigs(
		receiverWithEndpoint, map[string]any{
			endpointConfigKey: "an.endpoint",
		}, map[string]any{}, false, // endpointAutoSet=false (user specified)
	)
	require.Equal(t, map[string]any{endpointConfigKey: "an.endpoint"}, inheritedEndpointConfMap.ToStringMap())
	require.Equal(t, "an.endpoint", inheritedEndpoint)
	require.NoError(t, inheritedErr)

	// Test: endpoint auto-set + receiver does NOT support endpoint field -> endpoint is removed
	setEndpointConfMap, setEndpoint, setErr = mergeTemplatedAndDiscoveredConfigs(
		receiverWithoutEndpoint, nil, map[string]any{
			endpointConfigKey: "an.endpoint",
		}, true, // endpointAutoSet=true
	)
	require.Equal(t, map[string]any{}, setEndpointConfMap.ToStringMap())
	require.Equal(t, "an.endpoint", setEndpoint)
	require.NoError(t, setErr)

	// Test: endpoint user-specified but receiver doesn't support it -> endpoint is preserved (user's choice)
	inheritedEndpointConfMap, inheritedEndpoint, inheritedErr = mergeTemplatedAndDiscoveredConfigs(
		receiverWithoutEndpoint, map[string]any{
			endpointConfigKey: "an.endpoint",
		}, map[string]any{}, false, // endpointAutoSet=false
	)
	require.Equal(t, map[string]any{endpointConfigKey: "an.endpoint"}, inheritedEndpointConfMap.ToStringMap())
	require.Equal(t, "an.endpoint", inheritedEndpoint)
	require.NoError(t, inheritedErr)
}

// TestMergeTemplatedAndDiscoveredConfigsDoesNotMutateInputs verifies that
// mergeTemplatedAndDiscoveredConfigs does not mutate its input maps.
// This is important because the caller stores these maps for later comparison
// during OnChange handling.
func TestMergeTemplatedAndDiscoveredConfigsDoesNotMutateInputs(t *testing.T) {
	type configWithoutEndpoint struct {
		NotEndpoint any `mapstructure:"not.endpoint"`
	}

	receiverWithoutEndpoint := receiver.NewFactory(component.MustNewType("without_endpoint"), func() component.Config {
		return &configWithoutEndpoint{}
	})

	// Create a discovered map with endpoint that will be removed by the merge
	// because the receiver doesn't support it
	discovered := map[string]any{
		endpointConfigKey: "localhost:1234",
		"other":           "value",
	}
	// Keep a copy of the original
	originalDiscovered := map[string]any{
		endpointConfigKey: "localhost:1234",
		"other":           "value",
	}

	// Call merge - this used to mutate the discovered map
	_, _, err := mergeTemplatedAndDiscoveredConfigs(
		receiverWithoutEndpoint,
		map[string]any{},
		discovered,
		true, // endpointAutoSet=true triggers the delete path
	)
	require.NoError(t, err)

	// The discovered map should NOT have been mutated
	assert.Equal(t, originalDiscovered, discovered, "mergeTemplatedAndDiscoveredConfigs should not mutate its input maps")
}
