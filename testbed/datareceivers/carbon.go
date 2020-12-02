// Copyright 2020 OpenTelemetry Authors
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

package datareceivers

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/testbed/testbed"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/protocol"
)

// CarbonDataReceiver implements Carbon format receiver.
type CarbonDataReceiver struct {
	testbed.DataReceiverBase
	receiver component.MetricsReceiver
}

// Ensure CarbonDataReceiver implements MetricDataSender.
var _ testbed.DataReceiver = (*CarbonDataReceiver)(nil)

// NewCarbonDataReceiver creates a new CarbonDataReceiver that will listen on the
// specified port after Start is called.
func NewCarbonDataReceiver(port int) *CarbonDataReceiver {
	return &CarbonDataReceiver{DataReceiverBase: testbed.DataReceiverBase{Port: port}}
}

// Start the receiver.
func (cr *CarbonDataReceiver) Start(_ consumer.TracesConsumer, mc consumer.MetricsConsumer, _ consumer.LogsConsumer) error {
	addr := fmt.Sprintf("localhost:%d", cr.Port)
	config := carbonreceiver.Config{
		NetAddr: confignet.NetAddr{
			Endpoint: addr,
		},
		Parser: &protocol.Config{
			Type:   "plaintext",
			Config: &protocol.PlaintextConfig{},
		},
	}
	var err error
	cr.receiver, err = carbonreceiver.New(zap.L(), config, mc)
	if err != nil {
		return err
	}

	return cr.receiver.Start(context.Background(), cr)
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
    endpoint: "localhost:%d"`, cr.Port)
}

// ProtocolName returns protocol name as it is specified in Collector config.
func (cr *CarbonDataReceiver) ProtocolName() string {
	return "carbon"
}
