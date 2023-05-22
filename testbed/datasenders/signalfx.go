// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasenders // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// SFxMetricsDataSender implements MetricDataSender for SignalFx metrics protocol.
type SFxMetricsDataSender struct {
	testbed.DataSenderBase
	consumer.Metrics
}

// Ensure SFxMetricsDataSender implements MetricDataSenderOld.
var _ testbed.MetricDataSender = (*SFxMetricsDataSender)(nil)

// NewSFxMetricDataSender creates a new SignalFx metric protocol sender that will send
// to the specified port after Start is called.
func NewSFxMetricDataSender(port int) *SFxMetricsDataSender {
	return &SFxMetricsDataSender{
		DataSenderBase: testbed.DataSenderBase{
			Port: port,
			Host: testbed.DefaultHost,
		},
	}
}

// Start the sender.
func (sf *SFxMetricsDataSender) Start() error {
	factory := signalfxexporter.NewFactory()
	cfg := &signalfxexporter.Config{
		IngestURL:   fmt.Sprintf("http://%s", sf.GetEndpoint()),
		APIURL:      "http://127.0.0.1",
		AccessToken: "access_token",
	}
	params := exportertest.NewNopCreateSettings()
	params.Logger = zap.L()

	exporter, err := factory.CreateMetricsExporter(context.Background(), params, cfg)
	if err != nil {
		return err
	}

	err = exporter.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		return err
	}

	sf.Metrics = exporter
	return nil
}

// GenConfigYAMLStr returns receiver config for the agent.
func (sf *SFxMetricsDataSender) GenConfigYAMLStr() string {
	// Note that this generates a receiver config for agent.
	return fmt.Sprintf(`
  signalfx:
    endpoint: "%s"`, sf.GetEndpoint())
}

// ProtocolName returns protocol name as it is specified in Collector config.
func (sf *SFxMetricsDataSender) ProtocolName() string {
	return "signalfx"
}
