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
)

type receiverSocketHandler struct {
	logger          *log.Logger
	pipeline        *pipeline
	receiverFactory component.ReceiverFactory
}

func (h receiverSocketHandler) handle(ws *websocket.Conn) {
	err := h.doHandle(ws)
	if err != nil {
		sendErr(ws, h.logger, "receiverSocketHandler", err)
	}
}

func (h receiverSocketHandler) doHandle(ws *websocket.Conn) error {
	pipelineType, conf, err := readSocket(ws)
	if err != nil {
		return err
	}

	receiverConfig := h.receiverFactory.CreateDefaultConfig()
	err = unmarshalReceiverConfig(receiverConfig, conf)
	if err != nil {
		return err
	}

	switch pipelineType {
	case "metrics":
		h.startMetricsReceiver(ws, receiverConfig)
	case "logs":
		h.startLogsReceiver(ws, receiverConfig)
	case "traces":
		h.startTracesReceiver(ws, receiverConfig)
	}
	return nil
}

func (h receiverSocketHandler) startMetricsReceiver(
	ws *websocket.Conn,
	cfg config.Receiver,
) {
	stop := make(chan struct{})
	repeater := &metricsRepeater{
		logger:    h.logger,
		ws:        ws,
		marshaler: pmetric.NewJSONMarshaler(),
		stop:      stop,
	}
	receiver, err := h.receiverFactory.CreateMetricsReceiver(
		context.Background(),
		componenttest.NewNopReceiverCreateSettings(),
		cfg,
		repeater,
	)
	if err != nil {
		sendErr(ws, h.logger, "failed to create metrics receiver", err)
		return
	}
	wrapper := metricsReceiverWrapper{
		MetricsReceiver: receiver,
		repeater:        repeater,
	}
	h.pipeline.connectMetricsReceiverWrapper(wrapper)
	err = wrapper.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		sendErr(ws, h.logger, "failed to start metrics receiver", err)
		return
	}
	<-stop
	err = wrapper.Shutdown(context.Background())
	if err != nil {
		sendErr(ws, h.logger, "failed to shut down metrics receiver", err)
		return
	}
	h.pipeline.disconnectMetricsReceiverWrapper()
}

func (h receiverSocketHandler) startLogsReceiver(
	ws *websocket.Conn,
	cfg config.Receiver,
) {
	stop := make(chan struct{})
	repeater := &logsRepeater{
		ws:        ws,
		marshaler: plog.NewJSONMarshaler(),
		stop:      stop,
	}
	receiver, err := h.receiverFactory.CreateLogsReceiver(
		context.Background(),
		componenttest.NewNopReceiverCreateSettings(),
		cfg,
		repeater,
	)
	if err != nil {
		sendErr(ws, h.logger, "failed to create logs receiver", err)
		return
	}
	wrapper := logsReceiverWrapper{
		LogsReceiver: receiver,
		repeater:     repeater,
	}
	h.pipeline.connectLogsReceiverWrapper(wrapper)
	err = wrapper.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		sendErr(ws, h.logger, "failed to start logs receiver", err)
		return
	}
	<-stop
	err = wrapper.Shutdown(context.Background())
	if err != nil {
		sendErr(ws, h.logger, "failed to shut down logs receiver", err)
		return
	}
	h.pipeline.disconnectLogsReceiverWrapper()
}

func (h receiverSocketHandler) startTracesReceiver(
	ws *websocket.Conn,
	cfg config.Receiver,
) {
	stop := make(chan struct{})
	repeater := &tracesRepeater{
		ws:        ws,
		marshaler: ptrace.NewJSONMarshaler(),
		stop:      stop,
	}
	receiver, err := h.receiverFactory.CreateTracesReceiver(
		context.Background(),
		componenttest.NewNopReceiverCreateSettings(),
		cfg,
		repeater,
	)
	if err != nil {
		sendErr(ws, h.logger, "failed to create traces receiver", err)
		return
	}
	wrapper := tracesReceiverWrapper{
		TracesReceiver: receiver,
		repeater:       repeater,
	}
	h.pipeline.connectTracesReceiverWrapper(wrapper)
	err = wrapper.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		sendErr(ws, h.logger, "failed to start traces receiver", err)
		return
	}
	<-stop
	err = wrapper.Shutdown(context.Background())
	if err != nil {
		sendErr(ws, h.logger, "failed to shut down traces receiver", err)
		return
	}
	h.pipeline.disconnectTracesReceiverWrapper()
}

func unmarshalReceiverConfig(receiverConfig config.Receiver, conf *confmap.Conf) error {
	if unmarshallable, ok := receiverConfig.(config.Unmarshallable); ok {
		return unmarshallable.Unmarshal(conf)
	}
	return conf.UnmarshalExact(receiverConfig)
}
