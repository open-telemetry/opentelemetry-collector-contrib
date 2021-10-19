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

package datasenders

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opencensusexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

type ocDataSender struct {
	testbed.DataSenderBase
}

func (ods *ocDataSender) fillConfig(cfg *opencensusexporter.Config) *opencensusexporter.Config {
	cfg.Endpoint = ods.GetEndpoint().String()
	cfg.TLSSetting = configtls.TLSClientSetting{
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
	params := componenttest.NewNopExporterCreateSettings()
	params.Logger = zap.L()

	exp, err := factory.CreateTracesExporter(context.Background(), params, cfg)
	if err != nil {
		return err
	}

	ote.Traces = exp
	return exp.Start(context.Background(), ote)
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
	params := componenttest.NewNopExporterCreateSettings()
	params.Logger = zap.L()

	exp, err := factory.CreateMetricsExporter(context.Background(), params, cfg)
	if err != nil {
		return err
	}

	ome.Metrics = exp
	return exp.Start(context.Background(), ome)
}
