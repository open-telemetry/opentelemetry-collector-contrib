// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datareceivers // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datareceivers"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// SFxMetricsDataReceiver implements SignalFx format receiver.
type SFxMetricsDataReceiver struct {
	testbed.DataReceiverBase
	receiver receiver.Metrics
}

// Ensure SFxMetricsDataReceiver implements MetricDataSender.
var _ testbed.DataReceiver = (*SFxMetricsDataReceiver)(nil)

// NewSFxMetricsDataReceiver creates a new SFxMetricsDataReceiver that will listen on the
// specified port after Start is called.
func NewSFxMetricsDataReceiver(port int) *SFxMetricsDataReceiver {
	return &SFxMetricsDataReceiver{DataReceiverBase: testbed.DataReceiverBase{Port: port}}
}

// Start the receiver.
func (sr *SFxMetricsDataReceiver) Start(_ consumer.Traces, mc consumer.Metrics, _ consumer.Logs) error {
	config := signalfxreceiver.Config{
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: fmt.Sprintf("127.0.0.1:%d", sr.Port),
		},
	}
	var err error
	f := signalfxreceiver.NewFactory()
	sr.receiver, err = f.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), &config, mc)
	if err != nil {
		return err
	}

	return sr.receiver.Start(context.Background(), componenttest.NewNopHost())
}

// Stop the receiver.
func (sr *SFxMetricsDataReceiver) Stop() error {
	return sr.receiver.Shutdown(context.Background())
}

// GenConfigYAMLStr returns exporter config for the agent.
func (sr *SFxMetricsDataReceiver) GenConfigYAMLStr() string {
	// Note that this generates an exporter config for agent.
	return fmt.Sprintf(`
    signalfx:
      ingest_url: "http://127.0.0.1:%d"
      api_url: "http://127.0.0.1/"
      access_token: "access_token"`, sr.Port)
}

// ProtocolName returns protocol name as it is specified in Collector config.
func (sr *SFxMetricsDataReceiver) ProtocolName() string {
	return "signalfx"
}
