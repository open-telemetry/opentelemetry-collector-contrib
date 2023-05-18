// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testbed // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

// DataReceiver allows to receive traces or metrics. This is an interface that must
// be implemented by all protocols that want to be used in MockBackend.
// Note the terminology: DataReceiver is something that can listen and receive data
// from Collector and the corresponding entity in the Collector that sends this data is
// an exporter.
type DataReceiver interface {
	Start(tc consumer.Traces, mc consumer.Metrics, lc consumer.Logs) error
	Stop() error

	// GenConfigYAMLStr generates a config string to place in exporter part of collector config
	// so that it can send data to this receiver.
	GenConfigYAMLStr() string

	// ProtocolName returns exporterType name to use in collector config pipeline.
	ProtocolName() string
}

// DataReceiverBase implement basic functions needed by all receivers.
type DataReceiverBase struct {
	// Port on which to listen.
	Port int
}

// TODO: Move these constants.
const (
	DefaultHost     = "127.0.0.1"
	DefaultOTLPPort = 4317
)

// BaseOTLPDataReceiver implements the OTLP format receiver.
type BaseOTLPDataReceiver struct {
	DataReceiverBase
	// One of the "otlp" for OTLP over gRPC or "otlphttp" for OTLP over HTTP.
	exporterType    string
	traceReceiver   receiver.Traces
	metricsReceiver receiver.Metrics
	logReceiver     receiver.Logs
	compression     string
}

func (bor *BaseOTLPDataReceiver) Start(tc consumer.Traces, mc consumer.Metrics, lc consumer.Logs) error {
	factory := otlpreceiver.NewFactory()
	cfg := factory.CreateDefaultConfig().(*otlpreceiver.Config)
	if bor.exporterType == "otlp" {
		cfg.GRPC.NetAddr = confignet.NetAddr{Endpoint: fmt.Sprintf("127.0.0.1:%d", bor.Port), Transport: "tcp"}
		cfg.HTTP = nil
	} else {
		cfg.HTTP.Endpoint = fmt.Sprintf("127.0.0.1:%d", bor.Port)
		cfg.GRPC = nil
	}
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

	if err = bor.traceReceiver.Start(context.Background(), componenttest.NewNopHost()); err != nil {
		return err
	}
	if err = bor.metricsReceiver.Start(context.Background(), componenttest.NewNopHost()); err != nil {
		return err
	}
	return bor.logReceiver.Start(context.Background(), componenttest.NewNopHost())
}

func (bor *BaseOTLPDataReceiver) WithCompression(compression string) *BaseOTLPDataReceiver {
	bor.compression = compression
	return bor
}

func (bor *BaseOTLPDataReceiver) Stop() error {
	if err := bor.traceReceiver.Shutdown(context.Background()); err != nil {
		return err
	}
	if err := bor.metricsReceiver.Shutdown(context.Background()); err != nil {
		return err
	}
	return bor.logReceiver.Shutdown(context.Background())
}

func (bor *BaseOTLPDataReceiver) ProtocolName() string {
	return bor.exporterType
}

func (bor *BaseOTLPDataReceiver) GenConfigYAMLStr() string {
	addr := fmt.Sprintf("127.0.0.1:%d", bor.Port)
	if bor.exporterType == "otlphttp" {
		addr = "http://" + addr
	}
	// Note that this generates an exporter config for agent.
	str := fmt.Sprintf(`
  %s:
    endpoint: "%s"
    tls:
      insecure: true`, bor.exporterType, addr)

	comp := "none"
	if bor.compression != "" {
		comp = bor.compression
	}
	str += fmt.Sprintf(`
    compression: "%s"`, comp)

	return str
}

var _ DataReceiver = (*BaseOTLPDataReceiver)(nil)

// NewOTLPDataReceiver creates a new OTLP DataReceiver that will listen on the specified port after Start
// is called.
func NewOTLPDataReceiver(port int) *BaseOTLPDataReceiver {
	return &BaseOTLPDataReceiver{
		DataReceiverBase: DataReceiverBase{Port: port},
		exporterType:     "otlp",
	}
}

// NewOTLPHTTPDataReceiver creates a new OTLP/HTTP DataReceiver that will listen on the specified port after Start
// is called.
func NewOTLPHTTPDataReceiver(port int) *BaseOTLPDataReceiver {
	return &BaseOTLPDataReceiver{
		DataReceiverBase: DataReceiverBase{Port: port},
		exporterType:     "otlphttp",
	}
}
