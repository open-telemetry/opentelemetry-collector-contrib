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

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// CarbonDataReceiver implements Carbon format receiver.
type CarbonDataReceiver struct {
	testbed.DataReceiverBase
	receiver receiver.Metrics
}

// Ensure CarbonDataReceiver implements DataReceiver
var _ testbed.DataReceiver = (*CarbonDataReceiver)(nil)

// NewCarbonDataReceiver creates a new CarbonDataReceiver that will listen on the
// specified port after Start is called.
func NewCarbonDataReceiver(port int) *CarbonDataReceiver {
	return &CarbonDataReceiver{DataReceiverBase: testbed.DataReceiverBase{Port: port}}
}

// Start the receiver.
func (cr *CarbonDataReceiver) Start(_ consumer.Traces, mc consumer.Metrics, _ consumer.Logs) error {
	factory := carbonreceiver.NewFactory()
	cfg := factory.CreateDefaultConfig().(*carbonreceiver.Config)
	cfg.Endpoint = fmt.Sprintf("127.0.0.1:%d", cr.Port)

	set := receivertest.NewNopSettings()
	var err error
	cr.receiver, err = factory.CreateMetrics(context.Background(), set, cfg, mc)
	if err != nil {
		return err
	}

	return cr.receiver.Start(context.Background(), componenttest.NewNopHost())
}

// Stop the receiver.
func (cr *CarbonDataReceiver) Stop() error {
	return cr.receiver.Shutdown(context.Background())
}

// GenConfigYAMLStr returns exporter config for the agent.
func (cr *CarbonDataReceiver) GenConfigYAMLStr() string {
	// Note that this generates an exporter config for agent.
	return fmt.Sprintf(`
  carbon:
    sending_queue:
      enabled: false
    endpoint: "127.0.0.1:%d"`, cr.Port)
}

// ProtocolName returns protocol name as it is specified in Collector config.
func (cr *CarbonDataReceiver) ProtocolName() string {
	return "carbon"
}
