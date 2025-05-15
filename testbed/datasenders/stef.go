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

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stefexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

type stefDataSender struct {
	testbed.DataSenderBase
	consumer.Metrics
}

// NewStefDataSender creates a new STEF sender that will send
// to the specified port after Start is called.
func NewStefDataSender(host string, port int) testbed.MetricDataSender {
	return &stefDataSender{
		DataSenderBase: testbed.DataSenderBase{
			Port: port,
			Host: host,
		},
	}
}

func (sds *stefDataSender) Start() error {
	factory := stefexporter.NewFactory()
	cfg := factory.CreateDefaultConfig().(*stefexporter.Config)
	cfg.Endpoint = sds.GetEndpoint().String()
	cfg.TLSSetting = configtls.ClientConfig{
		Insecure: true,
	}
	params := exportertest.NewNopSettings(factory.Type())
	params.Logger = zap.L()

	exp, err := factory.CreateMetrics(context.Background(), params, cfg)
	if err != nil {
		return err
	}

	sds.Metrics = exp
	return exp.Start(context.Background(), componenttest.NewNopHost())
}

func (sds *stefDataSender) GenConfigYAMLStr() string {
	format := `
  stef:
    endpoint: "%s"`
	return fmt.Sprintf(format, sds.GetEndpoint())
}

func (sds *stefDataSender) ProtocolName() string {
	return "stef"
}
