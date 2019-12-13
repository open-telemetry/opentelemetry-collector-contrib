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

	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/exporter"
	"github.com/open-telemetry/opentelemetry-collector/testbed/testbed"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter"
)

// SapmDataSender implements TraceDataSender for SAPM protocol.
type SapmDataSender struct {
	exporter exporter.TraceExporter
	port     int
}

// Ensure SapmDataSender implements TraceDataSender.
var _ testbed.TraceDataSender = (*SapmDataSender)(nil)

// NewSapmDataSender creates a new Sapm protocol sender that will send
// to the specified port after Start is called.
func NewSapmDataSender(port int) *SapmDataSender {
	return &SapmDataSender{port: port}
}

// Start the sender.
func (je *SapmDataSender) Start() error {
	cfg := &sapmexporter.Config{
		Endpoint: fmt.Sprintf("http://localhost:%d/v2/trace", je.port),
	}

	var err error
	factory := sapmexporter.Factory{}
	exporter, err := factory.CreateTraceExporter(zap.L(), cfg)

	if err != nil {
		return err
	}

	je.exporter = exporter
	return err
}

// SendSpans sends spans. Can be called after Start.
func (je *SapmDataSender) SendSpans(traces consumerdata.TraceData) error {
	return je.exporter.ConsumeTraceData(context.Background(), traces)
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
