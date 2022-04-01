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

type processorSocketHandler struct {
	pipeline         *pipeline
	processorFactory component.ProcessorFactory
}

func (h processorSocketHandler) handle(webSocket *websocket.Conn) {
	msg := readStartComponentMessage(webSocket)
	m := map[string]interface{}{}
	err := yaml.Unmarshal([]byte(msg.ComponentYAML), &m)
	if err != nil {
		panic(err)
	}

	processorConfig := h.processorFactory.CreateDefaultConfig()
	conf := confmap.NewFromStringMap(m)
	err = unmarshalProcessor(processorConfig, conf)
	if err != nil {
		panic(err)
	}

	switch msg.PipelineType {
	case "metrics":
		h.attachMetricsProcessor(webSocket, processorConfig)
	case "logs":
		h.attachLogsProcessor(webSocket, processorConfig)
	case "traces":
		h.attachTracesProcessor(webSocket, processorConfig)
	}

}

func (h processorSocketHandler) attachMetricsProcessor(
	webSocket *websocket.Conn,
	processorConfig config.Processor,
) {
	stop := make(chan struct{})
	consumer := &repeatingMetricsConsumer{
		webSocket: webSocket,
		marshaler: pmetric.NewJSONMarshaler(),
		stop:      stop,
	}
	proc, err := h.processorFactory.CreateMetricsProcessor(
		context.Background(),
		componenttest.NewNopProcessorCreateSettings(),
		processorConfig,
		consumer,
	)
	if err != nil {
		panic(err)
	}
	wrapper := metricsProcessorWrapper{
		MetricsProcessor: proc,
		consumer:         consumer,
	}
	h.pipeline.connectMetricsProcessorWrapper(wrapper)
	<-stop
	h.pipeline.disconnectMetricsProcessorWrapper()
}

func (h processorSocketHandler) attachLogsProcessor(
	webSocket *websocket.Conn,
	processorConfig config.Processor,
) {
	stop := make(chan struct{})
	consumer := &repeatingLogsConsumer{
		webSocket: webSocket,
		marshaler: plog.NewJSONMarshaler(),
		stop:      stop,
	}
	proc, err := h.processorFactory.CreateLogsProcessor(
		context.Background(),
		componenttest.NewNopProcessorCreateSettings(),
		processorConfig,
		consumer,
	)
	if err != nil {
		panic(err)
	}
	wrapper := logsProcessorWrapper{
		LogsProcessor: proc,
		consumer:      consumer,
	}
	h.pipeline.connectLogsProcessorWrapper(wrapper)
	<-stop
	h.pipeline.disconnectLogsProcessorWrapper()
}

func (h processorSocketHandler) attachTracesProcessor(
	webSocket *websocket.Conn,
	processorConfig config.Processor,
) {
	stop := make(chan struct{})
	consumer := &repeatingTracesConsumer{
		webSocket: webSocket,
		marshaler: ptrace.NewJSONMarshaler(),
		stop:      stop,
	}
	proc, err := h.processorFactory.CreateTracesProcessor(
		context.Background(),
		componenttest.NewNopProcessorCreateSettings(),
		processorConfig,
		consumer,
	)
	if err != nil {
		panic(err)
	}
	wrapper := tracesProcessorWrapper{
		TracesProcessor: proc,
		consumer:        consumer,
	}
	h.pipeline.connectTracesProcessorWrapper(wrapper)
	<-stop
	h.pipeline.disconnectTracesProcessorWrapper()
}

func unmarshalProcessor(processorConfig config.Processor, conf *confmap.Conf) error {
	if unmarshallable, ok := processorConfig.(config.Unmarshallable); ok {
		return unmarshallable.Unmarshal(conf)
	}
	return conf.UnmarshalExact(processorConfig)
}
