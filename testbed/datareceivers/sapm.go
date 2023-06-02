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

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// SapmDataReceiver implements Sapm format receiver.
type SapmDataReceiver struct {
	testbed.DataReceiverBase
	receiver receiver.Traces
}

// NewSapmDataReceiver creates a new SapmDataReceiver.
func NewSapmDataReceiver(port int) *SapmDataReceiver {
	return &SapmDataReceiver{DataReceiverBase: testbed.DataReceiverBase{Port: port}}
}

// Start the receiver.
func (sr *SapmDataReceiver) Start(tc consumer.Traces, _ consumer.Metrics, _ consumer.Logs) error {
	sapmCfg := sapmreceiver.Config{
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: fmt.Sprintf("127.0.0.1:%d", sr.Port),
		},
		AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{AccessTokenPassthrough: true},
	}
	var err error
	params := receivertest.NewNopCreateSettings()
	sr.receiver, err = sapmreceiver.NewFactory().CreateTracesReceiver(context.Background(), params, &sapmCfg, tc)
	if err != nil {
		return err
	}

	return sr.receiver.Start(context.Background(), componenttest.NewNopHost())
}

// Stop the receiver.
func (sr *SapmDataReceiver) Stop() error {
	if sr.receiver != nil {
		return sr.receiver.Shutdown(context.Background())
	}
	return nil
}

// GenConfigYAMLStr returns exporter config for the agent.
func (sr *SapmDataReceiver) GenConfigYAMLStr() string {
	// Note that this generates an exporter config for agent.
	return fmt.Sprintf(`
  sapm:
    endpoint: "http://127.0.0.1:%d/v2/trace"
    disable_compression: true
    access_token_passthrough: true`, sr.Port)
}

// ProtocolName returns protocol name as it is specified in Collector config.
func (sr *SapmDataReceiver) ProtocolName() string {
	return "sapm"
}
