// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
			Statement: `route() where resource.attributes["X-Tenant"] == "acme"`,
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

func TestCreationFailsWithTooFewPipelines(t *testing.T) {
	cfg := &Config{
		Table: []RoutingTableItem{{
			Statement: `route() where resource.attributes["X-Tenant"] == "acme"`,
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

	assert.ErrorIs(t, err, errTooFewPipelines)
	assert.Nil(t, conn)
}
