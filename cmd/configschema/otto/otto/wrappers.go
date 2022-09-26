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
	"go.opentelemetry.io/collector/consumer"
	"golang.org/x/net/websocket"
)

type metricsReceiverWrapper struct {
	component.MetricsReceiver
	repeater *metricsRepeater
}

func newMetricsReceiverWrapper(logger *log.Logger, ws *websocket.Conn, cfg config.Receiver, factory component.ReceiverFactory) (metricsReceiverWrapper, error) {
	repeater := newMetricsRepeater(logger, ws)
	receiver, err := factory.CreateMetricsReceiver(
		context.Background(),
		componenttest.NewNopReceiverCreateSettings(),
		cfg,
		repeater,
	)
	return metricsReceiverWrapper{
		MetricsReceiver: receiver,
		repeater:        repeater,
	}, err
}

func (w metricsReceiverWrapper) setNextMetricsConsumer(next consumer.Metrics) {
	w.repeater.setNext(next)
}

func (w metricsReceiverWrapper) waitForStopMessage() {
	w.repeater.waitForStopMessage()
}

type logsReceiverWrapper struct {
	component.LogsReceiver
	repeater *logsRepeater
}

func newLogsReceiverWrapper(logger *log.Logger, ws *websocket.Conn, cfg config.Receiver, factory component.ReceiverFactory) (logsReceiverWrapper, error) {
	repeater := newLogsRepeater(logger, ws)
	receiver, err := factory.CreateLogsReceiver(
		context.Background(),
		componenttest.NewNopReceiverCreateSettings(),
		cfg,
		repeater,
	)
	return logsReceiverWrapper{
		LogsReceiver: receiver,
		repeater:     repeater,
	}, err
}

func (w logsReceiverWrapper) setNextLogsConsumer(next consumer.Logs) {
	w.repeater.setNext(next)
}

func (w logsReceiverWrapper) waitForStopMessage() {
	w.repeater.waitForStopMessage()
}

type tracesReceiverWrapper struct {
	component.TracesReceiver
	repeater *tracesRepeater
}

func newTracesReceiverWrapper(logger *log.Logger, ws *websocket.Conn, cfg config.Receiver, factory component.ReceiverFactory) (tracesReceiverWrapper, error) {
	repeater := newTracesRepeater(logger, ws)
	receiver, err := factory.CreateTracesReceiver(
		context.Background(),
		componenttest.NewNopReceiverCreateSettings(),
		cfg,
		repeater,
	)
	return tracesReceiverWrapper{
		TracesReceiver: receiver,
		repeater:       repeater,
	}, err
}

func (w tracesReceiverWrapper) setNextTracesConsumer(next consumer.Traces) {
	w.repeater.setNext(next)
}

func (w tracesReceiverWrapper) waitForStopMessage() {
	w.repeater.waitForStopMessage()
}

type metricsProcessorWrapper struct {
	component.MetricsProcessor
	repeater *metricsRepeater
}

func (w metricsProcessorWrapper) setNextMetricsConsumer(next consumer.Metrics) {
	w.repeater.setNext(next)
}

type logsProcessorWrapper struct {
	component.LogsProcessor
	repeater *logsRepeater
}

func (w logsProcessorWrapper) setNextLogsConsumer(next consumer.Logs) {
	w.repeater.setNext(next)
}

type tracesProcessorWrapper struct {
	component.TracesProcessor
	repeater *tracesRepeater
}

func (w tracesProcessorWrapper) setNextTracesConsumer(next consumer.Traces) {
	w.repeater.setNext(next)
}

type metricsExporterWrapper struct {
	component.MetricsExporter
	repeater *metricsRepeater
}

type logsExporterWrapper struct {
	component.LogsExporter
	repeater *logsRepeater
}

type tracesExporterWrapper struct {
	component.TracesExporter
	repeater *tracesRepeater
}
