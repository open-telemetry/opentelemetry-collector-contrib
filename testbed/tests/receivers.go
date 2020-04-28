// Copyright 2019 OpenTelemetry Authors
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

package tests

import (
	"context"
	"fmt"
	"log"

	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector/testbed/testbed"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/protocol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver"
)

// SapmDataReceiver implements Sapm format receiver.
type SapmDataReceiver struct {
	testbed.DataReceiverBase
	receiver component.TraceReceiver
}

// NewSapmDataReceiver creates a new SapmDataReceiver.
func NewSapmDataReceiver(port int) *SapmDataReceiver {
	return &SapmDataReceiver{DataReceiverBase: testbed.DataReceiverBase{Port: port}}
}

// Start the receiver.
func (sr *SapmDataReceiver) Start(tc *testbed.MockTraceConsumer, mc *testbed.MockMetricConsumer) error {
	sapmCfg := sapmreceiver.Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			Endpoint: fmt.Sprintf("localhost:%d", sr.Port),
		},
	}
	var err error
	params := component.ReceiverCreateParams{Logger: zap.L()}
	sr.receiver, err = sapmreceiver.New(context.Background(), params, &sapmCfg, tc)
	if err != nil {
		return err
	}

	return sr.receiver.Start(context.Background(), sr)
}

// Stop the receiver.
func (sr *SapmDataReceiver) Stop() {
	if sr.receiver != nil {
		if err := sr.receiver.Shutdown(context.Background()); err != nil {
			log.Printf("Cannot stop Sapm receiver: %s", err.Error())
		}
	}
}

// GenConfigYAMLStr returns exporter config for the agent.
func (sr *SapmDataReceiver) GenConfigYAMLStr() string {
	// Note that this generates an exporter config for agent.
	return fmt.Sprintf(`
  sapm:
    endpoint: "http://localhost:%d/v2/trace"
    disable_compression: true`, sr.Port)
}

// ProtocolName returns protocol name as it is specified in Collector config.
func (sr *SapmDataReceiver) ProtocolName() string {
	return "sapm"
}

// SFxMetricsDataReceiver implements SignalFx format receiver.
type SFxMetricsDataReceiver struct {
	testbed.DataReceiverBase
	receiver component.MetricsReceiver
}

// Ensure SFxMetricsDataReceiver implements MetricDataSender.
var _ testbed.DataReceiver = (*SFxMetricsDataReceiver)(nil)

// NewSFxMetricsDataReceiver creates a new SFxMetricsDataReceiver that will listen on the
// specified port after Start is called.
func NewSFxMetricsDataReceiver(port int) *SFxMetricsDataReceiver {
	return &SFxMetricsDataReceiver{DataReceiverBase: testbed.DataReceiverBase{Port: port}}
}

// Start the receiver.
func (sr *SFxMetricsDataReceiver) Start(tc *testbed.MockTraceConsumer, mc *testbed.MockMetricConsumer) error {
	addr := fmt.Sprintf("localhost:%d", sr.Port)
	config := signalfxreceiver.Config{
		ReceiverSettings: configmodels.ReceiverSettings{Endpoint: addr},
	}
	var err error
	sr.receiver, err = signalfxreceiver.New(zap.L(), config, mc)
	if err != nil {
		return err
	}

	return sr.receiver.Start(context.Background(), sr)
}

// Stop the receiver.
func (sr *SFxMetricsDataReceiver) Stop() {
	sr.receiver.Shutdown(context.Background())
}

// GenConfigYAMLStr returns exporter config for the agent.
func (sr *SFxMetricsDataReceiver) GenConfigYAMLStr() string {
	// Note that this generates an exporter config for agent.
	return fmt.Sprintf(`
  signalfx:
    url: "http://localhost:%d/v2/datapoint"`, sr.Port)
}

// ProtocolName returns protocol name as it is specified in Collector config.
func (sr *SFxMetricsDataReceiver) ProtocolName() string {
	return "signalfx"
}

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
func (cr *CarbonDataReceiver) Start(tc *testbed.MockTraceConsumer, mc *testbed.MockMetricConsumer) error {
	addr := fmt.Sprintf("localhost:%d", cr.Port)
	config := carbonreceiver.Config{
		ReceiverSettings: configmodels.ReceiverSettings{Endpoint: addr},
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
func (cr *CarbonDataReceiver) Stop() {
	cr.receiver.Shutdown(context.Background())
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
