// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package connectors

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/dataconnectors/routingdataconnector"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

func constructTraceSender(t *testing.T, name string) testbed.DataSender {
	if name == "otlp" {
		return testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t))
	}
	t.Errorf("unknown receiver type: %s", name)
	return nil
}

func constructReceiver(t *testing.T, name string) testbed.DataReceiver {
	if name == "otlp_grpc" {
		return testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))
	}
	t.Errorf("unknown exporter type: %s", name)
	return nil
}

func constructConnector(t *testing.T, name, receiverType string) testbed.DataConnector {
	if name == "routing" {
		return routingdataconnector.NewRoutingDataConnector(receiverType)
	}
	t.Errorf("unknown connector type: %s", name)
	return nil
}
