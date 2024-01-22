// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasenders // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// SplunkHecMetricsDataSender implements MetricDataSender for Splunk HEC protocol.
type SplunkHecMetricsDataSender struct {
	testbed.DataSenderBase
	consumer.Metrics
}

// Ensure SplunkHecMetricsDataSender implements MetricDataSenderOld.
var _ testbed.MetricDataSender = (*SplunkHecMetricsDataSender)(nil)

// NewSplunkHecMetricsDataSender creates a new Splunk HEC metric protocol sender that will send
// to the specified port after Start is called.
func NewSplunkHecMetricsDataSender(port int) *SplunkHecMetricsDataSender {
	return &SplunkHecMetricsDataSender{
		DataSenderBase: testbed.DataSenderBase{
			Port: port,
			Host: testbed.DefaultHost,
		},
	}
}

// Start the sender.
func (sf *SplunkHecMetricsDataSender) Start() error {
	factory := splunkhecexporter.NewFactory()
	clientSettings := confighttp.NewDefaultHTTPClientSettings()
	clientSettings.Endpoint = fmt.Sprintf("http://%s", sf.GetEndpoint())
	cfg := &splunkhecexporter.Config{
		HTTPClientSettings: clientSettings,
		Token:              "access_token",
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
func (sf *SplunkHecMetricsDataSender) GenConfigYAMLStr() string {
	// Note that this generates a receiver config for agent.
	return fmt.Sprintf(`
  splunk_hec:
    endpoint: "%s"`, sf.GetEndpoint())
}

// ProtocolName returns protocol name as it is specified in Collector config.
func (sf *SplunkHecMetricsDataSender) ProtocolName() string {
	return "splunk_hec"
}
