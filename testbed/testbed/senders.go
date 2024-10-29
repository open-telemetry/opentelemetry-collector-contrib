// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testbed // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"

import (
	"context"
	"fmt"
	"net"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
	"go.uber.org/zap"
)

// DataSender defines the interface that allows sending data. This is an interface
// that must be implemented by all protocols that want to be used in ProviderSender.
// Note the terminology: DataSender is something that sends data to Collector
// and the corresponding entity that receives the data in the Collector is a receiver.
type DataSender interface {
	// Start sender and connect to the configured endpoint. Must be called before
	// sending data.
	Start() error

	// Flush sends any accumulated data.
	Flush()

	// GetEndpoint returns the address to which this sender will send data.
	GetEndpoint() net.Addr

	// GenConfigYAMLStr generates a config string to place in receiver part of collector config
	// so that it can receive data from this sender.
	GenConfigYAMLStr() string

	// ProtocolName returns exporter name to use in collector config pipeline.
	ProtocolName() string
}

// TraceDataSender defines the interface that allows sending trace data. It adds ability
// to send a batch of Spans to the DataSender interface.
type TraceDataSender interface {
	DataSender
	consumer.Traces
}

// MetricDataSender defines the interface that allows sending metric data. It adds ability
// to send a batch of Metrics to the DataSender interface.
type MetricDataSender interface {
	DataSender
	consumer.Metrics
}

// LogDataSender defines the interface that allows sending log data. It adds ability
// to send a batch of Logs to the DataSender interface.
type LogDataSender interface {
	DataSender
	consumer.Logs
}

type DataSenderBase struct {
	Port int
	Host string
}

func (dsb *DataSenderBase) GetEndpoint() net.Addr {
	addr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", dsb.Host, dsb.Port))
	return addr
}

func (dsb *DataSenderBase) Flush() {
	// Exporter interface does not support Flush, so nothing to do.
}

type otlpHTTPDataSender struct {
	DataSenderBase
	compression configcompression.Type
}

func (ods *otlpHTTPDataSender) fillConfig(cfg *otlphttpexporter.Config) *otlphttpexporter.Config {
	cfg.Endpoint = fmt.Sprintf("http://%s", ods.GetEndpoint())
	// Disable retries, we should push data and if error just log it.
	cfg.RetryConfig.Enabled = false
	// Disable sending queue, we should push data from the caller goroutine.
	cfg.QueueConfig.Enabled = false
	cfg.TLSSetting = configtls.ClientConfig{
		Insecure: true,
	}
	cfg.Compression = ods.compression
	return cfg
}

func (ods *otlpHTTPDataSender) GenConfigYAMLStr() string {
	// Note that this generates a receiver config for agent.
	return fmt.Sprintf(`
  otlp:
    protocols:
      http:
        endpoint: "%s"`, ods.GetEndpoint())
}

func (ods *otlpHTTPDataSender) ProtocolName() string {
	return "otlp"
}

// otlpHTTPTraceDataSender implements TraceDataSender for OTLP/HTTP trace exporter.
type otlpHTTPTraceDataSender struct {
	otlpHTTPDataSender
	consumer.Traces
}

// NewOTLPHTTPTraceDataSender creates a new TraceDataSender for OTLP/HTTP traces exporter.
func NewOTLPHTTPTraceDataSender(host string, port int, compression configcompression.Type) TraceDataSender {
	return &otlpHTTPTraceDataSender{
		otlpHTTPDataSender: otlpHTTPDataSender{
			DataSenderBase: DataSenderBase{
				Port: port,
				Host: host,
			},
			compression: compression,
		},
	}
}

func (ote *otlpHTTPTraceDataSender) Start() error {
	factory := otlphttpexporter.NewFactory()
	cfg := ote.fillConfig(factory.CreateDefaultConfig().(*otlphttpexporter.Config))
	params := exportertest.NewNopSettings()
	params.Logger = zap.L()

	exp, err := factory.CreateTraces(context.Background(), params, cfg)
	if err != nil {
		return err
	}

	ote.Traces = exp
	return exp.Start(context.Background(), componenttest.NewNopHost())
}

// otlpHTTPMetricsDataSender implements MetricDataSender for OTLP/HTTP metrics exporter.
type otlpHTTPMetricsDataSender struct {
	otlpHTTPDataSender
	consumer.Metrics
}

// NewOTLPHTTPMetricDataSender creates a new OTLP/HTTP metrics exporter sender that will send
// to the specified port after Start is called.
func NewOTLPHTTPMetricDataSender(host string, port int) MetricDataSender {
	return &otlpHTTPMetricsDataSender{
		otlpHTTPDataSender: otlpHTTPDataSender{
			DataSenderBase: DataSenderBase{
				Port: port,
				Host: host,
			},
		},
	}
}

func (ome *otlpHTTPMetricsDataSender) Start() error {
	factory := otlphttpexporter.NewFactory()
	cfg := ome.fillConfig(factory.CreateDefaultConfig().(*otlphttpexporter.Config))
	params := exportertest.NewNopSettings()
	params.Logger = zap.L()

	exp, err := factory.CreateMetrics(context.Background(), params, cfg)
	if err != nil {
		return err
	}

	ome.Metrics = exp
	return exp.Start(context.Background(), componenttest.NewNopHost())
}

// otlpHTTPLogsDataSender implements LogsDataSender for OTLP/HTTP logs exporter.
type otlpHTTPLogsDataSender struct {
	otlpHTTPDataSender
	consumer.Logs
}

// NewOTLPHTTPLogsDataSender creates a new OTLP/HTTP logs exporter sender that will send
// to the specified port after Start is called.
func NewOTLPHTTPLogsDataSender(host string, port int) LogDataSender {
	return &otlpHTTPLogsDataSender{
		otlpHTTPDataSender: otlpHTTPDataSender{
			DataSenderBase: DataSenderBase{
				Port: port,
				Host: host,
			},
		},
	}
}

func (olds *otlpHTTPLogsDataSender) Start() error {
	factory := otlphttpexporter.NewFactory()
	cfg := olds.fillConfig(factory.CreateDefaultConfig().(*otlphttpexporter.Config))
	params := exportertest.NewNopSettings()
	params.Logger = zap.L()

	exp, err := factory.CreateLogs(context.Background(), params, cfg)
	if err != nil {
		return err
	}

	olds.Logs = exp
	return exp.Start(context.Background(), componenttest.NewNopHost())
}

type otlpDataSender struct {
	DataSenderBase
}

func (ods *otlpDataSender) fillConfig(cfg *otlpexporter.Config) *otlpexporter.Config {
	cfg.Endpoint = ods.GetEndpoint().String()
	// Disable retries, we should push data and if error just log it.
	cfg.RetryConfig.Enabled = false
	// Disable sending queue, we should push data from the caller goroutine.
	cfg.QueueConfig.Enabled = false
	cfg.TLSSetting = configtls.ClientConfig{
		Insecure: true,
	}
	return cfg
}

func (ods *otlpDataSender) GenConfigYAMLStr() string {
	// Note that this generates a receiver config for agent.
	return fmt.Sprintf(`
  otlp:
    protocols:
      grpc:
        endpoint: "%s"`, ods.GetEndpoint())
}

func (ods *otlpDataSender) ProtocolName() string {
	return "otlp"
}

// otlpTraceDataSender implements TraceDataSender for OTLP traces exporter.
type otlpTraceDataSender struct {
	otlpDataSender
	consumer.Traces
}

// NewOTLPTraceDataSender creates a new TraceDataSender for OTLP traces exporter.
func NewOTLPTraceDataSender(host string, port int) TraceDataSender {
	return &otlpTraceDataSender{
		otlpDataSender: otlpDataSender{
			DataSenderBase: DataSenderBase{
				Port: port,
				Host: host,
			},
		},
	}
}

func (ote *otlpTraceDataSender) Start() error {
	factory := otlpexporter.NewFactory()
	cfg := ote.fillConfig(factory.CreateDefaultConfig().(*otlpexporter.Config))
	params := exportertest.NewNopSettings()
	params.Logger = zap.L()

	exp, err := factory.CreateTraces(context.Background(), params, cfg)
	if err != nil {
		return err
	}

	ote.Traces = exp
	return exp.Start(context.Background(), componenttest.NewNopHost())
}

// otlpMetricsDataSender implements MetricDataSender for OTLP metrics exporter.
type otlpMetricsDataSender struct {
	otlpDataSender
	consumer.Metrics
}

// NewOTLPMetricDataSender creates a new OTLP metric exporter sender that will send
// to the specified port after Start is called.
func NewOTLPMetricDataSender(host string, port int) MetricDataSender {
	return &otlpMetricsDataSender{
		otlpDataSender: otlpDataSender{
			DataSenderBase: DataSenderBase{
				Port: port,
				Host: host,
			},
		},
	}
}

func (ome *otlpMetricsDataSender) Start() error {
	factory := otlpexporter.NewFactory()
	cfg := ome.fillConfig(factory.CreateDefaultConfig().(*otlpexporter.Config))
	params := exportertest.NewNopSettings()
	params.Logger = zap.L()

	exp, err := factory.CreateMetrics(context.Background(), params, cfg)
	if err != nil {
		return err
	}

	ome.Metrics = exp
	return exp.Start(context.Background(), componenttest.NewNopHost())
}

// otlpLogsDataSender implements LogsDataSender for OTLP logs exporter.
type otlpLogsDataSender struct {
	otlpDataSender
	consumer.Logs
}

// NewOTLPLogsDataSender creates a new OTLP logs exporter sender that will send
// to the specified port after Start is called.
func NewOTLPLogsDataSender(host string, port int) LogDataSender {
	return &otlpLogsDataSender{
		otlpDataSender: otlpDataSender{
			DataSenderBase: DataSenderBase{
				Port: port,
				Host: host,
			},
		},
	}
}

func (olds *otlpLogsDataSender) Start() error {
	factory := otlpexporter.NewFactory()
	cfg := olds.fillConfig(factory.CreateDefaultConfig().(*otlpexporter.Config))
	params := exportertest.NewNopSettings()
	params.Logger = zap.L()

	exp, err := factory.CreateLogs(context.Background(), params, cfg)
	if err != nil {
		return err
	}

	olds.Logs = exp
	return exp.Start(context.Background(), componenttest.NewNopHost())
}
