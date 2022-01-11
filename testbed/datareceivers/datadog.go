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

package datareceivers // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datareceivers"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// datadogDataReceiver implements Datadog v3/v4/v5 format receiver.
type datadogDataReceiver struct {
	testbed.DataReceiverBase
	receiver component.TracesReceiver
}

// NewDataDogDataReceiver creates a new DD DataReceiver that will listen on the specified port after Start
// is called.
func NewDataDogDataReceiver() testbed.DataReceiver {
	return &datadogDataReceiver{DataReceiverBase: testbed.DataReceiverBase{}}
}

func (dd *datadogDataReceiver) Start(tc consumer.Traces, _ consumer.Metrics, _ consumer.Logs) error {
	factory := datadogreceiver.NewFactory()
	cfg := factory.CreateDefaultConfig().(*datadogreceiver.Config)
	cfg.Endpoint = "0.0.0.0:8126"

	set := componenttest.NewNopReceiverCreateSettings()
	var err error
	dd.receiver, err = factory.CreateTracesReceiver(context.Background(), set, cfg, tc)

	if err != nil {
		return err
	}

	return dd.receiver.Start(context.Background(), dd)
}

func (dd *datadogDataReceiver) Stop() error {
	return dd.receiver.Shutdown(context.Background())
}

func (dd *datadogDataReceiver) GenConfigYAMLStr() string {
	// Note that this generates an exporter config for agent.
	return `
  datadog:
    endpoint: 0.0.0.0:8126
    `
}

func (dd *datadogDataReceiver) ProtocolName() string {
	return "datadog"
}
