// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogconnector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

var _ component.Component = (*connectorImp)(nil) // testing that the connectorImp properly implements the type Component interface

// create test to create a connector, check that basic code compiles
func TestNewConnector(t *testing.T) {

	factory := NewFactory()

	creationParams := connectortest.NewNopCreateSettings()
	cfg := factory.CreateDefaultConfig().(*Config)

	traceToMetricsConnector, err := factory.CreateTracesToMetrics(context.Background(), creationParams, cfg, consumertest.NewNop())
	assert.NoError(t, err)

	_, ok := traceToMetricsConnector.(*connectorImp)
	assert.True(t, ok) // checks if the created connector implements the connectorImp struct

	traceToTracesConnector, err := factory.CreateTracesToTraces(context.Background(), creationParams, cfg, consumertest.NewNop())
	assert.NoError(t, err)

	_, ok = traceToTracesConnector.(*connectorImp)
	assert.True(t, ok) // checks if the created connector implements the connectorImp struct
}
