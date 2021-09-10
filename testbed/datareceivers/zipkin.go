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
	"go.opentelemetry.io/collector/consumer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// zipkinDataReceiver implements Zipkin format receiver.
type zipkinDataReceiver struct {
	testbed.DataReceiverBase
	receiver component.TracesReceiver
}

// NewZipkinDataReceiver creates a new Zipkin DataReceiver that will listen on the specified port after Start
// is called.
func NewZipkinDataReceiver(port int) testbed.DataReceiver {
	return &zipkinDataReceiver{DataReceiverBase: testbed.DataReceiverBase{Port: port}}
}

func (zr *zipkinDataReceiver) Start(tc consumer.Traces, _ consumer.Metrics, _ consumer.Logs) error {
	factory := zipkinreceiver.NewFactory()
	cfg := factory.CreateDefaultConfig().(*zipkinreceiver.Config)
	cfg.Endpoint = fmt.Sprintf("localhost:%d", zr.Port)

	set := componenttest.NewNopReceiverCreateSettings()
	var err error
	zr.receiver, err = factory.CreateTracesReceiver(context.Background(), set, cfg, tc)

	if err != nil {
		return err
	}

	return zr.receiver.Start(context.Background(), zr)
}

func (zr *zipkinDataReceiver) Stop() error {
	return zr.receiver.Shutdown(context.Background())
}

func (zr *zipkinDataReceiver) GenConfigYAMLStr() string {
	// Note that this generates an exporter config for agent.
	return fmt.Sprintf(`
  zipkin:
    endpoint: http://localhost:%d/api/v2/spans
    format: json`, zr.Port)
}

func (zr *zipkinDataReceiver) ProtocolName() string {
	return "zipkin"
}
