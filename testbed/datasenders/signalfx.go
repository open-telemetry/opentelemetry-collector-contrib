// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package datasenders

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/testbed/testbed"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter"
)

// SFxMetricsDataSender implements MetricDataSender for SignalFx metrics protocol.
type SFxMetricsDataSender struct {
	exporter component.MetricsExporterOld
	port     int
}

// Ensure SFxMetricsDataSender implements MetricDataSenderOld.
var _ testbed.MetricDataSenderOld = (*SFxMetricsDataSender)(nil)

// NewSFxMetricDataSender creates a new SignalFx metric protocol sender that will send
// to the specified port after Start is called.
func NewSFxMetricDataSender(port int) *SFxMetricsDataSender {
	return &SFxMetricsDataSender{port: port}
}

// Start the sender.
func (sf *SFxMetricsDataSender) Start() error {
	cfg := &signalfxexporter.Config{
		IngestURL:   fmt.Sprintf("http://localhost:%d/v2/datapoint", sf.port),
		APIURL:      "http://localhost",
		AccessToken: "access_token",
	}

	factory := signalfxexporter.Factory{}
	exporter, err := factory.CreateMetricsExporter(zap.L(), cfg)

	if err != nil {
		return err
	}

	sf.exporter = exporter
	return nil
}

// SendMetrics sends metrics. Can be called after Start.
func (sf *SFxMetricsDataSender) SendMetrics(metrics consumerdata.MetricsData) error {
	return sf.exporter.ConsumeMetricsData(context.Background(), metrics)
}

// Flush previously sent metrics.
func (sf *SFxMetricsDataSender) Flush() {
}

// GenConfigYAMLStr returns receiver config for the agent.
func (sf *SFxMetricsDataSender) GenConfigYAMLStr() string {
	// Note that this generates a receiver config for agent.
	return fmt.Sprintf(`
  signalfx:
    endpoint: "localhost:%d"`, sf.port)
}

// GetCollectorPort returns receiver port for the Collector.
func (sf *SFxMetricsDataSender) GetCollectorPort() int {
	return sf.port
}

// ProtocolName returns protocol name as it is specified in Collector config.
func (sf *SFxMetricsDataSender) ProtocolName() string {
	return "signalfx"
}
