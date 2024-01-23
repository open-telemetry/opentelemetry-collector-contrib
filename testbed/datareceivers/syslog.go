// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datareceivers // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datareceivers"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/tcp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/syslogreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// SyslogDataReceiver implements Syslog format receiver.
type SyslogDataReceiver struct {
	testbed.DataReceiverBase
	receiver receiver.Logs
	protocol string
}

// Ensure SyslogDataReceiver implements LogDataReceiver.
var _ testbed.DataReceiver = (*SyslogDataReceiver)(nil)

// NewSyslogDataReceiver creates a new SyslogDataReceiver that will listen on the
// specified port after Start is called.
func NewSyslogDataReceiver(protocol string, port int) *SyslogDataReceiver {
	return &SyslogDataReceiver{DataReceiverBase: testbed.DataReceiverBase{Port: port}, protocol: protocol}
}

// Start the receiver.
func (cr *SyslogDataReceiver) Start(_ consumer.Traces, _ consumer.Metrics, lc consumer.Logs) error {
	factory := syslogreceiver.NewFactory()
	addr := fmt.Sprintf("127.0.0.1:%d", cr.Port)
	cfg := factory.CreateDefaultConfig().(*syslogreceiver.SysLogConfig)
	cfg.InputConfig.TCP = &tcp.BaseConfig{
		ListenAddress: addr,
	}
	cfg.InputConfig.Protocol = cr.protocol

	set := receivertest.NewNopCreateSettings()
	var err error
	cr.receiver, err = factory.CreateLogsReceiver(context.Background(), set, cfg, lc)
	if err != nil {
		return err
	}

	return cr.receiver.Start(context.Background(), componenttest.NewNopHost())
}

// Stop the receiver.
func (cr *SyslogDataReceiver) Stop() error {
	return cr.receiver.Shutdown(context.Background())
}

// GenConfigYAMLStr returns receiver config for the agent.
func (cr *SyslogDataReceiver) GenConfigYAMLStr() string {
	// Note that this generates an receiver config for agent.
	return fmt.Sprintf(`
  syslog:
    endpoint: "127.0.0.1:%d"`, cr.Port)
}

// ProtocolName returns protocol name as it is specified in Collector config.
func (cr *SyslogDataReceiver) ProtocolName() string {
	return "tcp"
}
