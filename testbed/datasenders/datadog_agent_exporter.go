// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package datasenders // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/mockdatasenders/mockdatadogagentexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// datadogDataSender implements TraceDataSender for Zipkin http exporter.
type datadogDataSender struct {
	testbed.DataSenderBase
	consumer.Traces
}

// NewDatadogDataSender creates a new Zipkin exporter sender that will send
// to the specified port after Start is called.
func NewDatadogDataSender() testbed.TraceDataSender {
	return &datadogDataSender{
		DataSenderBase: testbed.DataSenderBase{
			Host: "127.0.0.1",
			Port: 8126,
		},
	}
}

func (dd *datadogDataSender) Start() error {
	factory := mockdatadogagentexporter.NewFactory()
	cfg := factory.CreateDefaultConfig().(*mockdatadogagentexporter.Config)
	cfg.Endpoint = fmt.Sprintf("http://%s:%v/v0.4/traces", dd.Host, dd.Port)
	params := componenttest.NewNopExporterCreateSettings()
	params.Logger = zap.L()

	exp, err := factory.CreateTracesExporter(context.Background(), params, cfg)
	if err != nil {
		return err
	}

	dd.Traces = exp
	return exp.Start(context.Background(), dd)
}

func (dd *datadogDataSender) GenConfigYAMLStr() string {
	return fmt.Sprintf(`
  datadog:
    endpoint: %s`, dd.GetEndpoint())
}

func (dd *datadogDataSender) ProtocolName() string {
	return "datadog"
}
