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

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema"
)

type pipeline struct {
	dr        configschema.DirResolver
	factories component.Factories

	metricsReceiverWrapper  *metricsReceiverWrapper
	metricsProcessorWrapper *metricsProcessorWrapper
	metricsExporterWrapper  *metricsExporterWrapper

	logsReceiverWrapper  *logsReceiverWrapper
	logsProcessorWrapper *logsProcessorWrapper
	logsExporterWrapper  *logsExporterWrapper

	tracesReceiverWrapper  *tracesReceiverWrapper
	tracesProcessorWrapper *tracesProcessorWrapper
	tracesExporterWrapper  *tracesExporterWrapper
}

// receivers

func (p *pipeline) connectMetricsReceiverWrapper(w metricsReceiverWrapper) {
	if p.metricsProcessorWrapper != nil {
		w.setNextMetricsConsumer(p.metricsProcessorWrapper)
	}
	p.metricsReceiverWrapper = &w
}

func (p *pipeline) disconnectMetricsReceiverWrapper() {
	p.metricsReceiverWrapper = nil
}

func (p *pipeline) connectLogsReceiverWrapper(w logsReceiverWrapper) {
	if p.logsProcessorWrapper != nil {
		w.setNextLogsConsumer(p.logsProcessorWrapper)
	}
	p.logsReceiverWrapper = &w
}

func (p *pipeline) disconnectLogsReceiverWrapper() {
	p.logsReceiverWrapper = nil
}

func (p *pipeline) connectTracesReceiverWrapper(w tracesReceiverWrapper) {
	if p.tracesProcessorWrapper != nil {
		w.setNextTracesConsumer(p.tracesProcessorWrapper)
	}
	p.tracesReceiverWrapper = &w
}

func (p *pipeline) disconnectTracesReceiverWrapper() {
	p.tracesReceiverWrapper = nil
}

// processors

func (p *pipeline) connectMetricsProcessorWrapper(w metricsProcessorWrapper) {
	if p.metricsReceiverWrapper != nil {
		p.metricsReceiverWrapper.setNextMetricsConsumer(w)
	}
	p.metricsProcessorWrapper = &w
}

func (p *pipeline) disconnectMetricsProcessorWrapper() {
	if p.metricsReceiverWrapper != nil {
		p.metricsReceiverWrapper.setNextMetricsConsumer(nil)
	}
	p.metricsProcessorWrapper = nil
}

func (p *pipeline) connectLogsProcessorWrapper(w logsProcessorWrapper) {
	if p.logsReceiverWrapper != nil {
		p.logsReceiverWrapper.setNextLogsConsumer(w)
	}
	p.logsProcessorWrapper = &w
}

func (p *pipeline) disconnectLogsProcessorWrapper() {
	if p.logsReceiverWrapper != nil {
		p.logsReceiverWrapper.setNextLogsConsumer(nil)
	}
	p.logsProcessorWrapper = nil
}

func (p *pipeline) connectTracesProcessorWrapper(w tracesProcessorWrapper) {
	if p.tracesReceiverWrapper != nil {
		p.tracesReceiverWrapper.setNextTracesConsumer(w)
	}
	p.tracesProcessorWrapper = &w
}

func (p *pipeline) disconnectTracesProcessorWrapper() {
	if p.tracesReceiverWrapper != nil {
		p.tracesReceiverWrapper.setNextTracesConsumer(nil)
	}
	p.tracesProcessorWrapper = nil
}

// exporters

func (p *pipeline) connectMetricsExporterWrapper(w *metricsExporterWrapper) {
	if p.metricsProcessorWrapper != nil {
		p.metricsProcessorWrapper.setNextMetricsConsumer(w)
	} else if p.metricsReceiverWrapper != nil {
		p.metricsReceiverWrapper.setNextMetricsConsumer(w)
	}
	p.metricsExporterWrapper = w
}

func (p *pipeline) disconnectMetricsExporterWrapper() {
	p.connectMetricsExporterWrapper(nil)
}

func (p *pipeline) connectLogsExporterWrapper(w *logsExporterWrapper) {
	if p.logsProcessorWrapper != nil {
		p.logsProcessorWrapper.setNextLogsConsumer(w)
	} else if p.logsReceiverWrapper != nil {
		p.logsReceiverWrapper.setNextLogsConsumer(w)
	}
	p.logsExporterWrapper = w
}

func (p *pipeline) disconnectLogsExporterWrapper() {
	p.connectLogsExporterWrapper(nil)
}

func (p *pipeline) connectTracesExporterWrapper(w *tracesExporterWrapper) {
	if p.tracesProcessorWrapper != nil {
		p.tracesProcessorWrapper.setNextTracesConsumer(w)
	} else if p.tracesReceiverWrapper != nil {
		p.tracesReceiverWrapper.setNextTracesConsumer(w)
	}
	p.tracesExporterWrapper = w
}

func (p *pipeline) disconnectTracesExporterWrapper() {
	p.connectTracesExporterWrapper(nil)
}
