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

package configwiz

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const invalidMsg = "Invalid input. Try again."

func pipelinesWizard(io Clio, factories component.Factories) map[string]interface{} {
	out := map[string]interface{}{}
	pr := io.newIndentingPrinter(0)
	for {
		pr.println(fmt.Sprintf("Current pipelines: [%s]", strings.Join(keys(out), ", ")))
		name, rpeWiz := singlePipelineWizard(io, factories)
		if name == "" {
			break
		}
		out[name] = rpeWiz
	}
	return out
}

func keys(input map[string]interface{}) []string {
	i := 0
	out := make([]string, len(input))
	for k := range input {
		out[i] = k
		i++
	}
	return out
}

func singlePipelineWizard(io Clio, factories component.Factories) (string, rpe) {
	pr := io.newIndentingPrinter(0)
	pr.println("Add pipeline (enter to skip)")
	pr.println("1: Metrics")
	pr.println("2: Traces")
	pr.print("> ")
	pipelineID := io.Read("")
	switch pipelineID {
	case "":
		return "", rpe{}
	case "1":
		return pipelineTypeWizard(
			io,
			"metrics",
			receiverNames(factories, isMetricsReceiver),
			processorNames(factories, isMetricProcessor),
			exporterNames(factories, isMetricsExporter),
			extensionNames(factories, isExtension))
	case "2":
		return pipelineTypeWizard(
			io,
			"traces",
			receiverNames(factories, isTracesReceiver),
			processorNames(factories, isTracesProcessor),
			exporterNames(factories, isTracesExporter),
			extensionNames(factories, isExtension))
	}
	pr.println(invalidMsg)
	return singlePipelineWizard(io, factories)
}

// pipelineTypeWizard for a given pipelineType (e.g. "metrics", "traces")
func pipelineTypeWizard(
	io Clio,
	pipelineType string,
	receivers []string,
	processors []string,
	exporters []string,
	extensions []string,
) (string, rpe) {
	pr := io.newIndentingPrinter(1)
	pr.print(fmt.Sprintf("%s pipeline extended name (optional) > ", strings.Title(pipelineType)))
	name := pipelineType
	nameExt := io.Read("")
	if nameExt != "" {
		name += "/" + nameExt
	}
	pr.print(fmt.Sprintf("Pipeline %q\n", name))
	rpeWiz := rpeWizard(io, pr, receivers, processors, exporters, extensions)
	return name, rpeWiz
}

func rpeWizard(
	io Clio,
	pr indentingPrinter,
	receiverNames []string,
	processorNames []string,
	exporterNames []string,
	extensionNames []string,
) rpe {
	out := rpe{}
	out.Receivers = componentListWizard(io, pr, "receiver", receiverNames)
	out.Processors = componentListWizard(io, pr, "processor", processorNames)
	out.Exporters = componentListWizard(io, pr, "exporter", exporterNames)
	out.Extensions = componentListWizard(io, pr, "extension", extensionNames)
	return out
}

type rpe struct {
	Receivers  []string
	Processors []string
	Exporters  []string
	Extensions []string
}

func componentListWizard(io Clio, pr indentingPrinter, componentGroup string, componentNames []string) (out []string) {
	for {
		pr.println(fmt.Sprintf("Current %ss: [%s]", componentGroup, strings.Join(out, ", ")))
		key, name := componentNameWizard(io, pr, componentGroup, componentNames)
		if key == "" {
			break
		}
		if name != "" {
			key += "/" + name
		}
		out = append(out, key)
	}
	return
}

func componentNameWizard(io Clio, pr indentingPrinter, componentType string, componentNames []string) (string, string) {
	pr.println(fmt.Sprintf("Add %s (enter to skip)", componentType))
	for i, name := range componentNames {
		pr.println(fmt.Sprintf("%d: %s", i, name))
	}
	pr.print("> ")
	choice := io.Read("")
	if choice == "" {
		return "", ""
	}
	i, _ := strconv.Atoi(choice)
	if i < 0 || i > len(componentNames)-1 {
		pr.level--
		pr.println(invalidMsg)
		pr.level++
		return componentNameWizard(io, pr, componentType, componentNames)
	}
	key := componentNames[i]
	pr.print(fmt.Sprintf("%s %s extended name (optional) > ", key, componentType))
	return key, io.Read("")
}

type receiverFactoryTest func(factory component.ReceiverFactory) bool

type exporterFactoryTest func(factory component.ExporterFactory) bool

type processorFactoryTest func(factory component.ProcessorFactory) bool

type extensionFactoryTest func(factory component.ExtensionFactory) bool

func receiverNames(c component.Factories, test receiverFactoryTest) []string {
	var receivers []string
	for k, v := range c.Receivers {
		if test(v) {
			receivers = append(receivers, string(k))
		}
	}
	sort.Strings(receivers)
	return receivers
}

func isTracesReceiver(f component.ReceiverFactory) bool {
	_, err := f.CreateTracesReceiver(
		context.Background(),
		component.ReceiverCreateSettings{
			TelemetrySettings: component.TelemetrySettings{Logger: zap.NewNop()},
		},
		f.CreateDefaultConfig(),
		consumertest.NewNop(),
	)
	return err != componenterror.ErrDataTypeIsNotSupported
}

func isMetricsReceiver(f component.ReceiverFactory) bool {
	_, err := f.CreateMetricsReceiver(
		context.Background(),
		component.ReceiverCreateSettings{
			TelemetrySettings: component.TelemetrySettings{Logger: zap.NewNop()},
		},
		f.CreateDefaultConfig(),
		consumertest.NewNop(),
	)
	return err != componenterror.ErrDataTypeIsNotSupported
}

func processorNames(c component.Factories, test processorFactoryTest) []string {
	var processors []string
	for k, v := range c.Processors {
		if k != "filter" && test(v) {
			processors = append(processors, string(k))
		}
	}
	sort.Strings(processors)
	return processors
}

func isMetricProcessor(f component.ProcessorFactory) bool {
	_, err := f.CreateMetricsProcessor(context.Background(), component.ProcessorCreateSettings{}, f.CreateDefaultConfig(), consumertest.NewNop())
	return err != componenterror.ErrDataTypeIsNotSupported
}

func isTracesProcessor(f component.ProcessorFactory) bool {
	_, err := f.CreateMetricsProcessor(context.Background(), component.ProcessorCreateSettings{}, f.CreateDefaultConfig(), consumertest.NewNop())
	return err != componenterror.ErrDataTypeIsNotSupported
}

func exporterNames(c component.Factories, test exporterFactoryTest) []string {
	var exporters []string
	for k, v := range c.Exporters {
		if test(v) {
			exporters = append(exporters, string(k))
		}
	}
	sort.Strings(exporters)
	return exporters
}

func isMetricsExporter(f component.ExporterFactory) bool {
	_, err := f.CreateMetricsExporter(
		context.Background(),
		component.ExporterCreateSettings{
			TelemetrySettings: component.TelemetrySettings{
				Logger:         zap.NewNop(),
				TracerProvider: trace.NewNoopTracerProvider(),
			},
		},
		f.CreateDefaultConfig())
	return err != componenterror.ErrDataTypeIsNotSupported
}

func isTracesExporter(f component.ExporterFactory) bool {
	_, err := f.CreateTracesExporter(
		context.Background(),
		component.ExporterCreateSettings{
			TelemetrySettings: component.TelemetrySettings{
				Logger:         zap.NewNop(),
				TracerProvider: trace.NewNoopTracerProvider(),
			},
		},
		f.CreateDefaultConfig())
	return err != componenterror.ErrDataTypeIsNotSupported
}

func extensionNames(c component.Factories, test extensionFactoryTest) []string {
	var extensions []string
	for k, v := range c.Extensions {
		if test(v) {
			extensions = append(extensions, string(k))
		}
	}
	sort.Strings(extensions)
	return extensions
}

func isExtension(f component.ExtensionFactory) bool {
	_, err := f.CreateExtension(context.Background(), component.ExtensionCreateSettings{}, f.CreateDefaultConfig())
	return err != componenterror.ErrDataTypeIsNotSupported
}
