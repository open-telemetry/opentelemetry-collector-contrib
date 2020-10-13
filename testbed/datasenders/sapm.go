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
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/testbed/testbed"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter"
)

// SapmDataSender implements TraceDataSender for SAPM protocol.
type SapmDataSender struct {
	testbed.DataSenderBase
	consumer.TraceConsumer
}

// Ensure SapmDataSender implements TraceDataSenderOld.
var _ testbed.TraceDataSender = (*SapmDataSender)(nil)

// NewSapmDataSender creates a new Sapm protocol sender that will send
// to the specified port after Start is called.
func NewSapmDataSender(port int) *SapmDataSender {
	return &SapmDataSender{
		DataSenderBase: testbed.DataSenderBase{
			Port: port,
			Host: testbed.DefaultHost,
		},
	}
}

// Start the sender.
func (je *SapmDataSender) Start() error {
	cfg := &sapmexporter.Config{
		Endpoint:           fmt.Sprintf("http://%s/v2/trace", je.GetEndpoint()),
		DisableCompression: true,
		AccessToken:        "MyToken",
	}

	var err error
	factory := sapmexporter.NewFactory()
	params := component.ExporterCreateParams{Logger: zap.L()}
	exporter, err := factory.CreateTraceExporter(context.Background(), params, cfg)

	if err != nil {
		return err
	}

	je.TraceConsumer = exporter
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
