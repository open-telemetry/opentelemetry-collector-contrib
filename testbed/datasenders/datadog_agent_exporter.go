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
	cfg.Endpoint = fmt.Sprintf("http://%s:%v/v0.4/traces", testbed.DefaultHost, 8126)

	params := exportertest.NewNopCreateSettings()
	params.Logger = zap.L()

	exp, err := factory.CreateTracesExporter(context.Background(), params, cfg)

	if err != nil {
		return err
	}

	dd.Traces = exp
	return exp.Start(context.Background(), componenttest.NewNopHost())
}

func (dd *datadogDataSender) GenConfigYAMLStr() string {
	return fmt.Sprintf(`
  datadog:
    endpoint: %s`, dd.GetEndpoint())
}

func (dd *datadogDataSender) ProtocolName() string {
	return "datadog"
}
