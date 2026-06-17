// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestConnectorCreatedWithValidConfiguration(t *testing.T) {
	cfg := &Config{
		Table: []RoutingTableItem{{
			Statement: `route() where attributes["X-Tenant"] == "acme"`,
			Pipelines: []pipeline.ID{
				pipeline.NewIDWithName(pipeline.SignalTraces, "0"),
			},
		}},
	}

	router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
		pipeline.NewIDWithName(pipeline.SignalTraces, "default"): consumertest.NewNop(),
		pipeline.NewIDWithName(pipeline.SignalTraces, "0"):       consumertest.NewNop(),
	})

	factory := NewFactory()
	conn, err := factory.CreateTracesToTraces(t.Context(),
		connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Traces))

	assert.NoError(t, err)
	assert.NotNil(t, conn)
}

func TestCreationFailsWithIncorrectConsumer(t *testing.T) {
	cfg := &Config{
		Table: []RoutingTableItem{{
			Statement: `route() where attributes["X-Tenant"] == "acme"`,
			Pipelines: []pipeline.ID{
				pipeline.NewIDWithName(pipeline.SignalTraces, "0"),
			},
		}},
	}

	// in the real world, the factory will always receive a consumer with a concrete type of a
	// connector router. this tests failure when a consumer of another type is passed in.
	consumer := &consumertest.TracesSink{}

	factory := NewFactory()
	conn, err := factory.CreateTracesToTraces(t.Context(),
		connectortest.NewNopSettings(metadata.Type), cfg, consumer)

	assert.ErrorIs(t, err, errUnexpectedConsumer)
	assert.Nil(t, conn)
}

func TestDefaultErrorModeWithFeatureGate(t *testing.T) {
	tests := []struct {
		name               string
		featureGateEnabled bool
		expectedErrorMode  ottl.ErrorMode
	}{
		{
			name:               "feature gate disabled",
			featureGateEnabled: false,
			expectedErrorMode:  ottl.PropagateError,
		},
		{
			name:               "feature gate enabled",
			featureGateEnabled: true,
			expectedErrorMode:  ottl.IgnoreError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previousValue := metadata.ConnectorRoutingDefaultErrorModeIgnoreFeatureGate.IsEnabled()
			require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ConnectorRoutingDefaultErrorModeIgnoreFeatureGate.ID(), tt.featureGateEnabled))
			defer func() {
				require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ConnectorRoutingDefaultErrorModeIgnoreFeatureGate.ID(), previousValue))
			}()

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			assert.Equal(t, tt.expectedErrorMode, cfg.ErrorMode)
		})
	}
}
