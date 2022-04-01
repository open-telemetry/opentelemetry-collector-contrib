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
	"go.opentelemetry.io/collector/consumer"
)

type metricsReceiverWrapper struct {
	component.MetricsReceiver
	consumer *repeatingMetricsConsumer
}

func (w metricsReceiverWrapper) start() {
	err := w.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		panic(err)
	}
}

func (w metricsReceiverWrapper) setNextMetricsConsumer(next consumer.Metrics) {
	w.consumer.next = next
}

func (w metricsReceiverWrapper) shutdown() {
	err := w.Shutdown(context.Background())
	if err != nil {
		panic(err)
	}
}

type logsReceiverWrapper struct {
	component.LogsReceiver
	consumer *repeatingLogsConsumer
}

func (w logsReceiverWrapper) start() {
	err := w.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		panic(err)
	}
}

func (w logsReceiverWrapper) setNextLogsConsumer(next consumer.Logs) {
	w.consumer.next = next
}

func (w logsReceiverWrapper) shutdown() {
	err := w.Shutdown(context.Background())
	if err != nil {
		panic(err)
	}
}

type tracesReceiverWrapper struct {
	component.TracesReceiver
	consumer *repeatingTracesConsumer
}

func (w tracesReceiverWrapper) start() {
	err := w.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		panic(err)
	}
}

func (w tracesReceiverWrapper) setNextTracesConsumer(next consumer.Traces) {
	w.consumer.next = next
}

func (w tracesReceiverWrapper) shutdown() {
	err := w.Shutdown(context.Background())
	if err != nil {
		panic(err)
	}
}

type metricsProcessorWrapper struct {
	component.MetricsProcessor
	consumer *repeatingMetricsConsumer
}

func (w metricsProcessorWrapper) setNextMetricsConsumer(next consumer.Metrics) {
	w.consumer.next = next
}

type logsProcessorWrapper struct {
	component.LogsProcessor
	consumer *repeatingLogsConsumer
}

func (w logsProcessorWrapper) setNextLogsConsumer(next consumer.Logs) {
	w.consumer.next = next
}

type tracesProcessorWrapper struct {
	component.TracesProcessor
	consumer *repeatingTracesConsumer
}

func (w tracesProcessorWrapper) setNextTracesConsumer(next consumer.Traces) {
	w.consumer.next = next
}

type metricsExporterWrapper struct {
	component.MetricsExporter
	consumer *repeatingMetricsConsumer
}

type logsExporterWrapper struct {
	component.LogsExporter
	consumer *repeatingLogsConsumer
}

type tracesExporterWrapper struct {
	component.TracesExporter
	consumer *repeatingTracesConsumer
}
