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

	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector/receiver"
	"github.com/open-telemetry/opentelemetry-collector/testbed/testbed"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver"
)

// SapmDataReceiver implements Sapm format receiver.
type SapmDataReceiver struct {
	testbed.DataReceiverBase
	receiver receiver.TraceReceiver
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
	sr.receiver, err = sapmreceiver.New(context.Background(), zap.L(), &sapmCfg, tc)
	if err != nil {
		return err
	}

	return sr.receiver.StartTraceReception(sr)
}

// Stop the receiver.
func (sr *SapmDataReceiver) Stop() {
	if sr.receiver != nil {
		if err := sr.receiver.StopTraceReception(); err != nil {
			log.Printf("Cannot stop Sapm receiver: %s", err.Error())
		}
	}
}

// GenConfigYAMLStr returns exporter config for the agent.
func (sr *SapmDataReceiver) GenConfigYAMLStr() string {
	// Note that this generates an exporter config for agent.
	return fmt.Sprintf(`
  sapm:
    endpoint: "http://localhost:%d/v2/trace"`, sr.Port)
}

// ProtocolName returns protocol name as it is specified in Collector config.
func (sr *SapmDataReceiver) ProtocolName() string {
	return "sapm"
}

// SFxMetricsDataReceiver implements SignalFx format receiver.
type SFxMetricsDataReceiver struct {
	testbed.DataReceiverBase
	receiver receiver.MetricsReceiver
}

// Ensure SFxMetricsDataReceiver implements MetricDataSender.
var _ testbed.DataReceiver = (*SFxMetricsDataReceiver)(nil)

// NewSFxMetricsDataReceiver creates a new SFxMetricsDataReceiver that will listen on the
// specified port after Start is called.
func NewSFxMetricsDataReceiver(port int) *SFxMetricsDataReceiver {
	return &SFxMetricsDataReceiver{DataReceiverBase: testbed.DataReceiverBase{Port: port}}
}

// Start the receiver.
func (or *SFxMetricsDataReceiver) Start(tc *testbed.MockTraceConsumer, mc *testbed.MockMetricConsumer) error {
	addr := fmt.Sprintf("localhost:%d", or.Port)
	config := signalfxreceiver.Config{
		ReceiverSettings: configmodels.ReceiverSettings{Endpoint: addr},
	}
	var err error
	or.receiver, err = signalfxreceiver.New(zap.L(), config, mc)
	if err != nil {
		return err
	}

	return or.receiver.StartMetricsReception(or)
}

// Stop the receiver.
func (or *SFxMetricsDataReceiver) Stop() {
	or.receiver.StopMetricsReception()
}

// GenConfigYAMLStr returns exporter config for the agent.
func (or *SFxMetricsDataReceiver) GenConfigYAMLStr() string {
	// Note that this generates an exporter config for agent.
	return fmt.Sprintf(`
  signalfx:
    url: "http://localhost:%d/v2/datapoint"`, or.Port)
}

// ProtocolName returns protocol name as it is specified in Collector config.
func (or *SFxMetricsDataReceiver) ProtocolName() string {
	return "signalfx"
}
