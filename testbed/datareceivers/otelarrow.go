// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datareceivers // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datareceivers"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

// BaseOTelArrowDataReceiver implements the OTelArrow format receiver.
type OTelArrowDataReceiver struct {
	testbed.DataReceiverBase
	traceReceiver   receiver.Traces
	metricsReceiver receiver.Metrics
	logReceiver     receiver.Logs
	compression     string
	retry           string
	sendingQueue    string
}

func (bor *OTelArrowDataReceiver) Start(tc consumer.Traces, mc consumer.Metrics, lc consumer.Logs) error {
	factory := otelarrowreceiver.NewFactory()
	cfg := factory.CreateDefaultConfig().(*otelarrowreceiver.Config)
	cfg.GRPC.NetAddr = confignet.AddrConfig{Endpoint: fmt.Sprintf("127.0.0.1:%d", bor.Port), Transport: confignet.TransportTypeTCP}
	var err error
	set := receivertest.NewNopCreateSettings()
	if bor.traceReceiver, err = factory.CreateTracesReceiver(context.Background(), set, cfg, tc); err != nil {
		return err
	}
	if bor.metricsReceiver, err = factory.CreateMetricsReceiver(context.Background(), set, cfg, mc); err != nil {
		return err
	}
	if bor.logReceiver, err = factory.CreateLogsReceiver(context.Background(), set, cfg, lc); err != nil {
		return err
	}

	// we reuse the receiver across signals. Starting the log receiver starts the metrics and traces receiver.
	return bor.logReceiver.Start(context.Background(), componenttest.NewNopHost())
}

func (bor *OTelArrowDataReceiver) WithCompression(compression string) *OTelArrowDataReceiver {
	bor.compression = compression
	return bor
}

func (bor *OTelArrowDataReceiver) WithRetry(retry string) *OTelArrowDataReceiver {
	bor.retry = retry
	return bor
}

func (bor *OTelArrowDataReceiver) WithQueue(sendingQueue string) *OTelArrowDataReceiver {
	bor.sendingQueue = sendingQueue
	return bor
}

func (bor *OTelArrowDataReceiver) Stop() error {
	// we reuse the receiver across signals. Shutting down the log receiver shuts down the metrics and traces receiver.
	return bor.logReceiver.Shutdown(context.Background())
}

func (bor *OTelArrowDataReceiver) ProtocolName() string {
	return "otelarrow"
}

func (bor *OTelArrowDataReceiver) GenConfigYAMLStr() string {
	addr := fmt.Sprintf("127.0.0.1:%d", bor.Port)

	// Note that this generates an exporter config for agent.
	str := fmt.Sprintf(`
  otelarrow:
    endpoint: "%s"
    %s
    %s
    tls:
      insecure: true`, addr, bor.retry, bor.sendingQueue)
	comp := "none"
	if bor.compression != "" {
		comp = bor.compression
	}
	str += fmt.Sprintf(`
    compression: "%s"`, comp)

	return str
}

var _ testbed.DataReceiver = (*OTelArrowDataReceiver)(nil)

// NewOTelArrowDataReceiver creates a new OTelArrow DataReceiver that will listen on the specified port after Start
// is called.
func NewOTelArrowDataReceiver(port int) *OTelArrowDataReceiver {
	return &OTelArrowDataReceiver{
		DataReceiverBase: testbed.DataReceiverBase{Port: port},
	}
}
