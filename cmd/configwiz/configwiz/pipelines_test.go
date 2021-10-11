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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/extension/extensionhelper"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/collector/receiver/receiverhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configwiz/testcomponents"
)

type componentInputs struct {
	components []string
	inputs     []string
}

var testReceivers = []string{"rec1", "rec2", "rec3"}
var testProcecssors = []string{"proc1", "proc2"}
var testExporters = []string{"exps1", "exps2", "exps3", "exps4"}
var componentNames = []string{"comp1", "comp2", "comp3"}

const compType = "test"

var tab = strings.Repeat(" ", 4)

// PipelineWizard() Tests
func TestPipelineWizardTraces(t *testing.T) {
	w := fakeWriter{}
	r := fakeReader{userInput: []string{"2", ""}}
	io := Clio{w.write, r.read}
	testFact := createTestFactories()
	out := pipelinesWizard(io, testFact)
	expected, _ := testBuildPipelineWizard(testFact, []string{"traces"})
	assert.Equal(t, map[string]interface{}{
		"traces": rpe{},
	}, out)
	assert.Equal(t, expected, w.programOutput)
}

func TestPipelineWizardMetric(t *testing.T) {
	w := fakeWriter{}
	r := fakeReader{userInput: []string{"1", ""}}
	io := Clio{w.write, r.read}
	testFact := createTestFactories()
	out := pipelinesWizard(io, testFact)
	expected, _ := testBuildPipelineWizard(testFact, []string{"metrics"})
	assert.Equal(t, map[string]interface{}{
		"metrics": rpe{},
	}, out)
	assert.Equal(t, expected, w.programOutput)
}

func TestPipelineWizardEmpty(t *testing.T) {
	w := fakeWriter{}
	r := fakeReader{userInput: []string{""}}
	io := Clio{w.write, r.read}
	testFact := createTestFactories()
	out := pipelinesWizard(io, testFact)
	expected, rpe0 := testBuildPipelineWizard(testFact, []string{})
	assert.Equal(t, rpe{}, rpe0)
	assert.Equal(t, map[string]interface{}{}, out)
	assert.Equal(t, expected, w.programOutput)
}

func TestSinglePipelineWizardFail(t *testing.T) {
	w := fakeWriter{}
	r := fakeReader{userInput: []string{"-1", ""}}
	io := Clio{w.write, r.read}
	testFact := createTestFactories()
	_, rpeOut := singlePipelineWizard(io, testFact)
	expected := "Add pipeline (enter to skip)\n1: Metrics\n2: Traces\n> "
	expected += "Invalid input. Please try again.\n" + expected
	assert.Equal(t, rpe{}, rpeOut)
	assert.Equal(t, expected, w.programOutput)
}

func TestSinglePipelineWizardEmpty(t *testing.T) {
	w := fakeWriter{}
	r := fakeReader{userInput: []string{""}}
	io := Clio{w.write, r.read}
	testFact := createTestFactories()
	_, rpeOut := singlePipelineWizard(io, testFact)
	expected := "Add pipeline (enter to skip)\n1: Metrics\n2: Traces\n> "
	assert.Equal(t, rpe{}, rpeOut)
	assert.Equal(t, expected, w.programOutput)
}

func TestSinglePipelineWizardTraces(t *testing.T) {
	w := fakeWriter{}
	r := fakeReader{userInput: []string{"2", ""}}
	io := Clio{w.write, r.read}
	testFact := createTestFactories()
	name, rpeOut := singlePipelineWizard(io, testFact)
	expectedOut, rpe0 := testBuildSinglePipelineWiz(testFact, name)
	assert.Equal(t, rpe0, rpeOut)
	assert.Equal(t, expectedOut, w.programOutput)
}

func TestSinglePipelineWizardMetrics(t *testing.T) {
	w := fakeWriter{}
	r := fakeReader{userInput: []string{"1", ""}}
	io := Clio{w.write, r.read}
	testFact := createTestFactories()
	name, rpeOut := singlePipelineWizard(io, testFact)
	expectedOut, rpe0 := testBuildSinglePipelineWiz(testFact, name)
	assert.Equal(t, rpe0, rpeOut)
	assert.Equal(t, expectedOut, w.programOutput)
}

// pipeline wizard Tests
func TestPipelineTypeWizardEmpty(t *testing.T) {
	w := fakeWriter{}
	r := fakeReader{userInput: []string{""}}
	io := Clio{w.write, r.read}
	name, rpeOut := pipelineTypeWizard(io, "testing", testReceivers, testProcecssors, testExporters)
	expected0, rpe0 := testBuildPipelineType(
		name,
		componentInputs{components: testReceivers},
		componentInputs{components: testProcecssors},
		componentInputs{components: testExporters},
	)
	assert.Equal(t, "testing", name)
	assert.Equal(t, rpe0, rpeOut)
	assert.Equal(t, expected0, w.programOutput)
}

func TestPipelineTypeWizardBasicInp(t *testing.T) {
	w := fakeWriter{}
	r := fakeReader{userInput: []string{"", "0", "", "", "0", "", "", "0", "", "", "0", ""}}
	io := Clio{w.write, r.read}
	name, rpeOut := pipelineTypeWizard(io, "testing1", testReceivers, testProcecssors, testExporters)
	expected, rpe0 := testBuildPipelineType(
		name,
		componentInputs{components: testReceivers, inputs: []string{testReceivers[0]}},
		componentInputs{components: testProcecssors, inputs: []string{testProcecssors[0]}},
		componentInputs{components: testExporters, inputs: []string{testExporters[0]}},
	)
	assert.Equal(t, "testing1", name)
	assert.Equal(t, rpe0, rpeOut)
	assert.Equal(t, expected, w.programOutput)
}

func TestPipelineTypeWizardExtendedNames(t *testing.T) {
	w := fakeWriter{}
	r := fakeReader{userInput: []string{"extpip", "0", "extr", "", "0", "extp", "", "0", "extexp", "", "0", "extext", ""}}
	io := Clio{w.write, r.read}
	name, rpeOut := pipelineTypeWizard(io, "testingExt", testReceivers, testProcecssors, testExporters)
	expected, rpe0 := testBuildPipelineType(
		name,
		componentInputs{components: testReceivers, inputs: []string{testReceivers[0] + "/extr"}},
		componentInputs{components: testProcecssors, inputs: []string{testProcecssors[0] + "/extp"}},
		componentInputs{components: testExporters, inputs: []string{testExporters[0] + "/extexp"}},
	)
	assert.Equal(t, "testingExt"+"/extpip", name)
	assert.Equal(t, rpe0, rpeOut)
	assert.Equal(t, expected, w.programOutput)
}

// RpeWizard tests
func TestRpeWizardEmpty(t *testing.T) {
	w := fakeWriter{}
	r := fakeReader{userInput: []string{""}}
	io := Clio{w.write, r.read}
	pr := io.newIndentingPrinter(1)
	out := rpeWizard(io, pr, testReceivers, testProcecssors, testExporters)
	expected, expectedOut := testBuildRpeWizard(
		componentInputs{components: testReceivers},
		componentInputs{components: testProcecssors},
		componentInputs{components: testExporters},
	)
	assert.Equal(t, expectedOut, out)
	assert.Equal(t, expected, w.programOutput)
}

func TestRpeWizardBasicInp(t *testing.T) {
	w := fakeWriter{}
	r := fakeReader{userInput: []string{"0", "", "", "0", "", "", "0", "", "", "0", ""}}
	io := Clio{w.write, r.read}
	pr := io.newIndentingPrinter(1)
	out := rpeWizard(io, pr, testReceivers, testProcecssors, testExporters)
	expected, expectedOut := testBuildRpeWizard(
		componentInputs{components: testReceivers, inputs: []string{testReceivers[0]}},
		componentInputs{components: testProcecssors, inputs: []string{testProcecssors[0]}},
		componentInputs{components: testExporters, inputs: []string{testExporters[0]}},
	)
	assert.Equal(t, expectedOut, out)
	assert.Equal(t, expected, w.programOutput)
}

func TestRpeWizardMultipleInputs(t *testing.T) {
	w := fakeWriter{}
	r := fakeReader{userInput: []string{"0", "", "1", "extr", "", "0", "", "", "1", "", "", "0", ""}}
	io := Clio{w.write, r.read}
	pr := io.newIndentingPrinter(1)
	out := rpeWizard(io, pr, testReceivers, testProcecssors, testExporters)
	expected, expectedOut := testBuildRpeWizard(
		componentInputs{components: testReceivers, inputs: []string{testReceivers[0], testReceivers[1] + "/extr"}},
		componentInputs{components: testProcecssors, inputs: []string{testProcecssors[0]}},
		componentInputs{components: testExporters, inputs: []string{testExporters[1]}},
	)
	assert.Equal(t, expectedOut, out)
	assert.Equal(t, expected, w.programOutput)
}

func TestComponentListWizardEmpty(t *testing.T) {
	w := fakeWriter{}
	r := fakeReader{}
	io := Clio{w.write, r.read}
	pr := io.newIndentingPrinter(1)
	componentListWizard(io, pr, compType, componentNames)
	expected := testBuildListWizard(compType, componentNames, []string{})
	assert.Equal(t, expected, w.programOutput)
}

func TestComponentListWizardSingleInp(t *testing.T) {
	w := fakeWriter{}
	r := fakeReader{userInput: []string{"0", ""}, input: 0}
	io := Clio{w.write, r.read}
	pr := io.newIndentingPrinter(1)
	componentListWizard(io, pr, compType, componentNames)
	expected := testBuildListWizard(compType, componentNames, []string{componentNames[0]})
	assert.Equal(t, expected, w.programOutput)
}

func TestComponentListWizardMultipleInp(t *testing.T) {
	w := fakeWriter{}
	r := fakeReader{userInput: []string{"0", "", "1", "extension", "2", "", ""}}
	io := Clio{w.write, r.read}
	pr := io.newIndentingPrinter(1)
	componentListWizard(io, pr, compType, componentNames)
	expected := testBuildListWizard(compType, componentNames, []string{componentNames[0], componentNames[1] + "/extension", componentNames[2]})
	assert.Equal(t, expected, w.programOutput)
}

// Test ComponentNameWizard()
func TestComponentNameWizardEmpty(t *testing.T) {
	w := fakeWriter{}
	r := fakeReader{}
	io := Clio{w.write, r.read}
	pr := io.newIndentingPrinter(1)
	componentNameWizard(io, pr, compType, componentNames)
	expected := testBuildNameWizard("", compType, componentNames)
	assert.Equal(t, expected, w.programOutput)
}

func TestComponentNameWizardExtended(t *testing.T) {
	w := fakeWriter{}
	r := fakeReader{userInput: []string{"0"}}
	io := Clio{w.write, r.read}
	pr := io.newIndentingPrinter(1)
	out, val := componentNameWizard(io, pr, compType, componentNames)
	expected := testBuildNameWizard("", compType, componentNames)
	expected += fmt.Sprintf("%s%s %s extended name (optional) > ", tab, out, compType)
	assert.Equal(t, componentNames[0], out)
	assert.Equal(t, val, "0")
	assert.Equal(t, expected, w.programOutput)
}

func TestComponentNameWizardError(t *testing.T) {
	w := fakeWriter{}
	r := fakeReader{[]string{"-1", ""}, 0}
	io := Clio{w.write, r.read}
	pr := io.newIndentingPrinter(1)
	componentNameWizard(io, pr, compType, componentNames)
	expected := testBuildNameWizard("", compType, componentNames)
	expected += "Invalid input. Please try again.\n"
	expected += testBuildNameWizard("", compType, componentNames)
	assert.Equal(t, expected, w.programOutput)
}

// returns componentNameWizard() output, a list of all components
func testBuildNameWizard(prefix string, compType string, compNames []string) string {
	expected := fmt.Sprintf("%sAdd %s (enter to skip)\n", tab, compType)
	for i := 0; i < len(compNames); i++ {
		expected += fmt.Sprintf("%s%2d: %s\n", tab, i, compNames[i])
	}
	expected += tab + "> "
	return prefix + expected
}

// returns componentListWizard() output
func testBuildListWizard(compGroup string, compNames []string, inputs []string) string {
	expected := fmt.Sprintf("%sCurrent %ss: []\n", tab, compGroup)
	if len(inputs) == 0 {
		return testBuildNameWizard(expected, compGroup, compNames)
	}
	expected = testBuildNameWizard(expected, compGroup, compNames)
	for counter := 1; counter <= len(inputs); counter++ {
		theComp := strings.Split(inputs[counter-1], "/")[0]
		expected += fmt.Sprintf("%s%s %s extended name (optional) > ", tab, theComp, compGroup)
		expected += fmt.Sprintf("%sCurrent %ss: [", tab, compGroup)
		names := inputs[0:counter]
		for _, name := range names {
			expected += name + ", "
		}
		expected = expected[0 : len(expected)-2]
		expected += "]\n"
		expected = testBuildNameWizard(expected, compGroup, compNames)
	}
	return expected
}

// returns RpeWizard() output
func testBuildRpeWizard(
	receiverInput componentInputs,
	processorInput componentInputs,
	expporterInput componentInputs,
) (string, rpe) {
	expected := testBuildListWizard("receiver", receiverInput.components, receiverInput.inputs)
	expected += testBuildListWizard("processor", processorInput.components, processorInput.inputs)
	expected += testBuildListWizard("exporter", expporterInput.components, expporterInput.inputs)
	expectedRPE := rpe{
		Receivers:  receiverInput.inputs,
		Processors: processorInput.inputs,
		Exporters:  expporterInput.inputs,
	}
	return expected, expectedRPE
}

// returns pipelineTypeWizard() output
func testBuildPipelineType(
	name string,
	receiverInput componentInputs,
	processorInput componentInputs,
	exporterInput componentInputs,
) (string, rpe) {
	wizOutput, rpe0 := testBuildRpeWizard(receiverInput, processorInput, exporterInput)
	expected := fmt.Sprintf("%s%s pipeline extended name (optional) > %s", tab, strings.Split(strings.Title(name), "/")[0], tab)
	expected += fmt.Sprintf("Pipeline \"%s\"\n", name)
	expected += wizOutput
	return expected, rpe0
}

func testBuildComponentInputs(testFactory component.Factories,
	pipeType bool,
	recInp []string,
	procInp []string,
	expInp []string,
	extInp []string,
) []componentInputs {
	if pipeType {
		return []componentInputs{
			{receiverNames(testFactory, isMetricsReceiver), recInp},
			{processorNames(testFactory, isMetricProcessor), procInp},
			{exporterNames(testFactory, isMetricsExporter), expInp},
			{extensionNames(testFactory, isExtension), extInp},
		}
	}
	return []componentInputs{
		{receiverNames(testFactory, isTracesReceiver), recInp},
		{processorNames(testFactory, isTracesProcessor), procInp},
		{exporterNames(testFactory, isTracesExporter), expInp},
		{extensionNames(testFactory, isExtension), extInp},
	}
}

func testBuildPipelineWizard(testFact component.Factories, inputs []string) (string, rpe) {
	expected := "Current pipelines: []\n"
	addPipe := "Add pipeline (enter to skip)\n1: Metrics\n2: Traces\n> "
	if len(inputs) == 0 {
		return expected + addPipe, rpe{}
	}
	singlePipe, rpe0 := testBuildSinglePipelineWiz(testFact, inputs[0])
	expected += singlePipe
	for i := range inputs {
		expected += "Current pipelines: ["
		currPipes := inputs[:i+1]
		for _, pipe := range currPipes {
			expected += pipe + ", "
		}
		expected = expected[0 : len(expected)-2]
		expected += "]\n" + addPipe
	}
	return expected, rpe0
}

func testBuildSinglePipelineWiz(testFact component.Factories, name string) (string, rpe) {
	expected := "Add pipeline (enter to skip)\n1: Metrics\n2: Traces\n> "
	ci := testBuildComponentInputs(testFact, false, nil, nil, nil, nil)
	expectedOut, rpe0 := testBuildPipelineType(name, ci[0], ci[1], ci[2])
	return expected + expectedOut, rpe0
}

func createTestFactories() component.Factories {
	exampleReceiverFactory := testcomponents.ExampleReceiverFactory
	exampleProcessorFactory := testcomponents.ExampleProcessorFactory
	exampleExporterFactory := testcomponents.ExampleExporterFactory
	badExtensionFactory := testNewBadExtensionFactory()
	badReceiverFactory := testNewBadReceiverFactory()
	badProcessorFactory := testNewBadProcessorFactory()
	badExporterFactory := testNewBadExporterFactory()

	factories := component.Factories{
		Extensions: map[config.Type]component.ExtensionFactory{
			badExtensionFactory.Type(): badExtensionFactory,
		},
		Receivers: map[config.Type]component.ReceiverFactory{
			exampleReceiverFactory.Type(): exampleReceiverFactory,
			badReceiverFactory.Type():     badReceiverFactory,
		},
		Processors: map[config.Type]component.ProcessorFactory{
			exampleProcessorFactory.Type(): exampleProcessorFactory,
			badProcessorFactory.Type():     badProcessorFactory,
		},
		Exporters: map[config.Type]component.ExporterFactory{
			exampleExporterFactory.Type(): exampleExporterFactory,
			badExporterFactory.Type():     badExporterFactory,
		},
	}

	return factories
}

func testNewBadReceiverFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory("bf", func() config.Receiver {
		return &struct {
			config.ReceiverSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
		}{
			ReceiverSettings: config.NewReceiverSettings(config.NewComponentID("bf")),
		}
	})
}

func testNewBadProcessorFactory() component.ProcessorFactory {
	return processorhelper.NewFactory("bf", func() config.Processor {
		return &struct {
			config.ProcessorSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
		}{
			ProcessorSettings: config.NewProcessorSettings(config.NewComponentID("bf")),
		}
	})
}

func testNewBadExporterFactory() component.ExporterFactory {
	return exporterhelper.NewFactory("bf", func() config.Exporter {
		return &struct {
			config.ExporterSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
		}{
			ExporterSettings: config.NewExporterSettings(config.NewComponentID("bf")),
		}
	})
}

func testNewBadExtensionFactory() component.ExtensionFactory {
	return extensionhelper.NewFactory(
		"bf",
		func() config.Extension {
			return &struct {
				config.ExtensionSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
			}{
				ExtensionSettings: config.NewExtensionSettings(config.NewComponentID("bf")),
			}
		},
		func(ctx context.Context, params component.ExtensionCreateSettings, extension config.Extension) (component.Extension, error) {
			return nil, nil
		},
	)
}
