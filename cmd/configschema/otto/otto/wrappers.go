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
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
)

type metricsReceiverWrapper struct {
	component.MetricsReceiver
	repeater *metricsRepeater
}

func (w metricsReceiverWrapper) setNextMetricsConsumer(next consumer.Metrics) {
	w.repeater.setNext(next)
}

type logsReceiverWrapper struct {
	component.LogsReceiver
	repeater *logsRepeater
}

func (w logsReceiverWrapper) setNextLogsConsumer(next consumer.Logs) {
	w.repeater.setNext(next)
}

type tracesReceiverWrapper struct {
	component.TracesReceiver
	repeater *tracesRepeater
}

func (w tracesReceiverWrapper) setNextTracesConsumer(next consumer.Traces) {
	w.repeater.setNext(next)
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
