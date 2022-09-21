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

func (h exporterSocketHandler) handle(ws *websocket.Conn) {
	msg, err := readStartComponentMessage(ws)
	if err != nil {
		sendErr(ws, h.logger, "error reading start component message", err)
		return
	}
	m := map[string]interface{}{}
	err = yaml.Unmarshal([]byte(msg.ComponentYAML), &m)
	if err != nil {
		sendErr(ws, h.logger, "failed to unmarshal yaml", err)
		return
	}

	exporterConfig := h.exporterFactory.CreateDefaultConfig()
	conf := confmap.NewFromStringMap(m)
	err = unmarshalExporterConfig(exporterConfig, conf)
	if err != nil {
		sendErr(ws, h.logger, "failed to unmarshal exporter config", err)
		return
	}

	switch msg.PipelineType {
	case "metrics":
		h.connectMetricsExporter(ws, exporterConfig)
	case "logs":
		h.connectLogsExporter(ws, exporterConfig)
	case "traces":
		h.connectTracesExporter(ws, exporterConfig)
	}

}

func (h exporterSocketHandler) connectMetricsExporter(
	ws *websocket.Conn,
	exporterConfig config.Exporter,
) {
	stop := make(chan struct{})
	repeater := &metricsRepeater{
		logger:    h.logger,
		ws:        ws,
		marshaler: pmetric.NewJSONMarshaler(),
		stop:      stop,
	}
	exporter, err := h.exporterFactory.CreateMetricsExporter(
		context.Background(),
		componenttest.NewNopExporterCreateSettings(),
		exporterConfig,
	)
	if err != nil {
		sendErr(ws, h.logger, "failed to create metrics expoerter", err)
		return
	}
	wrapper := metricsExporterWrapper{
		MetricsExporter: exporter,
		repeater:        repeater,
	}
	h.pipeline.connectMetricsExporterWrapper(&wrapper)
	err = wrapper.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		sendErr(ws, h.logger, "failed to start metrics exporter", err)
		return
	}
	<-stop
	err = wrapper.Shutdown(context.Background())
	if err != nil {
		sendErr(ws, h.logger, "failed to unmarshal receiver", err)
		return
	}
	h.pipeline.disconnectMetricsExporterWrapper()
}

func (h exporterSocketHandler) connectLogsExporter(
	ws *websocket.Conn,
	exporterConfig config.Exporter,
) {
	stop := make(chan struct{})
	repeater := &logsRepeater{
		ws:        ws,
		marshaler: plog.NewJSONMarshaler(),
		stop:      stop,
	}
	proc, err := h.exporterFactory.CreateLogsExporter(
		context.Background(),
		componenttest.NewNopExporterCreateSettings(),
		exporterConfig,
	)
	if err != nil {
		sendErr(ws, h.logger, "failed to create logs exporter", err)
		return
	}
	wrapper := logsExporterWrapper{
		LogsExporter: proc,
		repeater:     repeater,
	}
	err = wrapper.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		sendErr(ws, h.logger, "failed to start logs exporter", err)
		return
	}
	h.pipeline.connectLogsExporterWrapper(&wrapper)
	<-stop
	err = wrapper.Shutdown(context.Background())
	if err != nil {
		sendErr(ws, h.logger, "failed to shut down logs exporter", err)
		return
	}
	h.pipeline.disconnectLogsExporterWrapper()
}

func (h exporterSocketHandler) connectTracesExporter(
	ws *websocket.Conn,
	exporterConfig config.Exporter,
) {
	stop := make(chan struct{})
	repeater := &tracesRepeater{
		ws:        ws,
		marshaler: ptrace.NewJSONMarshaler(),
		stop:      stop,
	}
	exporter, err := h.exporterFactory.CreateTracesExporter(
		context.Background(),
		componenttest.NewNopExporterCreateSettings(),
		exporterConfig,
	)
	if err != nil {
		sendErr(ws, h.logger, "failed to create traces exporter", err)
		return
	}
	wrapper := tracesExporterWrapper{
		TracesExporter: exporter,
		repeater:       repeater,
	}
	err = wrapper.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		sendErr(ws, h.logger, "failed to start traces exporter", err)
		return
	}
	h.pipeline.connectTracesExporterWrapper(&wrapper)
	<-stop
	err = wrapper.Shutdown(context.Background())
	if err != nil {
		sendErr(ws, h.logger, "failed to shut down traces exporter", err)
		return
	}
	h.pipeline.disconnectTracesExporterWrapper()
}

func unmarshalExporterConfig(exporterConfig config.Exporter, conf *confmap.Conf) error {
	if unmarshallable, ok := exporterConfig.(config.Unmarshallable); ok {
		return unmarshallable.Unmarshal(conf)
	}
	return conf.UnmarshalExact(exporterConfig)
}
