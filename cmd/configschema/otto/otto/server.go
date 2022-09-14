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
	"log"
	"net/http"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/components"
)

func Server() {
	mux := http.NewServeMux()

	mux.Handle("/", http.FileServer(http.Dir("static")))

	factories, err := components.Components()
	if err != nil {
		panic(err)
	}

	logger := log.Default()

	mux.Handle("/components", componentHandler{
		logger:    logger,
		factories: factories,
	})

	ottoPipeline := &pipeline{
		dr:        configschema.NewDirResolver("../../..", configschema.DefaultModule),
		factories: factories,
	}
	mux.Handle("/cfgschema/", cfgschemaHandler{
		logger:   logger,
		pipeline: ottoPipeline,
	})

	mux.Handle("/jsonToYAML", jsonToYAMLHandler{
		logger: logger,
	})

	wsHandlers := map[string]wsHandler{}
	registerReceiverHandlers(factories, wsHandlers, ottoPipeline)
	registerProcessorHandlers(factories, wsHandlers, ottoPipeline)
	registerExporterHandlers(factories, wsHandlers, ottoPipeline)
	mux.Handle("/ws/", httpWsHandler{handlers: wsHandlers})

	svr := http.Server{
		Addr:    "localhost:8888",
		Handler: mux,
	}
	panic(svr.ListenAndServe())
}

func registerReceiverHandlers(factories component.Factories, handlers map[string]wsHandler, ppln *pipeline) {
	for componentName, factory := range factories.Receivers {
		const componentType = "receiver"
		path := "/ws/" + componentType + "/" + string(componentName)
		handlers[path] = receiverSocketHandler{
			pipeline:        ppln,
			receiverFactory: factory,
		}
	}
}

func registerProcessorHandlers(factories component.Factories, handlers map[string]wsHandler, ppln *pipeline) {
	for componentName, factory := range factories.Processors {
		const componentType = "processor"
		path := "/ws/" + componentType + "/" + string(componentName)
		handlers[path] = processorSocketHandler{
			pipeline:         ppln,
			processorFactory: factory,
		}
	}
}

func registerExporterHandlers(factories component.Factories, handlers map[string]wsHandler, ppln *pipeline) {
	for componentName, factory := range factories.Exporters {
		const componentType = "exporter"
		path := "/ws/" + componentType + "/" + string(componentName)
		handlers[path] = exporterSocketHandler{
			pipeline:        ppln,
			exporterFactory: factory,
		}
	}
}
