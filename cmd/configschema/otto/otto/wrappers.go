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

func newMetricsProcessorWrapper(logger *log.Logger, ws *websocket.Conn, cfg config.Processor, factory component.ProcessorFactory) (metricsProcessorWrapper, error) {
	repeater := newMetricsRepeater(logger, ws)
	processor, err := factory.CreateMetricsProcessor(
		context.Background(),
		componenttest.NewNopProcessorCreateSettings(),
		cfg,
		repeater,
	)
	return metricsProcessorWrapper{
		MetricsProcessor: processor,
		repeater:         repeater,
	}, err
}

func (w metricsProcessorWrapper) setNextMetricsConsumer(next consumer.Metrics) {
	w.repeater.setNext(next)
}

func (w metricsProcessorWrapper) waitForStopMessage() {
	w.repeater.waitForStopMessage()
}

type logsProcessorWrapper struct {
	component.LogsProcessor
	repeater *logsRepeater
}

func newLogsProcessorWrapper(logger *log.Logger, ws *websocket.Conn, cfg config.Processor, factory component.ProcessorFactory) (logsProcessorWrapper, error) {
	repeater := newLogsRepeater(logger, ws)
	processor, err := factory.CreateLogsProcessor(
		context.Background(),
		componenttest.NewNopProcessorCreateSettings(),
		cfg,
		repeater,
	)
	return logsProcessorWrapper{
		LogsProcessor: processor,
		repeater:      repeater,
	}, err
}

func (w logsProcessorWrapper) setNextLogsConsumer(next consumer.Logs) {
	w.repeater.setNext(next)
}

func (w logsProcessorWrapper) waitForStopMessage() {
	w.repeater.waitForStopMessage()
}

type tracesProcessorWrapper struct {
	component.TracesProcessor
	repeater *tracesRepeater
}

func newTracesProcessorWrapper(logger *log.Logger, ws *websocket.Conn, cfg config.Processor, factory component.ProcessorFactory) (tracesProcessorWrapper, error) {
	repeater := newTracesRepeater(logger, ws)
	processor, err := factory.CreateTracesProcessor(
		context.Background(),
		componenttest.NewNopProcessorCreateSettings(),
		cfg,
		repeater,
	)
	return tracesProcessorWrapper{
		TracesProcessor: processor,
		repeater:        repeater,
	}, err
}

func (w tracesProcessorWrapper) setNextTracesConsumer(next consumer.Traces) {
	w.repeater.setNext(next)
}

func (w tracesProcessorWrapper) waitForStopMessage() {
	w.repeater.waitForStopMessage()
}

type metricsExporterWrapper struct {
	component.MetricsExporter
	repeater *metricsRepeater
}

func newMetricsExporterWrapper(logger *log.Logger, ws *websocket.Conn, cfg config.Exporter, factory component.ExporterFactory) (metricsExporterWrapper, error) {
	repeater := newMetricsRepeater(logger, ws)
	exporter, err := factory.CreateMetricsExporter(
		context.Background(),
		componenttest.NewNopExporterCreateSettings(),
		cfg,
	)
	return metricsExporterWrapper{
		MetricsExporter: exporter,
		repeater:        repeater,
	}, err
}

func (w metricsExporterWrapper) waitForStopMessage() {
	w.repeater.waitForStopMessage()
}

type logsExporterWrapper struct {
	component.LogsExporter
	repeater *logsRepeater
}

func newLogsExporterWrapper(logger *log.Logger, ws *websocket.Conn, cfg config.Exporter, factory component.ExporterFactory) (logsExporterWrapper, error) {
	exporter, err := factory.CreateLogsExporter(
		context.Background(),
		componenttest.NewNopExporterCreateSettings(),
		cfg,
	)
	return logsExporterWrapper{
		LogsExporter: exporter,
		repeater:     newLogsRepeater(logger, ws),
	}, err
}

func (w logsExporterWrapper) waitForStopMessage() {
	w.repeater.waitForStopMessage()
}

type tracesExporterWrapper struct {
	component.TracesExporter
	repeater *tracesRepeater
}

func newTracesExporterWrapper(logger *log.Logger, ws *websocket.Conn, cfg config.Exporter, factory component.ExporterFactory) (tracesExporterWrapper, error) {
	exporter, err := factory.CreateTracesExporter(
		context.Background(),
		componenttest.NewNopExporterCreateSettings(),
		cfg,
	)
	return tracesExporterWrapper{
		TracesExporter: exporter,
		repeater:       newTracesRepeater(logger, ws),
	}, err
}

func (w tracesExporterWrapper) waitForStopMessage() {
	w.repeater.waitForStopMessage()
}
