// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pipeline"
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
	conn, err := factory.CreateTracesToTraces(context.Background(),
		connectortest.NewNopSettings(), cfg, router.(consumer.Traces))

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
	conn, err := factory.CreateTracesToTraces(context.Background(),
		connectortest.NewNopSettings(), cfg, consumer)

	assert.ErrorIs(t, err, errUnexpectedConsumer)
	assert.Nil(t, conn)
}
