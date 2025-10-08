// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datareceivers // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datareceivers"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// OtelarrowDataReceiver implements Otel Arrow format receiver.
type OtelarrowDataReceiver struct {
	testbed.DataReceiverBase
	receiver receiver.Metrics
}

// Ensure OtelarrowDataReceiver implements MetricDataSender.
var _ testbed.DataReceiver = (*OtelarrowDataReceiver)(nil)

// NewOtelarrowDataReceiver creates a new OtelarrowDataReceiver that will listen on the
// specified port after Start is called.
func NewOtelarrowDataReceiver(port int) *OtelarrowDataReceiver {
	return &OtelarrowDataReceiver{DataReceiverBase: testbed.DataReceiverBase{Port: port}}
}

// Start the receiver.
func (dr *OtelarrowDataReceiver) Start(_ consumer.Traces, mc consumer.Metrics, _ consumer.Logs) error {
	var err error
	f := otelarrowreceiver.NewFactory()
	config := f.CreateDefaultConfig()
	config.(*otelarrowreceiver.Config).GRPC.NetAddr.Endpoint = fmt.Sprintf("127.0.0.1:%d", dr.Port)

	dr.receiver, err = f.CreateMetrics(context.Background(), receivertest.NewNopSettings(f.Type()), config, mc)
	if err != nil {
		return err
	}

	return dr.receiver.Start(context.Background(), componenttest.NewNopHost())
}

// Stop the receiver.
func (dr *OtelarrowDataReceiver) Stop() error {
	return dr.receiver.Shutdown(context.Background())
}

// GenConfigYAMLStr returns exporter config for the agent.
func (dr *OtelarrowDataReceiver) GenConfigYAMLStr() string {
	// Note that this generates an exporter config for agent.
	return fmt.Sprintf(
		`
    otelarrow:
      endpoint: "127.0.0.1:%d"
      compression: none
      tls:
        insecure: true
`, dr.Port,
	)
}

// ProtocolName returns protocol name as it is specified in Collector config.
func (*OtelarrowDataReceiver) ProtocolName() string {
	return "otelarrow"
}
