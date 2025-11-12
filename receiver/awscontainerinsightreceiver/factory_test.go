// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscontainerinsightreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	c := NewFactory()
	assert.NotNil(t, c)
	assert.Equal(t, metadata.Type, c.Type())
}

func TestNewDeprecatedFactory(t *testing.T) {
	c := NewDeprecatedFactory()
	assert.NotNil(t, c)
	assert.Equal(t, component.MustNewType("awscontainerinsightreceiver"), c.Type())
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

func TestCreateDeprecatedMetricsReceiver(t *testing.T) {
	// Set up observer to capture log messages
	core, recorded := observer.New(zap.WarnLevel)
	logger := zap.New(core)

	settings := receivertest.NewNopSettings(component.MustNewType("awscontainerinsightreceiver"))
	settings.Logger = logger

	metricsReceiver, err := createDeprecatedMetricsReceiver(
		t.Context(),
		settings,
		createDefaultConfig(),
		consumertest.NewNop(),
	)

	require.NoError(t, err)
	require.NotNil(t, metricsReceiver)

	// Verify deprecation warning was logged
	logs := recorded.All()
	require.Len(t, logs, 1)
	assert.Contains(t, logs[0].Message, "deprecated")
	assert.Contains(t, logs[0].Message, "awscontainerinsightreceiver")
	assert.Contains(t, logs[0].Message, "awscontainerinsight")

	// Verify the warning is only logged once (test the sync.Once behavior)
	metricsReceiver2, err2 := createDeprecatedMetricsReceiver(
		t.Context(),
		settings,
		createDefaultConfig(),
		consumertest.NewNop(),
	)
	require.NoError(t, err2)
	require.NotNil(t, metricsReceiver2)

	// Should still only have one log entry
	logs2 := recorded.All()
	assert.Len(t, logs2, 1, "Deprecation warning should only be logged once")
}
