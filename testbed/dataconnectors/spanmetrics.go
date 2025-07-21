// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dataconnectors // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/dataconnectors"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

type SpanMetricDataConnector struct {
	testbed.DataConnectorBase
}

var _ testbed.DataConnector = (*SpanMetricDataConnector)(nil)

func NewSpanMetricDataConnector(receiverDataType string) *SpanMetricDataConnector {
	return &SpanMetricDataConnector{DataConnectorBase: testbed.DataConnectorBase{ReceiverDataType: receiverDataType}}
}

func (*SpanMetricDataConnector) GenConfigYAMLStr() string {
	// Note that this generates an exporter config for agent.
	return `
  spanmetrics:`
}

// ProtocolName returns protocol name as it is specified in Collector config.
func (*SpanMetricDataConnector) ProtocolName() string {
	return "spanmetrics"
}

func (smc *SpanMetricDataConnector) GetReceiverType() string {
	return smc.ReceiverDataType
}
