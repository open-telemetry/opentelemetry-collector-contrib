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

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/stefreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// StefDataReceiver implements SignalFx format receiver.
type StefDataReceiver struct {
	testbed.DataReceiverBase
	receiver receiver.Metrics
}

// Ensure StefDataReceiver implements MetricDataSender.
var _ testbed.DataReceiver = (*StefDataReceiver)(nil)

// NewStefDataReceiver creates a new StefDataReceiver that will listen on the
// specified port after Start is called.
func NewStefDataReceiver(port int) *StefDataReceiver {
	return &StefDataReceiver{DataReceiverBase: testbed.DataReceiverBase{Port: port}}
}

// Start the receiver.
func (sr *StefDataReceiver) Start(_ consumer.Traces, mc consumer.Metrics, _ consumer.Logs) error {
	var err error
	f := stefreceiver.NewFactory()
	config := f.CreateDefaultConfig()
	config.(*stefreceiver.Config).NetAddr.Endpoint = fmt.Sprintf("127.0.0.1:%d", sr.Port)

	sr.receiver, err = f.CreateMetrics(context.Background(), receivertest.NewNopSettings(f.Type()), config, mc)
	if err != nil {
		return err
	}

	return sr.receiver.Start(context.Background(), componenttest.NewNopHost())
}

// Stop the receiver.
func (sr *StefDataReceiver) Stop() error {
	return sr.receiver.Shutdown(context.Background())
}

// GenConfigYAMLStr returns exporter config for the agent.
func (sr *StefDataReceiver) GenConfigYAMLStr() string {
	// Note that this generates an exporter config for agent.
	return fmt.Sprintf(
		`
    stef:
      endpoint: "127.0.0.1:%d"
      tls:
        insecure: true
`, sr.Port,
	)
}

// ProtocolName returns protocol name as it is specified in Collector config.
func (sr *StefDataReceiver) ProtocolName() string {
	return "stef"
}
