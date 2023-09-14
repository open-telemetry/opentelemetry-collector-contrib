// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasenders // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// SapmDataSender implements TraceDataSender for SAPM protocol.
type SapmDataSender struct {
	testbed.DataSenderBase
	consumer.Traces
	compression string
}

// Ensure SapmDataSender implements TraceDataSenderOld.
var _ testbed.TraceDataSender = (*SapmDataSender)(nil)

// NewSapmDataSender creates a new Sapm protocol sender that will send
// to the specified port after Start is called.
func NewSapmDataSender(port int, compression string) *SapmDataSender {
	return &SapmDataSender{
		DataSenderBase: testbed.DataSenderBase{
			Port: port,
			Host: testbed.DefaultHost,
		},
		compression: compression,
	}
}

// Start the sender.
func (je *SapmDataSender) Start() error {
	factory := sapmexporter.NewFactory()
	cfg := &sapmexporter.Config{
		Endpoint:    fmt.Sprintf("http://%s/v2/trace", je.GetEndpoint()),
		Compression: je.compression,
		AccessToken: "MyToken",
	}
	if je.compression == "" {
		cfg.DisableCompression = true
	}
	params := exportertest.NewNopCreateSettings()
	params.Logger = zap.L()

	exporter, err := factory.CreateTracesExporter(context.Background(), params, cfg)
	if err != nil {
		return err
	}

	je.Traces = exporter
	return err
}

// GenConfigYAMLStr returns receiver config for the agent.
func (je *SapmDataSender) GenConfigYAMLStr() string {
	return fmt.Sprintf(`
  sapm:
    endpoint: "%s"`, je.GetEndpoint())
}

// ProtocolName returns protocol name as it is specified in Collector config.
func (je *SapmDataSender) ProtocolName() string {
	return "sapm"
}
