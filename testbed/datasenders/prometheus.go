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

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

type prometheusDataSender struct {
	testbed.DataSenderBase
	consumer.Metrics
	namespace string
}

// NewPrometheusDataSender creates a new Prometheus sender that will expose data
// on the specified port after Start is called.
func NewPrometheusDataSender(host string, port int) testbed.MetricDataSender {
	return &prometheusDataSender{
		DataSenderBase: testbed.DataSenderBase{
			Port: port,
			Host: host,
		},
	}
}

func (pds *prometheusDataSender) Start() error {
	factory := prometheusexporter.NewFactory()
	cfg := factory.CreateDefaultConfig().(*prometheusexporter.Config)
	cfg.Endpoint = pds.GetEndpoint().String()
	cfg.Namespace = pds.namespace
	params := exportertest.NewNopCreateSettings()
	params.Logger = zap.L()

	exp, err := factory.CreateMetricsExporter(context.Background(), params, cfg)
	if err != nil {
		return err
	}

	pds.Metrics = exp
	return exp.Start(context.Background(), componenttest.NewNopHost())
}

func (pds *prometheusDataSender) GenConfigYAMLStr() string {
	format := `
  prometheus:
    config:
      scrape_configs:
        - job_name: 'testbed'
          scrape_interval: 100ms
          static_configs:
            - targets: ['%s']
`
	return fmt.Sprintf(format, pds.GetEndpoint())
}

func (pds *prometheusDataSender) ProtocolName() string {
	return "prometheus"
}
