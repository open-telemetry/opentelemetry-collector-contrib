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
	"golang.org/x/net/websocket"
)

type exporterSocketHandler struct {
	logger          *log.Logger
	pipeline        *pipeline
	exporterFactory component.ExporterFactory
}

func (h exporterSocketHandler) handle(ws *websocket.Conn) {
	err := h.doHandle(ws)
	if err != nil {
		sendErr(ws, h.logger, "exporterSocketHandler", err)
	}
}

func (h exporterSocketHandler) doHandle(ws *websocket.Conn) error {
	pipelineType, conf, err := readSocket(ws)
	if err != nil {
		return err
	}
	exporterConfig := h.exporterFactory.CreateDefaultConfig()
	err = unmarshalExporterConfig(exporterConfig, conf)
	if err != nil {
		return err
	}
	switch pipelineType {
	case "metrics":
		h.connectMetricsExporter(ws, exporterConfig)
	case "logs":
		h.connectLogsExporter(ws, exporterConfig)
	case "traces":
		h.connectTracesExporter(ws, exporterConfig)
	}
	return nil
}

func (h exporterSocketHandler) connectMetricsExporter(
	ws *websocket.Conn,
	cfg config.Exporter,
) {
	wrapper, err := newMetricsExporterWrapper(h.logger, ws, cfg, h.exporterFactory)
	if err != nil {
		sendErr(ws, h.logger, "failed to create metrics processor wrapper", err)
		return
	}
	h.pipeline.connectMetricsExporterWrapper(&wrapper)
	err = wrapper.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		sendErr(ws, h.logger, "failed to start metrics exporter", err)
		return
	}
	wrapper.waitForStopMessage()
	err = wrapper.Shutdown(context.Background())
	if err != nil {
		sendErr(ws, h.logger, "failed to unmarshal receiver", err)
		return
	}
	h.pipeline.disconnectMetricsExporterWrapper()
}

func (h exporterSocketHandler) connectLogsExporter(
	ws *websocket.Conn,
	cfg config.Exporter,
) {
	wrapper, err := newLogsExporterWrapper(h.logger, ws, cfg, h.exporterFactory)
	if err != nil {
		sendErr(ws, h.logger, "failed to create logs processor wrapper", err)
		return
	}
	h.pipeline.connectLogsExporterWrapper(&wrapper)
	err = wrapper.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		sendErr(ws, h.logger, "failed to start logs exporter", err)
		return
	}
	wrapper.waitForStopMessage()
	err = wrapper.Shutdown(context.Background())
	if err != nil {
		sendErr(ws, h.logger, "failed to unmarshal receiver", err)
		return
	}
	h.pipeline.disconnectLogsExporterWrapper()
}

func (h exporterSocketHandler) connectTracesExporter(
	ws *websocket.Conn,
	cfg config.Exporter,
) {
	wrapper, err := newTracesExporterWrapper(h.logger, ws, cfg, h.exporterFactory)
	if err != nil {
		sendErr(ws, h.logger, "failed to create traces processor wrapper", err)
		return
	}
	h.pipeline.connectTracesExporterWrapper(&wrapper)
	err = wrapper.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		sendErr(ws, h.logger, "failed to start traces exporter", err)
		return
	}
	wrapper.waitForStopMessage()
	err = wrapper.Shutdown(context.Background())
	if err != nil {
		sendErr(ws, h.logger, "failed to unmarshal receiver", err)
		return
	}
	h.pipeline.disconnectTracesExporterWrapper()
}

func unmarshalExporterConfig(exporterConfig config.Exporter, conf *confmap.Conf) error {
	if unmarshallable, ok := exporterConfig.(confmap.Unmarshaler); ok {
		return unmarshallable.Unmarshal(conf)
	}
	return conf.UnmarshalExact(exporterConfig)
}
