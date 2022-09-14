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
	"encoding/json"
	"log"
	"net/http"
	"reflect"
	"sort"

	"go.opentelemetry.io/collector/component"
)

type componentHandler struct {
	logger    *log.Logger
	factories component.Factories
}

func (h componentHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	jsn, err := factoriesToComponentTypeJSON(h.factories)
	if err != nil {
		h.logger.Printf("componentHandler: ServeHTTP: error getting components: %v", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, err = resp.Write(jsn)
	if err != nil {
		h.logger.Printf("componentHandler: ServeHTTP: error writing response: %v", err)
	}
}

func factoriesToComponentTypeJSON(factories component.Factories) ([]byte, error) {
	cmp := factoriesToComponentTypes(factories)
	return json.Marshal(cmp)
}

type componentTypes struct {
	Metrics rpe `json:"metrics"`
	Logs    rpe `json:"logs"`
	Traces  rpe `json:"traces"`
}

type rpe struct {
	Receivers  []string `json:"receivers"`
	Processors []string `json:"processors"`
	Exporters  []string `json:"exporters"`
}

func factoriesToComponentTypes(factories component.Factories) componentTypes {
	out := componentTypes{}
	for name, factory := range factories.Receivers {
		sp := receiverSupportedPipelines(factory)
		if sp.metrics {
			out.Metrics.Receivers = append(out.Metrics.Receivers, string(name))
		}
		if sp.traces {
			out.Traces.Receivers = append(out.Traces.Receivers, string(name))
		}
		if sp.logs {
			out.Logs.Receivers = append(out.Logs.Receivers, string(name))
		}
	}
	sort.Strings(out.Metrics.Receivers)
	sort.Strings(out.Traces.Receivers)
	sort.Strings(out.Logs.Receivers)

	for name, factory := range factories.Processors {
		sp := processorSupportedPipelines(factory)
		if sp.metrics {
			out.Metrics.Processors = append(out.Metrics.Processors, string(name))
		}
		if sp.traces {
			out.Traces.Processors = append(out.Traces.Processors, string(name))
		}
		if sp.logs {
			out.Logs.Processors = append(out.Logs.Processors, string(name))
		}
	}
	sort.Strings(out.Metrics.Processors)
	sort.Strings(out.Traces.Processors)
	sort.Strings(out.Logs.Processors)

	for name, factory := range factories.Exporters {
		sp := exporterSupportedPipelines(factory)
		if sp.metrics {
			out.Metrics.Exporters = append(out.Metrics.Exporters, string(name))
		}
		if sp.traces {
			out.Traces.Exporters = append(out.Traces.Exporters, string(name))
		}
		if sp.logs {
			out.Logs.Exporters = append(out.Logs.Exporters, string(name))
		}
	}
	sort.Strings(out.Metrics.Exporters)
	sort.Strings(out.Traces.Exporters)
	sort.Strings(out.Logs.Exporters)

	return out
}

type supportedPipelines struct {
	metrics bool
	traces  bool
	logs    bool
}

func receiverSupportedPipelines(fact component.ReceiverFactory) supportedPipelines {
	out := supportedPipelines{}
	factV := reflect.ValueOf(fact).Elem()
	if !factV.FieldByName("CreateMetricsReceiverFunc").IsNil() {
		out.metrics = true
	}
	if !factV.FieldByName("CreateTracesReceiverFunc").IsNil() {
		out.traces = true
	}
	if !factV.FieldByName("CreateLogsReceiverFunc").IsNil() {
		out.logs = true
	}
	return out
}

func processorSupportedPipelines(fact component.ProcessorFactory) supportedPipelines {
	out := supportedPipelines{}
	factV := reflect.ValueOf(fact).Elem()
	if !factV.FieldByName("CreateMetricsProcessorFunc").IsNil() {
		out.metrics = true
	}
	if !factV.FieldByName("CreateTracesProcessorFunc").IsNil() {
		out.traces = true
	}
	if !factV.FieldByName("CreateLogsProcessorFunc").IsNil() {
		out.logs = true
	}
	return out
}

func exporterSupportedPipelines(fact component.ExporterFactory) supportedPipelines {
	out := supportedPipelines{}
	factV := reflect.ValueOf(fact).Elem()
	if !factV.IsValid() {
		return out
	}
	if f := factV.FieldByName("CreateMetricsExporterFunc"); f.IsValid() && !f.IsNil() {
		out.metrics = true
	}
	if f := factV.FieldByName("CreateTracesExporterFunc"); f.IsValid() && !f.IsNil() {
		out.traces = true
	}
	if f := factV.FieldByName("CreateLogsExporterFunc"); f.IsValid() && !f.IsNil() {
		out.logs = true
	}
	return out
}
