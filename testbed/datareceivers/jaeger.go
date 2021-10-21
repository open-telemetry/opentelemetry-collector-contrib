// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
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
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// jaegerDataReceiver implements Jaeger format receiver.
type jaegerDataReceiver struct {
	testbed.DataReceiverBase
	receiver component.TracesReceiver
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
		NetAddr: confignet.NetAddr{Endpoint: fmt.Sprintf("localhost:%d", jr.Port), Transport: "tcp"},
	}
	var err error
	set := componenttest.NewNopReceiverCreateSettings()
	jr.receiver, err = factory.CreateTracesReceiver(context.Background(), set, cfg, tc)
	if err != nil {
		return err
	}

	return jr.receiver.Start(context.Background(), jr)
}

func (jr *jaegerDataReceiver) Stop() error {
	return jr.receiver.Shutdown(context.Background())
}

func (jr *jaegerDataReceiver) GenConfigYAMLStr() string {
	// Note that this generates an exporter config for agent.
	return fmt.Sprintf(`
  jaeger:
    endpoint: "localhost:%d"
    tls:
      insecure: true`, jr.Port)
}

func (jr *jaegerDataReceiver) ProtocolName() string {
	return "jaeger"
}
