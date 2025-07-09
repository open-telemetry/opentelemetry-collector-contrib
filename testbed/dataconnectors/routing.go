// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dataconnectors // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/dataconnectors"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

type RoutingDataConnector struct {
	testbed.DataConnectorBase
}

var _ testbed.DataConnector = (*RoutingDataConnector)(nil)

func NewRoutingDataConnector(receiverDataType string) *RoutingDataConnector {
	return &RoutingDataConnector{DataConnectorBase: testbed.DataConnectorBase{ReceiverDataType: receiverDataType}}
}

func (rc *RoutingDataConnector) GenConfigYAMLStr() string {
	// Note that this generates an exporter config for agent.
	return `
  routing:
    table:
      - statement: route()
        pipelines: [traces/out]`
}

// ProtocolName returns protocol name as it is specified in Collector config.
func (rc *RoutingDataConnector) ProtocolName() string {
	return "routing"
}

func (rc *RoutingDataConnector) GetReceiverType() string {
	return rc.ReceiverDataType
}
