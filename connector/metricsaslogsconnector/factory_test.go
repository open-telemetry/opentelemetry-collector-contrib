// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsaslogsconnector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()

	assert.NotNil(t, factory)
	assert.Equal(t, component.MustNewType("metricsaslogs"), factory.Type())
	assert.Equal(t, component.StabilityLevelAlpha, factory.MetricsToLogsStability())
}

func TestCreateMetricsToLogs(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	set := connectortest.NewNopSettings(component.MustNewType("metricsaslogs"))
	consumer := &consumertest.LogsSink{}

	connector, err := factory.CreateMetricsToLogs(
		t.Context(),
		set,
		cfg,
		consumer,
	)

	require.NoError(t, err)
	assert.NotNil(t, connector)
	assert.IsType(t, &metricsAsLogs{}, connector)

	// Verify the connector is properly configured
	mal := connector.(*metricsAsLogs)
	assert.Equal(t, consumer, mal.logsConsumer)
	assert.Equal(t, cfg, mal.config)
	assert.NotNil(t, mal.logger)
}
