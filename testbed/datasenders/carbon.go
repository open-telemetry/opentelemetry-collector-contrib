// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasenders // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// CarbonDataSender implements MetricDataSender for Carbon metrics protocol.
type CarbonDataSender struct {
	testbed.DataSenderBase
	consumer.Metrics
}

// Ensure CarbonDataSender implements MetricDataSenderOld.
var _ testbed.MetricDataSender = (*CarbonDataSender)(nil)

// NewCarbonDataSender creates a new Carbon metric protocol sender that will send
// to the specified port after Start is called.
func NewCarbonDataSender(port int) *CarbonDataSender {
	return &CarbonDataSender{
		DataSenderBase: testbed.DataSenderBase{
			Port: port,
			Host: testbed.DefaultHost,
		},
	}
}

// Start the sender.
func (cs *CarbonDataSender) Start() error {
	factory := carbonexporter.NewFactory()
	cfg := &carbonexporter.Config{
		TCPAddr: confignet.TCPAddr{
			Endpoint: cs.GetEndpoint().String(),
		},
		TimeoutSettings: exporterhelper.TimeoutSettings{
			Timeout: 5 * time.Second,
		},
	}
	params := exportertest.NewNopCreateSettings()
	params.Logger = zap.L()

	exporter, err := factory.CreateMetricsExporter(context.Background(), params, cfg)
	if err != nil {
		return err
	}

	cs.Metrics = exporter
	return nil
}

// GenConfigYAMLStr returns receiver config for the agent.
func (cs *CarbonDataSender) GenConfigYAMLStr() string {
	// Note that this generates a receiver config for agent.
	return fmt.Sprintf(`
  carbon:
    endpoint: %s`, cs.GetEndpoint())
}

// ProtocolName returns protocol name as it is specified in Collector config.
func (cs *CarbonDataSender) ProtocolName() string {
	return "carbon"
}
