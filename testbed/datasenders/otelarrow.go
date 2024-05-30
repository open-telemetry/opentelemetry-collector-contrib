// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasenders // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.uber.org/zap"
)

type otelarrowDataSender struct {
	testbed.DataSenderBase
}

func (ods *otelarrowDataSender) fillConfig(cfg *otelarrowexporter.Config) *otelarrowexporter.Config {
	cfg.Endpoint = ods.GetEndpoint().String()
	// Disable retries, we should push data and if error just log it.
	cfg.RetryConfig.Enabled = false
	// Disable sending queue, we should push data from the caller goroutine.
	cfg.QueueSettings.Enabled = false
	cfg.TLSSetting = configtls.ClientConfig{
		Insecure: true,
	}
	return cfg
}

func (ods *otelarrowDataSender) GenConfigYAMLStr() string {
	// Note that this generates a receiver config for agent.
	return fmt.Sprintf(`
  otelarrow:
    protocols:
      grpc:
        endpoint: "%s"`, ods.GetEndpoint())
}

func (ods *otelarrowDataSender) ProtocolName() string {
	return "otelarrow"
}

// otelarrowTraceDataSender implements TraceDataSender for OTelArrow traces exporter.
type otelarrowTraceDataSender struct {
	otelarrowDataSender
	consumer.Traces
}

// NewOTelArrowTraceDataSender creates a new TraceDataSender for OTelArrow traces exporter.
func NewOTelArrowTraceDataSender(host string, port int) testbed.TraceDataSender {
	return &otelarrowTraceDataSender{
		otelarrowDataSender: otelarrowDataSender{
			DataSenderBase: testbed.DataSenderBase{
				Port: port,
				Host: host,
			},
		},
	}
}

func (ote *otelarrowTraceDataSender) Start() error {
	factory := otelarrowexporter.NewFactory()
	cfg := ote.fillConfig(factory.CreateDefaultConfig().(*otelarrowexporter.Config))
	params := exportertest.NewNopCreateSettings()
	params.Logger = zap.L()

	exp, err := factory.CreateTracesExporter(context.Background(), params, cfg)
	if err != nil {
		return err
	}

	ote.Traces = exp
	return exp.Start(context.Background(), componenttest.NewNopHost())
}

// otelarrowMetricsDataSender implements MetricDataSender for OTelArrow metrics exporter.
type otelarrowMetricsDataSender struct {
	otelarrowDataSender
	consumer.Metrics
}

// NewOTelArrowMetricDataSender creates a new OTelArrow metric exporter sender that will send
// to the specified port after Start is called.
func NewOTelArrowMetricDataSender(host string, port int) testbed.MetricDataSender {
	return &otelarrowMetricsDataSender{
		otelarrowDataSender: otelarrowDataSender{
			DataSenderBase: testbed.DataSenderBase{
				Port: port,
				Host: host,
			},
		},
	}
}

func (ome *otelarrowMetricsDataSender) Start() error {
	factory := otelarrowexporter.NewFactory()
	cfg := ome.fillConfig(factory.CreateDefaultConfig().(*otelarrowexporter.Config))
	params := exportertest.NewNopCreateSettings()
	params.Logger = zap.L()

	exp, err := factory.CreateMetricsExporter(context.Background(), params, cfg)
	if err != nil {
		return err
	}

	ome.Metrics = exp
	return exp.Start(context.Background(), componenttest.NewNopHost())
}

// otelarrowLogsDataSender implements LogsDataSender for OTelArrow logs exporter.
type otelarrowLogsDataSender struct {
	otelarrowDataSender
	consumer.Logs
}

// NewOTelArrowLogsDataSender creates a new OTelArrow logs exporter sender that will send
// to the specified port after Start is called.
func NewOTelArrowLogsDataSender(host string, port int) testbed.LogDataSender {
	return &otelarrowLogsDataSender{
		otelarrowDataSender: otelarrowDataSender{
			DataSenderBase: testbed.DataSenderBase{
				Port: port,
				Host: host,
			},
		},
	}
}

func (olds *otelarrowLogsDataSender) Start() error {
	factory := otelarrowexporter.NewFactory()
	cfg := olds.fillConfig(factory.CreateDefaultConfig().(*otelarrowexporter.Config))
	params := exportertest.NewNopCreateSettings()
	params.Logger = zap.L()

	exp, err := factory.CreateLogsExporter(context.Background(), params, cfg)
	if err != nil {
		return err
	}

	olds.Logs = exp
	return exp.Start(context.Background(), componenttest.NewNopHost())
}
