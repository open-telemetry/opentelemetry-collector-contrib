// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscontainerinsightreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	// TestNewFactory checks the default behavior (feature gate disabled by default at StageAlpha)
	// which should use the old type name for backward compatibility
	originalValue := useNewTypeNameGate.IsEnabled()
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(useNewTypeNameGate.ID(), originalValue))
	}()

	c := NewFactory()
	assert.NotNil(t, c)
	// By default (feature gate disabled), should use old type name for backward compatibility
	assert.Equal(t, component.MustNewType("awscontainerinsightreceiver"), c.Type())
}

func TestNewFactory_FeatureGateEnabled(t *testing.T) {
	// Enable the feature gate
	originalValue := useNewTypeNameGate.IsEnabled()
	defer func() {
		if originalValue {
			require.NoError(t, featuregate.GlobalRegistry().Set(useNewTypeNameGate.ID(), true))
		} else {
			require.NoError(t, featuregate.GlobalRegistry().Set(useNewTypeNameGate.ID(), false))
		}
	}()

	require.NoError(t, featuregate.GlobalRegistry().Set(useNewTypeNameGate.ID(), true))

	c := NewFactory()
	assert.NotNil(t, c)
	assert.Equal(t, component.MustNewType("awscontainerinsight"), c.Type())
}

func TestCreateMetrics(t *testing.T) {
	metricsReceiver, _ := createMetricsReceiver(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
		createDefaultConfig(),
		consumertest.NewNop(),
	)

	require.NotNil(t, metricsReceiver)
}
