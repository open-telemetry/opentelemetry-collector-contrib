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

type receiverSocketHandler struct {
	pipeline        *pipeline
	receiverFactory component.ReceiverFactory
}

func (h receiverSocketHandler) handle(ws *websocket.Conn) {
	msg := readStartComponentMessage(ws)
	m := map[string]interface{}{}
	err := yaml.Unmarshal([]byte(msg.ComponentYAML), &m)
	if err != nil {
		panic(err)
	}

	receiverConfig := h.receiverFactory.CreateDefaultConfig()
	conf := confmap.NewFromStringMap(m)
	err = unmarshalReceiver(receiverConfig, conf)
	if err != nil {
		panic(err)
	}

	switch msg.PipelineType {
	case "metrics":
		h.startMetricsReceiver(ws, receiverConfig)
	case "logs":
		h.startLogsReceiver(ws, receiverConfig)
	case "traces":
		h.startTracesReceiver(ws, receiverConfig)
	}
}

func (h receiverSocketHandler) startMetricsReceiver(
	webSocket *websocket.Conn,
	cfg config.Receiver,
) {
	stop := make(chan struct{})
	repeater := &repeatingMetricsConsumer{
		webSocket: webSocket,
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
		panic(err)
	}
	wrapper := metricsReceiverWrapper{
		MetricsReceiver: receiver,
		consumer:        repeater,
	}
	h.pipeline.connectMetricsReceiverWrapper(wrapper)
	wrapper.start()
	<-stop
	wrapper.shutdown()
	h.pipeline.disconnectMetricsReceiverWrapper()
}

func (h receiverSocketHandler) startLogsReceiver(
	webSocket *websocket.Conn,
	cfg config.Receiver,
) {
	stop := make(chan struct{})
	consumer := &repeatingLogsConsumer{
		webSocket: webSocket,
		marshaler: plog.NewJSONMarshaler(),
		stop:      stop,
	}
	receiver, err := h.receiverFactory.CreateLogsReceiver(
		context.Background(),
		componenttest.NewNopReceiverCreateSettings(),
		cfg,
		consumer,
	)
	if err != nil {
		panic(err)
	}
	wrapper := logsReceiverWrapper{
		LogsReceiver: receiver,
		consumer:     consumer,
	}
	h.pipeline.connectLogsReceiverWrapper(wrapper)
	wrapper.start()
	<-stop
	wrapper.shutdown()
	h.pipeline.disconnectLogsReceiverWrapper()
}

func (h receiverSocketHandler) startTracesReceiver(
	webSocket *websocket.Conn,
	cfg config.Receiver,
) {
	stop := make(chan struct{})
	consumer := &repeatingTracesConsumer{
		webSocket: webSocket,
		marshaler: ptrace.NewJSONMarshaler(),
		stop:      stop,
	}
	receiver, err := h.receiverFactory.CreateTracesReceiver(
		context.Background(),
		componenttest.NewNopReceiverCreateSettings(),
		cfg,
		consumer,
	)
	if err != nil {
		panic(err)
	}
	wrapper := tracesReceiverWrapper{
		TracesReceiver: receiver,
		consumer:       consumer,
	}
	h.pipeline.connectTracesReceiverWrapper(wrapper)
	wrapper.start()
	<-stop
	wrapper.shutdown()
	h.pipeline.disconnectTracesReceiverWrapper()
}

func unmarshalReceiver(receiverConfig config.Receiver, conf *confmap.Conf) error {
	if unmarshallable, ok := receiverConfig.(config.Unmarshallable); ok {
		return unmarshallable.Unmarshal(conf)
	}
	return conf.UnmarshalExact(receiverConfig)
}
