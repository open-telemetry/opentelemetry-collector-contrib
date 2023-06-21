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

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/fanoutconsumer"
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

	consumer := fanoutconsumer.NewTracesRouter(
		map[component.ID]consumer.Traces{
			component.NewIDWithName(component.DataTypeTraces, "default"): &consumertest.TracesSink{},
			component.NewIDWithName(component.DataTypeTraces, "0"):       &consumertest.TracesSink{},
		})

	factory := NewFactory()
	conn, err := factory.CreateTracesToTraces(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, consumer)

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

	// if a connector is not a receiver in multiple pipelines the factory will receive a normal,
	// non-fanout consumer
	consumer := &consumertest.TracesSink{}

	factory := NewFactory()
	conn, err := factory.CreateTracesToTraces(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, consumer)

	assert.ErrorIs(t, err, errUnexpectedConsumer)
	assert.Nil(t, conn)
}
