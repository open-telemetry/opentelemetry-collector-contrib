// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datareceivers // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datareceivers"

import (
	"context"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// datadogDataReceiver implements Datadog v3/v4/v5 format receiver.
type datadogDataReceiver struct {
	testbed.DataReceiverBase
	receiver receiver.Traces
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

	set := receiver.CreateSettings{}
	var err error
	dd.receiver, err = factory.CreateTracesReceiver(context.Background(), set, cfg, tc)

	if err != nil {
		return err
	}

	return dd.receiver.Start(context.Background(), componenttest.NewNopHost())
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
