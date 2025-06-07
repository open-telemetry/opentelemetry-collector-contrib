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

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opencensusexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

type ocDataSender struct {
	testbed.DataSenderBase
}

func (ods *ocDataSender) fillConfig(cfg *opencensusexporter.Config) *opencensusexporter.Config {
	cfg.Endpoint = ods.GetEndpoint().String()
	cfg.TLS = configtls.ClientConfig{
		Insecure: true,
	}
	return cfg
}

func (ods *ocDataSender) GenConfigYAMLStr() string {
	// Note that this generates a receiver config for agent.
	return fmt.Sprintf(`
  opencensus:
    endpoint: "%s"`, ods.GetEndpoint())
}

func (ods *ocDataSender) ProtocolName() string {
	return "opencensus"
}

// ocTracesDataSender implements TraceDataSender for OpenCensus trace exporter.
type ocTracesDataSender struct {
	ocDataSender
	consumer.Traces
}

// NewOCTraceDataSender creates a new ocTracesDataSender that will send
// to the specified port after Start is called.
func NewOCTraceDataSender(host string, port int) testbed.TraceDataSender {
	return &ocTracesDataSender{
		ocDataSender: ocDataSender{
			DataSenderBase: testbed.DataSenderBase{
				Port: port,
				Host: host,
			},
		},
	}
}

func (ote *ocTracesDataSender) Start() error {
	factory := opencensusexporter.NewFactory()
	cfg := ote.fillConfig(factory.CreateDefaultConfig().(*opencensusexporter.Config))
	params := exportertest.NewNopSettings(factory.Type())
	params.Logger = zap.L()

	exp, err := factory.CreateTraces(context.Background(), params, cfg)
	if err != nil {
		return err
	}

	ote.Traces = exp
	return exp.Start(context.Background(), componenttest.NewNopHost())
}

// ocMetricsDataSender implements MetricDataSender for OpenCensus metrics exporter.
type ocMetricsDataSender struct {
	ocDataSender
	consumer.Metrics
}

// NewOCMetricDataSender creates a new OpenCensus metric exporter sender that will send
// to the specified port after Start is called.
func NewOCMetricDataSender(host string, port int) testbed.MetricDataSender {
	return &ocMetricsDataSender{
		ocDataSender: ocDataSender{
			DataSenderBase: testbed.DataSenderBase{
				Port: port,
				Host: host,
			},
		},
	}
}

func (ome *ocMetricsDataSender) Start() error {
	factory := opencensusexporter.NewFactory()
	cfg := ome.fillConfig(factory.CreateDefaultConfig().(*opencensusexporter.Config))
	params := exportertest.NewNopSettings(factory.Type())
	params.Logger = zap.L()

	exp, err := factory.CreateMetrics(context.Background(), params, cfg)
	if err != nil {
		return err
	}

	ome.Metrics = exp
	return exp.Start(context.Background(), componenttest.NewNopHost())
}
