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

package testbed // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"

import (
	"context"
	"fmt"
	"net"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
	"go.uber.org/zap"
)

// DataSender defines the interface that allows sending data. This is an interface
// that must be implemented by all protocols that want to be used in LoadGenerator.
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

// DataSenderBase is an abstract base that simplifies many of the testbed senders.
type DataSenderBase struct {
	Port int
	Host string
}

// ExporterCreateSettings creates standard exporter settings for the testbed.
func ExporterCreateSettings() component.ExporterCreateSettings {
	params := componenttest.NewNopExporterCreateSettings()
	params.Logger = zap.L()
	return params
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
	options []func(*otlphttpexporter.Config)
}

func (ods *otlpHTTPDataSender) fillConfig(cfg *otlphttpexporter.Config) *otlphttpexporter.Config {
	cfg.Endpoint = fmt.Sprintf("http://%s", ods.GetEndpoint())
	// Disable retries, we should push data and if error just log it.
	cfg.RetrySettings.Enabled = false
	// Disable sending queue, we should push data from the caller goroutine.
	cfg.QueueSettings.Enabled = false
	cfg.TLSSetting = configtls.TLSClientSetting{
		Insecure: true,
	}
	for _, opt := range ods.options {
		opt(cfg)
	}
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
//
// This object configures a test OTLP Receiver with the specified options applied.
func NewOTLPHTTPTraceDataSender(host string, port int, options ...func(*otlphttpexporter.Config)) TraceDataSender {
	return &otlpHTTPTraceDataSender{
		otlpHTTPDataSender: otlpHTTPDataSender{
			DataSenderBase: DataSenderBase{
				Port: port,
				Host: host,
			},
			options: options,
		},
	}
}

func (ote *otlpHTTPTraceDataSender) Start() error {
	factory := otlphttpexporter.NewFactory()
	cfg := ote.fillConfig(factory.CreateDefaultConfig().(*otlphttpexporter.Config))

	exp, err := factory.CreateTracesExporter(context.Background(), ExporterCreateSettings(), cfg)
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
//
// This object configures a OTLP Receiver with the specified options applied.
func NewOTLPHTTPMetricDataSender(host string, port int, options ...func(*otlphttpexporter.Config)) MetricDataSender {
	return &otlpHTTPMetricsDataSender{
		otlpHTTPDataSender: otlpHTTPDataSender{
			DataSenderBase: DataSenderBase{
				Port: port,
				Host: host,
			},
			options: options,
		},
	}
}

func (ome *otlpHTTPMetricsDataSender) Start() error {
	factory := otlphttpexporter.NewFactory()
	cfg := ome.fillConfig(factory.CreateDefaultConfig().(*otlphttpexporter.Config))

	exp, err := factory.CreateMetricsExporter(context.Background(), ExporterCreateSettings(), cfg)
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
func NewOTLPHTTPLogsDataSender(host string, port int, options ...func(*otlphttpexporter.Config)) LogDataSender {
	return &otlpHTTPLogsDataSender{
		otlpHTTPDataSender: otlpHTTPDataSender{
			DataSenderBase: DataSenderBase{
				Port: port,
				Host: host,
			},
			options: options,
		},
	}
}

func (olds *otlpHTTPLogsDataSender) Start() error {
	factory := otlphttpexporter.NewFactory()
	cfg := olds.fillConfig(factory.CreateDefaultConfig().(*otlphttpexporter.Config))

	exp, err := factory.CreateLogsExporter(context.Background(), ExporterCreateSettings(), cfg)
	if err != nil {
		return err
	}

	olds.Logs = exp
	return exp.Start(context.Background(), componenttest.NewNopHost())
}

type otlpDataSender struct {
	DataSenderBase
	options []func(*otlpexporter.Config)
}

func (ods *otlpDataSender) fillConfig(cfg *otlpexporter.Config) *otlpexporter.Config {
	cfg.Endpoint = ods.GetEndpoint().String()
	// Disable retries, we should push data and if error just log it.
	cfg.RetrySettings.Enabled = false
	// Disable sending queue, we should push data from the caller goroutine.
	cfg.QueueSettings.Enabled = false
	cfg.TLSSetting = configtls.TLSClientSetting{
		Insecure: true,
	}
	for _, opt := range ods.options {
		opt(cfg)
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
func NewOTLPTraceDataSender(host string, port int, options ...func(*otlpexporter.Config)) TraceDataSender {
	return &otlpTraceDataSender{
		otlpDataSender: otlpDataSender{
			DataSenderBase: DataSenderBase{
				Port: port,
				Host: host,
			},
			options: options,
		},
	}
}

func (ote *otlpTraceDataSender) Start() error {
	factory := otlpexporter.NewFactory()
	cfg := ote.fillConfig(factory.CreateDefaultConfig().(*otlpexporter.Config))

	exp, err := factory.CreateTracesExporter(context.Background(), ExporterCreateSettings(), cfg)
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
func NewOTLPMetricDataSender(host string, port int, options ...func(*otlpexporter.Config)) MetricDataSender {
	return &otlpMetricsDataSender{
		otlpDataSender: otlpDataSender{
			DataSenderBase: DataSenderBase{
				Port: port,
				Host: host,
			},
			options: options,
		},
	}
}

func (ome *otlpMetricsDataSender) Start() error {
	factory := otlpexporter.NewFactory()
	cfg := ome.fillConfig(factory.CreateDefaultConfig().(*otlpexporter.Config))

	exp, err := factory.CreateMetricsExporter(context.Background(), ExporterCreateSettings(), cfg)
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
func NewOTLPLogsDataSender(host string, port int, options ...func(*otlpexporter.Config)) LogDataSender {
	return &otlpLogsDataSender{
		otlpDataSender: otlpDataSender{
			DataSenderBase: DataSenderBase{
				Port: port,
				Host: host,
			},
			options: options,
		},
	}
}

func (olds *otlpLogsDataSender) Start() error {
	factory := otlpexporter.NewFactory()
	cfg := olds.fillConfig(factory.CreateDefaultConfig().(*otlpexporter.Config))

	exp, err := factory.CreateLogsExporter(context.Background(), ExporterCreateSettings(), cfg)
	if err != nil {
		return err
	}

	olds.Logs = exp
	return exp.Start(context.Background(), componenttest.NewNopHost())
}
