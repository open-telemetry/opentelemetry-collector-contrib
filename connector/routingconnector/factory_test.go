// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestConnectorCreatedWithValidConfiguration(t *testing.T) {
	cfg := &Config{
		Table: []RoutingTableItem{{
			Statement: `route() where attributes["X-Tenant"] == "acme"`,
			Pipelines: []component.ID{
				component.NewIDWithName(component.DataTypeTraces, "0"),
			},
		}},
	}

	router := connectortest.NewTracesRouter(
		connectortest.WithNopTraces(component.NewIDWithName(component.DataTypeTraces, "default")),
		connectortest.WithNopTraces(component.NewIDWithName(component.DataTypeTraces, "0")),
	)

	factory := NewFactory()
	conn, err := factory.CreateTracesToTraces(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, router.(consumer.Traces))

	assert.NoError(t, err)
	assert.NotNil(t, conn)
}

func TestCreationFailsWithIncorrectConsumer(t *testing.T) {
	cfg := &Config{
		Table: []RoutingTableItem{{
			Statement: `route() where attributes["X-Tenant"] == "acme"`,
			Pipelines: []component.ID{
				component.NewIDWithName(component.DataTypeTraces, "0"),
			},
		}},
	}

	// in the real world, the factory will always receive a consumer with a concerete type of a
	// connector router. this tests failure when a consumer of another type is passed in.
	consumer := &consumertest.TracesSink{}

	factory := NewFactory()
	conn, err := factory.CreateTracesToTraces(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, consumer)

	assert.ErrorIs(t, err, errUnexpectedConsumer)
	assert.Nil(t, conn)
}
