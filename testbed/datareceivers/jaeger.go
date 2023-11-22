// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datareceivers // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datareceivers"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// jaegerDataReceiver implements Jaeger format receiver.
type jaegerDataReceiver struct {
	testbed.DataReceiverBase
	receiver receiver.Traces
}

// NewJaegerDataReceiver creates a new Jaeger DataReceiver that will listen on the specified port after Start
// is called.
func NewJaegerDataReceiver(port int) testbed.DataReceiver {
	return &jaegerDataReceiver{DataReceiverBase: testbed.DataReceiverBase{Port: port}}
}

func (jr *jaegerDataReceiver) Start(tc consumer.Traces, _ consumer.Metrics, _ consumer.Logs) error {
	factory := jaegerreceiver.NewFactory()
	cfg := factory.CreateDefaultConfig().(*jaegerreceiver.Config)
	cfg.Protocols.GRPC = &configgrpc.GRPCServerSettings{
		NetAddr: confignet.NetAddr{Endpoint: fmt.Sprintf("127.0.0.1:%d", jr.Port), Transport: "tcp"},
	}
	var err error
	set := receivertest.NewNopCreateSettings()
	jr.receiver, err = factory.CreateTracesReceiver(context.Background(), set, cfg, tc)
	if err != nil {
		return err
	}

	return jr.receiver.Start(context.Background(), componenttest.NewNopHost())
}

func (jr *jaegerDataReceiver) Stop() error {
	return jr.receiver.Shutdown(context.Background())
}

func (jr *jaegerDataReceiver) GenConfigYAMLStr() string {
	// Note that this generates an exporter config for agent.
	// The Jaeger exporter is no longer supported, therefore
	// we export data using OTLP instead
	return fmt.Sprintf(`
  otlp/jaeger:
    endpoint: "127.0.0.1:%d"
    tls:
      insecure: true`, jr.Port)
}

func (jr *jaegerDataReceiver) ProtocolName() string {
	return "otlp/jaeger"
}
