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

type processorSocketHandler struct {
	logger           *log.Logger
	pipeline         *pipeline
	processorFactory component.ProcessorFactory
}

func (h processorSocketHandler) handle(ws *websocket.Conn) {
	msg, err := readStartComponentMessage(ws)
	if err != nil {
		sendErr(ws, h.logger, "error reading start component message", err)
		return
	}
	m := map[string]interface{}{}
	err = yaml.Unmarshal([]byte(msg.ComponentYAML), &m)
	if err != nil {
		sendErr(ws, h.logger, "failed to unmarshal processor yaml", err)
		return
	}

	processorConfig := h.processorFactory.CreateDefaultConfig()
	conf := confmap.NewFromStringMap(m)
	err = unmarshalProcessorConfig(processorConfig, conf)
	if err != nil {
		sendErr(ws, h.logger, "failed to unmarshal processor config", err)
		return
	}

	switch msg.PipelineType {
	case "metrics":
		h.attachMetricsProcessor(ws, processorConfig)
	case "logs":
		h.attachLogsProcessor(ws, processorConfig)
	case "traces":
		h.attachTracesProcessor(ws, processorConfig)
	}

}

func (h processorSocketHandler) attachMetricsProcessor(
	ws *websocket.Conn,
	processorConfig config.Processor,
) {
	stop := make(chan struct{})
	repeater := &metricsRepeater{
		logger:    h.logger,
		ws:        ws,
		marshaler: pmetric.NewJSONMarshaler(),
		stop:      stop,
	}
	proc, err := h.processorFactory.CreateMetricsProcessor(
		context.Background(),
		componenttest.NewNopProcessorCreateSettings(),
		processorConfig,
		repeater,
	)
	if err != nil {
		sendErr(ws, h.logger, "failed to create metrics processor", err)
		return
	}
	wrapper := metricsProcessorWrapper{
		MetricsProcessor: proc,
		repeater:         repeater,
	}
	h.pipeline.connectMetricsProcessorWrapper(wrapper)
	<-stop
	h.pipeline.disconnectMetricsProcessorWrapper()
}

func (h processorSocketHandler) attachLogsProcessor(
	ws *websocket.Conn,
	processorConfig config.Processor,
) {
	stop := make(chan struct{})
	repeater := &logsRepeater{
		ws:        ws,
		marshaler: plog.NewJSONMarshaler(),
		stop:      stop,
	}
	proc, err := h.processorFactory.CreateLogsProcessor(
		context.Background(),
		componenttest.NewNopProcessorCreateSettings(),
		processorConfig,
		repeater,
	)
	if err != nil {
		sendErr(ws, h.logger, "failed to create logs processor", err)
		return
	}
	wrapper := logsProcessorWrapper{
		LogsProcessor: proc,
		repeater:      repeater,
	}
	h.pipeline.connectLogsProcessorWrapper(wrapper)
	<-stop
	h.pipeline.disconnectLogsProcessorWrapper()
}

func (h processorSocketHandler) attachTracesProcessor(
	ws *websocket.Conn,
	processorConfig config.Processor,
) {
	stop := make(chan struct{})
	repeater := &tracesRepeater{
		ws:        ws,
		marshaler: ptrace.NewJSONMarshaler(),
		stop:      stop,
	}
	proc, err := h.processorFactory.CreateTracesProcessor(
		context.Background(),
		componenttest.NewNopProcessorCreateSettings(),
		processorConfig,
		repeater,
	)
	if err != nil {
		sendErr(ws, h.logger, "failed to create traces processor", err)
		return
	}
	wrapper := tracesProcessorWrapper{
		TracesProcessor: proc,
		repeater:        repeater,
	}
	h.pipeline.connectTracesProcessorWrapper(wrapper)
	<-stop
	h.pipeline.disconnectTracesProcessorWrapper()
}

func unmarshalProcessorConfig(processorConfig config.Processor, conf *confmap.Conf) error {
	if unmarshallable, ok := processorConfig.(config.Unmarshallable); ok {
		return unmarshallable.Unmarshal(conf)
	}
	return conf.UnmarshalExact(processorConfig)
}
