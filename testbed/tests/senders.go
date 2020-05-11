// Copyright 2019 OpenTelemetry Authors
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

package tests

import (
	"context"
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/consumer/pdata"
	"github.com/open-telemetry/opentelemetry-collector/testbed/testbed"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter"
)

// SapmDataSender implements TraceDataSender for SAPM protocol.
type SapmDataSender struct {
	exporter component.TraceExporter
	port     int
}

// Ensure SapmDataSender implements TraceDataSenderOld.
var _ testbed.TraceDataSender = (*SapmDataSender)(nil)

// NewSapmDataSender creates a new Sapm protocol sender that will send
// to the specified port after Start is called.
func NewSapmDataSender(port int) *SapmDataSender {
	return &SapmDataSender{port: port}
}

// Start the sender.
func (je *SapmDataSender) Start() error {
	cfg := &sapmexporter.Config{
		Endpoint:           fmt.Sprintf("http://localhost:%d/v2/trace", je.port),
		DisableCompression: true,
	}

	var err error
	factory := sapmexporter.Factory{}
	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	exporter, err := factory.CreateTraceExporter(context.Background(), params, cfg)

	if err != nil {
		return err
	}

	je.exporter = exporter
	return err
}

// SendSpans sends spans. Can be called after Start.
func (je *SapmDataSender) SendSpans(traces pdata.Traces) error {
	return je.exporter.ConsumeTraces(context.Background(), traces)
}

// Flush previously sent spans.
func (je *SapmDataSender) Flush() {
}

// GenConfigYAMLStr returns receiver config for the agent.
func (je *SapmDataSender) GenConfigYAMLStr() string {
	return fmt.Sprintf(`
  sapm:
    endpoint: "localhost:%d"`, je.port)
}

// GetCollectorPort returns receiver port for the Collector.
func (je *SapmDataSender) GetCollectorPort() int {
	return je.port
}

// ProtocolName returns protocol name as it is specified in Collector config.
func (je *SapmDataSender) ProtocolName() string {
	return "sapm"
}

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
		IngestURL: fmt.Sprintf("http://localhost:%d/v2/datapoint", sf.port),
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

// CarbonDataSender implements MetricDataSender for Carbon metrics protocol.
type CarbonDataSender struct {
	exporter component.MetricsExporterOld
	port     int
}

// Ensure CarbonDataSender implements MetricDataSenderOld.
var _ testbed.MetricDataSenderOld = (*CarbonDataSender)(nil)

// NewCarbonDataSender creates a new Carbon metric protocol sender that will send
// to the specified port after Start is called.
func NewCarbonDataSender(port int) *CarbonDataSender {
	return &CarbonDataSender{port: port}
}

// Start the sender.
func (cs *CarbonDataSender) Start() error {
	cfg := &carbonexporter.Config{
		Endpoint: fmt.Sprintf("localhost:%d", cs.port),
		Timeout:  5 * time.Second,
	}

	factory := carbonexporter.Factory{}
	exporter, err := factory.CreateMetricsExporter(zap.L(), cfg)

	if err != nil {
		return err
	}

	cs.exporter = exporter
	return nil
}

// SendMetrics sends metrics. Can be called after Start.
func (cs *CarbonDataSender) SendMetrics(metrics consumerdata.MetricsData) error {
	return cs.exporter.ConsumeMetricsData(context.Background(), metrics)
}

// Flush previously sent metrics.
func (cs *CarbonDataSender) Flush() {
}

// GenConfigYAMLStr returns receiver config for the agent.
func (cs *CarbonDataSender) GenConfigYAMLStr() string {
	// Note that this generates a receiver config for agent.
	return fmt.Sprintf(`
  carbon:
    endpoint: localhost:%d`, cs.port)
}

// GetCollectorPort returns receiver port for the Collector.
func (cs *CarbonDataSender) GetCollectorPort() int {
	return cs.port
}

// ProtocolName returns protocol name as it is specified in Collector config.
func (cs *CarbonDataSender) ProtocolName() string {
	return "carbon"
}
