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

package otto

import (
	"context"
	"log"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"golang.org/x/net/websocket"
	"gopkg.in/yaml.v2"
)

type exporterSocketHandler struct {
	logger          *log.Logger
	pipeline        *pipeline
	exporterFactory component.ExporterFactory
}

func (h exporterSocketHandler) handle(webSocket *websocket.Conn) {
	msg := readStartComponentMessage(webSocket)
	m := map[string]interface{}{}
	err := yaml.Unmarshal([]byte(msg.ComponentYAML), &m)
	if err != nil {
		panic(err)
	}

	exporterConfig := h.exporterFactory.CreateDefaultConfig()
	conf := confmap.NewFromStringMap(m)
	err = unmarshalExporter(exporterConfig, conf)
	if err != nil {
		panic(err)
	}

	switch msg.PipelineType {
	case "metrics":
		h.connectMetricsExporter(webSocket, exporterConfig)
	case "logs":
		h.connectLogsExporter(webSocket, exporterConfig)
	case "traces":
		h.connectTracesExporter(webSocket, exporterConfig)
	}

}

func (h exporterSocketHandler) connectMetricsExporter(
	webSocket *websocket.Conn,
	exporterConfig config.Exporter,
) {
	stop := make(chan struct{})
	consumer := &repeatingMetricsConsumer{
		webSocket: webSocket,
		marshaler: pmetric.NewJSONMarshaler(),
		stop:      stop,
	}
	exporter, err := h.exporterFactory.CreateMetricsExporter(
		context.Background(),
		componenttest.NewNopExporterCreateSettings(),
		exporterConfig,
	)
	if err != nil {
		panic(err)
	}
	wrapper := metricsExporterWrapper{
		MetricsExporter: exporter,
		consumer:        consumer,
	}
	h.pipeline.connectMetricsExporterWrapper(&wrapper)
	err = wrapper.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		panic(err)
	}
	<-stop
	err = wrapper.Shutdown(context.Background())
	if err != nil {
		panic(err)
	}
	h.pipeline.disconnectMetricsExporterWrapper()
}

func (h exporterSocketHandler) connectLogsExporter(
	webSocket *websocket.Conn,
	exporterConfig config.Exporter,
) {
	stop := make(chan struct{})
	consumer := &repeatingLogsConsumer{
		webSocket: webSocket,
		marshaler: plog.NewJSONMarshaler(),
		stop:      stop,
	}
	proc, err := h.exporterFactory.CreateLogsExporter(
		context.Background(),
		componenttest.NewNopExporterCreateSettings(),
		exporterConfig,
	)
	if err != nil {
		panic(err)
	}
	wrapper := logsExporterWrapper{
		LogsExporter: proc,
		consumer:     consumer,
	}
	err = wrapper.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		panic(err)
	}
	h.pipeline.connectLogsExporterWrapper(&wrapper)
	<-stop
	err = wrapper.Shutdown(context.Background())
	if err != nil {
		panic(err)
	}
	h.pipeline.disconnectLogsExporterWrapper()
}

func (h exporterSocketHandler) connectTracesExporter(
	webSocket *websocket.Conn,
	exporterConfig config.Exporter,
) {
	stop := make(chan struct{})
	consumer := &repeatingTracesConsumer{
		webSocket: webSocket,
		marshaler: ptrace.NewJSONMarshaler(),
		stop:      stop,
	}
	exporter, err := h.exporterFactory.CreateTracesExporter(
		context.Background(),
		componenttest.NewNopExporterCreateSettings(),
		exporterConfig,
	)
	if err != nil {
		panic(err)
	}
	wrapper := tracesExporterWrapper{
		TracesExporter: exporter,
		consumer:       consumer,
	}
	err = wrapper.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		panic(err)
	}
	h.pipeline.connectTracesExporterWrapper(&wrapper)
	<-stop
	err = wrapper.Shutdown(context.Background())
	if err != nil {
		panic(err)
	}
	h.pipeline.disconnectTracesExporterWrapper()
}

func unmarshalExporter(exporterConfig config.Exporter, conf *confmap.Conf) error {
	if unmarshallable, ok := exporterConfig.(config.Unmarshallable); ok {
		return unmarshallable.Unmarshal(conf)
	}
	return conf.UnmarshalExact(exporterConfig)
}
