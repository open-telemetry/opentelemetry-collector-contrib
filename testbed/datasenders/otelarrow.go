// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasenders // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

type otelarrowDataSender struct {
	testbed.DataSenderBase
	consumer.Metrics
}

// NewOtelarrowDataSender creates a new sender that will send
// to the specified port after Start is called.
func NewOtelarrowDataSender(host string, port int) testbed.MetricDataSender {
	return &otelarrowDataSender{
		DataSenderBase: testbed.DataSenderBase{
			Port: port,
			Host: host,
		},
	}
}

func (ds *otelarrowDataSender) Start() error {
	factory := otelarrowexporter.NewFactory()
	cfg := factory.CreateDefaultConfig().(*otelarrowexporter.Config)
	cfg.Endpoint = ds.GetEndpoint().String()
	cfg.TLS = configtls.ClientConfig{
		Insecure: true,
	}
	cfg.Compression = "none"
	params := exportertest.NewNopSettings(factory.Type())
	params.Logger = zap.L()

	exp, err := factory.CreateMetrics(context.Background(), params, cfg)
	if err != nil {
		return err
	}

	ds.Metrics = exp
	return exp.Start(context.Background(), componenttest.NewNopHost())
}

func (ds *otelarrowDataSender) GenConfigYAMLStr() string {
	format := `
  otelarrow:
    protocols:
      grpc:
        endpoint: "%s"`
	return fmt.Sprintf(format, ds.GetEndpoint())
}

func (*otelarrowDataSender) ProtocolName() string {
	return "otelarrow"
}
