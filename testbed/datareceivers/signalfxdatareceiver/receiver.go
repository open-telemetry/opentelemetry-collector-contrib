// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Deprecated: use testbed instead
package signalfxdatareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datareceivers/signalfxdatareceiver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver" //nolint:staticcheck // SA1019
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// SFxMetricsDataReceiver implements SignalFx format receiver.
//
// Deprecated: use testbed.BaseOTLPDataReceiver instead
type SFxMetricsDataReceiver struct {
	testbed.DataReceiverBase
	receiver receiver.Metrics
}

// Ensure SFxMetricsDataReceiver implements MetricDataSender.
var _ testbed.DataReceiver = (*SFxMetricsDataReceiver)(nil)

// NewSFxMetricsDataReceiver creates a new SFxMetricsDataReceiver that will listen on the
// specified port after Start is called.
//
// Deprecated: use testbed.NewOTLPDataReceiver(port int) instead
func NewSFxMetricsDataReceiver(port int) *SFxMetricsDataReceiver {
	return &SFxMetricsDataReceiver{DataReceiverBase: testbed.DataReceiverBase{Port: port}}
}

// Start the receiver.
func (sr *SFxMetricsDataReceiver) Start(_ consumer.Traces, mc consumer.Metrics, _ consumer.Logs) error {
	serverConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	serverConfig.WriteTimeout = 0
	serverConfig.ReadHeaderTimeout = 0
	serverConfig.IdleTimeout = 0           //nolint:staticcheck // SA1019: see TODO above
	serverConfig.KeepAlivesEnabled = false //nolint:staticcheck // SA1019: see TODO above
	serverConfig.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  fmt.Sprintf("127.0.0.1:%d", sr.Port),
	}
	config := signalfxreceiver.Config{
		ServerConfig: serverConfig,
	}
	var err error
	f := signalfxreceiver.NewFactory()
	sr.receiver, err = f.CreateMetrics(context.Background(), receivertest.NewNopSettings(f.Type()), &config, mc)
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
func (*SFxMetricsDataReceiver) ProtocolName() string {
	return "signalfx"
}
