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

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/zipkinexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// zipkinDataSender implements TraceDataSender for Zipkin http exporter.
type zipkinDataSender struct {
	testbed.DataSenderBase
	consumer.Traces
}

// NewZipkinDataSender creates a new Zipkin exporter sender that will send
// to the specified port after Start is called.
func NewZipkinDataSender(host string, port int) testbed.TraceDataSender {
	return &zipkinDataSender{
		DataSenderBase: testbed.DataSenderBase{
			Port: port,
			Host: host,
		},
	}
}

func (zs *zipkinDataSender) Start() error {
	factory := zipkinexporter.NewFactory()
	cfg := factory.CreateDefaultConfig().(*zipkinexporter.Config)
	cfg.Endpoint = fmt.Sprintf("http://%s/api/v2/spans", zs.GetEndpoint())
	// Disable retries, we should push data and if error just log it.
	cfg.RetrySettings.Enabled = false
	// Disable sending queue, we should push data from the caller goroutine.
	cfg.QueueSettings.Enabled = false
	params := exportertest.NewNopCreateSettings()
	params.Logger = zap.L()

	exp, err := factory.CreateTracesExporter(context.Background(), params, cfg)
	if err != nil {
		return err
	}

	zs.Traces = exp
	return exp.Start(context.Background(), componenttest.NewNopHost())
}

func (zs *zipkinDataSender) GenConfigYAMLStr() string {
	return fmt.Sprintf(`
  zipkin:
    endpoint: %s`, zs.GetEndpoint())
}

func (zs *zipkinDataSender) ProtocolName() string {
	return "zipkin"
}
