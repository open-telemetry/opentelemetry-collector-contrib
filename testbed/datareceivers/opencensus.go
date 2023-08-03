// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datareceivers // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datareceivers"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// ocDataReceiver implements OpenCensus format receiver.
type ocDataReceiver struct {
	testbed.DataReceiverBase
	traceReceiver   receiver.Traces
	metricsReceiver receiver.Metrics
}

// NewOCDataReceiver creates a new ocDataReceiver that will listen on the specified port after Start
// is called.
func NewOCDataReceiver(port int) testbed.DataReceiver {
	return &ocDataReceiver{DataReceiverBase: testbed.DataReceiverBase{Port: port}}
}

func (or *ocDataReceiver) Start(tc consumer.Traces, mc consumer.Metrics, _ consumer.Logs) error {
	factory := opencensusreceiver.NewFactory()
	cfg := factory.CreateDefaultConfig().(*opencensusreceiver.Config)
	cfg.NetAddr = confignet.NetAddr{Endpoint: fmt.Sprintf("127.0.0.1:%d", or.Port), Transport: "tcp"}
	var err error
	set := receivertest.NewNopCreateSettings()
	if or.traceReceiver, err = factory.CreateTracesReceiver(context.Background(), set, cfg, tc); err != nil {
		return err
	}
	if or.metricsReceiver, err = factory.CreateMetricsReceiver(context.Background(), set, cfg, mc); err != nil {
		return err
	}
	if err = or.traceReceiver.Start(context.Background(), componenttest.NewNopHost()); err != nil {
		return err
	}
	return or.metricsReceiver.Start(context.Background(), componenttest.NewNopHost())
}

func (or *ocDataReceiver) Stop() error {
	if err := or.traceReceiver.Shutdown(context.Background()); err != nil {
		return err
	}
	return or.metricsReceiver.Shutdown(context.Background())
}

func (or *ocDataReceiver) GenConfigYAMLStr() string {
	// Note that this generates an exporter config for agent.
	return fmt.Sprintf(`
  opencensus:
    endpoint: "127.0.0.1:%d"
    tls:
      insecure: true`, or.Port)
}

func (or *ocDataReceiver) ProtocolName() string {
	return "opencensus"
}
