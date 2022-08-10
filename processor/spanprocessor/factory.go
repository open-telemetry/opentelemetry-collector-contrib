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

package spanprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanprocessor"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	// typeStr is the value of "type" Span processor in the configuration.
	typeStr = "span"
	// The stability level of the processor.
	stability = component.StabilityLevelAlpha
)

const (
	// status represents span status
	statusCodeUnset = "Unset"
	statusCodeError = "Error"
	statusCodeOk    = "Ok"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

// errMissingRequiredField is returned when a required field in the config
// is not specified.
// TODO https://github.com/open-telemetry/opentelemetry-collector/issues/215
//
//	Move this to the error package that allows for span name and field to be specified.
var (
	errMissingRequiredField       = errors.New("error creating \"span\" processor: either \"from_attributes\" or \"to_attributes\" must be specified in \"name:\" or \"setStatus\" must be specified")
	errIncorrectStatusCode        = errors.New("error creating \"span\" processor: \"status\" must have specified \"code\" as \"Ok\" or \"Error\" or \"Unset\"")
	errIncorrectStatusDescription = errors.New("error creating \"span\" processor: \"description\" can be specified only for \"code\" \"Error\"")
)

// NewFactory returns a new factory for the Span processor.
func NewFactory() component.ProcessorFactory {
	return component.NewProcessorFactory(
		typeStr,
		createDefaultConfig,
		component.WithTracesProcessor(createTracesProcessor, stability))
}

func createDefaultConfig() config.Processor {
	return &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
	}
}

func createTracesProcessor(
	_ context.Context,
	_ component.ProcessorCreateSettings,
	cfg config.Processor,
	nextConsumer consumer.Traces,
) (component.TracesProcessor, error) {

	// 'from_attributes' or 'to_attributes' under 'name' has to be set for the span
	// processor to be valid. If not set and not enforced, the processor would do no work.
	oCfg := cfg.(*Config)
	if len(oCfg.Rename.FromAttributes) == 0 &&
		(oCfg.Rename.ToAttributes == nil || len(oCfg.Rename.ToAttributes.Rules) == 0) &&
		oCfg.SetStatus == nil {
		return nil, errMissingRequiredField
	}
	if oCfg.SetStatus != nil {
		if oCfg.SetStatus.Code != statusCodeUnset && oCfg.SetStatus.Code != statusCodeError && oCfg.SetStatus.Code != statusCodeOk {
			return nil, errIncorrectStatusCode
		}
		if len(oCfg.SetStatus.Description) > 0 && oCfg.SetStatus.Code != statusCodeError {
			return nil, errIncorrectStatusDescription
		}
	}

	sp, err := newSpanProcessor(*oCfg)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewTracesProcessor(
		cfg,
		nextConsumer,
		sp.processTraces,
		processorhelper.WithCapabilities(processorCapabilities))
}
