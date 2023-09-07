// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanprocessor"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanprocessor/internal/metadata"
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
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, metadata.TracesStability))
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createTracesProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {

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
		ctx,
		set,
		cfg,
		nextConsumer,
		sp.processTraces,
		processorhelper.WithCapabilities(processorCapabilities))
}
