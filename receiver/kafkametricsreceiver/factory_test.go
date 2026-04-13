// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

func TestFactory_CreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "default config not created")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

// TestFactory_ClusterAliasAttributeNotEnabledByDefault verifies that
// kafka.cluster.alias resource attribute is NOT enabled when cluster_alias is
// unset (the common default case).
func TestFactory_ClusterAliasAttributeNotEnabledByDefault(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.Empty(t, cfg.ClusterAlias)
	assert.False(t, cfg.ResourceAttributes.KafkaClusterAlias.Enabled,
		"kafka.cluster.alias must not be enabled when cluster_alias is empty")
}

// TestFactory_ClusterAliasAttributeEnabledWhenSet is a regression test for
// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/47573.
// Previously the attribute was never enabled because the check lived in
// createDefaultConfig(), where ClusterAlias is always "".
func TestFactory_ClusterAliasAttributeEnabledWhenSet(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ClusterAlias = "test-cluster"

	// Replace newMetricsReceiver so we can inspect the mutated Config without
	// needing a live Kafka cluster.
	var capturedConfig Config
	orig := newMetricsReceiver
	newMetricsReceiver = func(_ context.Context, c Config, _ receiver.Settings, _ consumer.Metrics) (receiver.Metrics, error) {
		capturedConfig = c
		return nil, nil
	}
	defer func() { newMetricsReceiver = orig }()

	_, _ = createMetricsReceiver(context.Background(), receiver.Settings{}, cfg, nil)

	assert.True(t, capturedConfig.ResourceAttributes.KafkaClusterAlias.Enabled,
		"kafka.cluster.alias resource attribute must be enabled when cluster_alias is set")
}
